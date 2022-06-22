// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"cloud.google.com/go/storage"
)

// Original source: https://github.com/googleapis/google-cloud-go/issues/1124#issuecomment-1100755501
type GSReadSeekCloser struct {
	*storage.ObjectHandle
	Context context.Context
	Closer  *func() error

	r          *storage.Reader
	pos        int64          // current position (like 'seen' in storage.Reader)
	filesize   int64          // if this is known and set, it enables io.SeekEnd
	TotalBytes int64          // REP: replaced offset with TotalBytes
	mux        sync.Mutex     // REP: Lock sequential reads -- recommend using either sequentially or randomly, not both
	wg         sync.WaitGroup // REP: Keep track of how many ReatAt readers
	closed     bool           // REP: no more reading allowed
}

//* create a new GSReadSeekCloser -- default starting position is 0
func NewGSReadSeekCloser(obj *storage.ObjectHandle, ctx context.Context, closer *func() error) (s GSReadSeekCloser, err error) {
	a, err := obj.Attrs(ctx)
	if err != nil {
		return
	}
	s = GSReadSeekCloser{ObjectHandle: obj, Context: ctx, Closer: closer, pos: 0, filesize: a.Size, closed: false}
	return
}

func (s *GSReadSeekCloser) Read(buf []byte) (int, error) {
	var err error
	if s.closed {
		return 0, io.EOF
	}
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.r == nil {
		if s.pos == 0 {
			//* REP use NewReader for pos == 0
			//* ReadAt will use their own readers

			s.r, err = s.NewReader(s.Context)
			if err != nil {
				return 0, err
			}
		} else {
			//! REP if you open a RangeReader, you cannot continue reading from a point without
			//! opening a new RangeReader. Using -1 enables this, however, it reads remainder of file into buffer.
			n, err := s.ReadAt(buf, s.pos)
			s.pos += int64(n)
			return n, err
		}

	}
	n, err := read(s.r, buf)

	//* REP: always move the file position even if there is an error
	s.pos += int64(n)
	atomic.AddInt64(&s.TotalBytes, int64(n))
	return n, err
}

// ReadAt simply reads from a specific position in the file, always relative to 0 (beginning)
// REP as specified, ReadAt should not modify state of reader
// REP added to complete interface requirements
func (s *GSReadSeekCloser) ReadAt(buf []byte, offset int64) (n int, err error) {
	var atReader *storage.Reader
	if s.closed {
		return 0, io.EOF
	}
	s.wg.Add(1)
	defer s.wg.Done()
	atReader, err = s.NewRangeReader(s.Context, offset, int64(len(buf)))
	if err != nil {
		return
	}
	defer atReader.Close()
	n, err = read(atReader, buf)
	atomic.AddInt64(&s.TotalBytes, int64(n))
	return
}

func (s *GSReadSeekCloser) Seek(offset int64, whence int) (int64, error) {
	// Seeking is not actually possible. As a proxy, we close the current
	// connection, reset the reader, and reset the offset.
	var newPos int64
	if s.closed {
		return 0, io.EOF
	}
	s.mux.Lock()
	s.mux.Unlock()

	// Set the offset for the next Read or Write to offset, interpreted
	// according to whence
	switch whence {
	case io.SeekStart:
		// Set our new offset relative to 0
		newPos = offset
	case io.SeekCurrent:
		// Set our new offset relative to our current offset
		newPos = s.pos + offset
	case io.SeekEnd:
		// Set our new offset relative to the end of the file. If the end of the
		// file is not known, fail.
		if s.filesize == 0 {
			return 0, fmt.Errorf("GSReadSeekCloser.Seek: io.Seeker 'whence' value %d is not implemented", whence)
		}

		newPos = s.filesize - offset
	}

	// Close the current reader and remove it.
	if s.r != nil && s.pos != newPos {
		s.r.Close()
		s.r = nil
	}

	// Update our offset
	s.pos = newPos

	// Return the new offset relative to the start of the file
	return s.pos, nil
}

// Satisfies io.Closer. If s.close is not set, this is a nop.
// REP: o -> s for consistency
func (s *GSReadSeekCloser) Close() error {
	if s.closed {
		return io.EOF // Already closed
	}
	s.closed = true
	if s.Closer != nil {
		return (*s.Closer)()
	}
	// Wait for any ReadAt's to complete
	s.wg.Wait()
	return nil
}

// REP: general purpose read
func read(rdr *storage.Reader, buf []byte) (n int, err error) {
	var r int

	for n < len(buf) && err == nil {
		r, err = rdr.Read(buf[n:])
		n += r
	}
	return n, err
}

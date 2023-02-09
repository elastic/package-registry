// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
)

// PackageFile is the interface that files in the file system need to implement.
// Seeker interface is needed for http helpers to serve files.
type PackageFile = io.ReadSeekCloser

type PackageFileSystem interface {
	Stat(name string) (os.FileInfo, error)
	Open(name string) (PackageFile, error)
	Glob(pattern string) (matches []string, err error)
	Close() error
}

// ExtractedPackageFileSystem provides utils to access files in an extracted package.
type ExtractedPackageFileSystem struct {
	path string
	root fs.FS
}

func NewExtractedPackageFileSystem(p *Package) (*ExtractedPackageFileSystem, error) {
	return &ExtractedPackageFileSystem{path: p.BasePath, root: os.DirFS(p.BasePath)}, nil
}

func (fsys *ExtractedPackageFileSystem) Stat(name string) (os.FileInfo, error) {
	name = strings.TrimPrefix(name, "/")
	return fs.Stat(fsys.root, name)
}

func (fsys *ExtractedPackageFileSystem) Open(name string) (PackageFile, error) {
	name = strings.TrimPrefix(name, "/")
	f, err := fsys.root.Open(name)
	if err != nil {
		return nil, err
	}
	pf, ok := f.(PackageFile)
	if !ok {
		defer f.Close()
		return nil, fmt.Errorf("file does not implement PackageFile interface: %q", name)
	}
	return pf, nil
}

func (fsys *ExtractedPackageFileSystem) Glob(pattern string) (matches []string, err error) {
	fs.WalkDir(fsys.root, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk path %q: %w", p, err)
		}
		match, err := path.Match(pattern, p)
		if err != nil {
			return fmt.Errorf("failed to obtain path under package root path (%s): %w", pattern, err)
		}
		if match {
			name := p
			matches = append(matches, name)
		}
		return nil
	})
	return
}

func (fs *ExtractedPackageFileSystem) Close() error { return nil }

// ZipPackageFileSystem provides utils to access files in a zipped package.
type ZipPackageFileSystem struct {
	root   string
	reader *zip.ReadCloser
}

func NewZipPackageFileSystem(p *Package) (*ZipPackageFileSystem, error) {
	reader, err := zip.OpenReader(p.BasePath)
	if err != nil {
		return nil, err
	}
	var root string
	found := false
	for _, f := range reader.File {
		name := path.Clean(f.Name)
		dir, base := path.Split(name)
		dir = path.Clean(dir)
		// Find manifest files at the root of the package
		if dir != "." && path.Dir(dir) == "." && base == "manifest.yml" {
			root = dir
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("failed to determine root directory in package (path: %s)", p.BasePath)
	}
	return &ZipPackageFileSystem{
		root:   root,
		reader: reader,
	}, nil
}

func (fs *ZipPackageFileSystem) Stat(name string) (os.FileInfo, error) {
	path := path.Join(fs.root, name)
	f, err := fs.reader.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Stat()
}

func (fs *ZipPackageFileSystem) Open(name string) (PackageFile, error) {
	path := path.Join(fs.root, name)
	f, err := fs.reader.Open(path)
	if err != nil {
		return nil, err
	}
	return &zipFileSeeker{
		File:   f,
		reader: fs.reader,
		path:   path,
	}, nil
}

func (fs *ZipPackageFileSystem) Glob(pattern string) (matches []string, err error) {
	pattern = path.Join(fs.root, pattern)
	for _, f := range fs.reader.File {
		match, err := path.Match(pattern, path.Clean(f.Name))
		if err != nil {
			return nil, err
		}
		if match {
			name := strings.TrimPrefix(f.Name, fs.root+"/")
			matches = append(matches, name)
		}
	}
	return
}

func (fs *ZipPackageFileSystem) Close() error {
	return fs.reader.Close()
}

// zipFileSeeker implements the seeker interface for zip files.
type zipFileSeeker struct {
	fs.File

	reader *zip.ReadCloser
	path   string
}

// Seek implements the seeker interface for zip files. This is inefficient, it shouldn't
// be frequently used.
func (f *zipFileSeeker) Seek(offset int64, whence int) (n int64, err error) {
	switch whence {
	case io.SeekStart:
		f.File.Close()
		f.File, err = f.reader.Open(f.path)
		if err != nil {
			return -1, err
		}
		if offset > 0 {
			r := io.LimitReader(f.File, offset)
			n, err = io.Copy(io.Discard, r)
			if err != nil {
				return -1, err
			}
			offset = int64(n)
		}
		return offset, nil
	case io.SeekEnd:
		if offset != 0 {
			return -1, fmt.Errorf("unsupported offset")
		}
		info, err := f.File.Stat()
		if err != nil {
			return -1, err
		}
		_, err = io.Copy(io.Discard, f.File)
		if err != nil {
			return -1, err
		}
		return info.Size(), nil
	default:
		return -1, fmt.Errorf("unsupported whence")
	}
}

func ReadAll(fs PackageFileSystem, name string) ([]byte, error) {
	f, err := fs.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return io.ReadAll(f)
}

// VirtualPackageFileSystem provide utils for package objects that don't correspond to
// any real package in any backend. Used mainly for testing purpouses.
type VirtualPackageFileSystem struct{}

func NewVirtualPackageFileSystem() (*VirtualPackageFileSystem, error) {
	return &VirtualPackageFileSystem{}, nil
}

func (fs *VirtualPackageFileSystem) Stat(name string) (os.FileInfo, error) {
	return nil, os.ErrNotExist
}

func (fs *VirtualPackageFileSystem) Open(name string) (PackageFile, error) {
	return nil, os.ErrNotExist
}

func (fs *VirtualPackageFileSystem) Glob(pattern string) (matches []string, err error) {
	return []string{}, nil
}

func (fs *VirtualPackageFileSystem) Close() error { return nil }

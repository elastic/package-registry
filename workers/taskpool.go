// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package workers

import (
	"errors"
	"sync"
)

type taskPool struct {
	wg   sync.WaitGroup
	pool chan struct{}

	errLock sync.Mutex
	errors  []error
}

func NewTaskPool(size int) *taskPool {
	p := &taskPool{
		pool: make(chan struct{}, size),
	}
	return p
}

// Do runs the task in a goroutine, ensuring no more tasks are running than the size of the pool.
func (p *taskPool) Do(task func() error) {
	p.pool <- struct{}{}
	p.wg.Go(func() {
		defer func() { <-p.pool }()

		err := task()
		p.recordError(err)
	})
}

func (p *taskPool) recordError(err error) {
	if err == nil {
		return
	}

	p.errLock.Lock()
	p.errors = append(p.errors, err)
	p.errLock.Unlock()
}

// Wait waits for all the tasks to finish, and joins the errors found. The pool cannot be used after calling Wait.
func (p *taskPool) Wait() error {
	close(p.pool)
	p.wg.Wait()
	return errors.Join(p.errors...)
}

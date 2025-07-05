// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"errors"
	"sync"
)

type taskPool struct {
	wg     sync.WaitGroup
	pool   chan struct{}
	errC   chan error
	errors []error
}

func newTaskPool(size int) *taskPool {
	p := &taskPool{
		pool: make(chan struct{}, size),
		errC: make(chan error),
	}
	go p.errorLoop()
	return p
}

func (p *taskPool) errorLoop() {
	go func() {
		for err := range p.errC {
			if err != nil {
				p.errors = append(p.errors, err)
			}
		}
	}()
}

// Do runs the task in a goroutine, ensuring no more tasks are running than the size of the pool.
func (p *taskPool) Do(task func() error) {
	p.pool <- struct{}{}
	p.wg.Add(1)
	go func() {
		defer func() { _ = <-p.pool }()
		defer p.wg.Done()
		p.errC <- task()
	}()
}

// Wait waits for all the tasks to finish, and joins the errors found. The pool cannot be used after calling Wait.
func (p *taskPool) Wait() error {
	close(p.pool)
	p.wg.Wait()
	close(p.errC)
	return errors.Join(p.errors...)
}

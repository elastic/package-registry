// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"log"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/radovskyb/watcher"
)

const watcherPollingPeriod = time.Second

var (
	w *watcher.Watcher

	watchedPackages []Package
)

func MustUsePackageWatcher(packagePaths []string) {
	log.Println("Use package watcher")

	var err error
	watchedPackages, err = getPackagesFromFilesystem(packagePaths)
	if err != nil {
		log.Println(errors.Wrap(err, "watcher error: reading packages failed"))
	}

	w = watcher.New()
	w.SetMaxEvents(1)

	for _, p := range packagePaths {
		err = w.AddRecursive(p)
		if err != nil && !os.IsNotExist(err) {
			log.Fatal(errors.Wrapf(err, "watching directory failed (path: %s)", p))
		}
	}

	go func() {
		go w.Start(watcherPollingPeriod)

		for {
			select {
			case _, ok := <-w.Event:
				if !ok {
					log.Println("Package watcher is stopped")
					return // channel is closed
				}

				log.Println("Reloading packages...")
				watchedPackages, err = getPackagesFromFilesystem(packagePaths)
				if err != nil {
					log.Println(errors.Wrap(err, "watcher error: reading packages failed"))
				}
			case err, ok := <-w.Error:
				if !ok {
					log.Println("Package watcher is stopped")
					return // channel is closed
				}
				log.Println(errors.Wrap(err, "watcher error"))
			}
		}
	}()
}

func ClosePackageWatcher() {
	if !packageWatcherEnabled() {
		return
	}
	w.Close()
}

func packageWatcherEnabled() bool {
	return w != nil
}

func getWatchedPackages() []Package {
	return watchedPackages
}

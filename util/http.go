// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"go.elastic.co/apm"

	"github.com/elastic/package-registry/archiver"
)

func ServePackage(w http.ResponseWriter, r *http.Request, p *Package) {
	span, _ := apm.StartSpan(r.Context(), "ServePackage", "app")
	defer span.End()

	packagePath := p.BasePath
	f, err := os.Stat(packagePath)
	if err != nil {
		log.Printf("stat package path '%s' failed: %v", packagePath, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/gzip")

	if f.IsDir() {
		err = archiver.ArchivePackage(w, archiver.PackageProperties{
			Name:    p.Name,
			Version: p.Version,
			Path:    packagePath,
		})
		if err != nil {
			log.Printf("archiving package path '%s' failed: %v", packagePath, err)
			return
		}
	} else {
		http.ServeFile(w, r, packagePath)
	}
}

func ServeFile(w http.ResponseWriter, r *http.Request, p *Package, name string) {
	span, _ := apm.StartSpan(r.Context(), "ServePackage", "app")
	defer span.End()

	fs, err := p.fs()
	if os.IsNotExist(err) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	info, err := fs.Stat(name)
	if os.IsNotExist(err) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if info.IsDir() {
		// TODO: Is this needed? It was done by previous implementation.
		name = path.Join(name, "index.json")
	}

	f, err := fs.Open(name)
	if os.IsNotExist(err) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	http.ServeContent(w, r, name, time.Time{}, f)
}

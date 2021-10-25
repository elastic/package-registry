// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"log"
	"net/http"
	"os"

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
		http.Error(w, "resource not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("failed to open filesystem for package: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	stat, err := fs.Stat(name)
	if os.IsNotExist(err) {
		http.Error(w, "resource not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("stat failed for %s: %v", name, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	f, err := fs.Open(name)
	if err != nil {
		log.Printf("failed to open file (%s) in package: %v", name, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	http.ServeContent(w, r, name, stat.ModTime(), f)
}

func ServeSignature(w http.ResponseWriter, r *http.Request, p *Package) {
	http.ServeFile(w, r, p.BasePath+".sig")
}

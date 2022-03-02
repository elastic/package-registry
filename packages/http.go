// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"net/http"
	"os"

	"go.elastic.co/apm"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/archiver"
	"github.com/elastic/package-registry/util"
)

func ServePackage(w http.ResponseWriter, r *http.Request, p *Package) {
	span, _ := apm.StartSpan(r.Context(), "ServePackage", "app")
	defer span.End()

	packagePath := p.BasePath
	logger := util.Logger().With(zap.String("file.name", packagePath))

	f, err := os.Stat(packagePath)
	if err != nil {
		logger.Error("stat package path failed", zap.Error(err))
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
			logger.Error("archiving package path failed", zap.Error(err))
			return
		}
	} else {
		http.ServeFile(w, r, packagePath)
	}
}

func ServeFile(w http.ResponseWriter, r *http.Request, p *Package, name string) {
	span, _ := apm.StartSpan(r.Context(), "ServePackage", "app")
	defer span.End()

	logger := util.Logger().With(zap.String("file.name", name))

	fs, err := p.fs()
	if os.IsNotExist(err) {
		http.Error(w, "resource not found", http.StatusNotFound)
		return
	}
	if err != nil {
		logger.Error("failed to open filesystem", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	stat, err := fs.Stat(name)
	if os.IsNotExist(err) {
		http.Error(w, "resource not found", http.StatusNotFound)
		return
	}
	if err != nil {
		logger.Error("stat failed", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	f, err := fs.Open(name)
	if err != nil {
		logger.Error("failed to open file", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	http.ServeContent(w, r, name, stat.ModTime(), f)
}

func ServeSignature(w http.ResponseWriter, r *http.Request, p *Package) {
	http.ServeFile(w, r, p.BasePath+".sig")
}

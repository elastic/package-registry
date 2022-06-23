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

// ServeLocalPackage is used by artifactsHandler to serve packages and signatures.
func ServeLocalPackage(w http.ResponseWriter, r *http.Request, p *Package, packagePath string) {
	span, _ := apm.StartSpan(r.Context(), "ServePackage", "app")
	defer span.End()

	logger := util.Logger().With(zap.String("file.name", packagePath))

	f, err := os.Stat(packagePath)
	if err != nil {
		logger.Error("stat package path failed", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Only packages stored locally in the unpacked form can be archived.
	if f.IsDir() {
		err = archiver.ArchivePackage(w, archiver.PackageProperties{
			Name:    p.Name,
			Version: p.Version,
			Path:    packagePath,
		})
		if err != nil {
			logger.Error("archiving package path failed", zap.Error(err))
		}
		return
	}

	stream, err := os.Open(packagePath)
	if err != nil {
		logger.Error("failed to open file", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	http.ServeContent(w, r, packagePath, f.ModTime(), stream)
}

// ServeLocalPackageResource is used by staticHandler.
func ServeLocalPackageResource(w http.ResponseWriter, r *http.Request, p *Package, packageFilePath string) {
	span, _ := apm.StartSpan(r.Context(), "ServePackage", "app")
	defer span.End()

	logger := util.Logger().With(zap.String("file.name", packageFilePath))

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

	stat, err := fs.Stat(packageFilePath)
	if os.IsNotExist(err) {
		http.Error(w, "resource not found", http.StatusNotFound)
		return
	}
	if err != nil {
		logger.Error("stat failed", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	f, err := fs.Open(packageFilePath)
	if err != nil {
		logger.Error("failed to open file", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	http.ServeContent(w, r, packageFilePath, stat.ModTime(), f)
}

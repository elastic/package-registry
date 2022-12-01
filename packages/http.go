// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"net/http"
	"os"

	"go.elastic.co/apm"
	"go.uber.org/zap"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/elastic/package-registry/archiver"
	"github.com/elastic/package-registry/metrics"
	"github.com/elastic/package-registry/util"
)

const (
	localLocationPrometheusLabel  = "local"
	remoteLocationPrometheusLabel = "remote"

	artifactComponentPrometheusLabel  = "artifacts"
	staticComponentPrometheusLabel    = "statics"
	signatureComponentPrometheusLabel = "signatures"
)

// ServePackage is used by artifactsHandler to serve packages and signatures.
func ServePackage(w http.ResponseWriter, r *http.Request, p *Package) {
	span, _ := apm.StartSpan(r.Context(), "ServePackage", "app")
	defer span.End()

	if p.RemoteResolver() != nil {
		p.RemoteResolver().ArtifactsHandler(w, r, p)
		metrics.StorageRequestsTotal.With(
			prometheus.Labels{"location": remoteLocationPrometheusLabel, "component": artifactComponentPrometheusLabel},
		).Inc()
		return
	}
	serveLocalPackage(w, r, p, p.BasePath)
	metrics.StorageRequestsTotal.With(
		prometheus.Labels{"location": localLocationPrometheusLabel, "component": artifactComponentPrometheusLabel},
	).Inc()
}

// ServePackageSignature is used by signaturesHandler to serve signatures.
func ServePackageSignature(w http.ResponseWriter, r *http.Request, p *Package) {
	span, _ := apm.StartSpan(r.Context(), "ServePackageSignature", "app")
	defer span.End()

	if p.RemoteResolver() != nil {
		p.RemoteResolver().SignaturesHandler(w, r, p)
		metrics.StorageRequestsTotal.With(
			prometheus.Labels{"location": remoteLocationPrometheusLabel, "component": signatureComponentPrometheusLabel},
		).Inc()
		return
	}
	serveLocalPackage(w, r, p, p.BasePath+".sig")
	metrics.StorageRequestsTotal.With(
		prometheus.Labels{"location": localLocationPrometheusLabel, "component": signatureComponentPrometheusLabel},
	).Inc()
}

func serveLocalPackage(w http.ResponseWriter, r *http.Request, p *Package, packagePath string) {
	span, _ := apm.StartSpan(r.Context(), "ServePackage", "app")
	defer span.End()

	logger := util.Logger().With(zap.String("file.name", packagePath))

	f, err := os.Stat(packagePath)
	if err != nil {
		logger.Error("stat package path failed", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if f.IsDir() {
		w.Header().Set("Content-Type", "application/zip")
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

	http.ServeFile(w, r, packagePath)
}

// ServePackageResource is used by staticHandler.
func ServePackageResource(w http.ResponseWriter, r *http.Request, p *Package, packageFilePath string) {
	span, _ := apm.StartSpan(r.Context(), "ServePackage", "app")
	defer span.End()

	if p.RemoteResolver() != nil {
		p.RemoteResolver().StaticHandler(w, r, p, packageFilePath)
		metrics.StorageRequestsTotal.With(
			prometheus.Labels{"location": remoteLocationPrometheusLabel, "component": staticComponentPrometheusLabel},
		).Inc()
		return
	}

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
	metrics.StorageRequestsTotal.With(
		prometheus.Labels{"location": localLocationPrometheusLabel, "component": staticComponentPrometheusLabel},
	).Inc()
}

// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/elastic/package-registry/packages"
)

type storageResolver struct {
	artifactsPackagesURL url.URL
	artifactsStaticURL   url.URL
}

func (resolver storageResolver) RedirectArtifactsHandler(w http.ResponseWriter, r *http.Request, p *packages.Package) {
	nameVersionZip := fmt.Sprintf("%s-%s.zip", p.Name, p.Version)
	artifactURL := resolver.artifactsPackagesURL.ResolveReference(&url.URL{Path: nameVersionZip})
	http.Redirect(w, r, artifactURL.String(), http.StatusSeeOther)
}

func (resolver storageResolver) RedirectStaticHandler(w http.ResponseWriter, r *http.Request, p *packages.Package, resourcePath string) {
	nameVersion := fmt.Sprintf("%s-%s/", p.Name, p.Version)
	staticURL := resolver.artifactsStaticURL.
		ResolveReference(&url.URL{Path: nameVersion}).
		ResolveReference(&url.URL{Path: resourcePath})
	http.Redirect(w, r, staticURL.String(), http.StatusSeeOther)
}

func (resolver storageResolver) RedirectSignaturesHandler(w http.ResponseWriter, r *http.Request, p *packages.Package) {
	nameVersionSigZip := fmt.Sprintf("%s-%s.zip.sig", p.Name, p.Version)
	signatureURL := resolver.artifactsPackagesURL.ResolveReference(&url.URL{Path: nameVersionSigZip})
	http.Redirect(w, r, signatureURL.String(), http.StatusSeeOther)
}

var _ packages.RemoteResolver = new(storageResolver)

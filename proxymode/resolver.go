// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package proxymode

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/elastic/package-registry/packages"
)

type proxyResolver struct {
	destinationURL url.URL
}

func (pr proxyResolver) RedirectArtifactsHandler(w http.ResponseWriter, r *http.Request, p *packages.Package) {
	remotePath := fmt.Sprintf("/epr/package/%s-%s.zip", p.Name, p.Version)
	anURL := pr.destinationURL.
		ResolveReference(&url.URL{Path: remotePath})
	http.Redirect(w, r, anURL.String(), http.StatusMovedPermanently)
}

func (pr proxyResolver) RedirectStaticHandler(w http.ResponseWriter, r *http.Request, p *packages.Package, resourcePath string) {
	remotePath := fmt.Sprintf("/package/%s/%s/%s", p.Name, p.Version, resourcePath)
	staticURL := pr.destinationURL.
		ResolveReference(&url.URL{Path: remotePath})
	http.Redirect(w, r, staticURL.String(), http.StatusMovedPermanently)
}

func (pr proxyResolver) RedirectSignaturesHandler(w http.ResponseWriter, r *http.Request, p *packages.Package) {
	remotePath := fmt.Sprintf("/epr/package/%s-%s.zip.sig", p.Name, p.Version)
	anURL := pr.destinationURL.
		ResolveReference(&url.URL{Path: remotePath})
	http.Redirect(w, r, anURL.String(), http.StatusMovedPermanently)
}

var _ packages.RemoteResolver = new(proxyResolver)

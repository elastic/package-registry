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

func (pr proxyResolver) redirectRequest(w http.ResponseWriter, r *http.Request, remotePath string) {
	remoteURL := pr.destinationURL.
		ResolveReference(&url.URL{Path: remotePath})
	http.Redirect(w, r, remoteURL.String(), http.StatusMovedPermanently)
}

func (pr proxyResolver) ForwardArtifactsHandler(w http.ResponseWriter, r *http.Request, p *packages.Package) {
	remotePath := fmt.Sprintf("/epr/package/%s-%s.zip", p.Name, p.Version)
	pr.redirectRequest(w, r, remotePath)
}

func (pr proxyResolver) ForwardStaticHandler(w http.ResponseWriter, r *http.Request, p *packages.Package, resourcePath string) {
	remotePath := fmt.Sprintf("/package/%s/%s/%s", p.Name, p.Version, resourcePath)
	pr.redirectRequest(w, r, remotePath)
}

func (pr proxyResolver) ForwardSignaturesHandler(w http.ResponseWriter, r *http.Request, p *packages.Package) {
	remotePath := fmt.Sprintf("/epr/package/%s-%s.zip.sig", p.Name, p.Version)
	pr.redirectRequest(w, r, remotePath)
}

var _ packages.RemoteResolver = new(proxyResolver)

// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/elastic/package-registry/packages"
)

type storageResolver struct {
	artifactsPackagesURL url.URL
	artifactsStaticURL   url.URL
}

func (resolver storageResolver) pipeRequestProxy(w http.ResponseWriter, r *http.Request, remoteURL *url.URL) {
	r.URL = remoteURL
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, "error from package-storage server", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Set headers before setting the body. If not, first call to w.Write will
	// add some default values.
	addRequestHeadersToResponse(w, resp)
	w.WriteHeader(resp.StatusCode)

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, "error writing response", http.StatusInternalServerError)
		return
	}

	return
}

func addRequestHeadersToResponse(w http.ResponseWriter, resp *http.Response) {
	for header, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(header, value)
		}
	}
}

func (resolver storageResolver) ArtifactsHandler(w http.ResponseWriter, r *http.Request, p *packages.Package) {
	nameVersionZip := fmt.Sprintf("%s-%s.zip", p.Name, p.Version)
	artifactURL := resolver.artifactsPackagesURL.ResolveReference(&url.URL{Path: nameVersionZip})
	resolver.pipeRequestProxy(w, r, artifactURL)
}

func (resolver storageResolver) StaticHandler(w http.ResponseWriter, r *http.Request, p *packages.Package, resourcePath string) {
	nameVersion := fmt.Sprintf("%s-%s/", p.Name, p.Version)
	staticURL := resolver.artifactsStaticURL.
		ResolveReference(&url.URL{Path: nameVersion}).
		ResolveReference(&url.URL{Path: resourcePath})
	resolver.pipeRequestProxy(w, r, staticURL)
}

func (resolver storageResolver) SignaturesHandler(w http.ResponseWriter, r *http.Request, p *packages.Package) {
	nameVersionSigZip := fmt.Sprintf("%s-%s.zip.sig", p.Name, p.Version)
	signatureURL := resolver.artifactsPackagesURL.ResolveReference(&url.URL{Path: nameVersionSigZip})
	resolver.pipeRequestProxy(w, r, signatureURL)
}

var _ packages.RemoteResolver = new(storageResolver)

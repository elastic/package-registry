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

var acceptedHeaders = map[string]string{
	"Content-Length": "",
	"Content-Type":   "",
	"Last-Modified":  "",
}

func (resolver storageResolver) pipeRequestProxy(w http.ResponseWriter, remoteURL string) error {
	resp, err := http.Get(remoteURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for header, values := range resp.Header {
		if len(w.Header().Values(header)) > 0 {
			continue
		}
		if _, ok := acceptedHeaders[header]; !ok {
			continue
		}
		for _, value := range values {
			w.Header().Add(header, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func (resolver storageResolver) RedirectArtifactsHandler(w http.ResponseWriter, r *http.Request, p *packages.Package) {
	nameVersionZip := fmt.Sprintf("%s-%s.zip", p.Name, p.Version)
	artifactURL := resolver.artifactsPackagesURL.ResolveReference(&url.URL{Path: nameVersionZip})
	resolver.pipeRequestProxy(w, artifactURL.String())
}

func (resolver storageResolver) RedirectStaticHandler(w http.ResponseWriter, r *http.Request, p *packages.Package, resourcePath string) {
	nameVersion := fmt.Sprintf("%s-%s/", p.Name, p.Version)
	staticURL := resolver.artifactsStaticURL.
		ResolveReference(&url.URL{Path: nameVersion}).
		ResolveReference(&url.URL{Path: resourcePath})
	resolver.pipeRequestProxy(w, staticURL.String())
}

func (resolver storageResolver) RedirectSignaturesHandler(w http.ResponseWriter, r *http.Request, p *packages.Package) {
	nameVersionSigZip := fmt.Sprintf("%s-%s.zip.sig", p.Name, p.Version)
	signatureURL := resolver.artifactsPackagesURL.ResolveReference(&url.URL{Path: nameVersionSigZip})
	resolver.pipeRequestProxy(w, signatureURL.String())
}

var _ packages.RemoteResolver = new(storageResolver)

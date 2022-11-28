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
	"Accept-Ranges":  "",
	"Content-Range":  "",
	"Last-Modified":  "",
	"Date":           "",
}

func (resolver storageResolver) pipeRequestProxy(w http.ResponseWriter, r *http.Request, remoteURL string) error {
	client := &http.Client{}

	forwardRequest, err := http.NewRequestWithContext(r.Context(), r.Method, remoteURL, nil)
	addRequestHeadersToRequest(r, forwardRequest)

	resp, err := client.Do(forwardRequest)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	defer resp.Body.Close()

	addRequestHeadersToResponse(w, resp)

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.WriteHeader(resp.StatusCode)
	return nil
}

func addRequestHeadersToRequest(orig, forward *http.Request) {
	for header, values := range orig.Header {
		for _, value := range values {
			forward.Header.Add(header, value)
		}
	}
}

func addRequestHeadersToResponse(w http.ResponseWriter, resp *http.Response) {
	for header, values := range resp.Header {
		if len(w.Header().Values(header)) > 0 {
			// do not overwrite
			continue
		}
		if _, ok := acceptedHeaders[header]; !ok {
			continue
		}
		for _, value := range values {
			w.Header().Add(header, value)
		}
	}
}

func (resolver storageResolver) RedirectArtifactsHandler(w http.ResponseWriter, r *http.Request, p *packages.Package) {
	nameVersionZip := fmt.Sprintf("%s-%s.zip", p.Name, p.Version)
	artifactURL := resolver.artifactsPackagesURL.ResolveReference(&url.URL{Path: nameVersionZip})
	resolver.pipeRequestProxy(w, r, artifactURL.String())
}

func (resolver storageResolver) RedirectStaticHandler(w http.ResponseWriter, r *http.Request, p *packages.Package, resourcePath string) {
	nameVersion := fmt.Sprintf("%s-%s/", p.Name, p.Version)
	staticURL := resolver.artifactsStaticURL.
		ResolveReference(&url.URL{Path: nameVersion}).
		ResolveReference(&url.URL{Path: resourcePath})
	resolver.pipeRequestProxy(w, r, staticURL.String())
}

func (resolver storageResolver) RedirectSignaturesHandler(w http.ResponseWriter, r *http.Request, p *packages.Package) {
	nameVersionSigZip := fmt.Sprintf("%s-%s.zip.sig", p.Name, p.Version)
	signatureURL := resolver.artifactsPackagesURL.ResolveReference(&url.URL{Path: nameVersionSigZip})
	resolver.pipeRequestProxy(w, r, signatureURL.String())
}

var _ packages.RemoteResolver = new(storageResolver)

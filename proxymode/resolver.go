// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package proxymode

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/elastic/package-registry/packages"
)

type proxyResolver struct {
	destinationURL url.URL
}

var acceptedHeaders = map[string]string{
	"Content-Length": "",
	"Content-Type":   "",
	"Last-Modified":  "",
	"Age":            "",
}

func (pr proxyResolver) pipeRequestProxy(w http.ResponseWriter, remotePath string) error {
	remoteURL := pr.destinationURL.ResolveReference(&url.URL{Path: remotePath})

	resp, err := http.Get(remoteURL.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for header, values := range resp.Header {
		if len(w.Header().Values(header)) > 0 {
			log.Printf(">>>> Filtered header (already exists) %s:%v", header, values)
			continue
		}
		if _, ok := acceptedHeaders[header]; !ok {
			log.Printf(">>>> Filtered header %s:%v", header, values)
			continue
		}
		for _, value := range values {
			log.Printf(">>>> Adding header %s:%s", header, value)
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

func (pr proxyResolver) RedirectArtifactsHandler(w http.ResponseWriter, r *http.Request, p *packages.Package) {
	remotePath := fmt.Sprintf("/epr/package/%s-%s.zip", p.Name, p.Version)

	pr.pipeRequestProxy(w, remotePath)
}

func (pr proxyResolver) RedirectStaticHandler(w http.ResponseWriter, r *http.Request, p *packages.Package, resourcePath string) {
	remotePath := fmt.Sprintf("/package/%s/%s/%s", p.Name, p.Version, resourcePath)

	pr.pipeRequestProxy(w, remotePath)
}

func (pr proxyResolver) RedirectSignaturesHandler(w http.ResponseWriter, r *http.Request, p *packages.Package) {
	remotePath := fmt.Sprintf("/epr/package/%s-%s.zip.sig", p.Name, p.Version)

	pr.pipeRequestProxy(w, remotePath)
}

var _ packages.RemoteResolver = new(proxyResolver)

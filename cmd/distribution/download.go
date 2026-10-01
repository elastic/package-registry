// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/ProtonMail/go-crypto/openpgp"
)

type downloadAction struct {
	client  *http.Client
	keyRing openpgp.KeyRing

	Address     string `yaml:"address"`
	Destination string `yaml:"destination"`
}

// publicKey is the public key of the key used to sign elastic artifacts.
// Downloaded from https://artifacts.elastic.co/GPG-KEY-elasticsearch
//
//go:embed GPG-KEY-elasticsearch
var publicKey []byte

func (a *downloadAction) init(c config) error {
	a.client = httpClient
	if a.Address == "" {
		a.Address = c.Address
	}
	err := os.MkdirAll(a.Destination, 0755)
	if err != nil {
		return fmt.Errorf("failed to create desination directory: %w", err)
	}
	a.keyRing, err = openpgp.ReadArmoredKeyRing(bytes.NewReader(publicKey))
	if err != nil {
		return fmt.Errorf("failed to initialize public key: %w", err)
	}
	return nil
}

func (a *downloadAction) perform(i packageInfo) error {
	if i.SignaturePath == "" {
		return fmt.Errorf("package %s-%s has no signature path", i.Name, i.Version)
	}
	if valid, err := a.valid(i); valid {
		return nil
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "existing file invalid for %s, re-downloading: %v\n", i.Download, err)
	}
	if err := a.download(i.Download); err != nil {
		return fmt.Errorf("failed to download package %s: %w", i.Download, err)
	}
	if err := a.download(i.SignaturePath); err != nil {
		removeFile(a.destinationPath(i.Download))
		return fmt.Errorf("failed to download signature %s: %w", i.SignaturePath, err)
	}
	if _, err := a.valid(i); err != nil {
		removeFile(a.destinationPath(i.Download))
		removeFile(a.destinationPath(i.SignaturePath))
		return fmt.Errorf("signature verification failed for %s: %w", i.Download, err)
	}
	return nil
}

func (a *downloadAction) download(urlPath string) error {
	p, err := url.JoinPath(a.Address, urlPath)
	if err != nil {
		return fmt.Errorf("failed to build url: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p, nil)
	if err != nil {
		return fmt.Errorf("failed to build request for %s: %w", urlPath, err)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get %s: %w", urlPath, err)
	}
	defer drainAndClose(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get %s (status code %d)", urlPath, resp.StatusCode)
	}

	f, err := os.Create(a.destinationPath(urlPath))
	if err != nil {
		return fmt.Errorf("failed to open %s in %s: %w", path.Base(urlPath), a.Destination, err)
	}
	if _, err = io.Copy(f, resp.Body); err != nil {
		f.Close()
		removeFile(f.Name())
		return err
	}
	if err := f.Close(); err != nil {
		removeFile(f.Name())
		return err
	}
	return nil
}

// removeFile removes path and logs to stderr if the removal fails.
func removeFile(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "failed to remove %s: %v\n", path, err)
	}
}

func (a *downloadAction) destinationPath(urlPath string) string {
	return filepath.Join(a.Destination, path.Base(urlPath))
}

func (a *downloadAction) valid(info packageInfo) (bool, error) {
	signed, err := os.Open(a.destinationPath(info.Download))
	if err != nil {
		return false, err
	}
	defer signed.Close()

	signature, err := os.Open(a.destinationPath(info.SignaturePath))
	if err != nil {
		return false, err
	}
	defer signature.Close()

	_, err = openpgp.CheckArmoredDetachedSignature(a.keyRing, signed, signature, nil)
	if err != nil {
		return false, err
	}
	return true, nil
}

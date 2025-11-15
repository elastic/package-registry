// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"golang.org/x/crypto/openpgp"
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
	a.client = &http.Client{}
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
	if valid, _ := a.valid(i); valid {
		return nil
	}
	if err := a.download(i.Download); err != nil {
		return fmt.Errorf("failed to download package %s: %w", i.Download, err)
	}
	if err := a.download(i.SignaturePath); err != nil {
		return fmt.Errorf("failed to download signature %s: %w", i.SignaturePath, err)
	}
	return nil
}

func (a *downloadAction) download(urlPath string) error {
	p, err := url.JoinPath(a.Address, urlPath)
	if err != nil {
		return fmt.Errorf("failed to build url: %w", err)
	}
	resp, err := a.client.Get(p)
	if err != nil {
		return fmt.Errorf("failed to get %s: %w", urlPath, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get %s (status code %d)", urlPath, resp.StatusCode)
	}

	f, err := os.Create(a.destinationPath(urlPath))
	if err != nil {
		return fmt.Errorf("failed to open %s in %s: %w", path.Base(urlPath), a.Destination, err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
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

	_, err = openpgp.CheckArmoredDetachedSignature(a.keyRing, signed, signature)
	if err != nil {
		return false, err
	}
	return true, nil
}

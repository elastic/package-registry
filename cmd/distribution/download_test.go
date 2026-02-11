// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadActionInit(t *testing.T) {
	tempDir := t.TempDir()

	action := &downloadAction{
		Destination: tempDir,
	}

	err := action.init(config{})
	require.NoError(t, err)
	assert.NotNil(t, action.client)
	assert.NotNil(t, action.keyRing)

	// Verify destination directory was created
	info, err := os.Stat(tempDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestDownloadActionDestinationPath(t *testing.T) {
	destination := filepath.Join(os.TempDir(), "downloads")
	action := &downloadAction{
		Destination: destination,
	}

	path := action.destinationPath("epr/nginx/nginx-1.0.0.zip")
	require.Equal(t, filepath.Join(destination, "nginx-1.0.0.zip"), path)
}

func TestDownloadActionDownload(t *testing.T) {
	tempDir := t.TempDir()

	testContent := []byte("test package content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(testContent)
	}))
	defer server.Close()

	action := &downloadAction{
		Destination: tempDir,
		client:      &http.Client{},
		Address:     server.URL,
	}

	err := action.download("epr/nginx/nginx-1.0.0.zip")
	require.NoError(t, err)

	// Verify file was created with correct content
	downloaded, err := os.ReadFile(filepath.Join(tempDir, "nginx-1.0.0.zip"))
	require.NoError(t, err)
	assert.Equal(t, testContent, downloaded)
}

func TestDownloadActionDownloadHTTPError(t *testing.T) {
	tempDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	action := &downloadAction{
		Destination: tempDir,
		client:      &http.Client{},
		Address:     server.URL,
	}

	err := action.download("epr/nginx/nginx-1.0.0.zip")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status code 404")
}

func TestDownloadActionPerformSkipsIfAlreadyDownloaded(t *testing.T) {
	tempDir := t.TempDir()

	// Create a simple test key for signing
	entity, err := openpgp.NewEntity("test", "test", "test@example.com", nil)
	require.NoError(t, err)

	// Create test package content
	pkgContent := []byte("test package")
	pkgPath := filepath.Join(tempDir, "nginx-1.0.0.zip")
	err = os.WriteFile(pkgPath, pkgContent, 0644)
	require.NoError(t, err)

	// Create signature
	var sigBuf bytes.Buffer
	err = openpgp.ArmoredDetachSign(&sigBuf, entity, bytes.NewReader(pkgContent), nil)
	require.NoError(t, err)

	sigPath := filepath.Join(tempDir, "nginx-1.0.0.zip.sig")
	err = os.WriteFile(sigPath, sigBuf.Bytes(), 0644)
	require.NoError(t, err)

	// Create action with matching keyring
	keyring := openpgp.EntityList{entity}
	action := &downloadAction{
		Destination: tempDir,
		client:      &http.Client{},
		keyRing:     keyring,
	}

	// Server should not be called if package is valid
	serverCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	action.Address = server.URL

	info := packageInfo{
		Name:          "nginx",
		Version:       "1.0.0",
		Download:      "epr/nginx/nginx-1.0.0.zip",
		SignaturePath: "epr/nginx/nginx-1.0.0.zip.sig",
	}

	err = action.perform(info)
	require.NoError(t, err)
	assert.False(t, serverCalled, "Server should not be called when package is already valid")
}

func TestDownloadActionPerformDownloadsIfMissing(t *testing.T) {
	tempDir := t.TempDir()

	// Create a simple test key for signing
	entity, err := openpgp.NewEntity("test", "test", "test@example.com", nil)
	require.NoError(t, err)

	// Create test package content and signature
	pkgContent := []byte("test package")
	var sigBuf bytes.Buffer
	err = openpgp.ArmoredDetachSign(&sigBuf, entity, bytes.NewReader(pkgContent), nil)
	require.NoError(t, err)

	// Mock server that returns package and signature
	downloadCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloadCount++
		if r.URL.Path == "/epr/nginx/nginx-1.0.0.zip" {
			w.Write(pkgContent)
		} else if r.URL.Path == "/epr/nginx/nginx-1.0.0.zip.sig" {
			w.Write(sigBuf.Bytes())
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Use the test key's keyring for validation
	keyring := openpgp.EntityList{entity}

	action := &downloadAction{
		Destination: tempDir,
		client:      &http.Client{},
		Address:     server.URL,
		keyRing:     keyring,
	}

	info := packageInfo{
		Name:          "nginx",
		Version:       "1.0.0",
		Download:      "epr/nginx/nginx-1.0.0.zip",
		SignaturePath: "epr/nginx/nginx-1.0.0.zip.sig",
	}

	err = action.perform(info)
	// With matching key, download and verification should succeed
	require.NoError(t, err)
	assert.Equal(t, 2, downloadCount, "Should download both package and signature")

	// Verify files were created with correct content
	downloaded, err := os.ReadFile(filepath.Join(tempDir, "nginx-1.0.0.zip"))
	if assert.NoError(t, err) {
		assert.Equal(t, pkgContent, downloaded)
	}

	downloadedSignature, err := os.ReadFile(filepath.Join(tempDir, "nginx-1.0.0.zip.sig"))
	if assert.NoError(t, err) {
		assert.Equal(t, sigBuf.String(), string(downloadedSignature))
	}
}

func TestDownloadActionValid(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test key
	entity, err := openpgp.NewEntity("test", "test", "test@example.com", nil)
	require.NoError(t, err)

	// Create test package
	pkgContent := []byte("test package content")
	pkgPath := filepath.Join(tempDir, "nginx-1.0.0.zip")
	err = os.WriteFile(pkgPath, pkgContent, 0644)
	require.NoError(t, err)

	// Create valid signature
	var sigBuf bytes.Buffer
	err = openpgp.ArmoredDetachSign(&sigBuf, entity, bytes.NewReader(pkgContent), nil)
	require.NoError(t, err)
	sigPath := filepath.Join(tempDir, "nginx-1.0.0.zip.sig")
	err = os.WriteFile(sigPath, sigBuf.Bytes(), 0644)
	require.NoError(t, err)

	keyring := openpgp.EntityList{entity}
	action := &downloadAction{
		Destination: tempDir,
		keyRing:     keyring,
	}

	info := packageInfo{
		Name:          "nginx",
		Version:       "1.0.0",
		Download:      "epr/nginx/nginx-1.0.0.zip",
		SignaturePath: "epr/nginx/nginx-1.0.0.zip.sig",
	}

	valid, err := action.valid(info)
	require.NoError(t, err)
	assert.True(t, valid)
}

func TestDownloadActionValidInvalidSignature(t *testing.T) {
	tempDir := t.TempDir()

	// Create test package
	pkgContent := []byte("test package content")
	pkgPath := filepath.Join(tempDir, "nginx-1.0.0.zip")
	err := os.WriteFile(pkgPath, pkgContent, 0644)
	require.NoError(t, err)

	// Create signature with one key
	entity1, err := openpgp.NewEntity("test1", "test1", "test1@example.com", nil)
	require.NoError(t, err)
	var sigBuf bytes.Buffer
	err = openpgp.ArmoredDetachSign(&sigBuf, entity1, bytes.NewReader(pkgContent), nil)
	require.NoError(t, err)
	sigPath := filepath.Join(tempDir, "nginx-1.0.0.zip.sig")
	err = os.WriteFile(sigPath, sigBuf.Bytes(), 0644)
	require.NoError(t, err)

	// Verify with different key
	entity2, err := openpgp.NewEntity("test2", "test2", "test2@example.com", nil)
	require.NoError(t, err)
	keyring := openpgp.EntityList{entity2}

	action := &downloadAction{
		Destination: tempDir,
		keyRing:     keyring,
	}

	info := packageInfo{
		Name:          "nginx",
		Version:       "1.0.0",
		Download:      "epr/nginx/nginx-1.0.0.zip",
		SignaturePath: "epr/nginx/nginx-1.0.0.zip.sig",
	}

	valid, err := action.valid(info)
	require.Error(t, err)
	assert.False(t, valid)
}

func TestDownloadActionValidMissingFiles(t *testing.T) {
	tempDir := t.TempDir()

	action := &downloadAction{
		Destination: tempDir,
	}

	info := packageInfo{
		Name:          "nginx",
		Version:       "1.0.0",
		Download:      "epr/nginx/nginx-1.0.0.zip",
		SignaturePath: "epr/nginx/nginx-1.0.0.zip.sig",
	}

	valid, err := action.valid(info)
	require.Error(t, err)
	assert.False(t, valid)
}

func TestDownloadActionAddressInheritance(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name            string
		actionAddress   string
		configAddress   string
		expectedAddress string
	}{
		{
			name:            "action address takes precedence",
			actionAddress:   "https://action.elastic.co",
			configAddress:   "https://config.elastic.co",
			expectedAddress: "https://action.elastic.co",
		},
		{
			name:            "inherits from config",
			actionAddress:   "",
			configAddress:   "https://config.elastic.co",
			expectedAddress: "https://config.elastic.co",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := &downloadAction{
				Destination: tempDir,
				Address:     tt.actionAddress,
			}

			cfg := config{
				Address: tt.configAddress,
			}

			err := action.init(cfg)
			require.NoError(t, err)
			require.Equal(t, tt.expectedAddress, action.Address)
		})
	}
}

func TestDownloadActionIntegration(t *testing.T) {
	tempDir := t.TempDir()

	// Create test packages
	packages := []struct {
		name    string
		content []byte
	}{
		{"nginx-1.0.0.zip", []byte("nginx package")},
		{"apache-2.0.0.zip", []byte("apache package")},
	}

	// Create signing key
	entity, err := openpgp.NewEntity("test", "test", "test@example.com", nil)
	require.NoError(t, err)

	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, pkg := range packages {
			pkgPath := fmt.Sprintf("/epr/%s", pkg.name)
			sigPath := pkgPath + ".sig"

			if r.URL.Path == pkgPath {
				w.Write(pkg.content)
				return
			} else if r.URL.Path == sigPath {
				var sigBuf bytes.Buffer
				openpgp.ArmoredDetachSign(&sigBuf, entity, bytes.NewReader(pkg.content), nil)
				w.Write(sigBuf.Bytes())
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	action := &downloadAction{
		Destination: tempDir,
		Address:     server.URL,
	}

	err = action.init(config{})
	require.NoError(t, err)

	// Override keyring with test key
	action.keyRing = openpgp.EntityList{entity}

	// Download packages
	infos := []packageInfo{
		{
			Name:          "nginx",
			Version:       "1.0.0",
			Download:      "epr/nginx-1.0.0.zip",
			SignaturePath: "epr/nginx-1.0.0.zip.sig",
		},
		{
			Name:          "apache",
			Version:       "2.0.0",
			Download:      "epr/apache-2.0.0.zip",
			SignaturePath: "epr/apache-2.0.0.zip.sig",
		},
	}

	for _, info := range infos {
		err := action.perform(info)
		require.NoError(t, err)
	}

	// Verify all files were downloaded
	for _, pkg := range packages {
		content, err := os.ReadFile(filepath.Join(tempDir, pkg.name))
		require.NoError(t, err)
		require.Equal(t, pkg.content, content)

		// Verify signature files exist
		_, err = os.Stat(filepath.Join(tempDir, pkg.name+".sig"))
		require.NoError(t, err)
	}
}

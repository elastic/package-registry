// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package packages

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/elastic/package-registry/archiver"
)

const (
	iacPatchesFile  = "iac/account.cloudformation.patches.json"
	iacTemplateFile = "iac/account.cloudformation.json"
	iacBlueprintID  = "aws/federated-identity/account/v1"
	iacPatchesJSON  = `[{"op":"add","path":"/Resources/ExampleBucket","value":{"Type":"AWS::S3::Bucket"}}]`
	iacTemplateJSON = `{"AWSTemplateFormatVersion":"2010-09-09","Resources":{"ExampleBucket":{"Type":"AWS::S3::Bucket"}}}`
	iacTestPkgName  = "mypkg"
	iacTestPkgVer   = "1.0.0"
)

func TestPackageLoadAssetsIncludesIac(t *testing.T) {
	pkg := loadIacTestPackage(t)

	assert.True(t, hasAssetSuffix(pkg.Assets, "/"+iacPatchesFile), "assets should include %s, got %v", iacPatchesFile, pkg.Assets)
	assert.True(t, hasAssetSuffix(pkg.Assets, "/"+iacTemplateFile), "assets should include %s, got %v", iacTemplateFile, pkg.Assets)
}

func TestPackageIndexIncludesIaCBlueprints(t *testing.T) {
	pkg := loadIacTestPackage(t)

	require.NotEmpty(t, pkg.IaCBlueprints)
	assert.Equal(t, iacBlueprintID, pkg.IaCBlueprints[0].ID)
	assert.Equal(t, "cloudformation", pkg.IaCBlueprints[0].Format)
	assert.Equal(t, iacPatchesFile, pkg.IaCBlueprints[0].Patches)

	data, err := json.Marshal(pkg)
	require.NoError(t, err)

	var decoded map[string]interface{}
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	blueprints, ok := decoded["iac_blueprints"].([]interface{})
	require.True(t, ok, "package JSON should include iac_blueprints")
	require.NotEmpty(t, blueprints)
	first, ok := blueprints[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, iacBlueprintID, first["id"])
}

func TestServePackageResourceIac(t *testing.T) {
	pkg := loadIacTestPackage(t)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/package/"+iacTestPkgName+"/"+iacTestPkgVer+"/"+iacPatchesFile, nil)
	ServePackageResource(zap.NewNop(), recorder, req, pkg, iacPatchesFile)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, iacPatchesJSON, recorder.Body.String())
}

func TestArchivePackageIncludesIac(t *testing.T) {
	pkg := loadIacTestPackage(t)

	var buf bytes.Buffer
	err := archiver.ArchivePackage(&buf, archiver.PackageProperties{
		Name:    pkg.Name,
		Version: pkg.Version,
		Path:    pkg.BasePath,
	})
	require.NoError(t, err)

	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)

	root := iacTestPkgName + "-" + iacTestPkgVer + "/"
	assert.True(t, zipContains(reader, root+iacPatchesFile), "zip should contain %s", root+iacPatchesFile)
	assert.True(t, zipContains(reader, root+iacTemplateFile), "zip should contain %s", root+iacTemplateFile)
}

func loadIacTestPackage(t *testing.T) *Package {
	t.Helper()
	pkgDir := createIacTestPackage(t)
	p, err := NewPackage(zap.NewNop(), pkgDir, ExtractedFileSystemBuilder)
	require.NoError(t, err)
	return p
}

func createIacTestPackage(t *testing.T) string {
	t.Helper()

	pkgDir := filepath.Join(t.TempDir(), iacTestPkgName, iacTestPkgVer)
	err := os.MkdirAll(filepath.Join(pkgDir, "docs"), 0755)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(pkgDir, "iac"), 0755)
	require.NoError(t, err)

	manifest := `format_version: 1.0.0
name: ` + iacTestPkgName + `
title: My Package
description: Test package for iac folder serving
version: ` + iacTestPkgVer + `
type: integration
owner:
  github: elastic
iac_blueprints:
  - id: ` + iacBlueprintID + `
    format: cloudformation
    patches: ` + iacPatchesFile + `
    title: Account
`
	err = os.WriteFile(filepath.Join(pkgDir, "manifest.yml"), []byte(manifest), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(pkgDir, "docs", "README.md"), []byte("# My Package\n"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(pkgDir, filepath.FromSlash(iacPatchesFile)), []byte(iacPatchesJSON), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(pkgDir, filepath.FromSlash(iacTemplateFile)), []byte(iacTemplateJSON), 0644)
	require.NoError(t, err)

	return pkgDir
}

func hasAssetSuffix(assets []string, suffix string) bool {
	for _, a := range assets {
		if strings.HasSuffix(a, suffix) {
			return true
		}
	}
	return false
}

func zipContains(reader *zip.Reader, name string) bool {
	for _, f := range reader.File {
		if f.Name == name {
			return true
		}
	}
	return false
}

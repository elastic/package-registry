// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archiver

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ExtractPackage(archive string, target string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}

	defer r.Close()
	for _, f := range r.File {
		err := extractFile(f, target)
		if err != nil {
			return err
		}
	}

	return nil
}

func extractFile(f *zip.File, basePath string) error {
	zippedFile, err := f.Open()
	if err != nil {
		return err
	}

	defer zippedFile.Close()
	path := filepath.Join(basePath, adjustBaseDir(f.Name))
	if !strings.HasPrefix(path, filepath.Clean(basePath)+string(filepath.Separator)) {
		return fmt.Errorf("illegal path: %s", path)
	}

	if f.FileInfo().IsDir() {
		os.MkdirAll(path, f.Mode())
		return nil
	}

	os.MkdirAll(filepath.Dir(path), f.Mode())
	target, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer target.Close()

	_, err = io.Copy(target, zippedFile)
	if err != nil {
		return err
	}
	return nil
}

func adjustBaseDir(path string) string {
	fileName := filepath.Clean(path)
	if fileName == "" {
		return fileName
	}

	dirs := strings.SplitN(fileName, string(filepath.Separator), 2)

	// TODO: Check if there can be other - in the base directory.
	dirs[0] = strings.Replace(dirs[0], "-", "/", 1)

	if len(dirs) == 1 {
		return dirs[0]
	}

	return filepath.Join(dirs[0], dirs[1])
}

// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type PackageFileSystem interface {
	Stat(name string) (os.FileInfo, error)
	Open(name string) (io.ReadCloser, error)
	Glob(pattern string) (matches []string, err error)
	Close() error
}

func NewPackageFileSystem(path string) (PackageFileSystem, error) {
	if path == "" {
		return NewVirtualPackageFileSystem()
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	switch {
	case info.IsDir():
		return NewExtractedPackageFileSystem(path)
	case strings.HasSuffix(path, ".zip"):
		return NewZipPackageFileSystem(path)
	default:
		return nil, fmt.Errorf("unsupported file system in path: %s", path)
	}
}

// extractedPackageFileSystem provides utils to access files in an extracted package.
type extractedPackageFileSystem struct {
	path string
}

func NewExtractedPackageFileSystem(path string) (*extractedPackageFileSystem, error) {
	return &extractedPackageFileSystem{path: path}, nil
}

func (fs *extractedPackageFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(filepath.Join(fs.path, name))
}

func (fs *extractedPackageFileSystem) Open(name string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(fs.path, name))
}

func (fs *extractedPackageFileSystem) Glob(pattern string) (matches []string, err error) {
	matches, err = filepath.Glob(filepath.Join(fs.path, pattern))
	if err != nil {
		return
	}
	for i := range matches {
		matches[i] = matches[i][len(fs.path+string(filepath.Separator)):]
	}
	return
}

func (fs *extractedPackageFileSystem) Close() error { return nil }

// zipPackageFileSystem provides utils to access files in a zipped package.
type zipPackageFileSystem struct {
	root   string
	reader *zip.ReadCloser
}

func NewZipPackageFileSystem(path string) (*zipPackageFileSystem, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	var root string
	found := false
	for _, f := range reader.File {
		name := filepath.Clean(f.Name)
		parts := strings.Split(name, string(filepath.Separator))
		if len(parts) == 2 && parts[1] == "manifest.yml" {
			root = parts[0]
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("failed to determine root directory in package (path: %s)", path)
	}
	return &zipPackageFileSystem{
		root:   root,
		reader: reader,
	}, nil
}

func (fs *zipPackageFileSystem) Stat(name string) (os.FileInfo, error) {
	path := filepath.Join(fs.root, name)
	f, err := fs.reader.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Stat()
}

func (fs *zipPackageFileSystem) Open(name string) (io.ReadCloser, error) {
	path := filepath.Join(fs.root, name)
	f, err := fs.reader.Open(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (fs *zipPackageFileSystem) Glob(pattern string) (matches []string, err error) {
	pattern = filepath.Join(fs.root, pattern)
	for _, f := range fs.reader.File {
		match, err := filepath.Match(pattern, filepath.Clean(f.Name))
		if err != nil {
			return nil, err
		}
		if match {
			name := strings.TrimPrefix(f.Name, fs.root+string(filepath.Separator))
			matches = append(matches, name)
		}
	}
	return
}

func (fs *zipPackageFileSystem) Close() error {
	return fs.reader.Close()
}

func ReadAll(fs PackageFileSystem, name string) ([]byte, error) {
	f, err := fs.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return ioutil.ReadAll(f)
}

// virtualPackageFileSystem provide utils for package objects that don't correspond to
// any real package in any backend. Used mainly for testing purpouses.
type virtualPackageFileSystem struct {
}

func NewVirtualPackageFileSystem() (*virtualPackageFileSystem, error) {
	return &virtualPackageFileSystem{}, nil
}

func (fs *virtualPackageFileSystem) Stat(name string) (os.FileInfo, error) {
	return nil, os.ErrNotExist
}

func (fs *virtualPackageFileSystem) Open(name string) (io.ReadCloser, error) {
	return nil, os.ErrNotExist
}

func (fs *virtualPackageFileSystem) Glob(pattern string) (matches []string, err error) {
	return []string{}, nil
}

func (fs *virtualPackageFileSystem) Close() error { return nil }

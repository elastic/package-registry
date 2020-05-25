// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archiver

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/joeshaw/multierror"
	"github.com/pkg/errors"

	"github.com/elastic/package-registry/util"
)

// PackageProperties defines properties describing the package. The structure is used for archiving.
type PackageProperties struct {
	Name    string
	Version string
	Path    string
}

// ArchivePackage method builds and streams an archive with package content.
func ArchivePackage(w io.Writer, properties PackageProperties) (err error) {
	gzipWriter := gzip.NewWriter(w)
	tarWriter := tar.NewWriter(gzipWriter)
	defer func() {
		var multiErr multierror.Errors

		if err != nil {
			multiErr = append(multiErr, err)
		}

		err = tarWriter.Close()
		if err != nil {
			multiErr = append(multiErr, errors.Wrapf(err, "closing tar writer failed"))
		}

		err = gzipWriter.Close()
		if err != nil {
			multiErr = append(multiErr, errors.Wrapf(err, "closing gzip writer failed"))
		}

		if multiErr != nil {
			err = multiErr.Err()
		}
	}()

	rootDir := fmt.Sprintf("%s-%s", properties.Name, properties.Version)

	err = writePackageIndexToArchive(properties.Path, rootDir, tarWriter)
	if err != nil {
		return errors.Wrapf(err, "writing package index failed")
	}

	err = filepath.Walk(properties.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(properties.Path, path)
		if err != nil {
			return errors.Wrapf(err, "finding relative path failed (packagePath: %s, path: %s)", properties.Path, path)
		}

		if relativePath == "." {
			return nil
		}

		header, err := buildArchiveHeader(info, filepath.Join(rootDir, relativePath))
		if err != nil {
			return errors.Wrapf(err, "building archive header failed (path: %s)", relativePath)
		}

		err = tarWriter.WriteHeader(header)
		if err != nil {
			return errors.Wrapf(err, "writing header failed (path: %s)", relativePath)
		}

		if !info.IsDir() {
			err = writeFileContentToArchive(path, tarWriter)
			if err != nil {
				return errors.Wrapf(err, "archiving file content failed (path: %s)", path)
			}
		}
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "processing package path '%s' failed", properties.Path)
	}

	err = tarWriter.Flush()
	if err != nil {
		return errors.Wrap(err, "flushing tar writer failed")
	}

	err = gzipWriter.Flush()
	if err != nil {
		return errors.Wrap(err, "flushing gzip writer failed")
	}
	return nil
}

func buildArchiveHeader(info os.FileInfo, relativePath string) (*tar.Header, error) {
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return nil, errors.Wrapf(err, "reading file info header failed (info: %s)", info.Name())
	}

	header.Name = relativePath
	if info.IsDir() && !strings.HasSuffix(header.Name, "/") {
		header.Name = header.Name + "/"
	}
	return header, nil
}

func writePackageIndexToArchive(path, rootDir string, tarWriter *tar.Writer) error {
	aPackage, err := util.NewPackage(path)
	if err != nil {
		return errors.Wrapf(err, "building package failed (path: %s)", path)
	}

	packageIndexBody, err := json.MarshalIndent(aPackage, "", "  ")
	if err != nil {
		return errors.Wrapf(err, "marshaling package 'index.json' failed (path: %s)", path)
	}

	err = tarWriter.WriteHeader(&tar.Header{
		Name: filepath.Join(rootDir, "index.json"),
		Size: int64(len(packageIndexBody)),
		Mode: 0644,
	})
	if err != nil {
		return errors.Wrapf(err, "writing package 'index.json' header failed (path: %s)", path)
	}

	_, err = tarWriter.Write(packageIndexBody)
	if err != nil {
		return errors.Wrapf(err, "writing package 'index.json' body failed (path: %s)", path)
	}
	return nil
}

func writeFileContentToArchive(path string, writer io.Writer) (err error) {
	var f *os.File
	f, err = os.Open(path)
	if err != nil {
		return errors.Wrapf(err, "opening file failed (path: %s)", path)
	}
	defer func() {
		var multiErr multierror.Errors
		if err != nil {
			multiErr = append(multiErr, err)
		}

		err = f.Close()
		if err != nil {
			multiErr = append(multiErr, errors.Wrapf(err, "closing file failed (path: %s)", path))
		}

		if multiErr != nil {
			err = multiErr.Err()
		}
	}()

	_, err = io.Copy(writer, f)
	if err != nil {
		return errors.Wrapf(err, "copying file content failed (path: %s)", path)
	}
	return nil
}

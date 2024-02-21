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

	"github.com/joeshaw/multierror"
)

// PackageProperties defines properties describing the package. The structure is used for archiving.
type PackageProperties struct {
	Name    string
	Version string
	Path    string
}

// ArchivePackage method builds and streams an archive with package content.
func ArchivePackage(w io.Writer, properties PackageProperties) (err error) {
	zipWriter := zip.NewWriter(w)
	defer func() {
		var multiErr multierror.Errors

		if err != nil {
			multiErr = append(multiErr, err)
		}

		err = zipWriter.Close()
		if err != nil {
			multiErr = append(multiErr, fmt.Errorf("closing zip writer failed: %w", err))
		}

		if multiErr != nil {
			err = multiErr.Err()
		}
	}()

	rootDir := fmt.Sprintf("%s-%s", properties.Name, properties.Version)
	err = filepath.Walk(properties.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(properties.Path, path)
		if err != nil {
			return fmt.Errorf("finding relative path failed (packagePath: %s, path: %s): %w", properties.Path, path, err)
		}

		if relativePath == "." {
			return nil
		}

		header, err := buildArchiveHeader(info, filepath.Join(rootDir, relativePath))
		if err != nil {
			return fmt.Errorf("building archive header failed (path: %s): %w", relativePath, err)
		}

		w, err = zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("writing header failed (path: %s): %w", relativePath, err)
		}

		if !info.IsDir() {
			err = writeFileContentToArchive(path, w)
			if err != nil {
				return fmt.Errorf("archiving file content failed (path: %s): %w", path, err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("processing package path '%s' failed: %w", properties.Path, err)
	}

	err = zipWriter.Flush()
	if err != nil {
		return fmt.Errorf("flushing zip writer failed: %w", err)
	}
	return nil
}

func buildArchiveHeader(info os.FileInfo, relativePath string) (*zip.FileHeader, error) {
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return nil, fmt.Errorf("reading file info header failed (info: %s): %w", info.Name(), err)
	}

	header.Method = zip.Deflate
	header.Name = relativePath
	if info.IsDir() && !strings.HasSuffix(header.Name, "/") {
		header.Name = header.Name + "/"
	}
	return header, nil
}

func writeFileContentToArchive(path string, writer io.Writer) (err error) {
	var f *os.File
	f, err = os.Open(path)
	if err != nil {
		return fmt.Errorf("opening file failed (path: %s): %w", path, err)
	}
	defer func() {
		var multiErr multierror.Errors
		if err != nil {
			multiErr = append(multiErr, err)
		}

		err = f.Close()
		if err != nil {
			multiErr = append(multiErr, fmt.Errorf("closing file failed (path: %s): %w", path, err))
		}

		if multiErr != nil {
			err = multiErr.Err()
		}
	}()

	_, err = io.Copy(writer, f)
	if err != nil {
		return fmt.Errorf("copying file content failed (path: %s): %w", path, err)
	}
	return nil
}

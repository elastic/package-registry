// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"

	"github.com/pkg/errors"

	"github.com/elastic/package-registry/util"
)

var imageRe = regexp.MustCompile(`image::[^\[]+`)

type imageContent struct {
	source string
}

func createImages(beatDocsPath, modulePath string) ([]imageContent, error) {
	var images []imageContent

	moduleDocsPath := path.Join(modulePath, "_meta", "docs.asciidoc")
	moduleDocsFile, err := ioutil.ReadFile(moduleDocsPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrapf(err, "reading module docs file failed (path: %s)", moduleDocsPath)
	} else if os.IsNotExist(err) {
		log.Printf("\tNo docs found (path: %s), skipped")
	} else {
		log.Printf("\tDocs found (path: %s)", moduleDocsPath)
		images = append(images, extractImages(beatDocsPath, moduleDocsFile)...)
	}

	datasetDirs, err := ioutil.ReadDir(modulePath)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read module directory %s", modulePath)
	}

	for _, datasetDir := range datasetDirs {
		if !datasetDir.IsDir() {
			continue
		}
		datasetName := datasetDir.Name()

		if datasetName == "_meta" {
			continue
		}

		log.Printf("\t%s: dataset found", datasetName)

		datasetDocsPath := path.Join(modulePath, datasetName, "_meta", "docs.asciidoc")
		datasetDocsFile, err := ioutil.ReadFile(datasetDocsPath)
		if err != nil && !os.IsNotExist(err) {
			return nil, errors.Wrapf(err, "reading dataset docs file failed (path: %s)", datasetDocsPath)
		} else if os.IsNotExist(err) {
			log.Printf("\t%s: no docs found (path: %s), skipped", datasetName, datasetDocsPath)
			continue
		}

		log.Printf("\t%s: docs found (path: %s)", datasetName, datasetDocsPath)
		images = append(images, extractImages(beatDocsPath, datasetDocsFile)...)
	}

	return images, nil
}

func createScreenshots(images []imageContent) ([]util.Image, error) {
	return []util.Image{}, nil
}

func extractImages(beatDocsPath string, docsFile []byte) []imageContent {
	matches := imageRe.FindAll(docsFile, -1)

	var contents []imageContent
	for _, match := range matches {
		contents = append(contents, imageContent{
			source: path.Join(beatDocsPath, string(match[7:])), // skip: image::
		})
	}
	return contents
}

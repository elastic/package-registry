// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"os"
	"path/filepath"
	"text/template"

	"github.com/pkg/errors"
)

var emptyReadmeTemplate = template.Must(template.New("README.md").Parse("TODO"))

type docContent struct {
	fileName string
	body *template.Template
}

func createDocTemplates(packageDocsPath string) ([]docContent, error) {
	readmeTemplate, err := createReadmeTemplate(filepath.Join(packageDocsPath, "README.md"))
	if err != nil {
		return nil, errors.Wrapf(err, "creating README template failed")
	}
	return []docContent{
		{fileName: "README.md", body: readmeTemplate},
	}, nil
}

func createReadmeTemplate(readmePath string) (*template.Template, error) {
	t := template.New("README.md")
	t, err := t.ParseFiles(readmePath)
	if os.IsNotExist(err) {
		return emptyReadmeTemplate, nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "parsing template failed (path: %s)", readmePath)
	}
	return t, nil
}

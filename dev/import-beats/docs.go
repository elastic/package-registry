// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/pkg/errors"
)

var readmeTemplate = template.Must(template.New("readme").Parse(`# {{ .ModuleName }} Integration

TODO

## Compatibility

TODO

### Inputs

TODO

## Dashboard

TODO`))

type docContent struct {
	fileName string
	body     []byte
}

type readmeTemplateModel struct {
	ModuleName string
}

func createDocs(moduleName string) ([]docContent, error) {
	var body bytes.Buffer
	err := readmeTemplate.Execute(&body, readmeTemplateModel{
		ModuleName: correctSpelling(strings.Title(moduleName)),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "rendering README template failed")
	}
	return []docContent{
		{fileName: "README.md", body: body.Bytes()},
	}, nil
}

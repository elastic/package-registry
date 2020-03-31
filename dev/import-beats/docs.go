// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

var emptyReadmeTemplate = template.Must(template.New("README.md").Parse("TODO"))

type fieldsTableRecord struct {
	name        string
	description string
	aType       string
}

type docContent struct {
	fileName     string
	templatePath string
}

func createDocTemplates(packageDocsPath string) ([]docContent, error) {
	readmePath := filepath.Join(packageDocsPath, "README.md")
	_, err := os.Stat(readmePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrapf(err, "reading README template failed")
	}
	if os.IsNotExist(err) {
		readmePath = ""
	}
	return []docContent{
		{fileName: "README.md", templatePath: readmePath},
	}, nil
}

func renderExportedFields(packageDataset string, datasets datasetContentArray) (string, error) {
	for _, dataset := range datasets {
		if packageDataset == dataset.name {
			var buffer strings.Builder
			buffer.WriteString("**Exported fields**")
			buffer.WriteString("\n\n")

			if len(dataset.fields.files) == 0 {
				buffer.WriteString("(no fields available)")
			} else {
				collected, err := collectFields(dataset.fields)
				if err != nil {
					return "", errors.Wrapf(err, "collecting fields failed")
				}

				buffer.WriteString("| Field | Description | Type |\n")
				buffer.WriteString("|---|---|---|\n")
				for _, c := range collected {
					buffer.WriteString(fmt.Sprintf("| %s | %s | %s |\n", c.name, c.description, c.aType))
				}
			}
			return buffer.String(), nil
		}
	}
	return "", fmt.Errorf("missing dataset: %s", packageDataset)
}

func collectFields(content fieldsContent) ([]fieldsTableRecord, error) {
	var records []fieldsTableRecord
	for _, fieldsFile := range content.files {
		var fs []mapStr
		err := yaml.Unmarshal(fieldsFile, &fs)
		if err != nil {
			return nil, errors.Wrapf(err, "unmarshalling fields file failed")
		}

		for _, f := range fs {
			records, err = visitFields("", f, records)
			if err != nil {
				return nil, errors.Wrapf(err, "visiting fields failed")
			}
		}
	}

	sort.Slice(records, func(i, j int) bool {
		return sort.StringsAreSorted([]string{records[i].name, records[j].name})
	})
	return records, nil
}

func visitFields(namePrefix string, f mapStr, records []fieldsTableRecord) ([]fieldsTableRecord, error) {
	nameVal, err := f.getValue("name")
	if err != nil {
		return nil, errors.Wrapf(err, "retrieving field 'name' failed")
	}
	name := nameVal.(string)

	fieldsVal, err := f.getValue("fields")
	if err == errKeyNotFound {
		// name
		name = namePrefix + name

		// description
		var description string
		descriptionVal, err := f.getValue("description")
		if err != nil && err != errKeyNotFound {
			return nil, errors.Wrapf(err, "retrieving field 'description' failed (namePrefix: %s)", namePrefix)
		}
		if err != errKeyNotFound {
			description = descriptionVal.(string)
			description = strings.TrimSpace(strings.ReplaceAll(description, "\n", " "))
		}

		// type
		aType := "keyword" // default "type" iif there is no type defined
		typeVal, err := f.getValue("type")
		if err != nil && err != errKeyNotFound {
			return nil, errors.Wrapf(err, "retrieving field 'type' failed (namePrefix: %s)", namePrefix)
		}
		if err != errKeyNotFound {
			aType = typeVal.(string)
		}

		if description == "" && aType == "alias" {
			pathVal, err := f.getValue("path")
			if err != nil {
				return nil, errors.Wrapf(err, "retrieving field 'path' failed")
			}
			path := pathVal.(string)
			description = fmt.Sprintf(`Alias for field "%s"`, path)
		}

		records = append(records, fieldsTableRecord{
			name:        name,
			description: description,
			aType:       aType,
		})
		return records, nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "retrieving field 'fields' failed (namePrefix: %s)", namePrefix)
	}

	if _, ok := fieldsVal.([]interface{}); !ok {
		return records, nil
	}

	for _, fieldsEntryVal := range fieldsVal.([]interface{}) {
		fieldsEntry, err := toMapStr(fieldsEntryVal)
		if err != nil {
			return nil, errors.Wrapf(err, "mapping fields entry failed (namePrefix: %s)", namePrefix)
		}

		records, err = visitFields(namePrefix+name+".", fieldsEntry, records)
		if err != nil {
			return nil, errors.Wrapf(err, "recursive visiting fields failed (namePrefix: %s)", namePrefix)
		}
	}
	return records, nil
}

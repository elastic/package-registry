// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/elastic/package-registry/util"
)

type datasourceContent struct {
	moduleName  string
	moduleTitle string

	inputs map[string]datasourceInput // map[packageType]..
}

type datasourceInput struct {
	datasetNames []string
	vars         []util.Variable
}

func (ds datasourceContent) toMetadataDatasources() []util.Datasource {
	var packageTypes []string
	for packageType := range ds.inputs {
		packageTypes = append(packageTypes, packageType)
	}
	sort.Strings(packageTypes)

	var title, description string
	if len(ds.inputs) == 2 {
		title = toDatasourceTitleForTwoTypes(ds.moduleTitle, packageTypes[0], packageTypes[1])
		description = toDatasourceDescriptionForTwoTypes(ds.moduleTitle, packageTypes[0], packageTypes[1])
	} else {
		title = toDatasourceTitle(ds.moduleTitle, packageTypes[0])
		description = toDatasourceDescription(ds.moduleTitle, packageTypes[0])
	}

	var inputs []util.Input
	for _, packageType := range packageTypes {
		pt := packageType
		if pt == "metrics" {
			pt = fmt.Sprintf("%s/%s", ds.moduleName, pt)
		}

		inputs = append(inputs, util.Input{
			Type:        pt,
			Title:       toDatasourceInputTitle(ds.moduleName, packageType),
			Description: toDatasourceInputDescription(ds.moduleTitle, packageType, ds.inputs[packageType].datasetNames),
		})
	}

	return []util.Datasource{
		{
			Name:        ds.moduleName,
			Title:       title,
			Description: description,
			Inputs:      inputs,
		},
	}
}

type updateDatasourcesParameters struct {
	moduleName  string
	moduleTitle string
	packageType string

	datasetNames []string
	inputVars    map[string][]util.Variable
}

func updateDatasource(dsc datasourceContent, params updateDatasourcesParameters) (datasourceContent, error) {
	dsc.moduleName = params.moduleName
	dsc.moduleTitle = params.moduleTitle

	if dsc.inputs == nil {
		dsc.inputs = map[string]datasourceInput{}
	}

	dsc.inputs[params.packageType] = datasourceInput{
		datasetNames: params.datasetNames,
		vars:         params.inputVars[params.packageType],
	}
	return dsc, nil
}

func toDatasourceTitle(moduleTitle, packageType string) string {
	return fmt.Sprintf("%s %s", moduleTitle, packageType)
}

func toDatasourceDescription(moduleTitle, packageType string) string {
	return fmt.Sprintf("Collect %s from %s instances", packageType, moduleTitle)
}

func toDatasourceTitleForTwoTypes(moduleTitle, firstPackageType, secondPackageType string) string {
	return fmt.Sprintf("%s %s and %s", moduleTitle, firstPackageType, secondPackageType)
}

func toDatasourceDescriptionForTwoTypes(moduleTitle, firstPackageType, secondPackageType string) string {
	return fmt.Sprintf("Collect %s and %s from %s instances", firstPackageType, secondPackageType, moduleTitle)
}

func toDatasourceInputTitle(moduleTitle, packageType string) string {
	return fmt.Sprintf("Collecting %s from %s instances", packageType, moduleTitle)
}

func toDatasourceInputDescription(moduleTitle, packageType string, datasets []string) string {
	firstPart := datasets[:len(datasets)-1]
	secondPart := datasets[len(datasets)-1:]

	var description strings.Builder
	description.WriteString("Collecting ")
	description.WriteString(moduleTitle)
	description.WriteString(" ")

	if len(firstPart) > 0 {
		fp := strings.Join(firstPart, ", ")
		description.WriteString(fp)
		description.WriteString(" and ")
	}

	description.WriteString(secondPart[0])

	description.WriteString(" ")
	description.WriteString(packageType)

	// Collecting MySQL status and galera_status metrics
	return description.String()
}

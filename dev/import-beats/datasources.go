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

	datasets map[string][]string // map[packageType]datasetName
}

type datasourceContentArray []datasourceContent

func (datasources datasourceContentArray) toMetadataDatasources() []util.Datasource {
	var ud []util.Datasource
	for _, ds := range datasources {
		// list package types
		var packageTypes []string
		for packageType := range ds.datasets {
			packageTypes = append(packageTypes, packageType)
		}
		sort.Strings(packageTypes)

		var title, description string
		if len(ds.datasets) == 2 {
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
				Description: toDatasourceInputDescription(ds.moduleTitle, packageType, ds.datasets[packageType]),
			})
		}

		ud = append(ud, util.Datasource{
			Name:        ds.moduleName,
			Title:       title,
			Description: description,
			Inputs:      inputs,
		})
	}
	return ud
}

func updateDatasources(datasources datasourceContentArray, moduleName, moduleTitle, packageType string, datasetNames []string) (datasourceContentArray, error) {
	var updated datasourceContentArray

	if len(datasources) > 0 {
		updated = append(updated, datasources...)

		if _, ok := updated[0].datasets[packageType]; !ok { // there is always a single datasource
			updated[0].datasets[packageType] = datasetNames
		} else {
			updated[0].datasets[packageType] = append(updated[0].datasets[packageType], datasetNames...)
		}
	} else {
		datasets := map[string][]string{
			packageType: datasetNames,
		}

		updated = append(updated, datasourceContent{
			moduleName:  moduleName,
			moduleTitle: moduleTitle,
			datasets:    datasets,
		})
	}
	return updated, nil
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

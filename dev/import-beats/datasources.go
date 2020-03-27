// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"fmt"
	"strings"

	"github.com/elastic/package-registry/util"
)

type datasourceContent struct {
	packageTypes []string

	moduleName string
}

type datasourceContentArray []datasourceContent

func (datasources datasourceContentArray) toMetadataDatasources() []util.Datasource {
	var ud []util.Datasource
	for _, ds := range datasources {
		var title, description string
		if len(ds.packageTypes) == 2 {
			title = toDatasourceTitleForTwoTypes(ds.moduleName, ds.packageTypes[0], ds.packageTypes[1])
			description = toDatasourceDescriptionForTwoTypes(ds.moduleName, ds.packageTypes[0], ds.packageTypes[1])
		} else {
			title = toDatasourceTitle(ds.moduleName, ds.packageTypes[0])
			description = toDatasourceDescription(ds.moduleName, ds.packageTypes[0])
		}

		var inputs []util.Input
		for _, packageType := range ds.packageTypes {
			pt := packageType
			if pt == "metrics" {
				pt = fmt.Sprintf("%s/%s", ds.moduleName, pt)
			}

			inputs = append(inputs, util.Input{
				Type:        pt,
				Description: toDatasourceInputDescription(ds.moduleName, packageType),
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

func updateDatasources(datasources datasourceContentArray, moduleName, packageType string) (datasourceContentArray, error) {
	var updated datasourceContentArray

	if len(datasources) > 0 {
		updated = append(updated, datasources...)
		updated[0].packageTypes = append(updated[0].packageTypes, packageType)
	} else {
		updated = append(updated, datasourceContent{
			packageTypes: []string{packageType},
			moduleName:   moduleName,
		})
	}
	return updated, nil
}

func toDatasourceTitle(moduleName, packageType string) string {
	return correctSpelling(fmt.Sprintf("%s %s", strings.Title(moduleName), packageType))
}

func toDatasourceDescription(moduleName, packageType string) string {
	return correctSpelling(fmt.Sprintf("Collect %s from %s instances", packageType, strings.Title(moduleName)))
}

func toDatasourceTitleForTwoTypes(moduleName, firstPackageType, secondPackageType string) string {
	return correctSpelling(fmt.Sprintf("%s %s and %s", strings.Title(moduleName), firstPackageType, secondPackageType))
}

func toDatasourceDescriptionForTwoTypes(moduleName, firstPackageType, secondPackageType string) string {
	return correctSpelling(fmt.Sprintf("Collect %s and %s from %s instances", firstPackageType, secondPackageType, strings.Title(moduleName)))
}

func toDatasourceInputDescription(moduleName, packageType string) string {
	return correctSpelling(fmt.Sprintf("Collecting %s %s", strings.Title(moduleName), packageType))
}

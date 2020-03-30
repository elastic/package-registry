// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"fmt"

	"github.com/elastic/package-registry/util"
)

type datasourceContent struct {
	packageTypes []string

	moduleName  string
	moduleTitle string
}

type datasourceContentArray []datasourceContent

func (datasources datasourceContentArray) toMetadataDatasources() []util.Datasource {
	var ud []util.Datasource
	for _, ds := range datasources {
		var title, description string
		if len(ds.packageTypes) == 2 {
			title = toDatasourceTitleForTwoTypes(ds.moduleTitle, ds.packageTypes[0], ds.packageTypes[1])
			description = toDatasourceDescriptionForTwoTypes(ds.moduleTitle, ds.packageTypes[0], ds.packageTypes[1])
		} else {
			title = toDatasourceTitle(ds.moduleTitle, ds.packageTypes[0])
			description = toDatasourceDescription(ds.moduleTitle, ds.packageTypes[0])
		}

		var inputs []util.Input
		for _, packageType := range ds.packageTypes {
			pt := packageType
			if pt == "metrics" {
				pt = fmt.Sprintf("%s/%s", ds.moduleName, pt)
			}

			inputs = append(inputs, util.Input{
				Type:        pt,
				Description: toDatasourceInputDescription(ds.moduleTitle, packageType),
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

func updateDatasources(datasources datasourceContentArray, moduleName, moduleTitle, packageType string) (datasourceContentArray, error) {
	var updated datasourceContentArray

	if len(datasources) > 0 {
		updated = append(updated, datasources...)
		updated[0].packageTypes = append(updated[0].packageTypes, packageType)
	} else {
		updated = append(updated, datasourceContent{
			packageTypes: []string{packageType},
			moduleName:   moduleName,
			moduleTitle:  moduleTitle,
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

func toDatasourceInputDescription(moduleTitle, packageType string) string {
	return fmt.Sprintf("Collecting %s %s", moduleTitle, packageType)
}

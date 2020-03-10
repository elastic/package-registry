// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"github.com/elastic/package-registry/util"
)

var ignoredModules = map[string]bool{"apache2": true}

type packageContent struct {
	manifest util.Package
	datasets map[string]datasetContent
	images   []imageContent
	kibana   kibanaContent
}

func newPackageContent(name string) packageContent {
	title := strings.Title(name)
	return packageContent{
		manifest: util.Package{
			FormatVersion: "1.0.0",
			Name:          name,
			Description:   strings.Title(name + " integration"),
			Title:         &title,
			Version:       "0.0.1", // TODO
			Type:          "integration",
			License:       "basic",
		},
		datasets: map[string]datasetContent{},
		kibana: kibanaContent{
			dashboardFiles:     map[string][]byte{},
			visualizationFiles: map[string][]byte{},
		},
	}
}

func (pc *packageContent) addDatasets(ds map[string]datasetContent) {
	for k, v := range ds {
		pc.datasets[k] = v
	}
}

func (pc *packageContent) addKibanaContent(kc kibanaContent) {
	if kc.dashboardFiles != nil {
		for k, v := range kc.dashboardFiles {
			pc.kibana.dashboardFiles[k] = v
		}
	}

	if kc.visualizationFiles != nil {
		for k, v := range kc.visualizationFiles {
			pc.kibana.visualizationFiles[k] = v
		}
	}
}

type packageRepository struct {
	kibanaMigrator *kibanaMigrator
	packages       map[string]packageContent
}

func newPackageRepository(kibanaMigrator *kibanaMigrator) *packageRepository {
	return &packageRepository{
		kibanaMigrator: kibanaMigrator,
		packages:       map[string]packageContent{},
	}
}

func (r *packageRepository) createPackagesFromSource(beatsDir, beatName, packageType string) error {
	beatPath := filepath.Join(beatsDir, beatName)
	beatModulesPath := filepath.Join(beatPath, "module")

	moduleDirs, err := ioutil.ReadDir(beatModulesPath)
	if err != nil {
		return errors.Wrapf(err, "cannot read directory '%s'", beatModulesPath)
	}

	for _, moduleDir := range moduleDirs {
		if !moduleDir.IsDir() {
			continue
		}
		moduleName := moduleDir.Name()

		log.Printf("%s %s: module found\n", beatName, moduleName)
		if _, ok := ignoredModules[moduleName]; ok {
			log.Printf("%s %s: module skipped\n", beatName, moduleName)
			continue
		}

		_, ok := r.packages[moduleName]
		if !ok {
			r.packages[moduleName] = newPackageContent(moduleName)
		}

		aPackage := r.packages[moduleName]
		manifest := aPackage.manifest
		manifest.Categories = append(manifest.Categories, packageType)

		// dataset
		modulePath := path.Join(beatModulesPath, moduleName)
		datasets, err := createDatasets(modulePath, moduleName)
		if err != nil {
			return err
		}
		aPackage.addDatasets(datasets)

		// img
		beatDocsPath := selectDocsPath(beatsDir, beatName)
		images, err := createImages(beatDocsPath, modulePath)
		if err != nil {
			return err
		}

		// img/screenshots
		aPackage.images = append(aPackage.images, images...)
		screenshots, err := createScreenshots(images)
		if err != nil {
			return err
		}
		manifest.Screenshots = append(manifest.Screenshots, screenshots...)

		// kibana
		kibana, err := createKibanaContent(r.kibanaMigrator, modulePath)
		if err != nil {
			return err
		}
		aPackage.addKibanaContent(kibana)

		aPackage.manifest = manifest
		r.packages[moduleDir.Name()] = aPackage
	}
	return nil
}

func (r *packageRepository) save(outputDir string) error {
	for packageName, content := range r.packages {
		manifest := content.manifest

		log.Printf("%s-%s write package content\n", packageName, manifest.Version)

		packagePath := filepath.Join(outputDir, packageName+"-"+manifest.Version)
		err := os.MkdirAll(packagePath, 0755)
		if err != nil {
			return errors.Wrapf(err, "cannot make directory for module: '%s'", packagePath)
		}

		m, err := yaml.Marshal(content.manifest)
		if err != nil {
			return errors.Wrapf(err, "marshaling package content failed (package packageName: %s)", packageName)
		}

		manifestFilePath := filepath.Join(packagePath, "manifest.yml")
		err = ioutil.WriteFile(manifestFilePath, m, 0644)
		if err != nil {
			return errors.Wrapf(err, "writing manifest file failed (path: %s)", manifestFilePath)
		}

		// dataset
		for datasetName, dataset := range content.datasets {
			datasetPath := filepath.Join(packagePath, "dataset", datasetName)
			err := os.MkdirAll(datasetPath, 0755)
			if err != nil {
				return errors.Wrapf(err, "cannot make directory for dataset: '%s'", datasetPath)
			}

			if len(dataset.fields.files) > 0 {
				datasetFieldsPath := filepath.Join(datasetPath, "fields")
				err := os.MkdirAll(datasetFieldsPath, 0755)
				if err != nil {
					return errors.Wrapf(err, "cannot make directory for dataset fields: '%s'", datasetPath)
				}

				for fieldsFileName, fieldsFile := range dataset.fields.files {
					log.Printf("\t%s: write '%s' file\n", datasetName, fieldsFileName)

					fieldsFilePath := filepath.Join(datasetFieldsPath, fieldsFileName)
					err = ioutil.WriteFile(fieldsFilePath, fieldsFile, 0644)
					if err != nil {
						return errors.Wrapf(err, "writing fields file failed (path: %s)", fieldsFilePath)
					}
				}
			}
		}

		// img
		imgDstDir := path.Join(packagePath, "img")
		for _, image := range content.images {
			log.Printf("\tcopy image file '%s' to '%s'", image.source, imgDstDir)
			err := copyFile(image.source, imgDstDir)
			if err != nil {
				return errors.Wrapf(err, "copying file failed")
			}
		}

		// kibana/dashboard
		if len(content.kibana.dashboardFiles) > 0 {
			dashboardPath := filepath.Join(packagePath, "dashboard")
			err := os.MkdirAll(dashboardPath, 0755)
			if err != nil {
				return errors.Wrapf(err, "cannot make directory for dashboard files: '%s'", dashboardPath)
			}

			for fileName, body := range content.kibana.dashboardFiles {
				dashboardFilePath := filepath.Join(dashboardPath, fileName)

				log.Printf("\tcreate dashboard file: %s", dashboardFilePath)
				err = ioutil.WriteFile(dashboardFilePath, body, 0644)
				if err != nil {
					return errors.Wrapf(err, "writing dashboard file failed (path: %s)", dashboardFilePath)
				}
			}
		}

		// kibana/visualization
		if len(content.kibana.visualizationFiles) > 0 {
			visualizationPath := filepath.Join(packagePath, "visualization")
			err := os.MkdirAll(visualizationPath, 0755)
			if err != nil {
				return errors.Wrapf(err, "cannot make directory for visualization files: '%s'",
					visualizationPath)
			}

			for fileName, body := range content.kibana.dashboardFiles {
				visualizationFilePath := filepath.Join(visualizationPath, fileName)

				log.Printf("\tcreate visualization file: %s", visualizationFilePath)
				err = ioutil.WriteFile(visualizationFilePath, body, 0644)
				if err != nil {
					return errors.Wrapf(err, "writing visualization file failed (path: %s)",
						visualizationFilePath)
				}
			}
		}
	}
	return nil
}

func copyFile(src, dstDir string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return errors.Wrapf(err, "opening file failed (src: %s)", src)
	}
	defer sourceFile.Close()

	i := strings.LastIndex(sourceFile.Name(), "/")
	sourceFileName := sourceFile.Name()[i:]

	dst := path.Join(dstDir, sourceFileName)
	err = os.MkdirAll(dstDir, 0755)
	if err != nil {
		return errors.Wrapf(err, "cannot make directory: '%s'", dst)
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return errors.Wrapf(err, "creating target file failed (dst: %s)", dst)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, sourceFile)
	if err != nil {
		return errors.Wrapf(err, "copying file failed (src: %s, dst: %s)", src, dst)
	}
	return nil
}

func selectDocsPath(beatsDir, beatName string) string {
	if strings.HasPrefix(beatName, "x-pack/") {
		return path.Join(beatsDir, beatName[7:], "docs")
	}
	return path.Join(beatsDir, beatName, "docs")
}

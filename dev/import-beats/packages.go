// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"fmt"
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
	manifest    util.Package
	datasets    datasetContentArray
	images      []imageContent
	kibana      kibanaContent
	docs        []docContent
	datasources datasourceContentArray
}

func newPackageContent(name string) packageContent {
	return packageContent{
		manifest: util.Package{
			FormatVersion: "1.0.0",
			Name:          name,
			Version:       "0.0.1", // TODO
			Type:          "integration",
			License:       "basic",
		},
		kibana: kibanaContent{
			files: map[string]map[string][]byte{},
		},
	}
}

func (pc *packageContent) addDatasets(ds []datasetContent) {
	for _, dc := range ds {
		for i, v := range pc.datasets {
			if v.name == dc.name {
				pc.datasets[i].name = fmt.Sprintf("%s-%s", pc.datasets[i].name, pc.datasets[i].beatType)
				dc.name = fmt.Sprintf("%s-%s", dc.name, dc.beatType)
				pc.datasets = append(pc.datasets, dc)
				break
			}
		}
		pc.datasets = append(pc.datasets, dc)
	}
}

func (pc *packageContent) addKibanaContent(kc kibanaContent) {
	if kc.files != nil {
		for objectType, objects := range kc.files {
			if _, ok := pc.kibana.files[objectType]; !ok {
				pc.kibana.files[objectType] = map[string][]byte{}
			}

			for k, v := range objects {
				pc.kibana.files[objectType][k] = v
			}
		}
	}
}

type packageRepository struct {
	iconRepository *iconRepository
	kibanaMigrator *kibanaMigrator

	packages map[string]packageContent
}

func newPackageRepository(iconRepository *iconRepository, kibanaMigrator *kibanaMigrator) *packageRepository {
	return &packageRepository{
		iconRepository: iconRepository,
		kibanaMigrator: kibanaMigrator,
		packages:       map[string]packageContent{},
	}
}

func (r *packageRepository) createPackagesFromSource(beatsDir, beatName, beatType string) error {
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
		modulePath := path.Join(beatModulesPath, moduleName)

		_, ok := r.packages[moduleName]
		if !ok {
			r.packages[moduleName] = newPackageContent(moduleName)
		}

		aPackage := r.packages[moduleName]
		manifest := aPackage.manifest
		manifest.Categories = append(manifest.Categories, beatType)

		// fields
		moduleHeaderFields, moduleFields, err := loadModuleFields(modulePath)
		if err != nil {
			return err
		}

		// release
		manifest.Release, err = determinePackageRelease(manifest.Release, moduleHeaderFields)
		if err != nil {
			return err
		}

		// title
		maybeTitle, err := loadTitleFromFields(moduleHeaderFields)
		if err != nil {
			return err
		}
		if maybeTitle != "" {
			manifest.Title = &maybeTitle
			manifest.Description = maybeTitle + " Integration"
		}

		// dataset
		var moduleTitle = "TODO"
		if manifest.Title != nil {
			moduleTitle = *manifest.Title
		}
		datasets, err := createDatasets(modulePath, moduleName, moduleTitle, manifest.Release, moduleFields, beatType)
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
		aPackage.images = append(aPackage.images, images...)

		// img/icons
		// The condition prevents from adding an icon multiple times (e.g. for metricbeat and filebeat).
		if len(manifest.Icons) == 0 {
			icons, err := createIcons(r.iconRepository, moduleName)
			if err != nil {
				return err
			}
			aPackage.images = append(aPackage.images, icons...)

			manifestIcons, err := icons.toManifestImages()
			if err != nil {
				return err
			}
			manifest.Icons = append(manifest.Icons, manifestIcons...)
		}

		// img/screenshots
		screenshots, err := images.toManifestImages()
		if err != nil {
			return err
		}
		manifest.Screenshots = append(manifest.Screenshots, screenshots...)

		// kibana
		kibana, err := createKibanaContent(r.kibanaMigrator, modulePath, moduleName, datasets.names())
		if err != nil {
			return err
		}
		aPackage.addKibanaContent(kibana)
		manifest.Requirement, err = createRequirement(aPackage.kibana, aPackage.datasets)
		if err != nil {
			return err
		}

		// docs
		if len(aPackage.docs) == 0 {
			packageDocsPath := filepath.Join("dev/import-beats-resources", moduleDir.Name(), "docs")
			docs, err := createDocTemplates(packageDocsPath)
			if err != nil {
				return err
			}
			aPackage.docs = append(aPackage.docs, docs...)
		}

		// datasources
		aPackage.datasources, err = updateDatasources(aPackage.datasources, moduleName, moduleTitle, beatType)
		if err != nil {
			return err
		}
		manifest.Datasources = aPackage.datasources.toMetadataDatasources()

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
			return errors.Wrapf(err, "marshaling package manifest failed (packageName: %s)", packageName)
		}

		manifestFilePath := filepath.Join(packagePath, "manifest.yml")
		err = ioutil.WriteFile(manifestFilePath, m, 0644)
		if err != nil {
			return errors.Wrapf(err, "writing manifest file failed (path: %s)", manifestFilePath)
		}

		// dataset
		for _, dataset := range content.datasets {
			datasetPath := filepath.Join(packagePath, "dataset", dataset.name)
			err := os.MkdirAll(datasetPath, 0755)
			if err != nil {
				return errors.Wrapf(err, "cannot make directory for dataset: '%s'", datasetPath)
			}

			// dataset/manifest.yml
			m, err := yaml.Marshal(dataset.manifest)
			if err != nil {
				return errors.Wrapf(err, "marshaling dataset manifest failed (datasetName: %s)", dataset.name)
			}

			manifestFilePath := filepath.Join(datasetPath, "manifest.yml")
			err = ioutil.WriteFile(manifestFilePath, m, 0644)
			if err != nil {
				return errors.Wrapf(err, "writing dataset manifest file failed (path: %s)", manifestFilePath)
			}

			// dataset/fields
			if len(dataset.fields.files) > 0 {
				datasetFieldsPath := filepath.Join(datasetPath, "fields")
				err := os.MkdirAll(datasetFieldsPath, 0755)
				if err != nil {
					return errors.Wrapf(err, "cannot make directory for dataset fields: '%s'", datasetPath)
				}

				for fieldsFileName, fieldsFile := range dataset.fields.files {
					log.Printf("\t%s: write '%s' file\n", dataset.name, fieldsFileName)

					fieldsFilePath := filepath.Join(datasetFieldsPath, fieldsFileName)
					err = ioutil.WriteFile(fieldsFilePath, fieldsFile, 0644)
					if err != nil {
						return errors.Wrapf(err, "writing fields file failed (path: %s)", fieldsFilePath)
					}
				}
			}

			// dataset/elasticsearch
			if len(dataset.elasticsearch.ingestPipelines) > 0 {
				ingestPipelinePath := filepath.Join(datasetPath, "elasticsearch", "ingest-pipeline")
				err := os.MkdirAll(ingestPipelinePath, 0755)
				if err != nil {
					return errors.Wrapf(err, "cannot make directory for dataset ingest pipelines: '%s'", ingestPipelinePath)
				}

				for _, ingestPipeline := range dataset.elasticsearch.ingestPipelines {
					log.Printf("\tcopy ingest pipeline file '%s' to '%s'", ingestPipeline.source, ingestPipelinePath)
					err := copyFileToTarget(ingestPipeline.source, ingestPipelinePath, ingestPipeline.targetFileName)
					if err != nil {
						return errors.Wrapf(err, "copying file failed")
					}
				}
			}

			// dataset/agent/stream
			if len(dataset.agent.streams) > 0 {
				agentStreamPath := filepath.Join(datasetPath, "agent", "stream")
				err := os.MkdirAll(agentStreamPath, 0755)
				if err != nil {
					return errors.Wrapf(err, "cannot make directory for dataset agent stream: '%s'", agentStreamPath)
				}

				for _, agentStream := range dataset.agent.streams {
					err := ioutil.WriteFile(path.Join(agentStreamPath, agentStream.targetFileName), agentStream.body, 0644)
					if err != nil {
						return errors.Wrapf(err, "writing agent stream file failed")
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

		// kibana
		if len(content.kibana.files) > 0 {
			kibanaPath := filepath.Join(packagePath, "kibana")

			for objectType, objects := range content.kibana.files {
				resourcePath := filepath.Join(kibanaPath, objectType)

				err := os.MkdirAll(resourcePath, 0755)
				if err != nil {
					return errors.Wrapf(err, "cannot make directory for dashboard files: '%s'", resourcePath)
				}

				for fileName, body := range objects {
					resourceFilePath := filepath.Join(resourcePath, fileName)

					log.Printf("\tcreate resource file: %s", resourceFilePath)
					err = ioutil.WriteFile(resourceFilePath, body, 0644)
					if err != nil {
						return errors.Wrapf(err, "writing resource file failed (path: %s)", resourceFilePath)
					}
				}
			}
		}

		// docs
		if len(content.docs) > 0 {
			docsPath := filepath.Join(packagePath, "docs")
			err := os.MkdirAll(docsPath, 0755)
			if err != nil {
				return errors.Wrapf(err, "cannot make directory for docs: '%s'", docsPath)
			}

			for _, doc := range content.docs {
				log.Printf("\twrite '%s' file\n", doc.fileName)

				docFilePath := filepath.Join(docsPath, doc.fileName)
				f, err := os.OpenFile(docFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
				if err != nil {
					return errors.Wrapf(err, "opening doc file failed (path: %s)", docFilePath)
				}
				err = doc.body.Execute(f, nil)
				if err != nil {
					return errors.Wrapf(err, "rendering doc file failed (path: %s)", docFilePath)
				}
			}
		}
	}
	return nil
}

func copyFile(src, dstDir string) error {
	i := strings.LastIndex(src, "/")
	sourceFileName := src[i:]

	return copyFileToTarget(src, dstDir, sourceFileName)
}

func copyFileToTarget(src, dstDir, targetFileName string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return errors.Wrapf(err, "opening file failed (src: %s)", src)
	}
	defer sourceFile.Close()

	dst := path.Join(dstDir, targetFileName)
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

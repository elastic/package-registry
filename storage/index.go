// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package storage

import (
	"cloud.google.com/go/storage"
	"context"
	"encoding/json"
	"github.com/elastic/package-registry/packages"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"io/ioutil"

	"github.com/elastic/package-registry/util"
)

type searchIndexAll struct {
	Packages []packageIndex `json:"packages"`
}

type packageIndex struct {
	PackageManifest     packageManifest      `json:"package"`
	DataStreamManifests []dataStreamManifest `json:"data_streams"`

	Assets []string `json:"assets"`
}

type packageManifest struct {
	FormatVersion string  `json:"format_version,omitempty"`
	Name          string  `json:"name,omitempty"`
	Title         string  `json:"title,omitempty"`
	Version       string  `json:"version,omitempty"`
	Release       string  `json:"release,omitempty"`
	License       string  `json:"license,omitempty"`
	Description   string  `json:"description,omitempty"`
	Type          string  `json:"type,omitempty"`
	Icons         []image `json:"icons,omitempty"`
	Screenshots   []image `json:"screenshots,omitempty"`
	Conditions    *struct {
		Kibana *struct {
			Version string `json:"version,omitempty"`
		} `json:"kibana,omitempty"`
	} `json:"conditions,omitempty"`
	Owner *struct {
		Github string `json:"github,omitempty"`
	} `json:"owner,omitempty"`
	Categories []string `json:"categories,omitempty"`

	PolicyTemplates []struct {
		Name        string   `json:"name,omitempty"`
		Title       string   `json:"title,omitempty"`
		Categories  []string `json:"categories,omitempty"`
		DataStreams []string `json:"data_streams,omitempty"`
		Description string   `json:"description,omitempty"`
		Icons       []image  `json:"icons,omitempty"`
		Input       struct {
			Title        string     `json:"title,omitempty"`
			Type         string     `json:"type,omitempty"`
			Description  string     `json:"description,omitempty"`
			InputGroup   string     `json:"input_group,omitempty"`
			TemplatePath string     `json:"template_path,omitempty"`
			Vars         []variable `json:"vars,omitempty"`
		} `json:"input,omitempty"`
		Screenshots []image    `json:"screenshots,omitempty"`
		Vars        []variable `json:"vars,omitempty"`
	} `json:"policy_templates,omitempty"`
}

type image struct {
	Src   string `json:"src,omitempty"`
	Title string `json:"title,omitempty"`
	Size  string `json:"size,omitempty"`
	Type  string `json:"type,omitempty"`
}

type variable struct {
	Name        string      `json:"name,omitempty"`
	Type        string      `json:"type,omitempty"`
	Title       string      `json:"title,omitempty"`
	Description string      `json:"description,omitempty"`
	Multi       bool        `json:"multi,omitempty"`
	Required    bool        `json:"required,omitempty"`
	ShowUser    bool        `json:"show_user,omitempty"`
	Default     interface{} `json:"default,omitempty"`
}

type dataStreamManifest struct {
	Title           string `json:"title,omitempty"`
	Type            string `type:"type,omitempty"`
	Dataset         string `json:"dataset,omitempty"`
	Hidden          bool   `json:"hidden,omitempty"`
	IlmPolicy       string `json:"ilm_policy,omitempty"`
	DatasetIsPrefix bool   `json:"dataset_is_prefix,omitempty"`
	Release         string `json:"release,omitempty"`
	Streams         []struct {
		Title        string     `json:"title,omitempty"`
		Description  string     `json:"description,omitempty"`
		Enabled      bool       `json:"enabled,omitempty"`
		Input        string     `json:"input,omitempty"`
		TemplatePath string     `json:"template_path,omitempty"`
		Vars         []variable `json:"vars,omitempty"`
	} `json:"streams,omitempty" `
	Elasticsearch *struct {
		IndexTemplate *struct {
			Settings map[string]interface{} `json:"settings,omitempty"`
			Mappings map[string]interface{} `json:"mappings,omitempty"`
		}
		IngestPipeline *struct {
			Name string `json:"name,omitempty"`
		} `json:"ingest_pipeline,omitempty"`
		Privileges *struct {
			Indices []string `json:"indices,omitempty"`
		} `json:"privileges,omitempty"`
	} `json:"elasticsearch,omitempty"`
}

func loadSearchIndexAll(ctx context.Context, storageClient *storage.Client, bucketName, rootStoragePath string, aCursor cursor) (*searchIndexAll, error) {
	indexFile := searchIndexAllFile

	logger := util.Logger()
	logger.Debug("load search-index-all index", zap.String("index.file", indexFile))

	content, err := loadIndexContent(ctx, storageClient, indexFile, bucketName, rootStoragePath, aCursor)
	if err != nil {
		return nil, errors.Wrap(err, "can't load search-index-all content")
	}

	var sia searchIndexAll
	if content == nil {
		return &sia, nil
	}

	err = json.Unmarshal(content, &sia)
	if err != nil {
		return nil, errors.Wrap(err, "can't unmarshal search-index-all")
	}
	return &sia, nil
}

func loadIndexContent(ctx context.Context, storageClient *storage.Client, indexFile, bucketName, rootStoragePath string, aCursor cursor) ([]byte, error) {
	logger := util.Logger()
	logger.Debug("load index content", zap.String("index.file", indexFile))

	rootedIndexStoragePath := buildIndexStoragePath(rootStoragePath, aCursor, indexFile)
	objectReader, err := storageClient.Bucket(bucketName).Object(rootedIndexStoragePath).NewReader(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "can't read the index file (path: %s)", rootedIndexStoragePath)
	}
	defer objectReader.Close()

	b, err := ioutil.ReadAll(objectReader)
	if err != nil {
		return nil, errors.Wrapf(err, "ioutil.ReadAll failed")
	}

	return b, nil
}

func buildIndexStoragePath(rootStoragePath string, aCursor cursor, indexFile string) string {
	return joinObjectPaths(rootStoragePath, v2MetadataStoragePath, aCursor.Current, indexFile)
}

func transformSearchIndexAllToPackages(sia searchIndexAll) packages.Packages {
	var transformedPackages packages.Packages

	return transformedPackages
}

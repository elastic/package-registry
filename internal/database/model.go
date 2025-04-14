// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package database

type Package struct {
	ID      int64
	Name    string
	Path    string
	Version string
	Indexer string
	Data    string
}

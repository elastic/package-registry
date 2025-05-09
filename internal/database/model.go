// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package database

type Package struct {
	ID      int64
	Name    string
	Version string
	Type    string
	Path    string
	Data    string
}

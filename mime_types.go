// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"log"
	"mime"
)

func init() {
	mustAddMimeExtenstionType(".gz", "application/gzip")
	mustAddMimeExtenstionType(".ico", "image/x-icon")
	mustAddMimeExtenstionType(".md", "text/markdown; charset=utf-8")
	mustAddMimeExtenstionType(".yml", "text/yaml; charset=UTF-8")
}

func mustAddMimeExtenstionType(ext, typ string) {
	err := mime.AddExtensionType(ext, typ)
	if err != nil {
		log.Fatal(err)
	}
}

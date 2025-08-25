// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"mime"
)

// init method defines MIME types important for the package content. Definitions ensure that the same Content-Type
// will be returned if the /etc/mime.types is empty or tiny.
func init() {
	mustAddMimeExtensionType(".zip", "application/zip")
	mustAddMimeExtensionType(".ico", "image/x-icon")
	mustAddMimeExtensionType(".md", "text/markdown; charset=utf-8")
	mustAddMimeExtensionType(".yml", "text/yaml; charset=UTF-8")
}

func mustAddMimeExtensionType(ext, typ string) {
	err := mime.AddExtensionType(ext, typ)
	if err != nil {
		panic(err.Error())
	}
}

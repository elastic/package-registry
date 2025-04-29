// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package packages

import (
	"net/http"
)

type RemoteResolver interface {
	ArtifactsHandler(w http.ResponseWriter, r *http.Request, p *Package)
	StaticHandler(w http.ResponseWriter, r *http.Request, p *Package, resourcePath string)
	SignaturesHandler(w http.ResponseWriter, r *http.Request, p *Package)
}

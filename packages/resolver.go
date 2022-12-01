// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import "net/http"

type RemoteResolver interface {
	ForwardArtifactsHandler(w http.ResponseWriter, r *http.Request, p *Package)
	ForwardStaticHandler(w http.ResponseWriter, r *http.Request, p *Package, resourcePath string)
	ForwardSignaturesHandler(w http.ResponseWriter, r *http.Request, p *Package)
}

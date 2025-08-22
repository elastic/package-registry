// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingestion_methods

import (
	"bytes"
	_ "embed"
)

//go:embed ingestion_methods.yml
var defaultIngestionMethodsFile []byte

// DefaultIngestionMethods loads the default ingestion methods from the embedded file
func DefaultIngestionMethods() IngestionMethods {
	return MustReadIngestionMethods(bytes.NewReader(defaultIngestionMethodsFile))
}

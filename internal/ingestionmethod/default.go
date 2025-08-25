// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package ingestionmethod

import (
	"bytes"
	_ "embed"
)

//go:embed ingestion_method.yml
var defaultIngestionMethodFile []byte

// DefaultIngestionMethod loads the default ingestion methods from the embedded file
func DefaultIngestionMethod() IngestionMethod {
	return MustReadIngestionMethod(bytes.NewReader(defaultIngestionMethodFile))
}

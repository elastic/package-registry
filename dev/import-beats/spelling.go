// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"strings"
)

var spellingReplacer = strings.NewReplacer(
	"mysql", "MySQL", "Mysql", "MySQL")

func correctSpelling(phrase string) string {
	return spellingReplacer.Replace(phrase)
}

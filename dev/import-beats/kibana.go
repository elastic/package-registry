// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

type kibanaContent struct {
	dashboardFiles     map[string][]byte
	visualizationFiles map[string][]byte
}

func createKibanaContent(modulePath string) (kibanaContent, error) {
	return kibanaContent{}, nil
}

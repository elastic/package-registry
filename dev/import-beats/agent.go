// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

type agentContent struct {
	streams []streamContent
}

type streamContent struct {
	targetFileName string
	body           []byte
}

func createAgentContent(modulePath, datasetName string) (agentContent, error) {
	return agentContent{}, nil // TODO
}

// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import "github.com/elastic/package-registry/util"

type streamConfigTreeNode struct {
	nodeType string
	value    string
	nodes    []streamConfigTreeNode
}

func parseStreamConfig(content []byte) (*streamConfigTreeNode, error) {
	return nil, nil // TODO
}

func (sctn *streamConfigTreeNode) inputTypes() []string {
	return nil // TODO
}

func (sctn *streamConfigTreeNode) configForInput(inputType string) []byte {
	return nil // TODO
}

func (sctn *streamConfigTreeNode) filterVarsForInput(inputType string, vars []util.Variable) []util.Variable {
	return nil // TODO
}

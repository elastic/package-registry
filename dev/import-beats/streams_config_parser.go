// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"bytes"
	"text/template/parse"

	"github.com/pkg/errors"

	"github.com/elastic/package-registry/util"
)

type streamConfigParsed struct {
	tree *parse.Tree
}

func parseStreamConfig(content []byte) (*streamConfigParsed, error) {
	mapOfParsed, err := parse.Parse("hello", string(content), "", "", map[string]interface{}{
		"eq": func() {},
		"printf": func() {},
	})
	if err != nil {
		return nil, errors.Wrapf(err, "parsing template failed")
	}
	return &streamConfigParsed{
		tree: mapOfParsed["hello"],
	}, nil
}

func (scp *streamConfigParsed) inputTypes() []string {
	return uniqueStringValues(inputTypesForNode(scp.tree.Root))
}

func inputTypesForNode(node parse.Node) []string {
	textNode, isTextNode := node.(*parse.TextNode)
	if isTextNode {
		i := bytes.Index(textNode.Text, []byte("type: "))
		if i > -1 {
			aType := textNode.Text[i + 6:]
			j := bytes.IndexByte(aType, '\n')
			aType = aType[:j]
			return []string{string(aType)}
		}
		return nil
	}

	var inputTypes []string
	listNode, isListNode := node.(*parse.ListNode)
	if isListNode {
		for _, listedNode := range listNode.Nodes {
			it := inputTypesForNode(listedNode)
			inputTypes = append(inputTypes, it...)
		}
	}
	return inputTypes
}

func (scp *streamConfigParsed) configForInput(inputType string) []byte {
	return []byte("TODO: TODO") // TODO
}

func (scp *streamConfigParsed) filterVarsForInput(inputType string, vars []util.Variable) []util.Variable {
	variableNamesForInput := scp.variableNamesForInput(inputType)

	if variableNamesForInput == nil { // TODO remove once above method is implemented
		return vars
	}

	var filtered []util.Variable
	for _, aVar := range vars {
		var found bool
		for _, variableName := range variableNamesForInput {
			if aVar.Name == variableName {
				found = true
				break
			}
		}

		if found {
			filtered = append(filtered, aVar)
		}
	}
	return filtered
}

func (scp *streamConfigParsed) variableNamesForInput(inputType string) []string {
	return nil // TODO
}

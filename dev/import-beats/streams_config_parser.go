// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"bytes"
	"fmt"
	"text/template/parse"

	"github.com/pkg/errors"

	"github.com/elastic/package-registry/util"
)

type streamConfigParsed struct {
	tree *parse.Tree
}

func parseStreamConfig(content []byte) (*streamConfigParsed, error) {
	mapOfParsed, err := parse.Parse("hello", string(content), "", "", map[string]interface{}{
		"eq":     func() {},
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
		inputType, ok := extractInputTypeFromTextNode(textNode)
		if ok {
			return []string{inputType}
		}
		return nil
	}

	listNode, isListNode := node.(*parse.ListNode)
	if isListNode {
		return inputTypesForListNode(listNode)
	}

	ifNode, isIfNode := node.(*parse.IfNode)
	if isIfNode {
		var inputTypes []string

		if ifNode.List != nil {
			inputTypes = append(inputTypes, inputTypesForListNode(ifNode.List)...)
		}
		if ifNode.ElseList != nil {
			inputTypes = append(inputTypes, inputTypesForListNode(ifNode.ElseList)...)
		}
		return inputTypes
	}
	return nil
}

func extractInputTypeFromTextNode(textNode *parse.TextNode) (string, bool) {
	i := bytes.Index(textNode.Text, []byte("type: "))
	if i > -1 && (i == 0 || textNode.Text[i-1] == ' ' || textNode.Text[i-1] == '\n') {
		aType := textNode.Text[i+6:]
		j := bytes.IndexByte(aType, '\n')
		if j < 0 {
			j = len(textNode.Text)
		}
		aType = aType[:j]
		return string(aType), true
	}
	return "", false
}

func inputTypesForListNode(listNode *parse.ListNode) []string {
	var inputTypes []string
	for _, listedNode := range listNode.Nodes {
		it := inputTypesForNode(listedNode)
		inputTypes = append(inputTypes, it...)
	}
	return inputTypes
}

func (scp *streamConfigParsed) configForInput(inputType string) []byte {
	if inputType == "log" {
		inputType = "file"
	}
	return configForInputForNode(inputType, scp.tree.Root)
}

func configForInputForNode(inputType string, node parse.Node) []byte {
	var buffer bytes.Buffer

	textNode, isTextNode := node.(*parse.TextNode)
	if isTextNode {
		buffer.Write(textNode.Text)
		return buffer.Bytes()
	}

	listNode, isListNode := node.(*parse.ListNode)
	if isListNode {
		for _, listedNode := range listNode.Nodes {
			buf := configForInputForNode(inputType, listedNode)
			buffer.Write(buf)
		}
		return buffer.Bytes()
	}

	ifNode, isIfNode := node.(*parse.IfNode)
	if isIfNode {
		if isIfNodeEqInput(ifNode) {
			if isIfNodeEqInputInputType(ifNode, inputType) {
				if ifNode.List != nil {
					buffer.Write(configForInputForNode(inputType, ifNode.List))
				}
			} else {
				if ifNode.ElseList != nil {
					buffer.Write(configForInputForNode(inputType, ifNode.ElseList))
				}
			}
		} else {
			buffer.WriteString("<if>")
			if ifNode.List != nil {
				buffer.Write(configForInputForNode(inputType, ifNode.List))
			}
			buffer.WriteString("<else>")
			if ifNode.ElseList != nil {
				buffer.Write(configForInputForNode(inputType, ifNode.ElseList))
			}
			buffer.WriteString("</fi>")
		}
	}
	return buffer.Bytes()
}

func isIfNodeEqInput(ifNode *parse.IfNode) bool {
	if len(ifNode.Pipe.Cmds) > 0 {
		if len(ifNode.Pipe.Cmds[0].Args) > 1 {
			op := ifNode.Pipe.Cmds[0].Args[0].String()
			var1 := ifNode.Pipe.Cmds[0].Args[1].String()

			if op == "eq" && var1 == ".input" {
				return true
			}
		}
	}
	return false
}

func isIfNodeEqInputInputType(ifNode *parse.IfNode, inputType string) bool {
	if len(ifNode.Pipe.Cmds) > 0 {
		if len(ifNode.Pipe.Cmds[0].Args) > 1 {
			op := ifNode.Pipe.Cmds[0].Args[0].String()
			var1 := ifNode.Pipe.Cmds[0].Args[1].String()
			var2 := ifNode.Pipe.Cmds[0].Args[2].String()

			if op == "eq" && var1 == ".input" && var2 == fmt.Sprintf(`"%s"`, inputType) {
				return true
			}
		}
	}
	return false
}



func shouldIfNodeBeWritten(inputType string, ifNode *parse.IfNode) bool {
	if len(ifNode.Pipe.Cmds) > 0 {
		if len(ifNode.Pipe.Cmds[0].Args) > 1 {
			op := ifNode.Pipe.Cmds[0].Args[0].String()
			var1 := ifNode.Pipe.Cmds[0].Args[1].String()
			var2 := ifNode.Pipe.Cmds[0].Args[2].String()

			if op == "eq" && var1 == ".input" && var2 != fmt.Sprintf(`"%s"`, inputType) {
				return false
			}
		}
	}
	return true
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

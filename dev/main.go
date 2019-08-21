// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v2"
)

var packagesPath = "../public/package/"

type Package struct {
	Name        string `yaml:"name"`
	Title       string `yaml:"title,omitempty"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
}

func main() {
	fmt.Println("Creating integration packages")

	err := process()
	if err != nil {
		log.Fatal(err)
	}
}

func process() error {

	packages := make([]Package, 0)
	yamlFile, err := ioutil.ReadFile("packages.yml")
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(yamlFile, &packages)
	if err != nil {
		return err
	}

	for _, p := range packages {
		err := createPackage(p)
		if err != nil {
			return err
		}
	}

	return nil
}

func createPackage(p Package) error {
	fmt.Println(p)

	files, err := filepath.Glob("package-template/*")
	if err != nil {
		return err
	}
	moreFiles, err := filepath.Glob("package-template/*/*")
	if err != nil {
		return err
	}
	files = append(files, moreFiles...)

	moreFiles, err = filepath.Glob("package-template/*/*/*")
	if err != nil {
		return err
	}
	files = append(files, moreFiles...)

	for _, f := range files {
		fileInfo, err := os.Stat(f)
		if err != nil {
			return err
		}

		// Skip all directories
		if fileInfo.IsDir() {
			continue
		}

		// Skip hidden files
		if fileInfo.Name() == ".DS_Store" {
			continue
		}

		data, err := ioutil.ReadFile(f)
		if err != nil {
			return err
		}

		tmpl, err := template.New("package").Parse(string(data))
		if err != nil {
			return err
		}

		// Create directory and all files needed
		newFileName := strings.Replace(f, "package-template", p.Name+"-"+p.Version, -1)

		path := filepath.Dir(packagesPath + newFileName)
		os.MkdirAll(path, 0777)
		f, err := os.Create(packagesPath + newFileName)
		if err != nil {
			return err
		}
		defer f.Close()

		writer := bufio.NewWriter(f)

		err = tmpl.Execute(writer, p)
		if err != nil {
			return err
		}

		err = writer.Flush()
		if err != nil {
			return err
		}

		// Check if icon exists, if yes, copy over
		if _, err := os.Stat("./icons/" + p.Name + ".png"); err == nil {
			input, err := ioutil.ReadFile("./icons/" + p.Name + ".png")
			if err != nil {
				return err
			}

			err = os.MkdirAll(packagesPath+p.Name+"-"+p.Version+"/img", 0777)
			if err != nil {
				return err
			}
			err = ioutil.WriteFile(packagesPath+p.Name+"-"+p.Version+"/img/icon.png", input, 0644)
			if err != nil {
				return err
			}
		}

	}

	return nil
}

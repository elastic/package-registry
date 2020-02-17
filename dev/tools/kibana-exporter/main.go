package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"

	"github.com/elastic/beats/libbeat/common"
)

func main() {

	targetDir := flag.String("target", "", "Path to target dir, normally package dir")
	inputFile := flag.String("input", "", "Input file with the assets")
	flag.Parse()

	fmt.Println("Target dir: " + *targetDir)
	fmt.Println("Input file: " + *inputFile)

	yamlFile, err := ioutil.ReadFile(*inputFile)
	if err != nil {
		log.Fatalf("Error reading input file: #%v ", err)
	}

	export := &Export{}
	err = yaml.Unmarshal(yamlFile, export)
	if err != nil {
		log.Fatalf("Error converting yaml file: %v", err)
	}

	for _, a := range export.Assets {
		err := exportAsset(*targetDir, a)
		if err != nil {
			// Often if an asset does not exist
			fmt.Println("error exporting asset: " + err.Error())
		}
	}
}

type Export struct {
	Package string
	Assets  []Asset
}
type Asset struct {
	ID      string
	Type    string
	Service string
	Dataset string
	Name    string
}

func (a Asset) getPath() string {
	return "kibana/" + a.Type + "/" + a.ID + ".json"
}

func exportAsset(targetDir string, a Asset) error {
	if a.Name == "" {
		a.Name = a.ID
	}

	k, _ := New()
	data, err := k.GetObject(a.Type, a.ID)
	if err != nil {
		return err
	}

	mapStr := common.MapStr{}
	err = json.Unmarshal(data, &mapStr)
	if err != nil {
		return err
	}

	// Decoding trick, to make sure we have the same objects
	out := DecodeInner(mapStr)

	// Delete not needed attributes
	out.Delete("id")
	out.Delete("type")
	out.Delete("updated_at")
	out.Delete("version")

	path := targetDir + "/" + a.getPath()
	os.MkdirAll(filepath.Dir(path), 0755)
	return ioutil.WriteFile(path, []byte(out.StringToPrint()), 0644)
}

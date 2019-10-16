package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"

	"github.com/elastic/integrations-registry/util"
)

// BEAT_CATEGORIES maps beats to integration categories
var BEAT_CATEGORIES = map[string]string{
	"metricbeat":        "metrics",
	"x-pack/metricbeat": "metrics",
	"filebeat":          "logs",
	"x-pack/filebeat":   "logs",
}

var IGNORE = map[string]interface{}{
	"apache2": nil, // was renamed to apache
}

func main() {
	// Beats repo directory
	var beatsDir string
	// Target public directory where the generated packages should end up in
	var publicDir string

	flag.StringVar(&beatsDir, "beatsDir", "", "Path to the beats repository")
	flag.StringVar(&publicDir, "publicDir", "", "Path to the public directory ")
	flag.Parse()

	if beatsDir == "" || publicDir == "" {
		log.Fatal("beatsDir and publicDir must be set")
	}

	integrations := NewIntegrations()
	for beat, category := range BEAT_CATEGORIES {
		dirs, err := ioutil.ReadDir(filepath.Join(beatsDir, beat, "module"))
		if err != nil {
			log.Fatal(err)
		}

		for _, module := range dirs {
			if !module.IsDir() {
				continue
			}

			if _, ok := IGNORE[module.Name()]; ok {
				continue
			}

			integrations.AddBeatModule(category, filepath.Join(beatsDir, beat, "module", module.Name()))
		}
	}

	integrations.Write(publicDir)
}

type Integration struct {
	util.Package
	Path string
}

type Integrations struct {
	list map[string]*Integration
}

// NewIntegrations returns an new empty list of integrations
func NewIntegrations() *Integrations {
	return &Integrations{
		list: make(map[string]*Integration, 0),
	}
}

// AddBeatModule reads info from the module folder into the integrations structure
func (i *Integrations) AddBeatModule(category, path string) {
	meta := readMeta(path)
	if meta == nil {
		return
	}

	integration, ok := i.list[meta.Key]
	if !ok {
		integration = &Integration{
			Package: util.Package{
				Name: meta.Key,
				// TODO use stack version?
				Version: "1.0.0",
				Title:   &meta.Title,
				Requirement: util.Requirement{
					Kibana: util.Kibana{
						Min: "6.7.0",
						// TODO do we really require a max version?
						Max: "7.6.0",
					},
				},
			},
			Path: path,
		}
		i.list[meta.Key] = integration
	}

	integration.Categories = append(integration.Categories, category)

	// TODO come up with a general description
	integration.Description = meta.Description
}

// Write integration files & manifests to the given destination folder
func (i *Integrations) Write(destination string) {
	log.Println("Writing integration manifests and files to " + destination)

	for _, integration := range i.list {
		path := filepath.Join(destination, integration.Name+"-"+integration.Version)
		err := os.MkdirAll(path, 0755)
		if err != nil {
			log.Fatal("Could not create integration folder: ", err)
		}

		// Write manifest.yml
		data, err := yaml.Marshal(integration.Package)
		if err != nil {
			log.Fatal("Could not marshal integration manifest.yaml: ", err)
		}

		err = ioutil.WriteFile(filepath.Join(path, "manifest.yml"), data, 0644)
		if err != nil {
			log.Fatal("Could not write integration manifest.yaml: ", err)
		}

		// Copy assets

		// dashboards
		srcDashboardPath := filepath.Join(integration.Path, "_meta/kibana/7/dashboard")
		fmt.Println(srcDashboardPath)
		srcDashboards, err := ioutil.ReadDir(srcDashboardPath)
		if err != nil && !os.IsNotExist(err) {
			log.Fatal(err)
		}

		if len(srcDashboards) > 0 {
			dstDashboardPath := filepath.Join(path, "kibana", "dashboard")
			err = os.MkdirAll(dstDashboardPath, 0755)
			if err != nil {
				log.Fatal(err)
			}

			for _, dashboard := range srcDashboards {

				fmt.Println(
					filepath.Join(srcDashboardPath, dashboard.Name()),
					filepath.Join(dstDashboardPath, dashboard.Name()))
				copy(
					filepath.Join(srcDashboardPath, dashboard.Name()),
					filepath.Join(dstDashboardPath, dashboard.Name()),
				)
			}
		}
	}
}

func readMeta(path string) *fieldsYML {
	file := filepath.Join(path, "_meta", "fields.yml")
	data, err := ioutil.ReadFile(file)
	if err != nil {
		log.Println("Error reading "+file+", ignoring it: ", err)
		return nil
	}

	res := []fieldsYML{}
	err = yaml.Unmarshal(data, &res)
	if err != nil {
		log.Fatal(err)
	}

	if len(res) < 1 {
		log.Fatal("Wrong fields.yml:", file)
	}

	return &res[0]
}

type fieldsYML struct {
	Key, Title, Description, Release string
}

func copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

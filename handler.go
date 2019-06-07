package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	file := vars["name"]

	path := packagesPath + "/" + file + ".zip"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Println(err)
		http.NotFound(w, r)
		return
	}

	d, err := ioutil.ReadFile(path)
	if err != nil {
		log.Println(err)
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Description", "File Transfer")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+file+".zip\"")
	w.Header().Set("Content-Transfer-Encoding", "binary")

	fmt.Fprint(w, string(d))
}

func infoHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"version": "%s"}`, version)
	}
}

func packageHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		vars := mux.Vars(r)
		key := vars["name"]

		manifest, err := readManifest(key)
		if err != nil {
			log.Printf("Manifest not found: %s, %s", key, manifest)
			http.NotFound(w, r)
			return
		}
		// It's not set by default, generate it
		manifest.Icon = manifest.getIcon()

		data, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			log.Fatal(data)
		}

		fmt.Fprint(w, string(data))
	}
}

func imgHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	integration := vars["name"]
	file := vars["file"]

	img, err := readImage(integration, file)
	if err != nil {
		http.Error(w, "integration "+integration+" not found", 404)
		return
	}

	// Package exists but does not have an icon, so the default icon is shipped
	if img == nil {
		if file == "icon.png" {
			img, err = ioutil.ReadFile("./img/icon.png")
			if err != nil {
				http.NotFound(w, r)
				return
			}
		} else {
			http.NotFound(w, r)
			return
		}
	}

	// Safety check for too short paths
	if len(file) < 3 {
		http.NotFound(w, r)
		return
	}

	suffix := file[len(file)-3:]

	// Only .png and .jpg are supported at the moment
	if suffix == "png" {
		w.Header().Set("Content-Type", "image/png")
	} else if suffix == "jpg" {
		w.Header().Set("Content-Type", "image/jpeg")
	} else {
		http.NotFound(w, r)
		return
	}

	fmt.Fprint(w, string(img))
}

func listHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		integrations, err := getIntegrationPackages()
		if err != nil {
			http.NotFound(w, r)
			return
		}

		var output []map[string]string
		for _, i := range integrations {
			m, err := readManifest(i)
			if err != nil {
				http.NotFound(w, r)
				return
			}

			data := map[string]string{
				"name":        m.Name,
				"description": m.Description,
				"version":     m.Version,
				"icon":        m.getIcon(),
			}
			output = append(output, data)
		}
		j, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, string(j))
	}
}

package main

import (
	"log"
	"mime"
)

func init() {
	mustAddMimeExtenstionType(".gz", "application/gzip")
	mustAddMimeExtenstionType(".ico", "image/x-icon")
	mustAddMimeExtenstionType(".md", "text/markdown; charset=utf-8")
	mustAddMimeExtenstionType(".yml", "text/yaml; charset=UTF-8")
}

func mustAddMimeExtenstionType(ext, typ string) {
	err := mime.AddExtensionType(ext, typ)
	if err != nil {
		log.Fatal(err)
	}
}

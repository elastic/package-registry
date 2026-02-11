// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import "fmt"

func Example_config_urls() {
	config := config{
		Address: "http://localhost:8080",
		Queries: []configQuery{
			{},
			{Prerelease: true},
			{KibanaVersion: "7.19.0"},
			{KibanaVersion: "7.19.0", Prerelease: true},
			{SpecMin: "2.3", SpecMax: "3.0"},
		},
	}

	urls, err := config.searchURLs()
	if err != nil {
		panic(err)
	}

	for u := range urls {
		fmt.Println(u)
	}

	// Output:
	// http://localhost:8080/search
	// http://localhost:8080/search?prerelease=true
	// http://localhost:8080/search?kibana.version=7.19.0
	// http://localhost:8080/search?kibana.version=7.19.0&prerelease=true
	// http://localhost:8080/search?spec.max=3.0&spec.min=2.3
}

func Example_config_urls_matrix() {
	config := config{
		Address: "http://localhost:8080",
		Matrix: []configQuery{
			{},
			{Prerelease: true},
		},
		Queries: []configQuery{
			{},
			{KibanaVersion: "7.19.0"},
			{SpecMin: "2.3", SpecMax: "3.0"},
		},
	}

	urls, err := config.searchURLs()
	if err != nil {
		panic(err)
	}

	for u := range urls {
		fmt.Println(u)
	}

	// Output:
	// http://localhost:8080/search
	// http://localhost:8080/search?kibana.version=7.19.0
	// http://localhost:8080/search?spec.max=3.0&spec.min=2.3
	// http://localhost:8080/search?prerelease=true
	// http://localhost:8080/search?kibana.version=7.19.0&prerelease=true
	// http://localhost:8080/search?prerelease=true&spec.max=3.0&spec.min=2.3
}

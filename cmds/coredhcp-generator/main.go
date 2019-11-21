// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package main

import (
	"flag"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
)

const (
	defaultTemplateFile = "coredhcp.go.template"
)

var (
	flagTemplate = flag.String("template", defaultTemplateFile, "Template file name")
	flagOutfile  = flag.String("outfile", "", "Output file path")
)

func main() {
	flag.Parse()
	data, err := ioutil.ReadFile(*flagTemplate)
	if err != nil {
		log.Fatalf("Failed to read template file '%s': %v", *flagTemplate, err)
	}
	t, err := template.New("coredhcp").Parse(string(data))
	if err != nil {
		log.Fatalf("Template parsing failed: %v", err)
	}
	var plugins []string
	for _, pl := range flag.Args() {
		pl := strings.TrimSpace(pl)
		if pl == "" {
			continue
		}
		plugins = append(plugins, pl)
	}
	if len(plugins) == 0 {
		log.Fatalf("No plugin specified!")
	}
	outfile := *flagOutfile
	if outfile == "" {
		tmpdir, err := ioutil.TempDir("", "coredhcp")
		if err != nil {
			log.Fatalf("Cannot create temporary directory: %v", err)
		}
		outfile = path.Join(tmpdir, "coredhcp.go")
	}
	log.Printf("Generating output file '%s' with %d plugins", outfile, len(plugins))
	outFD, err := os.OpenFile(outfile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to create output file '%s': %v", outfile, err)
	}
	defer func() {
		if err := outFD.Close(); err != nil {
			log.Printf("Error while closing file descriptor for '%s': %v", outfile, err)
		}
	}()
	if err := t.Execute(outFD, plugins); err != nil {
		log.Fatalf("Template execution failed: %v", err)
	}
	log.Printf("Generated file '%s'. You can build it by running 'go build' in the output directory.", outfile)
}

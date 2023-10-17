// Copyright 2018-present the CoreDHCP Authors. All rights reserved
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package main

import (
	"bufio"
	"fmt"
	"html/template"
	"log"
	"os"
	"path"
	"sort"
	"strings"

	flag "github.com/spf13/pflag"
)

const (
	defaultTemplateFile = "coredhcp.go.template"
	importBase          = "github.com/coredhcp/coredhcp/"
)

var (
	flagTemplate = flag.StringP("template", "t", defaultTemplateFile, "Template file name")
	flagOutfile  = flag.StringP("outfile", "o", "", "Output file path")
	flagFromFile = flag.StringP("from", "f", "", "Optional file name to get the plugin list from, one import path per line")
)

var funcMap = template.FuncMap{
	"importname": func(importPath string) (string, error) {
		parts := strings.Split(importPath, "/")
		if len(parts) < 1 {
			return "", fmt.Errorf("no components found in import path '%s'", importPath)
		}
		return "pl_" + parts[len(parts)-1], nil
	},
}

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(),
		"%s [-template tpl] [-outfile out] [-from pluginlist] [plugin [plugin...]]\n",
		os.Args[0],
	)
	flag.PrintDefaults()
	fmt.Fprintln(flag.CommandLine.Output(), `  plugin
	Plugin name to include, as go import path.
	Short names can be used for builtin coredhcp plugins (eg "serverid")`)
}

func main() {
	flag.Usage = usage
	flag.Parse()

	data, err := os.ReadFile(*flagTemplate)
	if err != nil {
		log.Fatalf("Failed to read template file '%s': %v", *flagTemplate, err)
	}
	t, err := template.New("coredhcp").Funcs(funcMap).Parse(string(data))
	if err != nil {
		log.Fatalf("Template parsing failed: %v", err)
	}
	plugins := make(map[string]bool)
	for _, pl := range flag.Args() {
		pl := strings.TrimSpace(pl)
		if pl == "" {
			continue
		}
		if !strings.ContainsRune(pl, '/') {
			// A bare name was specified, not a full import path.
			// Coredhcp plugins aren't in the standard library, and it's unlikely someone
			// would put them at the base of $GOPATH/src.
			// Assume this is one of the builtin plugins. If needed, use the -from option
			// which always requires (and uses) exact paths

			// XXX: we could also look into github.com/coredhcp/plugins
			pl = importBase + pl
		}
		plugins[pl] = true
	}
	if *flagFromFile != "" {
		// additional plugin names from a text file, one line per plugin import
		// path
		fd, err := os.Open(*flagFromFile)
		if err != nil {
			log.Fatalf("Failed to read file '%s': %v", *flagFromFile, err)
		}
		defer func() {
			if err := fd.Close(); err != nil {
				log.Printf("Error closing file '%s': %v", *flagFromFile, err)
			}
		}()
		sc := bufio.NewScanner(fd)
		for sc.Scan() {
			pl := strings.TrimSpace(sc.Text())
			if pl == "" {
				continue
			}
			plugins[pl] = true
		}
		if err := sc.Err(); err != nil {
			log.Fatalf("Error reading file '%s': %v", *flagFromFile, err)
		}
	}
	if len(plugins) == 0 {
		log.Fatalf("No plugin specified!")
	}
	outfile := *flagOutfile
	if outfile == "" {
		tmpdir, err := os.MkdirTemp("", "coredhcp")
		if err != nil {
			log.Fatalf("Cannot create temporary directory: %v", err)
		}
		outfile = path.Join(tmpdir, "coredhcp.go")
	}

	log.Printf("Generating output file '%s' with %d plugin(s):", outfile, len(plugins))
	idx := 1
	for pl := range plugins {
		log.Printf("% 3d) %s", idx, pl)
		idx++
	}
	outFD, err := os.OpenFile(outfile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to create output file '%s': %v", outfile, err)
	}
	defer func() {
		if err := outFD.Close(); err != nil {
			log.Printf("Error while closing file descriptor for '%s': %v", outfile, err)
		}
	}()
	// WARNING: no escaping of the provided strings is done
	pluginList := make([]string, 0, len(plugins))
	for pl := range plugins {
		pluginList = append(pluginList, pl)
	}
	sort.Strings(pluginList)
	if err := t.Execute(outFD, pluginList); err != nil {
		log.Fatalf("Template execution failed: %v", err)
	}
	log.Printf("Generated file '%s'. You can build it by running 'go build' in the output directory.", outfile)
	fmt.Println(path.Dir(outfile))
}

// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

// The openapi-contract-check command verifies that HTTP paths used by the
// Terraform provider still exist in the LangSmith OpenAPI document.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"

	"github.com/bogware/terraform-provider-langsmith/internal/openapicheck"
)

func main() {
	openapiFlag := flag.String("openapi", "", "OpenAPI JSON/YAML file path or https URL (default: LANGSMITH_OPENAPI_URL or the public LangSmith spec)")
	dir := flag.String("dir", ".", "module root directory used for package loading")
	flag.Parse()

	src := strings.TrimSpace(*openapiFlag)
	if src == "" {
		src = strings.TrimSpace(os.Getenv("LANGSMITH_OPENAPI_URL"))
	}
	openapicheck.SetOpenAPISource(src)

	cfg := &packages.Config{
		Mode:  packages.LoadSyntax | packages.NeedModule,
		Dir:   *dir,
		Tests: true,
	}
	pkgs, err := packages.Load(cfg, "./internal/provider")
	if err != nil {
		fmt.Fprintf(os.Stderr, "openapi-contract-check: load packages: %v\n", err)
		os.Exit(1)
	}
	var loadErrs []string
	for _, p := range pkgs {
		for _, e := range p.Errors {
			loadErrs = append(loadErrs, e.Error())
		}
	}
	if len(loadErrs) > 0 {
		fmt.Fprintf(os.Stderr, "openapi-contract-check: package errors:\n%s\n", strings.Join(loadErrs, "\n"))
		os.Exit(1)
	}

	graph, err := checker.Analyze([]*analysis.Analyzer{openapicheck.Analyzer}, pkgs, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "openapi-contract-check: %v\n", err)
		os.Exit(1)
	}

	exit := 0
	for _, root := range graph.Roots {
		if root.Err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", root.Err)
			exit = 1
			continue
		}
		for _, d := range root.Diagnostics {
			pos := root.Package.Fset.Position(d.Pos)
			fmt.Fprintf(os.Stderr, "%s: %s\n", pos.String(), d.Message)
			exit = 1
		}
	}
	if exit != 0 {
		fmt.Fprintf(os.Stderr, "\nOpenAPI contract check failed. Paths come from internal/provider; "+
			"the spec was loaded from %q.\n", openapicheck.ResolvedOpenAPISource())
		os.Exit(1)
	}
}

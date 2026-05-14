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

	"github.com/bogware/terraform-provider-langsmith/internal/openapicheck"
)

func main() {
	openapiSrc := flag.String("openapi", "", "OpenAPI JSON file path or https URL (default: LANGSMITH_OPENAPI_URL or the public LangSmith spec)")
	dir := flag.String("dir", ".", "module root directory used for package loading")
	flag.Parse()

	src := strings.TrimSpace(*openapiSrc)
	if src == "" {
		src = strings.TrimSpace(os.Getenv("LANGSMITH_OPENAPI_URL"))
	}
	if src == "" {
		src = "https://api.smith.langchain.com/openapi.json"
	}

	idx, err := openapicheck.LoadSpecFromFileOrURL(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "openapi-contract-check: load spec: %v\n", err)
		os.Exit(1)
	}

	usages, unresolved, err := openapicheck.Extract(*dir, []string{"./internal/provider"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "openapi-contract-check: extract usages: %v\n", err)
		os.Exit(1)
	}

	exit := 0
	for _, u := range unresolved {
		fmt.Fprintf(os.Stderr, "%s: %s\n", u.Position.String(), u.Reason)
		exit = 1
	}
	for _, e := range openapicheck.ValidateUsages(idx, usages) {
		fmt.Fprintf(os.Stderr, "%v\n", e)
		exit = 1
	}
	if exit != 0 {
		fmt.Fprintf(os.Stderr, "\nOpenAPI contract check failed. Paths come from internal/provider; "+
			"the spec was loaded from %q.\n", src)
		os.Exit(1)
	}
}

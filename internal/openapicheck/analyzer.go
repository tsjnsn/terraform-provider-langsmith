// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package openapicheck

import (
	"os"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"
)

const defaultOpenAPISpecURL = "https://api.smith.langchain.com/openapi.json"

var configuredOpenAPISrc string

// SetOpenAPISource configures the OpenAPI document location before running [Analyzer].
// Empty string falls back to LANGSMITH_OPENAPI_URL, then the public LangSmith spec URL.
func SetOpenAPISource(src string) {
	configuredOpenAPISrc = strings.TrimSpace(src)
}

// ResolvedOpenAPISource returns the effective OpenAPI URL/path after defaults.
func ResolvedOpenAPISource() string {
	src := configuredOpenAPISrc
	if src == "" {
		src = strings.TrimSpace(os.Getenv("LANGSMITH_OPENAPI_URL"))
	}
	if src == "" {
		return defaultOpenAPISpecURL
	}
	return src
}

var (
	specOnce sync.Once
	specIdx  SpecIndex
	specErr  error
)

func loadSpecIndex() (SpecIndex, error) {
	specOnce.Do(func() {
		specIdx, specErr = LoadSpecFromFileOrURL(ResolvedOpenAPISource())
	})
	return specIdx, specErr
}

// Analyzer checks LangSmith REST paths used by *client.Client calls in the Terraform provider.
//
// Configure the OpenAPI document with [SetOpenAPISource], LANGSMITH_OPENAPI_URL, or rely on the default URL.
// The driver must load package [ProviderPkgPath] (for example pattern ./internal/provider).
var Analyzer = &analysis.Analyzer{
	Name: "langsmithopenapi",
	Doc: "reports LangSmith HTTP paths referenced by internal/client.Client calls that are missing from the OpenAPI specification.",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	if pass.Pkg.Path() != ProviderPkgPath {
		return nil, nil
	}

	idx, err := loadSpecIndex()
	if err != nil {
		return nil, err
	}

	usages, unresolved := extractFromPass(pass)
	reportUnresolved(pass, unresolved)
	reportUsageSpecMismatch(pass, idx, usages)
	return nil, nil
}

// ResetSpecLoaderForTesting clears cached OpenAPI state (tests only).
func ResetSpecLoaderForTesting() {
	specOnce = sync.Once{}
	specIdx = nil
	specErr = nil
	configuredOpenAPISrc = ""
}

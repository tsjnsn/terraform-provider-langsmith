// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package openapicheck_test

import (
	"testing"

	"github.com/bogware/terraform-provider-langsmith/internal/openapicheck"
)

func TestPatternMatchesOpenAPI(t *testing.T) {
	cases := []struct {
		name    string
		pattern []openapicheck.Segment
		spec    string
		want    bool
	}{
		{
			name: "literal list",
			pattern: litSegs("api", "v1", "info"),
			spec:    "/api/v1/info",
			want:    true,
		},
		{
			name: "wildcard vs param",
			pattern: []openapicheck.Segment{
				{Lit: "api"}, {Lit: "v1"}, {Lit: "datasets"}, {Wild: true},
			},
			spec: "/api/v1/datasets/{dataset_id}",
			want: true,
		},
		{
			name: "literal mismatch",
			pattern: litSegs("api", "v1", "datasets"),
			spec:    "/api/v1/sessions",
			want:    false,
		},
		{
			name: "segment count mismatch",
			pattern: litSegs("api", "v1", "datasets"),
			spec:    "/api/v1/datasets/{id}",
			want:    false,
		},
		{
			name: "commits latest",
			pattern: []openapicheck.Segment{
				{Lit: "commits"}, {Lit: "-"}, {Wild: true}, {Lit: "latest"},
			},
			spec: "/commits/{owner}/{repo}/{commit}",
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			specSegs := openapicheck.ParseOpenAPISegments(tc.spec)
			got := openapicheck.PatternMatchesOpenAPI(tc.pattern, specSegs)
			if got != tc.want {
				t.Fatalf("PatternMatchesOpenAPI(...) = %v, want %v (pattern=%v spec=%q)", got, tc.want, tc.pattern, tc.spec)
			}
		})
	}
}

func litSegs(parts ...string) []openapicheck.Segment {
	var s []openapicheck.Segment
	for _, p := range parts {
		s = append(s, openapicheck.Segment{Lit: p})
	}
	return s
}

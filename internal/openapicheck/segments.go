// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package openapicheck

import "strings"

// Segment is one URL path component after splitting on '/'.
type Segment struct {
	Lit  string // single path segment literal (never contains '/')
	Wild bool   // unknown / templated segment (matches OpenAPI {param})
}

// ParseOpenAPISegments splits an OpenAPI path template into path segments.
// Each segment is either a literal or "{parameter}".
func ParseOpenAPISegments(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func openapiSegIsParam(s string) bool {
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")
}

// PatternMatchesOpenAPI reports whether a provider path pattern matches a
// concrete OpenAPI path template for the same number of segments.
func PatternMatchesOpenAPI(pattern []Segment, openapiSegs []string) bool {
	if len(pattern) != len(openapiSegs) {
		return false
	}
	for i := range pattern {
		spec := openapiSegs[i]
		switch {
		case pattern[i].Wild:
			continue
		case openapiSegIsParam(spec):
			continue
		default:
			if pattern[i].Lit != spec {
				return false
			}
		}
	}
	return true
}

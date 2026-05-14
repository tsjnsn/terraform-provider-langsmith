// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package openapicheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

// SpecIndex maps normalized HTTP method (uppercase) to path -> present.
type SpecIndex map[string]map[string]struct{}

// IndexPaths builds a method→paths map from a parsed OpenAPI v3 document.
func IndexPaths(doc *openapi3.T) (SpecIndex, error) {
	if doc == nil {
		return nil, fmt.Errorf("openapi document is nil")
	}
	if doc.Paths == nil {
		return nil, fmt.Errorf("openapi document has no paths object")
	}
	idx := make(SpecIndex)
	for path, item := range doc.Paths.Map() {
		path = strings.TrimSpace(path)
		if path == "" || item == nil {
			continue
		}
		for method := range item.Operations() {
			m := strings.ToUpper(strings.TrimSpace(method))
			if m == "" {
				continue
			}
			if idx[m] == nil {
				idx[m] = make(map[string]struct{})
			}
			idx[m][path] = struct{}{}
		}
	}
	return idx, nil
}

// LoadSpecDoc loads and validates OpenAPI using kin-openapi. External refs are allowed;
// ResolveRefsIn runs relative to the document location when known.
//
// Documents that validate as OpenAPI v3.x work best; some servers publish OpenAPI 3.1
// schemas that kin-openapi cannot decode — use [LoadSpecFromFileOrURL] for indexing paths.
func LoadSpecDoc(src string) (*openapi3.T, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, fmt.Errorf("empty openapi source")
	}
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	var doc *openapi3.T
	var err error
	switch {
	case strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://"):
		var u *url.URL
		u, err = url.Parse(src)
		if err != nil {
			return nil, fmt.Errorf("parse openapi URL: %w", err)
		}
		doc, err = loader.LoadFromURI(u)
		if err != nil {
			return nil, fmt.Errorf("load openapi from URL: %w", err)
		}
		if err = loader.ResolveRefsIn(doc, u); err != nil {
			return nil, fmt.Errorf("resolve openapi refs: %w", err)
		}
	default:
		doc, err = loader.LoadFromFile(src)
		if err != nil {
			return nil, fmt.Errorf("load openapi file: %w", err)
		}
		base := &url.URL{Path: filepath.ToSlash(src)}
		if err = loader.ResolveRefsIn(doc, base); err != nil {
			return nil, fmt.Errorf("resolve openapi refs: %w", err)
		}
	}

	ctx := context.Background()
	if err = doc.Validate(ctx, openapi3.DisableExamplesValidation()); err != nil {
		return nil, fmt.Errorf("validate openapi document: %w", err)
	}
	return doc, nil
}

// LoadSpecFromFileOrURL loads OpenAPI from a local path or http(s) URL and builds [SpecIndex].
// It prefers kin-openapi parsing when possible; if unmarshalling fails (for example OpenAPI 3.1
// keywords kin treats as OpenAPI 3.0-only), it falls back to decoding only the top-level paths map.
func LoadSpecFromFileOrURL(src string) (SpecIndex, error) {
	doc, err := LoadSpecDoc(src)
	if err == nil {
		return IndexPaths(doc)
	}
	idx, ferr := loadPathsOnlyFromSrc(src)
	if ferr != nil {
		return nil, fmt.Errorf("%w; paths-only fallback: %w", err, ferr)
	}
	return idx, nil
}

// LoadSpec reads OpenAPI from r. It tries kin-openapi first, then paths-only decoding.
func LoadSpec(r io.Reader) (SpecIndex, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read openapi: %w", err)
	}
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(data)
	if err == nil {
		ctx := context.Background()
		if err = doc.Validate(ctx, openapi3.DisableExamplesValidation()); err == nil {
			return IndexPaths(doc)
		}
	}
	return loadPathsOnlyJSON(bytes.NewReader(data))
}

func loadPathsOnlyFromSrc(src string) (SpecIndex, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, fmt.Errorf("empty openapi source")
	}
	var body io.ReadCloser
	var err error
	switch {
	case strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://"):
		client := &http.Client{Timeout: 2 * time.Minute}
		var resp *http.Response
		resp, err = client.Get(src)
		if err != nil {
			return nil, fmt.Errorf("fetch openapi: %w", err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("fetch openapi: HTTP %s", resp.Status)
		}
		body = resp.Body
	default:
		body, err = os.Open(src)
		if err != nil {
			return nil, fmt.Errorf("open openapi file: %w", err)
		}
	}
	defer body.Close()
	return loadPathsOnlyJSON(body)
}

func loadPathsOnlyJSON(r io.Reader) (SpecIndex, error) {
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	dec := json.NewDecoder(r)
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode openapi paths: %w", err)
	}
	if doc.Paths == nil {
		return nil, fmt.Errorf("openapi document has no paths object")
	}
	idx := make(SpecIndex)
	for path, ops := range doc.Paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		for method := range ops {
			m := strings.ToUpper(strings.TrimSpace(method))
			if m == "" || m == "PARAMETERS" {
				continue
			}
			if idx[m] == nil {
				idx[m] = make(map[string]struct{})
			}
			idx[m][path] = struct{}{}
		}
	}
	return idx, nil
}

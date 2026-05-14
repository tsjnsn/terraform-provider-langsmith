// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package openapicheck

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// SpecIndex maps normalized HTTP method (uppercase) to path -> true.
type SpecIndex map[string]map[string]struct{}

// LoadSpec reads an OpenAPI 3.x JSON document from r and builds a method→paths index.
func LoadSpec(r io.Reader) (SpecIndex, error) {
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	dec := json.NewDecoder(r)
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode openapi json: %w", err)
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

// LoadSpecFromFileOrURL loads OpenAPI JSON from a local file path or http(s) URL.
func LoadSpecFromFileOrURL(src string) (SpecIndex, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, fmt.Errorf("empty openapi source")
	}
	var r io.ReadCloser
	var err error
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
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
		r = resp.Body
	} else {
		r, err = os.Open(src)
		if err != nil {
			return nil, fmt.Errorf("open openapi file: %w", err)
		}
	}
	defer r.Close()
	return LoadSpec(r)
}

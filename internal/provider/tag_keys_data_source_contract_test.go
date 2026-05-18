// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagKeysDataSource_contract(t *testing.T) {
	payload := []tagKeyAPIResponse{
		{
			ID:          "tk-1",
			Key:         "environment",
			Description: "Deployment target",
			CreatedAt:   "2026-01-01T00:00:00Z",
			UpdatedAt:   "2026-01-02T00:00:00Z",
		},
		{
			ID:          "tk-2",
			Key:         "team",
			Description: "",
			CreatedAt:   "2026-01-03T00:00:00Z",
			UpdatedAt:   "2026-01-03T00:00:00Z",
		},
	}
	raw, _ := json.Marshal(payload)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/workspaces/current/tag-keys":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(raw)
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_tag_keys" "k" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_tag_keys.k", "tag_keys.#", "2"),
					resource.TestCheckResourceAttr("data.langsmith_tag_keys.k", "tag_keys.0.id", "tk-1"),
					resource.TestCheckResourceAttr("data.langsmith_tag_keys.k", "tag_keys.0.key", "environment"),
					resource.TestCheckResourceAttr("data.langsmith_tag_keys.k", "tag_keys.0.description", "Deployment target"),
					resource.TestCheckResourceAttr("data.langsmith_tag_keys.k", "tag_keys.0.created_at", "2026-01-01T00:00:00Z"),
					resource.TestCheckResourceAttr("data.langsmith_tag_keys.k", "tag_keys.0.updated_at", "2026-01-02T00:00:00Z"),
					resource.TestCheckResourceAttr("data.langsmith_tag_keys.k", "tag_keys.1.id", "tk-2"),
					resource.TestCheckResourceAttr("data.langsmith_tag_keys.k", "tag_keys.1.key", "team"),
					resource.TestCheckNoResourceAttr("data.langsmith_tag_keys.k", "tag_keys.1.description"),
					resource.TestCheckResourceAttr("data.langsmith_tag_keys.k", "tag_keys.1.created_at", "2026-01-03T00:00:00Z"),
					resource.TestCheckResourceAttr("data.langsmith_tag_keys.k", "tag_keys.1.updated_at", "2026-01-03T00:00:00Z"),
				),
			},
		},
	})
}

func TestAccTagKeysDataSource_contract_empty(t *testing.T) {
	raw := []byte(`[]`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/workspaces/current/tag-keys":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(raw)
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "langsmith_tag_keys" "k" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_tag_keys.k", "tag_keys.#", "0"),
				),
			},
		},
	})
}

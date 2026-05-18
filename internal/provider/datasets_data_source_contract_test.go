// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasetsDataSource_contract(t *testing.T) {
	payload := `[{"id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","name":"Alpha","description":"first","data_type":"kv","inputs_schema_definition":null,"outputs_schema_definition":null,"externally_managed":false,"transformations":null,"metadata":null,"tenant_id":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb","created_at":"2024-01-02T00:00:00Z","modified_at":"2024-01-03T00:00:00Z","example_count":3,"session_count":1,"last_session_start_time":null},{"id":"cccccccc-cccc-cccc-cccc-cccccccccccc","name":"Beta","description":null,"data_type":"llm","inputs_schema_definition":null,"outputs_schema_definition":null,"externally_managed":null,"transformations":null,"metadata":null,"tenant_id":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb","created_at":"2024-01-04T00:00:00Z","modified_at":"2024-01-05T00:00:00Z","example_count":0,"session_count":2,"last_session_start_time":"2024-01-06T00:00:00Z"}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/datasets":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(payload))
			return
		default:
			http.Error(w, "not found: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
			return
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `data "langsmith_datasets" "d" {}`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_datasets.d", "id", "datasets"),
					resource.TestCheckResourceAttr("data.langsmith_datasets.d", "datasets.#", "2"),
					resource.TestCheckResourceAttr("data.langsmith_datasets.d", "datasets.0.id", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
					resource.TestCheckResourceAttr("data.langsmith_datasets.d", "datasets.0.name", "Alpha"),
					resource.TestCheckResourceAttr("data.langsmith_datasets.d", "datasets.0.description", "first"),
					resource.TestCheckResourceAttr("data.langsmith_datasets.d", "datasets.0.data_type", "kv"),
					resource.TestCheckResourceAttr("data.langsmith_datasets.d", "datasets.0.example_count", "3"),
					resource.TestCheckResourceAttr("data.langsmith_datasets.d", "datasets.0.session_count", "1"),
					resource.TestCheckResourceAttr("data.langsmith_datasets.d", "datasets.0.externally_managed", "false"),
					resource.TestCheckResourceAttr("data.langsmith_datasets.d", "datasets.1.id", "cccccccc-cccc-cccc-cccc-cccccccccccc"),
					resource.TestCheckResourceAttr("data.langsmith_datasets.d", "datasets.1.name", "Beta"),
					resource.TestCheckResourceAttr("data.langsmith_datasets.d", "datasets.1.data_type", "llm"),
					resource.TestCheckResourceAttr("data.langsmith_datasets.d", "datasets.1.example_count", "0"),
					resource.TestCheckResourceAttr("data.langsmith_datasets.d", "datasets.1.session_count", "2"),
					resource.TestCheckResourceAttr("data.langsmith_datasets.d", "datasets.1.last_session_start_time", "2024-01-06T00:00:00Z"),
				),
			},
		},
	})
}

func TestAccDatasetsDataSource_contract_queryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/datasets":
			q := r.URL.Query()
			if q.Get("limit") != "10" || q.Get("offset") != "5" || q.Get("sort_by") != "name" || q.Get("sort_by_desc") != "false" {
				http.Error(w, "unexpected query: "+r.URL.RawQuery, http.StatusBadRequest)
				return
			}
			if q.Get("name_contains") != "tf" || q.Get("metadata") != `{"k":"v"}` {
				http.Error(w, "unexpected filter query: "+r.URL.RawQuery, http.StatusBadRequest)
				return
			}
			if len(q["data_type"]) != 2 || q["data_type"][0] != "kv" || q["data_type"][1] != "chat" {
				http.Error(w, "unexpected data_type: "+r.URL.RawQuery, http.StatusBadRequest)
				return
			}
			if len(q["id"]) != 1 || q["id"][0] != "dddddddd-dddd-dddd-dddd-dddddddddddd" {
				http.Error(w, "unexpected id: "+r.URL.RawQuery, http.StatusBadRequest)
				return
			}
			if len(q["tag_value_id"]) != 1 || q["tag_value_id"][0] != "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee" {
				http.Error(w, "unexpected tag_value_id: "+r.URL.RawQuery, http.StatusBadRequest)
				return
			}
			if len(q["exclude"]) != 1 || q["exclude"][0] != "example_count" {
				http.Error(w, "unexpected exclude: "+r.URL.RawQuery, http.StatusBadRequest)
				return
			}
			if q.Get("exclude_corrections_datasets") != "true" {
				http.Error(w, "expected exclude_corrections_datasets=true", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
			return
		default:
			http.Error(w, "not found: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
			return
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `
data "langsmith_datasets" "q" {
  ids                          = ["dddddddd-dddd-dddd-dddd-dddddddddddd"]
  data_types                   = ["kv", "chat"]
  name_contains                = "tf"
  metadata                     = "{\"k\":\"v\"}"
  offset                       = 5
  limit                        = 10
  sort_by                      = "name"
  sort_by_desc                 = false
  tag_value_ids                = ["eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"]
  exclude_corrections_datasets = true
  exclude                      = ["example_count"]
}
`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
		},
	})
}

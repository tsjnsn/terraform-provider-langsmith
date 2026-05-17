// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	clientpkg "github.com/bogware/terraform-provider-langsmith/internal/client"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAnnotationQueueReviewerResource_Configure(t *testing.T) {
	t.Parallel()
	var r AnnotationQueueReviewerResource
	var resp fwresource.ConfigureResponse
	r.Configure(context.Background(), fwresource.ConfigureRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("expected no diagnostics for nil provider data")
	}

	r.Configure(context.Background(), fwresource.ConfigureRequest{ProviderData: "not-a-client"}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatalf("expected error for wrong provider data type")
	}
}

func TestAnnotationQueueReviewerResource_Metadata_Schema(t *testing.T) {
	t.Parallel()
	var r AnnotationQueueReviewerResource
	var meta fwresource.MetadataResponse
	r.Metadata(context.Background(), fwresource.MetadataRequest{ProviderTypeName: "langsmith"}, &meta)
	if want := "langsmith_annotation_queue_reviewer"; meta.TypeName != want {
		t.Fatalf("TypeName = %q, want %q", meta.TypeName, want)
	}
	var schemaResp fwresource.SchemaResponse
	r.Schema(context.Background(), fwresource.SchemaRequest{}, &schemaResp)
	if schemaResp.Schema.Attributes == nil {
		t.Fatal("expected schema attributes")
	}
}

// TestAnnotationQueueReviewerPlatformClientCRUD exercises the platform reviewer
// HTTP contract used by the resource (POST / GET / DELETE) against a mock transport.
func TestAnnotationQueueReviewerPlatformClientCRUD(t *testing.T) {
	reviewers := map[string]struct{}{}
	queueID := "00000000-0000-4000-8000-000000000001"
	identityID := "00000000-0000-4000-8000-000000000002"
	key := queueID + "/" + identityID

	rt := &mockRoundTripper{fn: func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/platform/annotation-queues/"+queueID+"/reviewers":
			body, _ := io.ReadAll(req.Body)
			var add annotationQueueReviewerAddRequest
			_ = json.Unmarshal(body, &add)
			if add.IdentityID != identityID {
				return &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader("wrong identity"))}
			}
			reviewers[key] = struct{}{}
			out := annotationQueueReviewerAPIResponse{IdentityID: identityID}
			b, _ := json.Marshal(out)
			return jsonResp(b)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/platform/annotation-queues/"+queueID+"/reviewers/"+identityID:
			if _, ok := reviewers[key]; !ok {
				return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{"detail":"not found"}`))}
			}
			out := annotationQueueReviewerAPIResponse{IdentityID: identityID}
			b, _ := json.Marshal(out)
			return jsonResp(b)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/platform/annotation-queues/"+queueID+"/reviewers/"+identityID:
			delete(reviewers, key)
			return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}
		default:
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("unexpected " + req.Method + " " + req.URL.Path)),
			}
		}
	}}

	c := clientpkg.NewClient("http://example", "key", "", "", "ua")
	c.HTTPClient.Transport = rt
	c.MaxRetries = 0
	ctx := context.Background()

	var created annotationQueueReviewerAPIResponse
	if err := c.Post(ctx, annotationQueueReviewerAPIPath(queueID, ""), annotationQueueReviewerAddRequest{IdentityID: identityID}, &created); err != nil {
		t.Fatal(err)
	}
	if created.IdentityID != identityID {
		t.Fatalf("created identity = %q", created.IdentityID)
	}

	var read annotationQueueReviewerAPIResponse
	if err := c.Get(ctx, annotationQueueReviewerAPIPath(queueID, identityID), nil, &read); err != nil {
		t.Fatal(err)
	}
	if read.IdentityID != identityID {
		t.Fatalf("read identity = %q", read.IdentityID)
	}

	if err := c.Delete(ctx, annotationQueueReviewerAPIPath(queueID, identityID)); err != nil {
		t.Fatal(err)
	}
	// Client returns APIError on GET 404 — resource Read uses IsNotFound to drop state.
	err := c.Get(ctx, annotationQueueReviewerAPIPath(queueID, identityID), nil, &read)
	if err == nil {
		t.Fatal("expected error after delete")
	}
	if !clientpkg.IsNotFound(err) {
		t.Fatalf("expected 404 APIError, got %v", err)
	}
}

// TestAnnotationQueueReviewerResource_terraformMockServer runs apply/import
// against a local mock API (no LangSmith credentials). IsUnitTest allows this
// to run under plain go test / make test without TF_ACC=1.
func TestAnnotationQueueReviewerResource_terraformMockServer(t *testing.T) {
	var mu sync.Mutex
	queues := map[string]annotationQueueAPIResponse{}
	reviewers := map[string]struct{}{}
	nextQueue := 1

	mockQueue := func(id, name string) annotationQueueAPIResponse {
		en := true
		n := int64(1)
		rm := int64(1)
		return annotationQueueAPIResponse{
			ID:                   id,
			Name:                 name,
			EnableReservations:   &en,
			NumReviewersPerItem:  &n,
			ReservationMinutes:   &rm,
			QueueType:            "human",
			TenantID:             "tenant-mock",
			CreatedAt:            "2020-01-01T00:00:00Z",
			UpdatedAt:            "2020-01-01T00:00:00Z",
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/annotation-queues":
			var body annotationQueueAPIRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			id := fmt.Sprintf("00000000-0000-4000-8000-%012d", nextQueue)
			nextQueue++
			q := mockQueue(id, body.Name)
			queues[id] = q
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(q)
			return

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/annotation-queues/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/annotation-queues/")
			q, ok := queues[id]
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(q)
			return

		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/v1/annotation-queues/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/annotation-queues/")
			q, ok := queues[id]
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			var body annotationQueueAPIRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Name != "" {
				q.Name = body.Name
			}
			if body.Description != nil {
				q.Description = body.Description
			}
			queues[id] = q
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"message":"ok"}`))
			return

		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/annotation-queues":
			qid := r.URL.Query().Get("queue_ids")
			delete(queues, qid)
			w.WriteHeader(http.StatusNoContent)
			return

		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/reviewers"):
			// POST .../annotation-queues/{id}/reviewers
			trim := strings.TrimPrefix(r.URL.Path, "/v1/platform/annotation-queues/")
			parts := strings.Split(trim, "/")
			if len(parts) != 2 || parts[1] != "reviewers" {
				http.Error(w, "bad path", http.StatusBadRequest)
				return
			}
			queueID := parts[0]
			if _, ok := queues[queueID]; !ok {
				http.Error(w, "unknown queue", http.StatusNotFound)
				return
			}
			var add annotationQueueReviewerAddRequest
			_ = json.NewDecoder(r.Body).Decode(&add)
			if add.IdentityID == "" {
				http.Error(w, "missing identity_id", http.StatusBadRequest)
				return
			}
			reviewers[queueID+"/"+add.IdentityID] = struct{}{}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(annotationQueueReviewerAPIResponse(add))
			return

		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/reviewers/"):
			trim := strings.TrimPrefix(r.URL.Path, "/v1/platform/annotation-queues/")
			parts := strings.Split(trim, "/")
			// queueID, reviewers, identityID
			if len(parts) != 3 || parts[1] != "reviewers" {
				http.Error(w, "bad path", http.StatusBadRequest)
				return
			}
			k := parts[0] + "/" + parts[2]
			if _, ok := reviewers[k]; !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(annotationQueueReviewerAPIResponse{IdentityID: parts[2]})
			return

		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/reviewers/"):
			trim := strings.TrimPrefix(r.URL.Path, "/v1/platform/annotation-queues/")
			parts := strings.Split(trim, "/")
			if len(parts) != 3 || parts[1] != "reviewers" {
				http.Error(w, "bad path", http.StatusBadRequest)
				return
			}
			k := parts[0] + "/" + parts[2]
			if _, ok := reviewers[k]; !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			delete(reviewers, k)
			w.WriteHeader(http.StatusNoContent)
			return

		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	rName := fmt.Sprintf("tf-aqr-fw-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	identityID := "00000000-0000-4000-8000-0000000000aa"

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             func(*terraform.State) error { return nil },
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationQueueReviewerResourceConfig(rName, identityID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("langsmith_annotation_queue_reviewer.test", "queue_id", "langsmith_annotation_queue.q", "id"),
					resource.TestCheckResourceAttr("langsmith_annotation_queue_reviewer.test", "identity_id", identityID),
					resource.TestCheckResourceAttrSet("langsmith_annotation_queue_reviewer.test", "id"),
				),
			},
			{
				ResourceName:      "langsmith_annotation_queue_reviewer.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFeedbackIngestTokenResource_contract(t *testing.T) {
	runID := "550e8400-e29b-41d4-a716-446655440000"
	tokenID := "660e8400-e29b-41d4-a716-446655440001"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/feedback/tokens":
			body, _ := io.ReadAll(r.Body)
			var cr feedbackTokenCreateRequest
			if err := json.Unmarshal(body, &cr); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if cr.RunID != runID || cr.FeedbackKey != "quality" {
				http.Error(w, "unexpected create body", http.StatusBadRequest)
				return
			}
			resp := feedbackTokenAPI{
				ID:          tokenID,
				URL:         "https://api.smith.langchain.com/api/v1/feedback/tokens/" + tokenID + "/ingest?sig=secret",
				ExpiresAt:   "2030-01-01T00:00:00Z",
				FeedbackKey: "quality",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/feedback/tokens":
			if r.URL.Query().Get("run_id") != runID {
				http.Error(w, "missing run_id", http.StatusBadRequest)
				return
			}
			resp := []feedbackTokenAPI{
				{
					ID:          tokenID,
					URL:         "https://api.smith.langchain.com/api/v1/feedback/tokens/" + tokenID + "/ingest?sig=secret",
					ExpiresAt:   "2030-01-01T00:00:00Z",
					FeedbackKey: "quality",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
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
resource "langsmith_feedback_ingest_token" "tok" {
  run_id        = "` + runID + `"
  feedback_key  = "quality"
}
`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_feedback_ingest_token.tok", "id", tokenID),
					resource.TestCheckResourceAttr("langsmith_feedback_ingest_token.tok", "run_id", runID),
					resource.TestCheckResourceAttr("langsmith_feedback_ingest_token.tok", "feedback_key", "quality"),
					resource.TestCheckResourceAttr("langsmith_feedback_ingest_token.tok", "expires_at", "2030-01-01T00:00:00Z"),
					resource.TestCheckResourceAttrSet("langsmith_feedback_ingest_token.tok", "url"),
				),
			},
		},
	})
}

func TestAccFeedbackIngestTokensDataSource_contract(t *testing.T) {
	runID := "770e8400-e29b-41d4-a716-446655440002"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/feedback/tokens":
			if r.URL.Query().Get("run_id") != runID {
				http.Error(w, "bad run_id", http.StatusBadRequest)
				return
			}
			resp := []feedbackTokenAPI{
				{
					ID:          "880e8400-e29b-41d4-a716-446655440003",
					URL:         "https://example.invalid/token",
					ExpiresAt:   "2031-06-15T12:00:00Z",
					FeedbackKey: "accuracy",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		default:
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `data "langsmith_feedback_ingest_tokens" "t" {
  run_id = "` + runID + `"
}`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_feedback_ingest_tokens.t", "id", runID),
					resource.TestCheckResourceAttr("data.langsmith_feedback_ingest_tokens.t", "run_id", runID),
					resource.TestCheckResourceAttr("data.langsmith_feedback_ingest_tokens.t", "tokens.#", "1"),
					resource.TestCheckResourceAttr("data.langsmith_feedback_ingest_tokens.t", "tokens.0.feedback_key", "accuracy"),
					resource.TestCheckResourceAttr("data.langsmith_feedback_ingest_tokens.t", "tokens.0.expires_at", "2031-06-15T12:00:00Z"),
					resource.TestCheckResourceAttr("data.langsmith_feedback_ingest_tokens.t", "tokens.0.id", "880e8400-e29b-41d4-a716-446655440003"),
				),
			},
		},
	})
}

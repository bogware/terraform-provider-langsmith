// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccEvaluatorResource_contract_code(t *testing.T) {
	var mu sync.Mutex
	store := map[string]evaluatorAPI{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return

		case r.Method == http.MethodPost && r.URL.Path == "/v1/platform/evaluators":
			body, _ := io.ReadAll(r.Body)
			var reqBody evaluatorCreateRequest
			if err := json.Unmarshal(body, &reqBody); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			id := strings.TrimSpace(reqBody.Name)
			if id == "" {
				id = "ev_auto"
			}
			if _, exists := store[id]; exists {
				http.Error(w, "duplicate", http.StatusConflict)
				return
			}
			ev := evaluatorAPI{
				ID:            id,
				Name:          reqBody.Name,
				Type:          reqBody.Type,
				TenantID:      "tenant-1",
				CreatedAt:     "2026-01-01T00:00:00Z",
				UpdatedAt:     "2026-01-01T00:00:00Z",
				CreatedBy:     "user-1",
				FeedbackKeys:  []string{"score"},
				RunRules:      json.RawMessage(`[]`),
				CodeEvaluator: reqBody.CodeEvaluator,
			}
			store[id] = ev
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(createEvaluatorAPIResponse{Evaluator: ev})
			return

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/platform/evaluators/"):
			id := strings.TrimPrefix(r.URL.Path, "/v1/platform/evaluators/")
			ev, ok := store[id]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(ev)
			return

		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/platform/evaluators/"):
			id := strings.TrimPrefix(r.URL.Path, "/v1/platform/evaluators/")
			ev, ok := store[id]
			if !ok {
				http.NotFound(w, r)
				return
			}
			body, _ := io.ReadAll(r.Body)
			var patch evaluatorPatchRequest
			if err := json.Unmarshal(body, &patch); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if patch.Name != nil {
				ev.Name = *patch.Name
			}
			if len(patch.CodeEvaluator) > 0 {
				ev.CodeEvaluator = patch.CodeEvaluator
			}
			ev.UpdatedAt = "2026-01-02T00:00:00Z"
			store[id] = ev
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(updateEvaluatorAPIResponse{Evaluator: ev})
			return

		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/platform/evaluators/"):
			id := strings.TrimPrefix(r.URL.Path, "/v1/platform/evaluators/")
			delete(store, id)
			w.WriteHeader(http.StatusNoContent)
			return

		default:
			http.Error(w, "not found: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	const evID = "ev_contract_code"
	cfg := `
resource "langsmith_evaluator" "e" {
  name  = "` + evID + `"
  type  = "code"
  code_evaluator = jsonencode({
    code     = "def score(outputs, reference_outputs): return 1.0"
    language = "python"
  })
}
`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_evaluator.e", "id", evID),
					resource.TestCheckResourceAttr("langsmith_evaluator.e", "name", evID),
					resource.TestCheckResourceAttr("langsmith_evaluator.e", "type", "code"),
					resource.TestCheckResourceAttr("langsmith_evaluator.e", "tenant_id", "tenant-1"),
					resource.TestCheckResourceAttr("langsmith_evaluator.e", "feedback_keys.#", "1"),
					resource.TestCheckResourceAttr("langsmith_evaluator.e", "feedback_keys.0", "score"),
				),
			},
			{
				Config: `
resource "langsmith_evaluator" "e" {
  name  = "` + evID + `"
  type  = "code"
  code_evaluator = jsonencode({
    code     = "def score(outputs, reference_outputs): return 2.0"
    language = "python"
  })
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_evaluator.e", "updated_at", "2026-01-02T00:00:00Z"),
				),
			},
			{
				ResourceName:      "langsmith_evaluator.e",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccEvaluatorDataSource_contract(t *testing.T) {
	ev := evaluatorAPI{
		ID:            "ev_ds_1",
		Name:          "ds-eval",
		Type:          "llm",
		TenantID:      "tenant-99",
		CreatedAt:     "2026-03-01T00:00:00Z",
		UpdatedAt:     "2026-03-01T00:00:00Z",
		CreatedBy:     "user-9",
		FeedbackKeys:  []string{"helpfulness"},
		RunRules:      json.RawMessage(`[{"id":"rr1","dataset_id":"d1"}]`),
		LLMEvaluator:  json.RawMessage(`{"prompt_repo_handle":"hub/prompt","commit_hash_or_tag":"main"}`),
		CodeEvaluator: nil,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/v1/platform/evaluators/ev_ds_1":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(ev)
			return
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	cfg := `
data "langsmith_evaluator" "x" {
  evaluator_id = "ev_ds_1"
}
`

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.langsmith_evaluator.x", "id", "ev_ds_1"),
					resource.TestCheckResourceAttr("data.langsmith_evaluator.x", "evaluator_id", "ev_ds_1"),
					resource.TestCheckResourceAttr("data.langsmith_evaluator.x", "name", "ds-eval"),
					resource.TestCheckResourceAttr("data.langsmith_evaluator.x", "type", "llm"),
					resource.TestCheckResourceAttr("data.langsmith_evaluator.x", "tenant_id", "tenant-99"),
					resource.TestCheckResourceAttr("data.langsmith_evaluator.x", "feedback_keys.#", "1"),
					resource.TestCheckResourceAttr("data.langsmith_evaluator.x", "feedback_keys.0", "helpfulness"),
				),
			},
		},
	})
}

func TestAccEvaluatorResource_contract_deleteRunRulesQuery(t *testing.T) {
	var mu sync.Mutex
	store := map[string]evaluatorAPI{}
	deleteWithRunRules := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/v1/platform/evaluators":
			body, _ := io.ReadAll(r.Body)
			var reqBody evaluatorCreateRequest
			if err := json.Unmarshal(body, &reqBody); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			ev := evaluatorAPI{
				ID:            "ev_delete_flag",
				Name:          reqBody.Name,
				Type:          reqBody.Type,
				TenantID:      "tenant-1",
				CreatedAt:     "2026-01-01T00:00:00Z",
				UpdatedAt:     "2026-01-01T00:00:00Z",
				CreatedBy:     "user-1",
				FeedbackKeys:  []string{"score"},
				RunRules:      json.RawMessage(`[]`),
				CodeEvaluator: reqBody.CodeEvaluator,
			}
			store[ev.ID] = ev
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(createEvaluatorAPIResponse{Evaluator: ev})
			return
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/platform/evaluators/"):
			id := strings.TrimPrefix(r.URL.Path, "/v1/platform/evaluators/")
			ev, ok := store[id]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(ev)
			return
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/platform/evaluators/"):
			if r.URL.Query().Get("delete_run_rules") == "true" {
				deleteWithRunRules = true
			}
			id := strings.TrimPrefix(r.URL.Path, "/v1/platform/evaluators/")
			delete(store, id)
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			http.Error(w, "not found: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			mu.Lock()
			defer mu.Unlock()
			if !deleteWithRunRules {
				return fmt.Errorf("expected delete_run_rules=true query parameter on delete")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: `
resource "langsmith_evaluator" "e" {
  name = "ev-delete"
  type = "code"
  delete_run_rules = true
  code_evaluator = jsonencode({
    code     = "def score(outputs, reference_outputs): return 1.0"
    language = "python"
  })
}
`,
				Check: resource.TestCheckResourceAttr("langsmith_evaluator.e", "delete_run_rules", "true"),
			},
		},
	})
}

func TestAccEvaluatorResource_contract_validation_missingPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
			return
		case r.Method == http.MethodPost && r.URL.Path == "/v1/platform/evaluators":
			http.Error(w, "unexpected post", http.StatusInternalServerError)
			return
		default:
			http.Error(w, "not found", http.StatusNotFound)
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
				Config: `
resource "langsmith_evaluator" "bad" {
  name = "invalid"
  type = "llm"
}
`,
				ExpectError: regexp.MustCompile(`Missing llm_evaluator for type llm`),
			},
		},
	})
}

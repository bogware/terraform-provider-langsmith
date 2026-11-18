// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// LangSmith retired the feedback formula endpoints (#98): every call returns
// 410 Gone. The resource is deprecated, and these tests pin the behavior that
// keeps existing configurations usable: creating or updating fails with the
// way forward, a formula already in state is dropped on refresh instead of
// failing every plan, and destroying one completes.

const feedbackFormulaTestID = "3b1c8d0e-0000-4000-8000-0000000000f1"

// TestAccFeedbackFormulaResource_createRejected checks that creating a formula
// fails with the retirement guidance, before any formula request is sent.
func TestAccFeedbackFormulaResource_createRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccFeedbackFormulaResourceConfig("3b1c8d0e-0000-4000-8000-000000000002"),
				ExpectError: regexp.MustCompile(`(?s)Feedback formulas have been retired.*composite\s+evaluators`),
			},
		},
	})
}

// feedbackFormulaStub serves one formula. GET answers 200 until readGone is
// set and 410 after; DELETE answers 410 when deleteGone is set. It records
// how many DELETE requests arrive.
type feedbackFormulaStub struct {
	mu         sync.Mutex
	readGone   bool
	deleteGone bool
	deletes    int
}

func (s *feedbackFormulaStub) set(f func(*feedbackFormulaStub)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(s)
}

func (s *feedbackFormulaStub) deleteCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deletes
}

func (s *feedbackFormulaStub) start(t *testing.T) {
	t.Helper()
	const retired = `{"detail":"The composite feedback formula endpoints have been retired. ` +
		`Use POST /api/v1/feedback/composite-evaluators to create a code evaluator from the feedback formula."}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		formulaPath := "/api/v1/feedback/formulas/" + feedbackFormulaTestID

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			_, _ = w.Write([]byte(`{"version":"test"}`))

		case r.Method == http.MethodGet && r.URL.Path == formulaPath:
			if s.readGone {
				w.WriteHeader(http.StatusGone)
				_, _ = w.Write([]byte(retired))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":               feedbackFormulaTestID,
				"feedback_key":     "composite",
				"aggregation_type": "avg",
				"formula_parts":    []map[string]any{{"part_type": "weighted_key", "weight": 1.0, "key": "correctness"}},
				"session_id":       "3b1c8d0e-0000-4000-8000-000000000002",
				"created_at":       "2026-01-01T00:00:00Z",
				"modified_at":      "2026-01-01T00:00:00Z",
				"tenant_id":        "ws-test",
			})

		case r.Method == http.MethodDelete && r.URL.Path == formulaPath:
			s.deletes++
			if s.deleteGone {
				w.WriteHeader(http.StatusGone)
				_, _ = w.Write([]byte(retired))
				return
			}
			w.WriteHeader(http.StatusNoContent)

		case strings.HasPrefix(r.URL.Path, "/api/v1/feedback/formulas"):
			w.WriteHeader(http.StatusGone)
			_, _ = w.Write([]byte(retired))

		default:
			http.Error(w, fmt.Sprintf("unexpected %s %s", r.Method, r.URL.Path), http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_WORKSPACE_ID", "ws-test")
	t.Setenv("LANGSMITH_API_URL", srv.URL)
}

// importStep puts the stub's formula into state, standing in for a formula
// created before the endpoints were retired.
func feedbackFormulaImportStep() resource.TestStep {
	return resource.TestStep{
		Config:             testAccFeedbackFormulaResourceConfig("3b1c8d0e-0000-4000-8000-000000000002"),
		ResourceName:       "langsmith_feedback_formula.test",
		ImportState:        true,
		ImportStateId:      feedbackFormulaTestID,
		ImportStatePersist: true,
	}
}

// TestAccFeedbackFormulaResource_retiredDroppedOnRefresh: once reads return
// 410, a formula in state must be dropped on refresh. Before the fix Read
// failed on the 410, which broke every plan in the configuration.
func TestAccFeedbackFormulaResource_retiredDroppedOnRefresh(t *testing.T) {
	stub := &feedbackFormulaStub{}
	stub.start(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			feedbackFormulaImportStep(),
			{
				PreConfig: func() { stub.set(func(s *feedbackFormulaStub) { s.readGone = true }) },
				// The formula is removed from configuration, as the
				// deprecation tells users to do. Refresh drops it from state,
				// so nothing is left to destroy and no DELETE is sent.
				Config: `# langsmith_feedback_formula removed`,
				Check: func(*terraform.State) error {
					if n := stub.deleteCount(); n != 0 {
						return fmt.Errorf("expected no DELETE once the formula was dropped from state, got %d", n)
					}
					return nil
				},
			},
		},
	})
}

// TestAccFeedbackFormulaResource_retiredDestroyCompletes: a destroy that meets
// a 410 must complete. Before the fix Delete failed on it, and the resource
// could never leave state.
func TestAccFeedbackFormulaResource_retiredDestroyCompletes(t *testing.T) {
	stub := &feedbackFormulaStub{deleteGone: true}
	stub.start(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			feedbackFormulaImportStep(),
			{
				Config: `# langsmith_feedback_formula removed`,
				Check: func(*terraform.State) error {
					if n := stub.deleteCount(); n != 1 {
						return fmt.Errorf("expected exactly one DELETE, got %d", n)
					}
					return nil
				},
			},
		},
	})
}

func testAccFeedbackFormulaResourceConfig(sessionID string) string {
	return fmt.Sprintf(`
resource "langsmith_feedback_formula" "test" {
  feedback_key     = "composite"
  aggregation_type = "avg"
  session_id       = %[1]q
  formula_parts    = jsonencode([
    {
      part_type = "weighted_key"
      weight    = 1.0
      key       = "correctness"
    }
  ])
}
`, sessionID)
}

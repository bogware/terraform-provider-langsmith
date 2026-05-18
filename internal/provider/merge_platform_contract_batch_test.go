// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccGatewayPolicyDataSource_contract(t *testing.T) {
	pid := "gw-pol-aaaaaaaa-bbbb-cccc-dddddddddddd"
	spend := 1.25
	api := gatewayPolicyAPI{
		ID: pid, Name: "P", Description: "d", PolicyType: "spend_cap", Action: "block",
		Priority: 10, Enabled: true, Config: map[string]interface{}{"a": 1},
		SubjectMatchers: []gatewayPolicySubjectMatcher{{Key: "k", Value: "v"}},
		OrganizationID:  "org-1", IsSystemGenerated: false, CurrentSpendUSD: &spend,
		CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z",
	}
	raw, _ := json.Marshal(api)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/platform/gateway-policies/"+pid:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(raw)
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
		Steps: []resource.TestStep{{
			Config: `data "langsmith_gateway_policy" "g" { id = "` + pid + `" }`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.langsmith_gateway_policy.g", "name", "P"),
				resource.TestCheckResourceAttr("data.langsmith_gateway_policy.g", "policy_type", "spend_cap"),
				resource.TestCheckResourceAttr("data.langsmith_gateway_policy.g", "current_spend_usd", "1.25"),
			),
		}},
	})
}

func TestAccGatewayPolicyResource_contract(t *testing.T) {
	id := "gw-new-bbbbbbbb-bbbb-cccc-dddddddddddd"
	var mu sync.Mutex
	exists := false
	name := "gw-tf"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := "/v1/platform/gateway-policies/" + id
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/platform/gateway-policies":
			mu.Lock()
			exists = true
			mu.Unlock()
			out := gatewayPolicyAPI{
				ID: id, Name: name, Description: "", PolicyType: "spend_cap", Action: "block",
				Priority: 5, Enabled: true, Config: map[string]interface{}{"amount_usd": 5},
				OrganizationID: "o1", IsSystemGenerated: false, CreatedAt: "2026-01-01T00:00:00Z",
				UpdatedAt: "2026-01-01T00:00:00Z", CreatedBy: "u1",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodGet && r.URL.Path == path:
			mu.Lock()
			ok := exists
			n := name
			mu.Unlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			out := gatewayPolicyAPI{
				ID: id, Name: n, PolicyType: "spend_cap", Action: "block",
				Priority: 5, Enabled: true, Config: map[string]interface{}{"amount_usd": 5},
				OrganizationID: "o1", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
				CreatedBy: "u1",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodPatch && r.URL.Path == path:
			mu.Lock()
			name = "gw-tf-renamed"
			n := name
			mu.Unlock()
			out := gatewayPolicyAPI{
				ID: id, Name: n, PolicyType: "spend_cap", Action: "block",
				Priority: 5, Enabled: true, Config: map[string]interface{}{"amount_usd": 5},
				OrganizationID: "o1", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-03T00:00:00Z",
				CreatedBy: "u1",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodDelete && r.URL.Path == path:
			mu.Lock()
			exists = false
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	resource.UnitTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             func(*terraform.State) error { return nil },
		Steps: []resource.TestStep{
			{
				Config: `
resource "langsmith_gateway_policy" "g" {
  name        = "gw-tf"
  policy_type = "spend_cap"
  action      = "block"
  enabled     = true
  priority    = 5
  config      = jsonencode({ amount_usd = 5 })
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_gateway_policy.g", "id", id),
				),
			},
			{
				Config: `
resource "langsmith_gateway_policy" "g" {
  name        = "gw-tf-renamed"
  policy_type = "spend_cap"
  action      = "block"
  enabled     = true
  priority    = 5
  config      = jsonencode({ amount_usd = 5 })
}
`,
			},
		},
	})
}

func TestAccEvaluatorDataSource_contract_only(t *testing.T) {
	eid := "eval-ds-cccccccc-cccc-cccc-cccccccccccc"
	api := evaluatorAPI{
		ID: eid, Name: "E", Type: "code", TenantID: "t1",
		CreatedAt: "a", UpdatedAt: "b", CreatedBy: "c",
		CodeEvaluator: &evaluatorCodeAPI{Code: "x=1", Language: "python"},
	}
	raw, _ := json.Marshal(api)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/platform/evaluators/"+eid:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(raw)
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
		Steps: []resource.TestStep{{
			Config: `data "langsmith_evaluator" "e" { id = "` + eid + `" }`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.langsmith_evaluator.e", "name", "E"),
				resource.TestCheckResourceAttr("data.langsmith_evaluator.e", "type", "code"),
			),
		}},
	})
}

func TestAccEvaluatorResource_contract_code(t *testing.T) {
	evID := "eval-res-dddddddd-dddd-dddd-dddddddddddd"
	var mu sync.Mutex
	exists := false
	evalName := "code-eval-1"

	apiEval := func(name string) evaluatorAPI {
		return evaluatorAPI{
			ID: evID, Name: name, Type: "code", TenantID: "ten",
			CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z", CreatedBy: "me",
			FeedbackKeys: []string{"score"},
			CodeEvaluator: &evaluatorCodeAPI{
				Code: "def perform_eval(run, example):\n  return {}", Language: "python",
			},
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := "/v1/platform/evaluators/" + evID
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/platform/evaluators":
			mu.Lock()
			exists = true
			body, _ := io.ReadAll(r.Body)
			var cr evaluatorCreateRequest
			_ = json.Unmarshal(body, &cr)
			if cr.Name != "" {
				evalName = cr.Name
			}
			n := evalName
			mu.Unlock()
			out := evaluatorCreateResponse{Evaluator: apiEval(n)}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodGet && r.URL.Path == path:
			mu.Lock()
			ok := exists
			n := evalName
			mu.Unlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			a := apiEval(n)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(a)
		case r.Method == http.MethodPatch && r.URL.Path == path:
			mu.Lock()
			evalName = "renamed"
			n := evalName
			mu.Unlock()
			out := evaluatorCreateResponse{Evaluator: apiEval(n)}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
		case r.Method == http.MethodDelete && r.URL.Path == path:
			mu.Lock()
			exists = false
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_API_URL", srv.URL)

	resource.UnitTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             func(*terraform.State) error { return nil },
		Steps: []resource.TestStep{
			{
				Config: `
resource "langsmith_evaluator" "e" {
  name = "code-eval-1"
  type = "code"
  code_evaluator = {
    code     = "def perform_eval(run, example):\n  return {}"
    language = "python"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_evaluator.e", "id", evID),
				),
			},
			{
				Config: `
resource "langsmith_evaluator" "e" {
  name = "renamed"
  type = "code"
  code_evaluator = {
    code     = "def perform_eval(run, example):\n  return {}"
    language = "python"
  }
}
`,
			},
		},
	})
}

func TestAccPromptDataSource_contract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/-/my-prompt":
			d := "hello"
			rme := "readme"
			out := promptDataSourceAPIResponse{}
			out.Repo.ID = "rid-1"
			out.Repo.RepoHandle = "my-prompt"
			out.Repo.Description = &d
			out.Repo.Readme = &rme
			out.Repo.TenantID = "t1"
			out.Repo.CreatedAt = "2026-01-01T00:00:00Z"
			out.Repo.UpdatedAt = "2026-01-02T00:00:00Z"
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
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
		Steps: []resource.TestStep{{
			Config: `data "langsmith_prompt" "p" { repo_handle = "my-prompt" }`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.langsmith_prompt.p", "id", "rid-1"),
				resource.TestCheckResourceAttr("data.langsmith_prompt.p", "description", "hello"),
			),
		}},
	})
}

func TestAccToolDataSource_contract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"test"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/platform/tools/ds-handle":
			out := toolAPI{
				ID: "tid-1", Handle: "ds-handle", Name: "N", Description: "D",
				Parameters: map[string]interface{}{"type": "object"},
				Returns:    map[string]interface{}{"type": "string"},
				Metadata:   map[string]interface{}{},
				Enabled:    true, TenantID: "t1",
				CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
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
		Steps: []resource.TestStep{{
			Config: `data "langsmith_tool" "t" { handle = "ds-handle" }`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.langsmith_tool.t", "name", "N"),
			),
		}},
	})
}

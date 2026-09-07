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

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccPromptResource_basic(t *testing.T) {
	handle := strings.ToLower(fmt.Sprintf("tf-prompt-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPromptResourceConfig(handle, false, "initial description", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_prompt.test", "id"),
					resource.TestCheckResourceAttr("langsmith_prompt.test", "repo_handle", handle),
					resource.TestCheckResourceAttr("langsmith_prompt.test", "is_public", "false"),
					resource.TestCheckResourceAttr("langsmith_prompt.test", "description", "initial description"),
					resource.TestCheckResourceAttr("langsmith_prompt.test", "restricted_mode", "false"),
					// owner may be empty for prompts created via a service account.
					resource.TestCheckResourceAttrSet("langsmith_prompt.test", "full_name"),
					resource.TestCheckResourceAttrSet("langsmith_prompt.test", "workspace_id"),
					// counters have been removed in 0.9.0 — verify they're truly gone.
					resource.TestCheckNoResourceAttr("langsmith_prompt.test", "num_likes"),
					resource.TestCheckNoResourceAttr("langsmith_prompt.test", "num_views"),
					resource.TestCheckNoResourceAttr("langsmith_prompt.test", "num_downloads"),
					resource.TestCheckNoResourceAttr("langsmith_prompt.test", "num_commits"),
					resource.TestCheckNoResourceAttr("langsmith_prompt.test", "last_commit_hash"),
				),
			},
			{
				Config: testAccPromptResourceConfig(handle, false, "updated description", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_prompt.test", "description", "updated description"),
					// restricted_mode must round-trip through update + refresh.
					resource.TestCheckResourceAttr("langsmith_prompt.test", "restricted_mode", "true"),
				),
			},
			// Idempotency: the owner/full_name path fixes must produce zero diff on replay.
			{
				Config:             testAccPromptResourceConfig(handle, false, "updated description", true),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TestAccPromptResource_partialCreateRecovery is a regression test for issue
// #61: the repo POST succeeds but the follow-up commit POST fails (unsupported
// manifest type). The provider must persist partial state so the repo is
// tracked (tainted) and replaced on the next apply, instead of being orphaned
// remotely and causing a 409 "already exists" on the retry.
func TestAccPromptResource_partialCreateRecovery(t *testing.T) {
	handle := strings.ToLower(fmt.Sprintf("tf-prompt-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: RunnableSequence manifests are rejected by the commit
			// endpoint with 400 "Manifest type ... is not supported", after the
			// repo itself has already been created.
			{
				Config: testAccPromptResourceConfigUnsupportedManifest(handle),
				// \s+ rather than literal spaces: Terraform hard-wraps diagnostic
				// text at the terminal width, so any space in the message can
				// arrive as a newline depending on how long the request path is.
				ExpectError: regexp.MustCompile(`is\s+not\s+supported`),
			},
			// Step 2: same resource address with a valid ChatPromptTemplate
			// manifest. The tainted repo from step 1 is destroyed and
			// recreated; this must apply cleanly with no 409 conflict.
			{
				Config: testAccPromptResourceConfigValidManifest(handle),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langsmith_prompt.test", "id"),
					resource.TestCheckResourceAttr("langsmith_prompt.test", "repo_handle", handle),
					resource.TestCheckResourceAttrSet("langsmith_prompt.test", "commit_hash"),
				),
			},
		},
	})
}

func testAccPromptResourceConfigUnsupportedManifest(handle string) string {
	return fmt.Sprintf(`
resource "langsmith_prompt" "test" {
  repo_handle = %[1]q
  is_public   = false
  description = "partial create recovery test"

  manifest = jsonencode({
    lc     = 1
    type   = "constructor"
    id     = ["langchain", "schema", "runnable", "RunnableSequence"]
    kwargs = {}
  })
}
`, handle)
}

func testAccPromptResourceConfigValidManifest(handle string) string {
	return fmt.Sprintf(`
resource "langsmith_prompt" "test" {
  repo_handle = %[1]q
  is_public   = false
  description = "partial create recovery test"
  # The server auto-tags the repo with the manifest type on commit; declare
  # it so the post-apply refresh plan is empty.
  tags = ["ChatPromptTemplate"]

  manifest = jsonencode({
    lc   = 1
    type = "constructor"
    id   = ["langchain", "prompts", "chat", "ChatPromptTemplate"]
    kwargs = {
      input_variables = ["question"]
      messages = [
        {
          lc   = 1
          type = "constructor"
          id   = ["langchain", "prompts", "chat", "HumanMessagePromptTemplate"]
          kwargs = {
            prompt = {
              lc   = 1
              type = "constructor"
              id   = ["langchain", "prompts", "prompt", "PromptTemplate"]
              kwargs = {
                input_variables = ["question"]
                template        = "{question}"
                template_format = "f-string"
              }
            }
          }
        }
      ]
    }
  })
}
`, handle)
}

// promptStub emulates the slice of the LangSmith API that langsmith_prompt
// touches, recording every commit request it receives so a test can assert on
// the wire payload. It needs no credentials: the provider is pointed at it via
// LANGSMITH_API_URL.
type promptStub struct {
	mu          sync.Mutex
	handle      string
	description string
	isPublic    bool
	numCommits  int
	latestHash  string
	manifest    json.RawMessage
	commits     []promptCommitRequest
	// failCommit is consulted with the 1-based commit number before a commit is
	// applied. Returning true answers 409 the way the API does on a repo whose
	// head it cannot resolve.
	failCommit func(n int) bool
}

// recorded returns a copy of the commit requests seen so far.
func (s *promptStub) recorded() []promptCommitRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]promptCommitRequest(nil), s.commits...)
}

// repoBody mirrors the API's nested shape, including the two details the
// provider has to cope with: owner is null (as it is for service-account
// prompts, so paths fall back to the "-" wildcard) and the workspace arrives
// as tenant_id rather than workspace_id.
func (s *promptStub) repoBody() map[string]interface{} {
	return map[string]interface{}{
		"repo": map[string]interface{}{
			"id":              "repo-1",
			"repo_handle":     s.handle,
			"description":     s.description,
			"readme":          "",
			"is_public":       s.isPublic,
			"is_archived":     false,
			"restricted_mode": false,
			"tags":            []string{},
			"tenant_id":       "ws-test",
			"num_commits":     s.numCommits,
			"owner":           nil,
			"full_name":       "-/" + s.handle,
			"created_at":      "2026-01-01T00:00:00Z",
			"updated_at":      "2026-01-01T00:00:00Z",
		},
	}
}

// start brings up the stub and points the provider at it for the duration of
// the test. The server address only reaches the provider through the
// environment, so there is nothing for the caller to hold on to.
func (s *promptStub) start(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()

		repoPath := "/api/v1/repos/-/" + s.handle
		commitPath := "/api/v1/commits/-/" + s.handle
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/info":
			_, _ = w.Write([]byte(`{"version":"test"}`))

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos":
			var cr promptCreateRequest
			_ = json.NewDecoder(r.Body).Decode(&cr)
			s.handle = cr.RepoHandle
			s.description = cr.Description
			s.isPublic = cr.IsPublic
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(s.repoBody())

		case r.Method == http.MethodPost && r.URL.Path == commitPath:
			var cm promptCommitRequest
			_ = json.NewDecoder(r.Body).Decode(&cm)
			s.commits = append(s.commits, cm)
			if s.failCommit != nil && s.failCommit(len(s.commits)) {
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"error":"Parent commit validation failed: reference ID 0f6c"}`))
				return
			}
			s.numCommits++
			s.latestHash = fmt.Sprintf("hash-%d", s.numCommits)
			// Echo the manifest back verbatim, as the API does, so a
			// round-trip mismatch cannot masquerade as the failure under test.
			s.manifest = cm.Manifest
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"commit": map[string]interface{}{
					"id":          fmt.Sprintf("commit-%d", s.numCommits),
					"commit_hash": s.latestHash,
					"manifest":    s.manifest,
				},
			})

		case r.Method == http.MethodGet && r.URL.Path == commitPath+"/latest":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"commit_hash": s.latestHash,
				"manifest":    s.manifest,
			})

		case r.Method == http.MethodGet && r.URL.Path == repoPath:
			_ = json.NewEncoder(w).Encode(s.repoBody())

		case r.Method == http.MethodPatch && r.URL.Path == repoPath:
			var ur promptUpdateRequest
			_ = json.NewDecoder(r.Body).Decode(&ur)
			if ur.Description != nil {
				s.description = *ur.Description
			}
			if ur.IsPublic != nil {
				s.isPublic = *ur.IsPublic
			}
			_, _ = w.Write([]byte(`{}`))

		case r.Method == http.MethodDelete && r.URL.Path == repoPath:
			w.WriteHeader(http.StatusNoContent)

		default:
			http.Error(w, fmt.Sprintf("unexpected %s %s", r.Method, r.URL.Path), http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv("LANGSMITH_API_KEY", "test-key")
	t.Setenv("LANGSMITH_WORKSPACE_ID", "ws-test")
	t.Setenv("LANGSMITH_API_URL", srv.URL)
}

// TestAccPromptResource_parentCommitOnUpdate is a regression test for issue
// #81: Update() committed a new manifest without a parent_commit, so the API
// had to infer where to attach it. That inference fails on a repo with more
// than one head, and every apply that changed a manifest died with a 409.
// The first commit on a fresh repo has no parent; every later one must be
// pinned to the head the provider last saw.
func TestAccPromptResource_parentCommitOnUpdate(t *testing.T) {
	handle := "tf-prompt-parent"
	stub := &promptStub{handle: handle}
	stub.start(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPromptResourceConfigManifest(handle, "first"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_prompt.test", "commit_hash", "hash-1"),
					func(*terraform.State) error {
						got := stub.recorded()
						if len(got) != 1 {
							return fmt.Errorf("expected 1 commit, got %d", len(got))
						}
						if got[0].ParentCommit != "" {
							return fmt.Errorf("first commit sent parent_commit %q, want empty (fresh repo has no history)", got[0].ParentCommit)
						}
						return nil
					},
				),
			},
			{
				Config: testAccPromptResourceConfigManifest(handle, "second"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_prompt.test", "commit_hash", "hash-2"),
					func(*terraform.State) error {
						got := stub.recorded()
						if len(got) != 2 {
							return fmt.Errorf("expected 2 commits, got %d", len(got))
						}
						if got[1].ParentCommit != "hash-1" {
							return fmt.Errorf("update commit sent parent_commit %q, want %q", got[1].ParentCommit, "hash-1")
						}
						return nil
					},
				),
			},
		},
	})
}

// TestAccPromptResource_parentCommitConflict covers the other half: when the
// parent Terraform sent is no longer the head, the raw 409 body says nothing a
// user can act on, so the provider explains what happened.
func TestAccPromptResource_parentCommitConflict(t *testing.T) {
	handle := "tf-prompt-conflict"
	stub := &promptStub{
		handle:     handle,
		failCommit: func(n int) bool { return n > 1 },
	}
	stub.start(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPromptResourceConfigManifest(handle, "first"),
				Check: resource.TestCheckResourceAttr("langsmith_prompt.test", "commit_hash", "hash-1"),
			},
			{
				Config: testAccPromptResourceConfigManifest(handle, "second"),
				// \s+ rather than literal spaces: Terraform hard-wraps
				// diagnostic text at the terminal width, so any space in the
				// message can arrive as a newline.
				ExpectError: regexp.MustCompile(`no\s+longer\s+this\s+repo`),
			},
		},
	})
}

func testAccPromptResourceConfigManifest(handle, template string) string {
	return fmt.Sprintf(`
resource "langsmith_prompt" "test" {
  repo_handle = %[1]q
  is_public   = false
  description = "parent commit test"

  manifest = jsonencode({
    lc       = 1
    template = %[2]q
    type     = "constructor"
  })
}
`, handle, template)
}

func testAccPromptResourceConfig(handle string, isPublic bool, description string, restrictedMode bool) string {
	return fmt.Sprintf(`
resource "langsmith_prompt" "test" {
  repo_handle     = %[1]q
  is_public       = %[2]t
  description     = %[3]q
  restricted_mode = %[4]t
}
`, handle, isPublic, description, restrictedMode)
}

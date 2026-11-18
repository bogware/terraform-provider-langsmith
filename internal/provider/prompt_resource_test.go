// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"errors"
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

	clientpkg "github.com/bogware/terraform-provider-langsmith/internal/client"
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

// TestPromptCommitConflictHint pins the hint to the one situation it can
// actually diagnose. A 409 is not self-evidently a parent-commit conflict, and
// the advice it carries ("the prompt was committed to outside Terraform") is
// specific enough to mislead if attached to some other conflict.
func TestPromptCommitConflictHint(t *testing.T) {
	const parentBody = `{"error":"Parent commit validation failed: reference ID 0f6c"}`

	cases := []struct {
		name       string
		err        error
		parent     string
		fromConfig bool
		want       string // substring the hint must contain; "" means no hint at all
	}{
		{
			name:   "parent conflict names the stale parent",
			err:    &clientpkg.APIError{StatusCode: http.StatusConflict, Body: parentBody},
			parent: "hash-1",
			want:   "hash-1",
		},
		{
			name:   "parent conflict with no known parent explains that instead",
			err:    &clientpkg.APIError{StatusCode: http.StatusConflict, Body: parentBody},
			parent: "",
			want:   "no known parent commit",
		},
		{
			// Refreshing cannot fix a parent the user pinned, so the hint must
			// not tell them to run plan and pick up the new head.
			name:       "parent conflict on a configured parent_commit points at the setting",
			err:        &clientpkg.APIError{StatusCode: http.StatusConflict, Body: parentBody},
			parent:     "abc123",
			fromConfig: true,
			want:       "parent_commit set in configuration (abc123) is not a current head",
		},
		{
			name:       "a configured parent_commit gets no diagnosis for an unrelated 409",
			err:        &clientpkg.APIError{StatusCode: http.StatusConflict, Body: `{"error":"repo is locked"}`},
			parent:     "abc123",
			fromConfig: true,
			want:       "",
		},
		{
			name:   "409 for an unrelated reason gets no diagnosis",
			err:    &clientpkg.APIError{StatusCode: http.StatusConflict, Body: `{"error":"repo is locked"}`},
			parent: "hash-1",
			want:   "",
		},
		{
			name:   "a non-409 parent-commit message is not a conflict",
			err:    &clientpkg.APIError{StatusCode: http.StatusBadRequest, Body: parentBody},
			parent: "hash-1",
			want:   "",
		},
		{
			name:   "a transport error is not an API conflict",
			err:    errors.New("connection reset"),
			parent: "hash-1",
			want:   "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := promptCommitConflictHint(tc.err, promptCommitOptions{ParentCommit: tc.parent, ParentFromConfig: tc.fromConfig})
			if tc.want == "" {
				if got != "" {
					t.Fatalf("expected no hint, got %q", got)
				}
				return
			}
			if !strings.Contains(got, tc.want) {
				t.Fatalf("hint = %q, want it to contain %q", got, tc.want)
			}
		})
	}
}

// TestAccPromptResource_updateConflictKeepsAppliedMetadata covers the partial
// update path. An apply that changes both metadata and the manifest issues a
// PATCH and then a commit; when the commit 409s, the PATCH has already landed
// server-side. terraform-plugin-framework seeds UpdateResponse.State from the
// *prior* state ("Require explicit provider updates for tracking successful
// updates"), so returning without setting state persists the pre-update
// values and silently diverges from the live repo.
//
// Step 3 observes it: a PlanOnly step plans both with and without a refresh,
// and the non-refresh plan reads state as Update left it. tag_value_ids is
// carried alongside description because Read never repopulates it, so the
// divergence survives a refresh too -- the assertion holds on either plan
// rather than depending on which one the harness runs first.
func TestAccPromptResource_updateConflictKeepsAppliedMetadata(t *testing.T) {
	handle := "tf-prompt-partial-update"
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
				Config: testAccPromptResourceConfigMetadata(handle, "first", "tv-1", "first"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langsmith_prompt.test", "commit_hash", "hash-1"),
					resource.TestCheckResourceAttr("langsmith_prompt.test", "tag_value_ids.0", "tv-1"),
				),
			},
			{
				// Metadata and manifest change together. The PATCH succeeds,
				// the commit is rejected.
				Config:      testAccPromptResourceConfigMetadata(handle, "second", "tv-2", "second"),
				ExpectError: regexp.MustCompile(`no\s+longer\s+this\s+repo`),
			},
			{
				// The manifest is back to the one the repo actually holds, so
				// the only thing that can still differ is the metadata the
				// failed apply had already written. An empty plan proves it
				// was kept.
				Config:             testAccPromptResourceConfigMetadata(handle, "second", "tv-2", "first"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func testAccPromptResourceConfigMetadata(handle, description, tagValueID, template string) string {
	return fmt.Sprintf(`
resource "langsmith_prompt" "test" {
  repo_handle   = %[1]q
  is_public     = false
  description   = %[2]q
  tag_value_ids = [%[3]q]

  manifest = jsonencode({
    lc       = 1
    template = %[4]q
    type     = "constructor"
  })
}
`, handle, description, tagValueID, template)
}

// TestAccPromptResource_commitOptions covers commit_description and
// skip_webhooks (#82): both ride on every commit the resource writes, and
// changing one on its own must not write a commit.
func TestAccPromptResource_commitOptions(t *testing.T) {
	handle := "tf-prompt-commit-options"
	stub := &promptStub{handle: handle}
	stub.start(t)

	commits := func(want int) resource.TestCheckFunc {
		return func(*terraform.State) error {
			if got := len(stub.recorded()); got != want {
				return fmt.Errorf("expected %d commits, got %d", want, got)
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPromptResourceConfigCommitOptions(handle, "first",
					`commit_description = "initial version"
  skip_webhooks      = true`),
				Check: resource.ComposeAggregateTestCheckFunc(
					commits(1),
					func(*terraform.State) error {
						got := stub.recorded()[0]
						if got.Description != "initial version" || !got.SkipWebhooks {
							return fmt.Errorf("first commit sent description=%q skip_webhooks=%t, want %q and true",
								got.Description, got.SkipWebhooks, "initial version")
						}
						return nil
					},
				),
			},
			{
				// Only the commit options change: state updates, no commit.
				Config: testAccPromptResourceConfigCommitOptions(handle, "first",
					`commit_description = "reworded"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					commits(1),
					resource.TestCheckResourceAttr("langsmith_prompt.test", "commit_description", "reworded"),
					resource.TestCheckNoResourceAttr("langsmith_prompt.test", "skip_webhooks"),
				),
			},
			{
				Config: testAccPromptResourceConfigCommitOptions(handle, "second",
					`commit_description = "reworded"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					commits(2),
					func(*terraform.State) error {
						got := stub.recorded()[1]
						if got.Description != "reworded" || got.SkipWebhooks {
							return fmt.Errorf("second commit sent description=%q skip_webhooks=%t, want %q and false",
								got.Description, got.SkipWebhooks, "reworded")
						}
						return nil
					},
				),
			},
		},
	})
}

// TestAccPromptResource_parentCommitOverride covers parent_commit (#83): when
// set it replaces the state-derived parent on a manifest commit, changing it
// on its own writes nothing, and it is refused at create time.
func TestAccPromptResource_parentCommitOverride(t *testing.T) {
	handle := "tf-prompt-parent-override"
	stub := &promptStub{handle: handle}
	stub.start(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Refused before the repo is created, so nothing is left behind.
				Config:      testAccPromptResourceConfigCommitOptions(handle, "first", `parent_commit = "abc123"`),
				ExpectError: regexp.MustCompile(`parent_commit\s+cannot\s+be\s+set\s+when\s+the\s+prompt\s+is\s+created`),
			},
			{
				Config: testAccPromptResourceConfigCommitOptions(handle, "first", ""),
				Check:  resource.TestCheckResourceAttr("langsmith_prompt.test", "commit_hash", "hash-1"),
			},
			{
				// Setting parent_commit alone does not commit.
				Config: testAccPromptResourceConfigCommitOptions(handle, "first", `parent_commit = "other-head"`),
				Check: func(*terraform.State) error {
					if got := len(stub.recorded()); got != 1 {
						return fmt.Errorf("expected 1 commit, got %d", got)
					}
					return nil
				},
			},
			{
				Config: testAccPromptResourceConfigCommitOptions(handle, "second", `parent_commit = "other-head"`),
				Check: func(*terraform.State) error {
					got := stub.recorded()
					if len(got) != 2 {
						return fmt.Errorf("expected 2 commits, got %d", len(got))
					}
					if got[1].ParentCommit != "other-head" {
						return fmt.Errorf("commit sent parent_commit %q, want the configured %q", got[1].ParentCommit, "other-head")
					}
					return nil
				},
			},
			{
				// Removed again: back to the head recorded in state.
				Config: testAccPromptResourceConfigCommitOptions(handle, "third", ""),
				Check: func(*terraform.State) error {
					got := stub.recorded()
					if len(got) != 3 {
						return fmt.Errorf("expected 3 commits, got %d", len(got))
					}
					if got[2].ParentCommit != "hash-2" {
						return fmt.Errorf("commit sent parent_commit %q, want the state head %q", got[2].ParentCommit, "hash-2")
					}
					return nil
				},
			},
		},
	})
}

// TestAccPromptResource_parentCommitOverrideConflict checks the advice on a
// 409 for a configured parent_commit. LangSmith accepts only the repo's
// current head as a parent, and refreshing cannot change a pinned value, so
// the hint has to point at the setting rather than at terraform plan.
func TestAccPromptResource_parentCommitOverrideConflict(t *testing.T) {
	handle := "tf-prompt-parent-override-conflict"
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
				Config: testAccPromptResourceConfigCommitOptions(handle, "first", ""),
			},
			{
				Config:      testAccPromptResourceConfigCommitOptions(handle, "second", `parent_commit = "old-commit"`),
				ExpectError: regexp.MustCompile(`parent_commit\s+set\s+in\s+configuration\s+\(old-commit\)\s+is\s+not\s+a\s+current\s+head`),
			},
		},
	})
}

func testAccPromptResourceConfigCommitOptions(handle, template, extra string) string {
	return fmt.Sprintf(`
resource "langsmith_prompt" "test" {
  repo_handle = %[1]q
  is_public   = false
  description = "commit options test"

  manifest = jsonencode({
    lc       = 1
    template = %[2]q
    type     = "constructor"
  })

  %[3]s
}
`, handle, template, extra)
}

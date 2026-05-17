// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

func TestParseAnnotationQueueReviewerImportID(t *testing.T) {
	t.Parallel()
	q, id, ok := parseAnnotationQueueReviewerImportID("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee/ffffffff-ffff-ffff-ffff-ffffffffffff")
	if !ok || q != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" || id != "ffffffff-ffff-ffff-ffff-ffffffffffff" {
		t.Fatalf("unexpected: ok=%v q=%q id=%q", ok, q, id)
	}
	if _, _, ok := parseAnnotationQueueReviewerImportID("no-slash"); ok {
		t.Fatal("expected not ok")
	}
	if _, _, ok := parseAnnotationQueueReviewerImportID("/only-right"); ok {
		t.Fatal("expected not ok")
	}
}

// testAccPreCheckAnnotationQueueReviewersAPI skips when the platform reviewer API
// is not available for this credential (for example 403 on self-hosted or
// restricted keys) or when the deployment does not expose the route (405).
func testAccPreCheckAnnotationQueueReviewersAPI(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	apiURL := os.Getenv("LANGSMITH_API_URL")
	if apiURL == "" {
		apiURL = "https://api.smith.langchain.com"
	}
	c := client.NewClient(
		apiURL,
		os.Getenv("LANGSMITH_API_KEY"),
		os.Getenv("LANGSMITH_TENANT_ID"),
		os.Getenv("LANGSMITH_ORGANIZATION_ID"),
		"terraform-provider-langsmith-acc-annotation-queue-reviewer-precheck",
	)
	// Synthetic UUIDs: expect 404 when the API is enabled for this key, or 403 when not.
	var probe map[string]any
	err := c.Get(ctx, "/v1/platform/annotation-queues/00000000-0000-4000-8000-000000000001/reviewers/00000000-0000-4000-8000-000000000002", nil, &probe)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.StatusCode {
			case http.StatusUnauthorized, http.StatusForbidden, http.StatusMethodNotAllowed:
				t.Skipf("annotation queue reviewers API returned %d; skipping acceptance test", apiErr.StatusCode)
			case http.StatusNotFound:
				return
			}
		}
		t.Fatalf("annotation queue reviewers API pre-check: %v", err)
	}
}

// testAccFirstWorkspaceMemberIdentityID returns a workspace member identity id
// suitable for reviewer membership, or skips the test when none are listed.
func testAccFirstWorkspaceMemberIdentityID(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	apiURL := os.Getenv("LANGSMITH_API_URL")
	if apiURL == "" {
		apiURL = "https://api.smith.langchain.com"
	}
	c := client.NewClient(
		apiURL,
		os.Getenv("LANGSMITH_API_KEY"),
		os.Getenv("LANGSMITH_TENANT_ID"),
		os.Getenv("LANGSMITH_ORGANIZATION_ID"),
		"terraform-provider-langsmith-acc-annotation-queue-reviewer-members",
	)
	var list struct {
		Members []struct {
			ID string `json:"id"`
		} `json:"members"`
	}
	if err := c.Get(ctx, "/api/v1/workspaces/current/members", nil, &list); err != nil {
		t.Fatalf("listing workspace members: %v", err)
	}
	if len(list.Members) == 0 {
		t.Skip("no workspace members returned; cannot pick identity_id for annotation queue reviewer test")
	}
	return list.Members[0].ID
}

func TestAccAnnotationQueueReviewerResource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	rName := fmt.Sprintf("tf-acc-aqr-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))

	testAccPreCheck(t)
	testAccPreCheckAnnotationQueueReviewersAPI(t)
	identityID := testAccFirstWorkspaceMemberIdentityID(t)

	resource.Test(t, resource.TestCase{
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

func testAccAnnotationQueueReviewerResourceConfig(queueName, identityID string) string {
	return fmt.Sprintf(`
resource "langsmith_annotation_queue" "q" {
  name = %[1]q
}

resource "langsmith_annotation_queue_reviewer" "test" {
  queue_id    = langsmith_annotation_queue.q.id
  identity_id = %[2]q
}
`, queueName, identityID)
}

// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	clientpkg "github.com/bogware/terraform-provider-langsmith/internal/client"
)

const memberBase = "/api/v1/orgs/current/members"

// stubMember answers 404 for every path except the one named, so each test
// pins down exactly which endpoint the helper ended up using.
func stubMember(t *testing.T, liveMethod, livePath string, liveStatus int) (*clientpkg.Client, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		if r.Method == liveMethod && r.URL.Path == livePath {
			w.WriteHeader(liveStatus)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"not found"}`))
	}))
	t.Cleanup(srv.Close)

	c := clientpkg.NewClient(srv.URL, "k", "ws", "ua", false, nil)
	c.MaxRetries = 0
	return c, &seen
}

func TestDeleteMember(t *testing.T) {
	cases := []struct {
		name       string
		pending    bool
		livePath   string // the only path the server accepts
		wantCalls  []string
		wantErrNil bool
	}{
		{
			name:       "accepted member deleted directly",
			pending:    false,
			livePath:   memberBase + "/m-1",
			wantCalls:  []string{"DELETE " + memberBase + "/m-1"},
			wantErrNil: true,
		},
		{
			name:       "pending member deleted at the pending endpoint",
			pending:    true,
			livePath:   memberBase + "/m-1/pending",
			wantCalls:  []string{"DELETE " + memberBase + "/m-1/pending"},
			wantErrNil: true,
		},
		{
			// The invitation was accepted between the last refresh and the
			// destroy, so the pending endpoint is gone and the helper must fall
			// through to the accepted one.
			name:     "stale pending state falls back",
			pending:  true,
			livePath: memberBase + "/m-1",
			wantCalls: []string{
				"DELETE " + memberBase + "/m-1/pending",
				"DELETE " + memberBase + "/m-1",
			},
			wantErrNil: true,
		},
		{
			// Already gone either way: a delete has nothing to complain about.
			name:    "absent from both endpoints succeeds",
			pending: false,
			wantCalls: []string{
				"DELETE " + memberBase + "/m-1",
				"DELETE " + memberBase + "/m-1/pending",
			},
			wantErrNil: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, seen := stubMember(t, http.MethodDelete, tc.livePath, http.StatusOK)

			err := deleteMember(context.Background(), c, memberBase, "m-1", tc.pending)
			if (err == nil) != tc.wantErrNil {
				t.Fatalf("err = %v, wantNil = %v", err, tc.wantErrNil)
			}
			if len(*seen) != len(tc.wantCalls) {
				t.Fatalf("calls = %v, want %v", *seen, tc.wantCalls)
			}
			for i, want := range tc.wantCalls {
				if (*seen)[i] != want {
					t.Fatalf("call %d = %q, want %q", i, (*seen)[i], want)
				}
			}
		})
	}
}

func TestPatchMember(t *testing.T) {
	t.Run("stale pending state falls back", func(t *testing.T) {
		c, seen := stubMember(t, http.MethodPatch, memberBase+"/m-1", http.StatusOK)

		if err := patchMember(context.Background(), c, memberBase, "m-1", true, map[string]string{}, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{
			"PATCH " + memberBase + "/m-1/pending",
			"PATCH " + memberBase + "/m-1",
		}
		if len(*seen) != len(want) || (*seen)[0] != want[0] || (*seen)[1] != want[1] {
			t.Fatalf("calls = %v, want %v", *seen, want)
		}
	})

	t.Run("absent from both endpoints is an error", func(t *testing.T) {
		// Unlike a delete, an update that finds nothing to update has failed.
		c, _ := stubMember(t, http.MethodPatch, "/nowhere", http.StatusOK)

		if err := patchMember(context.Background(), c, memberBase, "m-1", false, map[string]string{}, nil); err == nil {
			t.Fatal("expected an error when neither endpoint exists")
		}
	})
}

// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"
)

func TestDecodeFeedbackIngestTokenPostResponse_object(t *testing.T) {
	raw := []byte(`{"id":"a","url":"u","expires_at":"t","feedback_key":"k"}`)
	got, err := decodeFeedbackIngestTokenPostResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "a" || got.URL != "u" || got.ExpiresAt != "t" || got.FeedbackKey != "k" {
		t.Fatalf("unexpected decode: %+v", got)
	}
}

func TestDecodeFeedbackIngestTokenPostResponse_array(t *testing.T) {
	raw := []byte(`[{"id":"b","url":"u2","expires_at":"t2","feedback_key":"k2"}]`)
	got, err := decodeFeedbackIngestTokenPostResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "b" {
		t.Fatalf("want first element, got %+v", got)
	}
}

func TestDecodeFeedbackIngestTokenPostResponse_errors(t *testing.T) {
	if _, err := decodeFeedbackIngestTokenPostResponse([]byte(`[]`)); err == nil {
		t.Fatal("expected error for empty array")
	}
	if _, err := decodeFeedbackIngestTokenPostResponse([]byte(``)); err == nil {
		t.Fatal("expected error for empty body")
	}
}

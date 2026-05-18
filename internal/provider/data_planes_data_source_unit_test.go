// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"testing"
)

func TestRawJSONToStringValue(t *testing.T) {
	t.Run("returns null for missing raw json", func(t *testing.T) {
		got := rawJSONToStringValue(json.RawMessage{})
		if !got.IsNull() {
			t.Fatalf("expected null string value, got %q", got.ValueString())
		}
	})

	t.Run("returns string for present raw json", func(t *testing.T) {
		got := rawJSONToStringValue(json.RawMessage(`{"ready":true}`))
		if got.IsNull() || got.ValueString() != `{"ready":true}` {
			t.Fatalf("expected JSON string value, got %q", got.ValueString())
		}
	})
}

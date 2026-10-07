// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"
)

func TestJSONNumericValueStringSemanticEquals(t *testing.T) {
	tests := []struct {
		name  string
		prior string
		new   string
		want  bool
	}{
		{"identical", `{"cache_read":0.0000001}`, `{"cache_read":0.0000001}`, true},
		{"decimal vs scientific, same value", `{"cache_read":0.0000001}`, `{"cache_read":1e-7}`, true},
		{"scientific vs decimal, same value", `{"cache_read":1e-7}`, `{"cache_read":0.0000001}`, true},
		{"whitespace and key order differ", `{"a":1,"b":2}`, "{\n  \"b\": 2,\n  \"a\": 1\n}", true},
		{"genuinely different values", `{"cache_read":0.0000001}`, `{"cache_read":0.0000002}`, false},
		{"extra key is a real difference", `{"cache_read":1e-7}`, `{"cache_read":1e-7,"cache_write":2e-7}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prior := newJSONNumericValue(tt.prior)
			next := newJSONNumericValue(tt.new)

			got, diags := prior.StringSemanticEquals(context.Background(), next)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if got != tt.want {
				t.Errorf("StringSemanticEquals(%q, %q) = %v, want %v", tt.prior, tt.new, got, tt.want)
			}
		})
	}
}

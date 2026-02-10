// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestNormalizeJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"invalid json", "not json", "not json"},
		{"simple object", `{"b":1,"a":2}`, `{"a":2,"b":1}`},
		{"compact whitespace", `{ "a" : 1 }`, `{"a":1}`},
		{"nested object", `{"z":{"b":2,"a":1},"a":3}`, `{"a":3,"z":{"a":1,"b":2}}`},
		{"array preserved", `[3,1,2]`, `[3,1,2]`},
		{"null literal", `null`, `null`},
		{"string value", `"hello"`, `"hello"`},
		{"number", `42`, `42`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeJSON(tc.input)
			if got != tc.expected {
				t.Errorf("normalizeJSON(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestJsonStringValue(t *testing.T) {
	tests := []struct {
		name       string
		input      json.RawMessage
		expectNull bool
		expected   string
	}{
		{"nil", nil, true, ""},
		{"empty", json.RawMessage{}, true, ""},
		{"null literal", json.RawMessage(`null`), true, ""},
		{"object", json.RawMessage(`{"b":1,"a":2}`), false, `{"a":2,"b":1}`},
		{"array", json.RawMessage(`[1,2]`), false, `[1,2]`},
		{"string", json.RawMessage(`"hello"`), false, `"hello"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := jsonStringValue(tc.input)
			if tc.expectNull {
				if !got.IsNull() {
					t.Errorf("jsonStringValue(%q) should be null, got %q", string(tc.input), got.ValueString())
				}
			} else {
				if got.IsNull() {
					t.Errorf("jsonStringValue(%q) should not be null", string(tc.input))
				} else if got.ValueString() != tc.expected {
					t.Errorf("jsonStringValue(%q) = %q, want %q", string(tc.input), got.ValueString(), tc.expected)
				}
			}
		})
	}
}

func TestJsonEmptyArrayIsNull(t *testing.T) {
	tests := []struct {
		name       string
		input      json.RawMessage
		expectNull bool
		expected   string
	}{
		{"nil", nil, true, ""},
		{"empty", json.RawMessage{}, true, ""},
		{"null literal", json.RawMessage(`null`), true, ""},
		{"empty array", json.RawMessage(`[]`), true, ""},
		{"empty object", json.RawMessage(`{}`), true, ""},
		{"non-empty array", json.RawMessage(`[1,2]`), false, `[1,2]`},
		{"non-empty object", json.RawMessage(`{"a":1}`), false, `{"a":1}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := jsonEmptyArrayIsNull(tc.input)
			if tc.expectNull {
				if !got.IsNull() {
					t.Errorf("jsonEmptyArrayIsNull(%q) should be null, got %q", string(tc.input), got.ValueString())
				}
			} else {
				if got.IsNull() {
					t.Errorf("jsonEmptyArrayIsNull(%q) should not be null", string(tc.input))
				} else if got.ValueString() != tc.expected {
					t.Errorf("jsonEmptyArrayIsNull(%q) = %q, want %q", string(tc.input), got.ValueString(), tc.expected)
				}
			}
		})
	}
}

func TestSetOptionalString(t *testing.T) {
	var dst *string

	// Null value should not set.
	setOptionalString(&dst, types.StringNull())
	if dst != nil {
		t.Errorf("setOptionalString with null should leave dst nil")
	}

	// Unknown value should not set.
	setOptionalString(&dst, types.StringUnknown())
	if dst != nil {
		t.Errorf("setOptionalString with unknown should leave dst nil")
	}

	// Real value should set.
	setOptionalString(&dst, types.StringValue("hello"))
	if dst == nil || *dst != "hello" {
		t.Errorf("setOptionalString with value should set dst to 'hello', got %v", dst)
	}
}

func TestSetStateOptionalString(t *testing.T) {
	var dst types.String

	// nil should become null.
	setStateOptionalString(&dst, nil)
	if !dst.IsNull() {
		t.Errorf("setStateOptionalString with nil should set null")
	}

	// Non-nil should become value.
	v := "hello"
	setStateOptionalString(&dst, &v)
	if dst.IsNull() || dst.ValueString() != "hello" {
		t.Errorf("setStateOptionalString with value should set 'hello', got %v", dst)
	}
}

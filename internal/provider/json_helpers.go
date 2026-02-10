// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// normalizeJSON takes a JSON string and returns a normalized version with
// sorted keys and compact formatting. This prevents phantom Terraform diffs
// caused by key reordering or whitespace differences in API responses.
// Returns the original string if it is not valid JSON.
func normalizeJSON(s string) string {
	if s == "" {
		return s
	}
	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	normalized, err := json.Marshal(v)
	if err != nil {
		return s
	}
	return string(normalized)
}

// jsonStringValue creates a types.String from a json.RawMessage, normalizing
// the JSON to prevent drift. Returns types.StringNull() for nil, empty, or
// "null" values.
func jsonStringValue(raw json.RawMessage) types.String {
	if len(raw) == 0 || string(raw) == "null" {
		return types.StringNull()
	}
	return types.StringValue(normalizeJSON(string(raw)))
}

// jsonEmptyArrayIsNull returns types.StringNull() if the JSON is an empty
// array "[]" or empty object "{}", otherwise normalizes and returns.
func jsonEmptyArrayIsNull(raw json.RawMessage) types.String {
	s := string(raw)
	if len(raw) == 0 || s == "null" || s == "[]" || s == "{}" {
		return types.StringNull()
	}
	return types.StringValue(normalizeJSON(s))
}

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

// stripJSONKey removes a specific key from a JSON object and returns the
// result as a types.String. If the raw JSON is nil/null/empty or if stripping
// the key produces an empty object, falls back to the saved state value.
// This is used when the API injects keys (like dataset_split) that aren't
// part of the user's configuration.
func stripJSONKey(raw json.RawMessage, key string, saved types.String) types.String {
	if len(raw) == 0 || string(raw) == "null" {
		return saved
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return saved
	}
	delete(m, key)
	if len(m) == 0 {
		return saved
	}
	out, err := json.Marshal(m)
	if err != nil {
		return saved
	}
	return types.StringValue(string(out))
}

// jsonPreserveConfigSubset keeps the saved value when the API response only
// adds to it. LangSmith expands several JSON attributes on the way in — a code
// evaluator submitted as {"code": "..."} comes back as
// {"code": "...", "language": "python"} — and writing the expanded copy into
// state leaves a diff against the configuration on every plan, forever.
//
// The saved value wins when every key it specifies is present and equal in the
// response, recursively; anything the server added on top is ignored. A genuine
// server-side change still shows up, because then some key the user specified
// differs and the response is used instead.
func jsonPreserveConfigSubset(raw json.RawMessage, saved types.String) types.String {
	fresh := jsonEmptyArrayIsNull(raw)
	if saved.IsNull() || saved.IsUnknown() || fresh.IsNull() {
		return fresh
	}
	var savedVal, freshVal interface{}
	if err := json.Unmarshal([]byte(saved.ValueString()), &savedVal); err != nil {
		return fresh
	}
	if err := json.Unmarshal([]byte(fresh.ValueString()), &freshVal); err != nil {
		return fresh
	}
	if jsonSubset(savedVal, freshVal) {
		return types.StringValue(normalizeJSON(saved.ValueString()))
	}
	return fresh
}

// jsonSubset reports whether want is contained in got: objects may carry extra
// keys, arrays must line up element for element, scalars must be equal.
func jsonSubset(want, got interface{}) bool {
	switch w := want.(type) {
	case map[string]interface{}:
		g, ok := got.(map[string]interface{})
		if !ok {
			return false
		}
		for k, wv := range w {
			gv, present := g[k]
			if !present || !jsonSubset(wv, gv) {
				return false
			}
		}
		return true
	case []interface{}:
		g, ok := got.([]interface{})
		if !ok || len(g) != len(w) {
			return false
		}
		for i := range w {
			if !jsonSubset(w[i], g[i]) {
				return false
			}
		}
		return true
	default:
		return want == got
	}
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

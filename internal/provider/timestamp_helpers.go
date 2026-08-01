// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// timestampLayouts are the shapes LangSmith returns for a timestamp. The same
// instant comes back differently depending on the endpoint: a create response
// may omit the zone entirely ("2026-07-31T23:42:26.982665") where the
// corresponding GET includes it ("2026-07-31T23:42:26.982665+00:00").
var timestampLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
}

func parseTimestamp(s string) (time.Time, bool) {
	for _, layout := range timestampLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// normalizeTimestamp maps an API timestamp onto Terraform state without letting
// a change of spelling look like a change of value.
//
// When the saved value (the prior state, or the configured value on create)
// denotes the same instant as the API value, the saved spelling is kept, so a
// practitioner who writes "2026-01-01T00:00:00+00:00" does not see a perpetual
// diff against a server that echoes "2026-01-01T00:00:00Z".
//
// Otherwise the API value is canonicalised to UTC RFC 3339. That matters for
// import: import starts from empty state, so without a canonical form the
// imported value would be compared against a differently-spelled original and
// ImportStateVerify would fail even though nothing changed.
//
// A timestamp in an unrecognised format is passed through untouched.
func normalizeTimestamp(apiValue string, saved types.String) types.String {
	if apiValue == "" {
		return types.StringNull()
	}
	parsed, ok := parseTimestamp(apiValue)
	if !ok {
		return types.StringValue(apiValue)
	}
	if !saved.IsNull() && !saved.IsUnknown() {
		if savedTime, ok := parseTimestamp(saved.ValueString()); ok && savedTime.Equal(parsed) {
			return saved
		}
	}
	return types.StringValue(parsed.UTC().Format(time.RFC3339Nano))
}

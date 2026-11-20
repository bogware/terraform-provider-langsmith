// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestChartSeriesV1Detection covers #97: LangSmith rejects V1 chart series,
// so the provider spots V1 keys and explains the V2 translation instead of
// passing the bare 400 through.
func TestChartSeriesV1Detection(t *testing.T) {
	const v2 = `[{"name":"Runs","metric_definition":{"type":"count"},` +
		`"filter_definition":{"source_type":"tracing_project","project_ids":["p1"]}}]`

	cases := []struct {
		name   string
		series string
		want   string // substring of the rendered usage; "" means no V1 usage
	}{
		{name: "V2 series is clean", series: v2},
		{
			name:   "legacy metric and filters are flagged with their index",
			series: `[{"name":"Runs","metric":"run_count","filters":{"session":["p1"]}}]`,
			want:   "series[0] (metric, filters)",
		},
		{
			name:   "only the offending series are named",
			series: `[` + v2[1:len(v2)-1] + `,{"name":"Score","feedback_key":"correctness"}]`,
			want:   "series[1] (feedback_key)",
		},
		{
			// The server's own echo materializes every V1 key as null.
			name:   "V1 keys set to null are not usage",
			series: `[{"name":"Runs","metric":null,"filters":null,"feedback_key":null,"metric_definition":{"type":"count"}}]`,
		},
		{name: "malformed JSON is left to the API", series: `[{`},
		{name: "a non-array is left to the API", series: `{"metric":"run_count"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			usage := chartSeriesV1Usage(tc.series)
			got := describeChartSeriesV1Usage(usage)
			if tc.want == "" {
				if len(usage) != 0 {
					t.Fatalf("expected no V1 usage, got %q", got)
				}
				if chartSeriesV1Error(types.StringValue(tc.series)) != nil {
					t.Fatal("expected no error diagnostic")
				}
				return
			}
			if got != tc.want {
				t.Fatalf("usage = %q, want %q", got, tc.want)
			}
			d := chartSeriesV1Error(types.StringValue(tc.series))
			if d == nil || d.Severity() != diag.SeverityError {
				t.Fatalf("expected an error diagnostic, got %v", d)
			}
			if !strings.Contains(d.Detail(), "metric_definition") || !strings.Contains(d.Detail(), tc.want) {
				t.Fatalf("error detail should name the series and the V2 replacement, got:\n%s", d.Detail())
			}
		})
	}

	if chartSeriesV1Error(types.StringUnknown()) != nil || chartSeriesV1Error(types.StringNull()) != nil {
		t.Fatal("unknown or null series must not produce an error")
	}
}

// TestChartSeriesV1Validator checks the plan-time diagnostic is a warning:
// an untouched V1 chart can still be read, so it must not block plans.
func TestChartSeriesV1Validator(t *testing.T) {
	run := func(v types.String) diag.Diagnostics {
		var resp validator.StringResponse
		chartSeriesV1Validator{}.ValidateString(context.Background(),
			validator.StringRequest{Path: path.Root("series"), ConfigValue: v}, &resp)
		return resp.Diagnostics
	}

	d := run(types.StringValue(`[{"name":"Runs","metric":"run_count"}]`))
	if d.HasError() || d.WarningsCount() != 1 {
		t.Fatalf("want exactly one warning and no error, got %v", d)
	}
	if d := run(types.StringValue(`[{"name":"Runs","metric_definition":{"type":"count"}}]`)); len(d) != 0 {
		t.Fatalf("V2 series should produce no diagnostics, got %v", d)
	}
	if d := run(types.StringUnknown()); len(d) != 0 {
		t.Fatalf("unknown series should produce no diagnostics, got %v", d)
	}
}

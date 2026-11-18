// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// chartSeriesV1Keys are the series keys of LangSmith's V1 chart format. Since
// November 2026 LangSmith Cloud rejects any series that uses them, on create
// and on update, with `400 V1 chart series cannot be saved. Use V2 chart
// definitions instead.` (#97). The V2 replacements are metric_definition and
// filter_definition.
var chartSeriesV1Keys = []string{"metric", "filters", "feedback_key", "project_metric"}

// chartSeriesV1Usage lists, per series index, the V1 keys a series JSON array
// sets to a non-null value. It returns nil for anything it cannot parse; the
// API is left to report malformed JSON.
func chartSeriesV1Usage(series string) map[int][]string {
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(series), &entries); err != nil {
		return nil
	}
	var usage map[int][]string
	for i, entry := range entries {
		for _, k := range chartSeriesV1Keys {
			if v, ok := entry[k]; ok && string(v) != "null" {
				if usage == nil {
					usage = map[int][]string{}
				}
				usage[i] = append(usage[i], k)
			}
		}
	}
	return usage
}

// describeChartSeriesV1Usage renders the usage map as "series[0] (metric,
// filters), series[2] (feedback_key)".
func describeChartSeriesV1Usage(usage map[int][]string) string {
	idx := make([]int, 0, len(usage))
	for i := range usage {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	parts := make([]string, 0, len(idx))
	for _, i := range idx {
		parts = append(parts, fmt.Sprintf("series[%d] (%s)", i, strings.Join(usage[i], ", ")))
	}
	return strings.Join(parts, ", ")
}

const chartSeriesV2Guide = "Rewrite each series with metric_definition and filter_definition. For example:\n\n" +
	`  metric  = "run_count"              ->  metric_definition = { type = "count" }` + "\n" +
	`  metric  = "latency_p50"            ->  metric_definition = { type = "percentile", field = "latency_seconds", params = { p = 0.5 } }` + "\n" +
	`  filters = { session = [<project>] } ->  filter_definition = { source_type = "tracing_project", project_ids = [<project>] }` + "\n\n" +
	"See the series attribute in the langsmith_chart documentation for the full V2 shape."

// chartSeriesV1Error returns an error diagnostic when series still uses V1
// keys, so Create and Update stop with a translation guide instead of
// passing LangSmith's bare 400 through. Returns nil when series is fine or
// not yet known.
func chartSeriesV1Error(series types.String) diag.Diagnostic {
	if series.IsNull() || series.IsUnknown() {
		return nil
	}
	usage := chartSeriesV1Usage(series.ValueString())
	if len(usage) == 0 {
		return nil
	}
	return diag.NewAttributeErrorDiagnostic(path.Root("series"),
		"Chart series uses the retired V1 format",
		fmt.Sprintf("%s set V1 chart keys. LangSmith no longer accepts V1 series on create or update; it "+
			"answers \"V1 chart series cannot be saved. Use V2 chart definitions instead.\" Nothing was sent.\n\n%s",
			describeChartSeriesV1Usage(usage), chartSeriesV2Guide))
}

// chartSeriesV1Validator warns at plan time when series uses V1 keys. It is a
// warning rather than an error on purpose: an existing V1 chart can still be
// read, so an untouched one must not block every plan in the configuration.
// The error comes from chartSeriesV1Error, once a create or update would
// actually send the series.
type chartSeriesV1Validator struct{}

var _ validator.String = chartSeriesV1Validator{}

func (chartSeriesV1Validator) Description(context.Context) string {
	return "warns when series uses V1 chart keys, which LangSmith no longer accepts"
}

func (v chartSeriesV1Validator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (chartSeriesV1Validator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	usage := chartSeriesV1Usage(req.ConfigValue.ValueString())
	if len(usage) == 0 {
		return
	}
	resp.Diagnostics.AddAttributeWarning(req.Path,
		"Chart series uses the retired V1 format",
		fmt.Sprintf("%s set V1 chart keys. LangSmith rejects V1 series, so creating this chart or applying "+
			"any change to it will fail until the series is rewritten.\n\n%s",
			describeChartSeriesV1Usage(usage), chartSeriesV2Guide))
}

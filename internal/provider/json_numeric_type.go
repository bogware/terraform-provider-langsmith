// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/attr/xattr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// jsonNumericType is an attr.Type for JSON strings whose semantic equality
// treats differing number notations (e.g. "1e-07" vs "0.0000001") for the
// same value as equal, on top of the usual whitespace/key-order
// insensitivity. jsontypes.Normalized (terraform-plugin-framework-jsontypes)
// deliberately does NOT do this -- it decodes numbers with json.Number to
// avoid float64 precision loss, which preserves exactly the notation
// difference this type exists to ignore. Needed for prompt_cost_details /
// completion_cost_details, where LangSmith's API and Terraform's own
// jsonencode() format the same small float differently.
type jsonNumericType struct {
	basetypes.StringType
}

var _ basetypes.StringTypable = jsonNumericType{}

func (t jsonNumericType) String() string {
	return "provider.jsonNumericType"
}

func (t jsonNumericType) ValueType(_ context.Context) attr.Value {
	return jsonNumericValue{}
}

func (t jsonNumericType) Equal(o attr.Type) bool {
	other, ok := o.(jsonNumericType)
	if !ok {
		return false
	}
	return t.StringType.Equal(other.StringType)
}

func (t jsonNumericType) ValueFromString(_ context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	return jsonNumericValue{StringValue: in}, nil
}

func (t jsonNumericType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}

	stringValue, ok := attrValue.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type of %T", attrValue)
	}

	stringValuable, diags := t.ValueFromString(ctx, stringValue)
	if diags.HasError() {
		return nil, fmt.Errorf("unexpected error converting StringValue to StringValuable: %v", diags)
	}

	return stringValuable, nil
}

// jsonNumericValue is the attr.Value counterpart to jsonNumericType.
type jsonNumericValue struct {
	basetypes.StringValue
}

var (
	_ basetypes.StringValuable                   = jsonNumericValue{}
	_ basetypes.StringValuableWithSemanticEquals = jsonNumericValue{}
	_ xattr.ValidateableAttribute                = jsonNumericValue{}
)

func (v jsonNumericValue) Type(_ context.Context) attr.Type {
	return jsonNumericType{}
}

func (v jsonNumericValue) Equal(o attr.Value) bool {
	other, ok := o.(jsonNumericValue)
	if !ok {
		return false
	}
	return v.StringValue.Equal(other.StringValue)
}

// StringSemanticEquals decodes both sides with plain json.Unmarshal (numbers
// as float64, the default), unlike jsontypes.Normalized's json.Number
// decoding -- so "1e-07" and "0.0000001" compare equal here precisely
// because the float64 round-trip collapses them to the same value.
func (v jsonNumericValue) StringSemanticEquals(_ context.Context, newValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	newValue, ok := newValuable.(jsonNumericValue)
	if !ok {
		diags.AddError(
			"Semantic Equality Check Error",
			"An unexpected value type was received while performing semantic equality checks. "+
				"Please report this to the provider developers.\n\n"+
				"Expected Value Type: "+fmt.Sprintf("%T", v)+"\n"+
				"Got Value Type: "+fmt.Sprintf("%T", newValuable),
		)
		return false, diags
	}

	var a, b interface{}
	if err := json.Unmarshal([]byte(v.ValueString()), &a); err != nil {
		diags.AddError("Semantic Equality Check Error", "Could not parse prior JSON value: "+err.Error())
		return false, diags
	}
	if err := json.Unmarshal([]byte(newValue.ValueString()), &b); err != nil {
		diags.AddError("Semantic Equality Check Error", "Could not parse new JSON value: "+err.Error())
		return false, diags
	}

	aBytes, err := json.Marshal(a)
	if err != nil {
		diags.AddError("Semantic Equality Check Error", "Could not re-encode prior JSON value: "+err.Error())
		return false, diags
	}
	bBytes, err := json.Marshal(b)
	if err != nil {
		diags.AddError("Semantic Equality Check Error", "Could not re-encode new JSON value: "+err.Error())
		return false, diags
	}

	return string(aBytes) == string(bBytes), diags
}

func (v jsonNumericValue) ValidateAttribute(_ context.Context, req xattr.ValidateAttributeRequest, resp *xattr.ValidateAttributeResponse) {
	if v.IsUnknown() || v.IsNull() {
		return
	}

	if !json.Valid([]byte(v.ValueString())) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid JSON String Value",
			"A string value was provided that is not valid JSON string format (RFC 7159).\n\n"+
				"Given Value: "+v.ValueString()+"\n",
		)
	}
}

func newJSONNumericNull() jsonNumericValue {
	return jsonNumericValue{StringValue: basetypes.NewStringNull()}
}

func newJSONNumericValue(value string) jsonNumericValue {
	return jsonNumericValue{StringValue: basetypes.NewStringValue(value)}
}

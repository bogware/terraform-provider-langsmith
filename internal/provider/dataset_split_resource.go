// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &DatasetSplitResource{}
	_ resource.ResourceWithImportState = &DatasetSplitResource{}
)

func NewDatasetSplitResource() resource.Resource {
	return &DatasetSplitResource{}
}

type DatasetSplitResource struct {
	client *client.Client
}

type DatasetSplitResourceModel struct {
	ID         types.String `tfsdk:"id"`
	DatasetID  types.String `tfsdk:"dataset_id"`
	Name       types.String `tfsdk:"name"`
	ExampleIDs types.Set    `tfsdk:"example_ids"`
}

type splitMutation struct {
	SplitName string   `json:"split_name"`
	Examples  []string `json:"examples"`
	Remove    bool     `json:"remove"`
}

func (r *DatasetSplitResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset_split"
}

func (r *DatasetSplitResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Declares the membership of a named split within a dataset. The LangSmith API does not expose per-example split membership, so this resource can verify the split *name* exists but cannot detect drift in `example_ids`. Updates compute add/remove diffs against state; destroy removes every example currently in state from the split (which deletes the split server-side when empty).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"dataset_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the parent dataset.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Split name (e.g. `train`, `test`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"example_ids": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Set of example UUIDs that belong to this split.",
			},
		},
	}
}

func (r *DatasetSplitResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

func readStringSet(ctx context.Context, s types.Set) ([]string, error) {
	if s.IsNull() || s.IsUnknown() {
		return nil, nil
	}
	var out []string
	diags := s.ElementsAs(ctx, &out, false)
	if diags.HasError() {
		return nil, fmt.Errorf("%s", diags.Errors())
	}
	return out, nil
}

func (r *DatasetSplitResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DatasetSplitResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ids, err := readStringSet(ctx, data.ExampleIDs)
	if err != nil {
		resp.Diagnostics.AddError("Error reading example_ids", err.Error())
		return
	}
	body := splitMutation{SplitName: data.Name.ValueString(), Examples: ids, Remove: false}
	var result []string
	if err := r.client.Put(ctx, "/api/v1/datasets/"+data.DatasetID.ValueString()+"/splits", body, &result); err != nil {
		resp.Diagnostics.AddError("Error creating dataset split", err.Error())
		return
	}
	data.ID = types.StringValue(data.DatasetID.ValueString() + ":" + data.Name.ValueString())
	tflog.Trace(ctx, "created dataset split", map[string]interface{}{"id": data.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasetSplitResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DatasetSplitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var names []string
	if err := r.client.Get(ctx, "/api/v1/datasets/"+data.DatasetID.ValueString()+"/splits", nil, &names); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading dataset splits", err.Error())
		return
	}
	wanted := data.Name.ValueString()
	for _, n := range names {
		if n == wanted {
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *DatasetSplitResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state, plan DatasetSplitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	oldIDs, _ := readStringSet(ctx, state.ExampleIDs)
	newIDs, _ := readStringSet(ctx, plan.ExampleIDs)
	oldSet := make(map[string]struct{}, len(oldIDs))
	newSet := make(map[string]struct{}, len(newIDs))
	for _, v := range oldIDs {
		oldSet[v] = struct{}{}
	}
	for _, v := range newIDs {
		newSet[v] = struct{}{}
	}
	var toAdd, toRemove []string
	for _, v := range newIDs {
		if _, ok := oldSet[v]; !ok {
			toAdd = append(toAdd, v)
		}
	}
	for _, v := range oldIDs {
		if _, ok := newSet[v]; !ok {
			toRemove = append(toRemove, v)
		}
	}

	path := "/api/v1/datasets/" + plan.DatasetID.ValueString() + "/splits"
	if len(toAdd) > 0 {
		var ignore []string
		if err := r.client.Put(ctx, path, splitMutation{SplitName: plan.Name.ValueString(), Examples: toAdd, Remove: false}, &ignore); err != nil {
			resp.Diagnostics.AddError("Error adding examples to split", err.Error())
			return
		}
	}
	if len(toRemove) > 0 {
		var ignore []string
		if err := r.client.Put(ctx, path, splitMutation{SplitName: plan.Name.ValueString(), Examples: toRemove, Remove: true}, &ignore); err != nil {
			resp.Diagnostics.AddError("Error removing examples from split", err.Error())
			return
		}
	}
	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DatasetSplitResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DatasetSplitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ids, _ := readStringSet(ctx, data.ExampleIDs)
	if len(ids) == 0 {
		return
	}
	var ignore []string
	if err := r.client.Put(ctx, "/api/v1/datasets/"+data.DatasetID.ValueString()+"/splits",
		splitMutation{SplitName: data.Name.ValueString(), Examples: ids, Remove: true}, &ignore); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting dataset split", err.Error())
		return
	}
}

func (r *DatasetSplitResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"<dataset_id>:<split_name>\". Note: example_ids cannot be recovered from the API and will be empty after import.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("dataset_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

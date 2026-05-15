// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	_ resource.Resource                = &HubEnvironmentResource{}
	_ resource.ResourceWithImportState = &HubEnvironmentResource{}
)

func NewHubEnvironmentResource() resource.Resource {
	return &HubEnvironmentResource{}
}

type HubEnvironmentResource struct {
	client *client.Client
}

type HubEnvironmentResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Environments types.List   `tfsdk:"environments"`
}

type hubEnvEntry struct {
	Name string `json:"name"`
}

type hubEnvRequest struct {
	Environments []hubEnvEntry `json:"environments"`
}

type hubEnvAPI struct {
	ID           string        `json:"id"`
	Environments []hubEnvEntry `json:"environments"`
}

var hubEnvEntryObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType}}

func (r *HubEnvironmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hub_environment"
}

func (r *HubEnvironmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the workspace's prompt-hub environment list (between 1 and 4 named environments such as `staging` and `production`). Environments group prompt-tag promotions for hub repos. Note: the API exposes one per-workspace record holding all environments; removing one environment requires updating this resource without it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"environments": schema.ListNestedAttribute{
				Required:            true,
				MarkdownDescription: "Between 1 and 4 environment entries.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{Required: true, MarkdownDescription: "Environment name (1-64 chars)."},
					},
				},
			},
		},
	}
}

func (r *HubEnvironmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func readHubEnvEntries(ctx context.Context, list types.List, diags *diag.Diagnostics) []hubEnvEntry {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}
	var items []struct {
		Name types.String `tfsdk:"name"`
	}
	diags.Append(list.ElementsAs(ctx, &items, false)...)
	if diags.HasError() {
		return nil
	}
	out := make([]hubEnvEntry, 0, len(items))
	for _, it := range items {
		out = append(out, hubEnvEntry{Name: it.Name.ValueString()})
	}
	return out
}

func buildHubEnvList(entries []hubEnvEntry, diags *diag.Diagnostics) types.List {
	elems := make([]attr.Value, 0, len(entries))
	for _, e := range entries {
		ov, d := types.ObjectValue(hubEnvEntryObjectType.AttrTypes, map[string]attr.Value{"name": types.StringValue(e.Name)})
		diags.Append(d...)
		elems = append(elems, ov)
	}
	list, d := types.ListValue(hubEnvEntryObjectType, elems)
	diags.Append(d...)
	return list
}

func (r *HubEnvironmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data HubEnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	entries := readHubEnvEntries(ctx, data.Environments, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var api hubEnvAPI
	if err := r.client.Post(ctx, "/api/v1/hub/environments", hubEnvRequest{Environments: entries}, &api); err != nil {
		resp.Diagnostics.AddError("Error creating hub environments", err.Error())
		return
	}
	data.ID = types.StringValue(api.ID)
	data.Environments = buildHubEnvList(api.Environments, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Trace(ctx, "created hub environments", map[string]interface{}{"id": api.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HubEnvironmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data HubEnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api hubEnvAPI
	if err := r.client.Get(ctx, "/api/v1/hub/environments", nil, &api); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading hub environments", err.Error())
		return
	}
	if api.ID == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	data.ID = types.StringValue(api.ID)
	data.Environments = buildHubEnvList(api.Environments, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HubEnvironmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data HubEnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	entries := readHubEnvEntries(ctx, data.Environments, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var api hubEnvAPI
	if err := r.client.Patch(ctx, "/api/v1/hub/environments/"+data.ID.ValueString(), hubEnvRequest{Environments: entries}, &api); err != nil {
		resp.Diagnostics.AddError("Error updating hub environments", err.Error())
		return
	}
	data.ID = types.StringValue(api.ID)
	data.Environments = buildHubEnvList(api.Environments, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *HubEnvironmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data HubEnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/api/v1/hub/environments/"+data.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting hub environments", err.Error())
		return
	}
}

func (r *HubEnvironmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

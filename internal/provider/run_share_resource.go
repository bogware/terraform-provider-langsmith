// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

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
	_ resource.Resource                = &RunShareResource{}
	_ resource.ResourceWithImportState = &RunShareResource{}
)

// NewRunShareResource returns a resource for managing the public share state
// of a run trace.
func NewRunShareResource() resource.Resource {
	return &RunShareResource{}
}

// RunShareResource manages the public share state of a single run.
type RunShareResource struct {
	client *client.Client
}

// RunShareResourceModel maps the Terraform schema for a run share.
type RunShareResourceModel struct {
	RunID       types.String `tfsdk:"run_id"`
	ShareToken  types.String `tfsdk:"share_token"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
}

// runShareAPI is the wire format returned by the run share endpoints.
type runShareAPI struct {
	RunID      string `json:"run_id"`
	ShareToken string `json:"share_token"`
}

// normalizeWorkspaceID resolves a null/unknown workspace_id to a known value:
// the explicitly supplied value is kept, otherwise the effective client
// workspace or null. The share API does not return a workspace_id.
func (r *RunShareResource) normalizeWorkspaceID(data *RunShareResourceModel) {
	if data.WorkspaceID.IsNull() || data.WorkspaceID.IsUnknown() {
		if ws := effectiveClient(r.client, data.WorkspaceID).WorkspaceID; ws != "" {
			data.WorkspaceID = types.StringValue(ws)
		} else {
			data.WorkspaceID = types.StringNull()
		}
	}
}

func (r *RunShareResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_run_share"
}

func (r *RunShareResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the public share state of a run. Creating this resource generates a share token; destroying it unshares the run.",
		Attributes: map[string]schema.Attribute{
			"run_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the run to share.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"share_token": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The generated share token (used as the path segment in shared URLs).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "If set, overrides the provider-level `workspace_id` for all API calls made by this resource.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *RunShareResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *RunShareResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RunShareResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api runShareAPI
	if err := effectiveClient(r.client, data.WorkspaceID).Put(ctx, "/api/v1/runs/"+data.RunID.ValueString()+"/share", nil, &api); err != nil {
		resp.Diagnostics.AddError("Error sharing run", err.Error())
		return
	}
	data.ShareToken = types.StringValue(api.ShareToken)
	r.normalizeWorkspaceID(&data)
	tflog.Trace(ctx, "shared run", map[string]interface{}{"run_id": api.RunID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RunShareResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RunShareResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api *runShareAPI
	if err := effectiveClient(r.client, data.WorkspaceID).Get(ctx, "/api/v1/runs/"+data.RunID.ValueString()+"/share", nil, &api); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading run share state", err.Error())
		return
	}
	if api == nil || api.ShareToken == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	data.ShareToken = types.StringValue(api.ShareToken)
	r.normalizeWorkspaceID(&data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RunShareResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// run_id requires replacement, so the only updates that reach here are
	// no-ops (e.g. workspace_id reconciliation); re-issuing PUT is idempotent.
	var data RunShareResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api runShareAPI
	if err := effectiveClient(r.client, data.WorkspaceID).Put(ctx, "/api/v1/runs/"+data.RunID.ValueString()+"/share", nil, &api); err != nil {
		resp.Diagnostics.AddError("Error updating run share state", err.Error())
		return
	}
	data.ShareToken = types.StringValue(api.ShareToken)
	r.normalizeWorkspaceID(&data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RunShareResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RunShareResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := effectiveClient(r.client, data.WorkspaceID).Delete(ctx, "/api/v1/runs/"+data.RunID.ValueString()+"/share"); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error unsharing run", err.Error())
		return
	}
}

func (r *RunShareResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("run_id"), req, resp)
}

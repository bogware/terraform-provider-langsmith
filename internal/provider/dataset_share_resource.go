// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/url"

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
	_ resource.Resource                = &DatasetShareResource{}
	_ resource.ResourceWithImportState = &DatasetShareResource{}
)

func NewDatasetShareResource() resource.Resource {
	return &DatasetShareResource{}
}

type DatasetShareResource struct {
	client *client.Client
}

type DatasetShareResourceModel struct {
	DatasetID     types.String `tfsdk:"dataset_id"`
	ShareToken    types.String `tfsdk:"share_token"`
	ShareProjects types.Bool   `tfsdk:"share_projects"`
}

type datasetShareAPI struct {
	DatasetID  string `json:"dataset_id"`
	ShareToken string `json:"share_token"`
}

func (r *DatasetShareResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset_share"
}

func (r *DatasetShareResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the public share state of a dataset. Creating this resource generates a share token; destroying it unshares the dataset.",
		Attributes: map[string]schema.Attribute{
			"dataset_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the dataset to share.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"share_projects": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Whether to also expose linked projects in the share.",
				PlanModifiers:       []planmodifier.Bool{},
			},
			"share_token": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The generated share token (used as the path segment in shared URLs).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *DatasetShareResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DatasetShareResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DatasetShareResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	q := url.Values{}
	if !data.ShareProjects.IsNull() && !data.ShareProjects.IsUnknown() {
		if data.ShareProjects.ValueBool() {
			q.Set("share_projects", "true")
		} else {
			q.Set("share_projects", "false")
		}
	}
	var api datasetShareAPI
	if err := r.client.PutWithQuery(ctx, "/api/v1/datasets/"+data.DatasetID.ValueString()+"/share", q, nil, &api); err != nil {
		resp.Diagnostics.AddError("Error sharing dataset", err.Error())
		return
	}
	data.ShareToken = types.StringValue(api.ShareToken)
	tflog.Trace(ctx, "shared dataset", map[string]interface{}{"dataset_id": api.DatasetID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasetShareResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DatasetShareResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api *datasetShareAPI
	if err := r.client.Get(ctx, "/api/v1/datasets/"+data.DatasetID.ValueString()+"/share", nil, &api); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading dataset share state", err.Error())
		return
	}
	if api == nil || api.ShareToken == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	data.ShareToken = types.StringValue(api.ShareToken)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasetShareResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// share_projects is the only mutable attribute; re-issuing PUT is idempotent.
	var data DatasetShareResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	q := url.Values{}
	if !data.ShareProjects.IsNull() && !data.ShareProjects.IsUnknown() {
		if data.ShareProjects.ValueBool() {
			q.Set("share_projects", "true")
		} else {
			q.Set("share_projects", "false")
		}
	}
	var api datasetShareAPI
	if err := r.client.PutWithQuery(ctx, "/api/v1/datasets/"+data.DatasetID.ValueString()+"/share", q, nil, &api); err != nil {
		resp.Diagnostics.AddError("Error updating dataset share state", err.Error())
		return
	}
	data.ShareToken = types.StringValue(api.ShareToken)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DatasetShareResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DatasetShareResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/api/v1/datasets/"+data.DatasetID.ValueString()+"/share"); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error unsharing dataset", err.Error())
		return
	}
}

func (r *DatasetShareResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("dataset_id"), req, resp)
}

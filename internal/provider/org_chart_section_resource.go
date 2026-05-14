// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &OrgChartSectionResource{}
	_ resource.ResourceWithImportState = &OrgChartSectionResource{}
)

func NewOrgChartSectionResource() resource.Resource {
	return &OrgChartSectionResource{}
}

type OrgChartSectionResource struct {
	client *client.Client
}

type OrgChartSectionResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Title       types.String `tfsdk:"title"`
	Description types.String `tfsdk:"description"`
	Index       types.Int64  `tfsdk:"index"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

type orgChartSectionCreateRequest struct {
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	Index       *int64  `json:"index,omitempty"`
}

type orgChartSectionUpdateRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Index       *int64  `json:"index,omitempty"`
}

type orgChartSectionAPIResponse struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Index       *int64  `json:"index"`
	CreatedAt   *string `json:"created_at"`
	ModifiedAt  *string `json:"modified_at"`
}

func (r *OrgChartSectionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_chart_section"
}

func (r *OrgChartSectionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith organization-level chart section (`/api/v1/org-charts/*`). " +
			"The provider must set `organization_id` or `LANGSMITH_ORGANIZATION_ID` so the API receives `X-Organization-Id`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the org chart section.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"title": schema.StringAttribute{
				MarkdownDescription: "The title of the org chart section.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the org chart section.",
				Optional:            true,
			},
			"index": schema.Int64Attribute{
				MarkdownDescription: "The display order index.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last update timestamp.",
				Computed:            true,
			},
		},
	}
}

func (r *OrgChartSectionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *OrgChartSectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OrgChartSectionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := orgChartSectionCreateRequest{
		Title: data.Title.ValueString(),
	}
	setOptionalString(&body.Description, data.Description)
	if !data.Index.IsNull() && !data.Index.IsUnknown() {
		v := data.Index.ValueInt64()
		body.Index = &v
	}

	var result orgChartSectionAPIResponse
	err := r.client.Post(ctx, "/api/v1/org-charts/section", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating org chart section", err.Error())
		return
	}

	mapOrgChartSectionResponseToState(&data, &result)
	tflog.Trace(ctx, "created org chart section resource", map[string]interface{}{"id": result.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrgChartSectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrgChartSectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	savedCreatedAt := data.CreatedAt
	savedUpdatedAt := data.UpdatedAt

	body := struct {
		OmitData  bool   `json:"omit_data"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}{OmitData: true, StartTime: "2020-01-01T00:00:00Z", EndTime: "2020-01-01T00:01:00Z"}
	var result orgChartSectionAPIResponse
	err := r.client.Post(ctx, "/api/v1/org-charts/section/"+data.ID.ValueString(), body, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading org chart section", err.Error())
		return
	}

	mapOrgChartSectionResponseToState(&data, &result)
	data.CreatedAt = savedCreatedAt
	data.UpdatedAt = savedUpdatedAt
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrgChartSectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OrgChartSectionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	savedCreatedAt := data.CreatedAt

	body := orgChartSectionUpdateRequest{}
	setOptionalString(&body.Title, data.Title)
	setOptionalString(&body.Description, data.Description)
	if !data.Index.IsNull() && !data.Index.IsUnknown() {
		v := data.Index.ValueInt64()
		body.Index = &v
	}

	var result orgChartSectionAPIResponse
	err := r.client.Patch(ctx, "/api/v1/org-charts/section/"+data.ID.ValueString(), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error updating org chart section", err.Error())
		return
	}

	mapOrgChartSectionResponseToState(&data, &result)
	data.CreatedAt = savedCreatedAt
	tflog.Trace(ctx, "updated org chart section resource", map[string]interface{}{"id": result.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrgChartSectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OrgChartSectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/org-charts/section/"+data.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting org chart section", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted org chart section resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *OrgChartSectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func mapOrgChartSectionResponseToState(data *OrgChartSectionResourceModel, result *orgChartSectionAPIResponse) {
	data.ID = types.StringValue(result.ID)
	data.Title = types.StringValue(result.Title)
	setStateOptionalString(&data.CreatedAt, result.CreatedAt)
	setStateOptionalString(&data.UpdatedAt, result.ModifiedAt)
	setStateOptionalString(&data.Description, result.Description)
	if result.Index != nil {
		data.Index = types.Int64Value(*result.Index)
	} else {
		data.Index = types.Int64Null()
	}
}

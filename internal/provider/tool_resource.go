// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
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
	_ resource.Resource                = &ToolResource{}
	_ resource.ResourceWithImportState = &ToolResource{}
)

func NewToolResource() resource.Resource {
	return &ToolResource{}
}

type ToolResource struct {
	client *client.Client
}

type ToolResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Handle      types.String `tfsdk:"handle"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Parameters  types.String `tfsdk:"parameters"`
	Returns     types.String `tfsdk:"returns"`
	Metadata    types.String `tfsdk:"metadata"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	TenantID    types.String `tfsdk:"tenant_id"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

type toolCreate struct {
	Handle      string                 `json:"handle"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Returns     map[string]interface{} `json:"returns,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Enabled     *bool                  `json:"enabled,omitempty"`
}

type toolUpdate struct {
	Name        *string                `json:"name,omitempty"`
	Description *string                `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Returns     map[string]interface{} `json:"returns,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Enabled     *bool                  `json:"enabled,omitempty"`
}

type toolAPI struct {
	ID          string                 `json:"id"`
	Handle      string                 `json:"handle"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Returns     map[string]interface{} `json:"returns"`
	Metadata    map[string]interface{} `json:"metadata"`
	Enabled     bool                   `json:"enabled"`
	TenantID    string                 `json:"tenant_id"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
}

func (r *ToolResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tool"
}

func (r *ToolResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith platform-level tool definition. Tools are addressed by their immutable `handle`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"handle": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Stable handle used to reference the tool. Immutable after create.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name":        schema.StringAttribute{Required: true, MarkdownDescription: "Display name."},
			"description": schema.StringAttribute{Required: true, MarkdownDescription: "Tool description shown to model callers."},
			"parameters": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "JSON-encoded JSON Schema object describing the tool's input parameters.",
			},
			"returns": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "JSON-encoded JSON Schema object describing the tool's return type.",
			},
			"metadata": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "JSON-encoded free-form metadata.",
			},
			"enabled":    schema.BoolAttribute{Optional: true, Computed: true, MarkdownDescription: "Whether the tool is enabled."},
			"tenant_id":  schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"created_at": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"updated_at": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *ToolResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func unmarshalJSONObject(s types.String, field string, dest *map[string]interface{}, errs func(string, string)) bool {
	if s.IsNull() || s.IsUnknown() || s.ValueString() == "" {
		return true
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s.ValueString()), &m); err != nil {
		errs("Invalid "+field+" JSON", err.Error())
		return false
	}
	*dest = m
	return true
}

func (r *ToolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ToolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := toolCreate{
		Handle:      data.Handle.ValueString(),
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueString(),
	}
	if !unmarshalJSONObject(data.Parameters, "parameters", &body.Parameters, resp.Diagnostics.AddError) {
		return
	}
	if !unmarshalJSONObject(data.Returns, "returns", &body.Returns, resp.Diagnostics.AddError) {
		return
	}
	if !unmarshalJSONObject(data.Metadata, "metadata", &body.Metadata, resp.Diagnostics.AddError) {
		return
	}
	if !data.Enabled.IsNull() && !data.Enabled.IsUnknown() {
		v := data.Enabled.ValueBool()
		body.Enabled = &v
	}

	var api toolAPI
	if err := r.client.Post(ctx, "/v1/platform/tools", body, &api); err != nil {
		resp.Diagnostics.AddError("Error creating tool", err.Error())
		return
	}
	r.mapResponse(&api, &data)
	tflog.Trace(ctx, "created tool", map[string]interface{}{"handle": api.Handle})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ToolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ToolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api toolAPI
	if err := r.client.Get(ctx, "/v1/platform/tools/"+data.Handle.ValueString(), nil, &api); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading tool", err.Error())
		return
	}
	r.mapResponse(&api, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ToolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ToolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := toolUpdate{}
	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		v := data.Name.ValueString()
		body.Name = &v
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		v := data.Description.ValueString()
		body.Description = &v
	}
	if !unmarshalJSONObject(data.Parameters, "parameters", &body.Parameters, resp.Diagnostics.AddError) {
		return
	}
	if !unmarshalJSONObject(data.Returns, "returns", &body.Returns, resp.Diagnostics.AddError) {
		return
	}
	if !unmarshalJSONObject(data.Metadata, "metadata", &body.Metadata, resp.Diagnostics.AddError) {
		return
	}
	if !data.Enabled.IsNull() && !data.Enabled.IsUnknown() {
		v := data.Enabled.ValueBool()
		body.Enabled = &v
	}

	var api toolAPI
	if err := r.client.Patch(ctx, "/v1/platform/tools/"+data.Handle.ValueString(), body, &api); err != nil {
		resp.Diagnostics.AddError("Error updating tool", err.Error())
		return
	}
	r.mapResponse(&api, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ToolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ToolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/v1/platform/tools/"+data.Handle.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting tool", err.Error())
		return
	}
}

func (r *ToolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("handle"), req, resp)
}

func (r *ToolResource) mapResponse(api *toolAPI, data *ToolResourceModel) {
	data.ID = types.StringValue(api.ID)
	data.Handle = types.StringValue(api.Handle)
	data.Name = types.StringValue(api.Name)
	data.Description = types.StringValue(api.Description)
	if len(api.Parameters) > 0 {
		b, _ := json.Marshal(api.Parameters)
		data.Parameters = jsonStringValue(b)
	} else {
		data.Parameters = types.StringValue("{}")
	}
	if len(api.Returns) > 0 {
		b, _ := json.Marshal(api.Returns)
		data.Returns = jsonStringValue(b)
	} else {
		data.Returns = types.StringNull()
	}
	if len(api.Metadata) > 0 {
		b, _ := json.Marshal(api.Metadata)
		data.Metadata = jsonStringValue(b)
	} else {
		data.Metadata = types.StringNull()
	}
	data.Enabled = types.BoolValue(api.Enabled)
	data.TenantID = types.StringValue(api.TenantID)
	data.CreatedAt = types.StringValue(api.CreatedAt)
	data.UpdatedAt = types.StringValue(api.UpdatedAt)
}

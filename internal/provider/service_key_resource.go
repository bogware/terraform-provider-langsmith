// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &ServiceKeyResource{}
	_ resource.ResourceWithImportState = &ServiceKeyResource{}
)

// NewServiceKeyResource constructs a fresh ServiceKeyResource. Like a one-time
// telegraph code, the full key is only revealed at creation.
func NewServiceKeyResource() resource.Resource {
	return &ServiceKeyResource{}
}

// ServiceKeyResource manages a LangSmith service key (API key) — the
// credential that gets you through the door at the Long Branch.
type ServiceKeyResource struct {
	client *client.Client
}

// ServiceKeyResourceModel holds the Terraform state for a service key. The
// full key is sensitive and only surfaces once — like a whispered password at
// the saloon door.
type ServiceKeyResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Description        types.String `tfsdk:"description"`
	ReadOnly           types.Bool   `tfsdk:"read_only"`
	ShortKey           types.String `tfsdk:"short_key"`
	Key                types.String `tfsdk:"key"`
	CreatedAt          types.String `tfsdk:"created_at"`
	ExpiresAt          types.String `tfsdk:"expires_at"`
	DefaultWorkspaceID types.String `tfsdk:"default_workspace_id"`
	RoleID             types.String `tfsdk:"role_id"`
}

// serviceKeyAPICreateRequest is the wire format for minting a new service key.
// Optional fields ride along only when the caller pins them on — like a badge
// you choose to wear into Dodge City.
type serviceKeyAPICreateRequest struct {
	Description        string  `json:"description"`
	ReadOnly           bool    `json:"read_only"`
	ExpiresAt          *string `json:"expires_at,omitempty"`
	DefaultWorkspaceID *string `json:"default_workspace_id,omitempty"`
	RoleID             *string `json:"role_id,omitempty"`
}

// serviceKeyAPICreateResponse is the one-time response that includes the full
// API key — guard it like gold dust.
type serviceKeyAPICreateResponse struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	ReadOnly    bool   `json:"read_only"`
	ShortKey    string `json:"short_key"`
	Key         string `json:"key"`
	CreatedAt   string `json:"created_at"`
}

// serviceKeyAPIListItem is a single service key from the list response. The
// full key is long gone — only the short key remains as a calling card.
type serviceKeyAPIListItem struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	ReadOnly    bool   `json:"read_only"`
	ShortKey    string `json:"short_key"`
	CreatedAt   string `json:"created_at"`
}

// serviceKeyAPIListResponse is the full roster of service keys, minus their
// secrets.
type serviceKeyAPIListResponse []serviceKeyAPIListItem

func (r *ServiceKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_key"
}

func (r *ServiceKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith service key (API key). Service keys cannot be updated; changing any mutable attribute will force recreation. The full API key is only available at creation time.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the service key.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description for the service key.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("Default API key"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"read_only": schema.BoolAttribute{
				MarkdownDescription: "Whether the service key is read-only.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"short_key": schema.StringAttribute{
				MarkdownDescription: "The shortened version of the API key for display purposes.",
				Computed:            true,
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "The full API key. Only available at creation time; will be empty after import.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The creation timestamp of the service key.",
				Computed:            true,
			},
			"expires_at": schema.StringAttribute{
				MarkdownDescription: "ISO 8601 timestamp when the service key expires.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"default_workspace_id": schema.StringAttribute{
				MarkdownDescription: "The default workspace ID for the service key.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role_id": schema.StringAttribute{
				MarkdownDescription: "The role ID to assign to the service key.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *ServiceKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}

	r.client = c
}

func (r *ServiceKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServiceKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := serviceKeyAPICreateRequest{
		Description: data.Description.ValueString(),
		ReadOnly:    data.ReadOnly.ValueBool(),
	}

	// Strap on the optional gear before riding out — only if the caller packed it.
	if !data.ExpiresAt.IsNull() && !data.ExpiresAt.IsUnknown() {
		v := data.ExpiresAt.ValueString()
		body.ExpiresAt = &v
	}
	if !data.DefaultWorkspaceID.IsNull() && !data.DefaultWorkspaceID.IsUnknown() {
		v := data.DefaultWorkspaceID.ValueString()
		body.DefaultWorkspaceID = &v
	}
	if !data.RoleID.IsNull() && !data.RoleID.IsUnknown() {
		v := data.RoleID.ValueString()
		body.RoleID = &v
	}

	var result serviceKeyAPICreateResponse
	err := r.client.Post(ctx, "/api/v1/orgs/current/service-keys", body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error creating service key", err.Error())
		return
	}

	data.ID = types.StringValue(result.ID)
	data.Description = types.StringValue(result.Description)
	data.ReadOnly = types.BoolValue(result.ReadOnly)
	data.ShortKey = types.StringValue(result.ShortKey)
	data.Key = types.StringValue(result.Key)
	data.CreatedAt = types.StringValue(result.CreatedAt)

	tflog.Trace(ctx, "created service key resource", map[string]interface{}{"id": result.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var listResult serviceKeyAPIListResponse
	err := r.client.Get(ctx, "/api/v1/orgs/current/service-keys", nil, &listResult)
	if err != nil {
		resp.Diagnostics.AddError("Error reading service keys", err.Error())
		return
	}

	var found *serviceKeyAPIListItem
	for _, sk := range listResult {
		if sk.ID == data.ID.ValueString() {
			found = &sk
			break
		}
	}

	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.ID = types.StringValue(found.ID)
	data.Description = types.StringValue(found.Description)
	data.ReadOnly = types.BoolValue(found.ReadOnly)
	data.ShortKey = types.StringValue(found.ShortKey)
	data.CreatedAt = types.StringValue(found.CreatedAt)
	// The full key is never returned on read — that was a one-time reveal.
	// UseStateForUnknown keeps the original safe in state.

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ServiceKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Service keys cannot be updated. This is unexpected — all mutable attributes should have RequiresReplace set.",
	)
}

func (r *ServiceKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServiceKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, "/api/v1/orgs/current/service-keys/"+data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting service key", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted service key resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *ServiceKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

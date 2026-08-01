// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var (
	_ resource.Resource                = &DataPlaneResource{}
	_ resource.ResourceWithImportState = &DataPlaneResource{}
)

func NewDataPlaneResource() resource.Resource {
	return &DataPlaneResource{}
}

type DataPlaneResource struct {
	client *client.Client
}

type DataPlaneResourceModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Region     types.String `tfsdk:"region"`
	ExternalID types.String `tfsdk:"external_id"`
	RoleARN    types.String `tfsdk:"role_arn"`
	VPCCIDR    types.String `tfsdk:"vpc_cidr"`
	APIURL     types.String `tfsdk:"api_url"`
	Status     types.String `tfsdk:"status"`
	Workspaces types.String `tfsdk:"workspaces"`
	CreatedAt  types.String `tfsdk:"created_at"`
}

type dataPlaneCreateRequest struct {
	ExternalID string  `json:"external_id"`
	Name       *string `json:"name,omitempty"`
	Region     *string `json:"region,omitempty"`
	RoleARN    *string `json:"role_arn,omitempty"`
	VPCCIDR    *string `json:"vpc_cidr,omitempty"`
}

type dataPlaneResourceAPI struct {
	APIURL     string          `json:"api_url"`
	CreatedAt  string          `json:"created_at"`
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Region     string          `json:"region"`
	Status     string          `json:"status"`
	Workspaces json.RawMessage `json:"workspaces"`
}

type dataPlaneResourceListResponse struct {
	DataPlanes []dataPlaneResourceAPI `json:"data_planes"`
}

func (r *DataPlaneResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_data_plane"
}

func (r *DataPlaneResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Provisions a hybrid/self-hosted (BYOC) data plane for the current LangSmith organization. The API accepts the request and returns the data plane in status `requested`; provisioning continues asynchronously. Requires BYOC to be enabled on the org and org-admin permissions. **The API offers no update endpoint**, so every configurable attribute forces replacement. `terraform destroy` calls the delete endpoint; on a deployment that does not expose it (HTTP 404 or 405) the resource is still removed from state and a warning is raised — the data plane keeps running and must then be deprovisioned through LangSmith support.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "UUID of the data plane.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Display name of the data plane.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"region": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Cloud region the data plane is provisioned in (e.g. `us-east-1`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
			},
			"external_id": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Value LangSmith presents as `ExternalId` when assuming `role_arn`. Must match the `ExternalId` condition in the customer role's trust policy. Not returned by the API, so it cannot be refreshed from the server.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"role_arn": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "ARN of the customer IAM role LangSmith assumes to provision the data plane. Not returned by the API.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"vpc_cidr": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "CIDR block for the data plane VPC. Not returned by the API.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"api_url": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "API URL of the data plane once provisioned.",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Lifecycle status: `requested`, `provisioning`, `provisioning_failed`, `active`, `inactive`, `deprovisioning`, `deleted`, or `revoked`. Starts at `requested` and advances asynchronously.",
			},
			"workspaces": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "JSON-encoded list of workspaces (`[{\"id\": ..., \"name\": ...}]`) attached to this data plane.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *DataPlaneResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DataPlaneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DataPlaneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := dataPlaneCreateRequest{ExternalID: data.ExternalID.ValueString()}
	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		v := data.Name.ValueString()
		body.Name = &v
	}
	if !data.Region.IsNull() && !data.Region.IsUnknown() {
		v := data.Region.ValueString()
		body.Region = &v
	}
	if !data.RoleARN.IsNull() && !data.RoleARN.IsUnknown() {
		v := data.RoleARN.ValueString()
		body.RoleARN = &v
	}
	if !data.VPCCIDR.IsNull() && !data.VPCCIDR.IsUnknown() {
		v := data.VPCCIDR.ValueString()
		body.VPCCIDR = &v
	}

	var api dataPlaneResourceAPI
	if err := r.client.Post(ctx, "/orgs/current/data-planes", body, &api); err != nil {
		resp.Diagnostics.AddError("Error creating data plane", err.Error())
		return
	}
	r.mapResponse(&api, &data)
	tflog.Trace(ctx, "created data plane", map[string]interface{}{"id": api.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DataPlaneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DataPlaneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var list dataPlaneResourceListResponse
	if err := r.client.Get(ctx, "/orgs/current/data-planes", nil, &list); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error listing data planes", err.Error())
		return
	}
	for i := range list.DataPlanes {
		if list.DataPlanes[i].ID == data.ID.ValueString() {
			r.mapResponse(&list.DataPlanes[i], &data)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *DataPlaneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update Not Supported", "Data planes cannot be updated; all configurable attributes are marked RequiresReplace.")
}

func (r *DataPlaneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DataPlaneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(r.deleteDataPlane(ctx, data.ID.ValueString())...)
}

// deleteDataPlane calls the delete endpoint and maps the outcome to
// diagnostics. A deployment without the endpoint must not fail the destroy —
// the user would be stuck with a resource Terraform can never remove — so 404
// and 405 degrade to a warning while any other error is surfaced.
func (r *DataPlaneResource) deleteDataPlane(ctx context.Context, id string) diag.Diagnostics {
	var diags diag.Diagnostics

	err := r.client.Delete(ctx, "/orgs/current/data-planes/"+id)
	switch {
	case err == nil:
		tflog.Trace(ctx, "deleted data plane", map[string]interface{}{"id": id})
	case client.IsNotFound(err):
		// Already gone, or this deployment predates the delete endpoint. Both are
		// indistinguishable from here (a missing route and a missing object both
		// answer 404), so fall back to the warning rather than failing a destroy
		// the user cannot otherwise complete.
		diags.AddWarning(
			"Data plane may not have been deprovisioned",
			"Deleting the data plane returned 404. It was removed from Terraform state, but if this deployment does not support deleting data planes it keeps running — check the LangSmith console and contact support to deprovision it.",
		)
	case isMethodNotAllowed(err):
		diags.AddWarning(
			"Data plane not deprovisioned",
			"This LangSmith deployment does not support deleting data planes (HTTP 405). The data plane was removed from Terraform state but keeps running. Contact LangSmith support to deprovision it.",
		)
	default:
		diags.AddError("Error deleting data plane", err.Error())
	}
	return diags
}

// isMethodNotAllowed reports whether err is a 405. A self-hosted frontend also
// answers 405 when a request falls through to the SPA, so this doubles as the
// "endpoint is not reachable here" case.
func isMethodNotAllowed(err error) bool {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusMethodNotAllowed
	}
	return false
}

func (r *DataPlaneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import is intentionally unsupported: external_id (required) and
	// role_arn / vpc_cidr are never returned by the API, so an imported
	// resource would always plan a replacement — and with no delete endpoint
	// the replaced data plane could never be cleaned up.
	resp.Diagnostics.AddError(
		"Import Not Supported",
		"Data planes cannot be imported because external_id, role_arn, and vpc_cidr are not readable from the API; any configured value would force a replacement that the API cannot clean up.",
	)
}

func (r *DataPlaneResource) mapResponse(api *dataPlaneResourceAPI, data *DataPlaneResourceModel) {
	data.ID = types.StringValue(api.ID)
	if api.Name != "" {
		data.Name = types.StringValue(api.Name)
	} else if data.Name.IsUnknown() {
		data.Name = types.StringNull()
	}
	if api.Region != "" {
		data.Region = types.StringValue(api.Region)
	} else if data.Region.IsUnknown() {
		data.Region = types.StringNull()
	}
	if api.APIURL != "" {
		data.APIURL = types.StringValue(api.APIURL)
	} else {
		data.APIURL = types.StringNull()
	}
	data.Status = types.StringValue(api.Status)
	data.Workspaces = jsonStringValue(api.Workspaces)
	if api.CreatedAt != "" {
		data.CreatedAt = types.StringValue(api.CreatedAt)
	} else {
		data.CreatedAt = types.StringNull()
	}
	// external_id, role_arn, and vpc_cidr are write-only: keep configured values.
}

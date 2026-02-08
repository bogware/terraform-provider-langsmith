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
	_ resource.Resource                = &SecretResource{}
	_ resource.ResourceWithImportState = &SecretResource{}
)

// NewSecretResource returns a new SecretResource -- the lockbox where
// you stash things you don't want the whole territory to know about.
func NewSecretResource() resource.Resource {
	return &SecretResource{}
}

// SecretResource manages workspace secrets in LangSmith. Like Marshal
// Dillon's private dispatch pouch, only the key name is ever shown;
// the contents stay hidden from prying eyes.
type SecretResource struct {
	client *client.Client
}

// SecretResourceModel describes the Terraform state for a workspace secret.
type SecretResourceModel struct {
	ID    types.String `tfsdk:"id"`
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

// secretUpsertItem is a single entry in the upsert array. The API expects
// a list of secrets -- even if you are only moving one head of cattle.
type secretUpsertItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// secretDeleteItem is the request to clear a secret off the books.
// Setting Value to nil marshals as JSON null, which tells the API
// this secret has ridden off into the sunset.
type secretDeleteItem struct {
	Key   string  `json:"key"`
	Value *string `json:"value"`
}

// secretKeyResponse is what the API reveals when you ask about secrets --
// just the key name and nothing more. The value stays under lock and key.
type secretKeyResponse struct {
	Key string `json:"key"`
}

func (r *SecretResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret"
}

func (r *SecretResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a LangSmith workspace secret (key/value pair). The value is write-only and never returned by the API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The identifier of the secret (same as the key name).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "The secret key name.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"value": schema.StringAttribute{
				MarkdownDescription: "The secret value. This is write-only and will not be returned by the API after being set.",
				Required:            true,
				Sensitive:           true,
			},
		},
	}
}

func (r *SecretResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *SecretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SecretResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := []secretUpsertItem{{
		Key:   data.Key.ValueString(),
		Value: data.Value.ValueString(),
	}}

	// The API is upsert-based and returns no body worth reading --
	// like sending a telegram to Dodge City and getting silence back.
	err := r.client.Post(ctx, "/api/v1/workspaces/current/secrets", body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error creating secret", err.Error())
		return
	}

	data.ID = types.StringValue(data.Key.ValueString())
	tflog.Trace(ctx, "created secret resource", map[string]interface{}{"key": data.Key.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SecretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SecretResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API only hands back a list of key names -- no individual
	// lookups, and definitely no values. You have to round up the
	// whole herd and find your steer by brand.
	var results []secretKeyResponse
	err := r.client.Get(ctx, "/api/v1/workspaces/current/secrets", nil, &results)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading secrets", err.Error())
		return
	}

	var found bool
	for _, s := range results {
		if s.Key == data.Key.ValueString() {
			found = true
			break
		}
	}

	if !found {
		// The secret has skipped town -- remove it from state.
		resp.State.RemoveResource(ctx)
		return
	}

	// The API never reveals the secret value, so we preserve whatever
	// the state already holds. Like a good bartender at the Long Branch
	// Saloon, we keep what we know to ourselves.
	data.ID = types.StringValue(data.Key.ValueString())

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SecretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SecretResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := []secretUpsertItem{{
		Key:   data.Key.ValueString(),
		Value: data.Value.ValueString(),
	}}

	// Same upsert trail as Create -- the API doesn't care whether
	// you're a newcomer or an old hand, it treats you the same.
	err := r.client.Post(ctx, "/api/v1/workspaces/current/secrets", body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error updating secret", err.Error())
		return
	}

	data.ID = types.StringValue(data.Key.ValueString())
	tflog.Trace(ctx, "updated secret resource", map[string]interface{}{"key": data.Key.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SecretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SecretResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// To delete a secret, we POST with value=null. It's the frontier
	// way of saying "this one's been buried at Boot Hill."
	body := []secretDeleteItem{{
		Key:   data.Key.ValueString(),
		Value: nil,
	}}

	err := r.client.Post(ctx, "/api/v1/workspaces/current/secrets", body, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting secret", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted secret resource", map[string]interface{}{"key": data.Key.ValueString()})
}

func (r *SecretResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import passes the ID through, which maps to the key name.
	// Fair warning: the secret value won't be available after import --
	// like asking Chester to recall last month's dispatch word-for-word.
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

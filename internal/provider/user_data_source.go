// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

var _ datasource.DataSource = &UserDataSource{}

// NewUserDataSource returns a data source which looks up a LangSmith user by
// email and returns the canonical user ID.
func NewUserDataSource() datasource.DataSource {
	return &UserDataSource{}
}

type UserDataSource struct {
	client *client.Client
}

type UserDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Email       types.String `tfsdk:"email"`
	DisplayName types.String `tfsdk:"display_name"`
}

func (d *UserDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *UserDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lookup a LangSmith user by email and return the user ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The LangSmith user ID corresponding to the provided email.",
				Computed:            true,
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "The email address to look up. Required.",
				Required:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "The display name of the user (if available).",
				Computed:            true,
			},
		},
	}
}

func (d *UserDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}

	d.client = c
}

func (d *UserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UserDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Email.IsNull() || data.Email.IsUnknown() {
		resp.Diagnostics.AddError("Missing Required Attribute", "The attribute \"email\" must be specified.")
		return
	}

	// Try organization members first (returns org-level member list including user_id).
	var orgResp struct {
		Members []struct {
			ID       string  `json:"id"`
			Email    *string `json:"email"`
			FullName *string `json:"full_name"`
			UserID   string  `json:"user_id"`
		} `json:"members"`
	}

	if err := d.client.Get(ctx, "/api/v1/orgs/current/members", nil, &orgResp); err == nil {
		for i := range orgResp.Members {
			m := orgResp.Members[i]
			if m.Email != nil && *m.Email == data.Email.ValueString() {
				data.ID = types.StringValue(m.UserID)
				data.Email = types.StringValue(*m.Email)
				if m.FullName != nil {
					data.DisplayName = types.StringValue(*m.FullName)
				} else {
					data.DisplayName = types.StringNull()
				}
				tflog.Trace(ctx, "read user data source (org)", map[string]interface{}{"user_id": m.UserID, "email": *m.Email})
				resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
				return
			}
		}
	}

	// Fallback: try workspace members.
	var wsResp struct {
		Members []struct {
			ID       string  `json:"id"`
			UserID   string  `json:"user_id"`
			Email    string  `json:"email"`
			FullName *string `json:"full_name"`
		} `json:"members"`
	}

	if err := d.client.Get(ctx, "/api/v1/workspaces/current/members", nil, &wsResp); err == nil {
		for i := range wsResp.Members {
			m := wsResp.Members[i]
			if m.Email == data.Email.ValueString() {
				data.ID = types.StringValue(m.UserID)
				data.Email = types.StringValue(m.Email)
				if m.FullName != nil {
					data.DisplayName = types.StringValue(*m.FullName)
				} else {
					data.DisplayName = types.StringNull()
				}
				tflog.Trace(ctx, "read user data source (workspace)", map[string]interface{}{"user_id": m.UserID, "email": m.Email})
				resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
				return
			}
		}
	}

	resp.Diagnostics.AddError("User Not Found", fmt.Sprintf("No user found with email %q.", data.Email.ValueString()))
}

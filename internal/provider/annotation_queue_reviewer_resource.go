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
	_ resource.Resource                = &AnnotationQueueReviewerResource{}
	_ resource.ResourceWithImportState = &AnnotationQueueReviewerResource{}
)

func NewAnnotationQueueReviewerResource() resource.Resource {
	return &AnnotationQueueReviewerResource{}
}

type AnnotationQueueReviewerResource struct {
	client *client.Client
}

type AnnotationQueueReviewerResourceModel struct {
	ID         types.String `tfsdk:"id"`
	QueueID    types.String `tfsdk:"queue_id"`
	IdentityID types.String `tfsdk:"identity_id"`
}

type addReviewerRequest struct {
	IdentityID string `json:"identity_id"`
}

func (r *AnnotationQueueReviewerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_annotation_queue_reviewer"
}

func (r *AnnotationQueueReviewerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Assigns an identity as a reviewer on a LangSmith annotation queue. The LangSmith API does not expose a per-pair read endpoint; Read for this resource is a no-op (the membership is taken to exist as long as it is in state).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"queue_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the annotation queue.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"identity_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the reviewer identity (user or service account).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *AnnotationQueueReviewerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AnnotationQueueReviewerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AnnotationQueueReviewerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := addReviewerRequest{IdentityID: data.IdentityID.ValueString()}
	var result struct {
		IdentityID string `json:"identity_id"`
	}
	if err := r.client.Post(ctx, "/v1/platform/annotation-queues/"+data.QueueID.ValueString()+"/reviewers", body, &result); err != nil {
		resp.Diagnostics.AddError("Error adding annotation queue reviewer", err.Error())
		return
	}
	data.ID = types.StringValue(data.QueueID.ValueString() + ":" + data.IdentityID.ValueString())
	tflog.Trace(ctx, "added annotation queue reviewer", map[string]interface{}{"id": data.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AnnotationQueueReviewerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// No GET endpoint for reviewer pairs; treat state as authoritative.
}

func (r *AnnotationQueueReviewerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Both fields are RequiresReplace; this should never be called.
}

func (r *AnnotationQueueReviewerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected \"<queue_id>:<identity_id>\".")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("queue_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("identity_id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *AnnotationQueueReviewerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AnnotationQueueReviewerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/v1/platform/annotation-queues/"+data.QueueID.ValueString()+"/reviewers/"+data.IdentityID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error removing annotation queue reviewer", err.Error())
		return
	}
}

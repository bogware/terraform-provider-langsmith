// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/url"
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

// NewAnnotationQueueReviewerResource returns a resource that manages platform
// annotation queue reviewer membership.
func NewAnnotationQueueReviewerResource() resource.Resource {
	return &AnnotationQueueReviewerResource{}
}

// AnnotationQueueReviewerResource manages a reviewer identity on a LangSmith
// annotation queue via the platform API.
type AnnotationQueueReviewerResource struct {
	client *client.Client
}

// AnnotationQueueReviewerResourceModel is Terraform state for a queue reviewer.
type AnnotationQueueReviewerResourceModel struct {
	ID         types.String `tfsdk:"id"`
	QueueID    types.String `tfsdk:"queue_id"`
	IdentityID types.String `tfsdk:"identity_id"`
}

type annotationQueueReviewerAddRequest struct {
	IdentityID string `json:"identity_id"`
}

type annotationQueueReviewerAPIResponse struct {
	IdentityID string `json:"identity_id"`
}

func annotationQueueReviewerImportID(queueID, identityID string) string {
	return queueID + "/" + identityID
}

func annotationQueueReviewerAPIPath(queueID, identityID string) string {
	base := "/v1/platform/annotation-queues/" + url.PathEscape(queueID) + "/reviewers"
	if identityID == "" {
		return base
	}
	return base + "/" + url.PathEscape(identityID)
}

func (r *AnnotationQueueReviewerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_annotation_queue_reviewer"
}

func (r *AnnotationQueueReviewerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a **reviewer identity** on a LangSmith annotation queue using the platform API " +
			"(`POST /v1/platform/annotation-queues/{queue_id}/reviewers`, `GET /v1/platform/annotation-queues/{queue_id}/reviewers/{identity_id}`, " +
			"`DELETE /v1/platform/annotation-queues/{queue_id}/reviewers/{identity_id}`). " +
			"Membership cannot be edited in place; change `queue_id` or `identity_id` to replace the resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Stable composite identifier: `queue_id`/`identity_id`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"queue_id": schema.StringAttribute{
				MarkdownDescription: "The annotation queue ID (`langsmith_annotation_queue.id`).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"identity_id": schema.StringAttribute{
				MarkdownDescription: "The reviewer identity ID (for example a workspace member `id` from `GET /api/v1/workspaces/current/members`).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
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
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
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

	queueID := data.QueueID.ValueString()
	identityID := data.IdentityID.ValueString()

	body := annotationQueueReviewerAddRequest{IdentityID: identityID}
	var result annotationQueueReviewerAPIResponse
	err := r.client.Post(ctx, annotationQueueReviewerAPIPath(queueID, ""), body, &result)
	if err != nil {
		resp.Diagnostics.AddError("Error adding annotation queue reviewer", err.Error())
		return
	}

	if result.IdentityID != "" {
		data.IdentityID = types.StringValue(result.IdentityID)
	}
	data.QueueID = types.StringValue(queueID)
	data.ID = types.StringValue(annotationQueueReviewerImportID(queueID, data.IdentityID.ValueString()))

	tflog.Trace(ctx, "created annotation queue reviewer", map[string]interface{}{"queue_id": queueID, "identity_id": data.IdentityID.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AnnotationQueueReviewerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AnnotationQueueReviewerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	queueID := data.QueueID.ValueString()
	identityID := data.IdentityID.ValueString()

	var result annotationQueueReviewerAPIResponse
	err := r.client.Get(ctx, annotationQueueReviewerAPIPath(queueID, identityID), nil, &result)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading annotation queue reviewer", err.Error())
		return
	}

	if result.IdentityID != "" {
		data.IdentityID = types.StringValue(result.IdentityID)
	}
	data.QueueID = types.StringValue(queueID)
	data.ID = types.StringValue(annotationQueueReviewerImportID(queueID, data.IdentityID.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AnnotationQueueReviewerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Annotation queue reviewer membership cannot be updated in place. Change `queue_id` or `identity_id` to replace the resource.",
	)
}

func (r *AnnotationQueueReviewerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AnnotationQueueReviewerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Delete(ctx, annotationQueueReviewerAPIPath(data.QueueID.ValueString(), data.IdentityID.ValueString()))
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error removing annotation queue reviewer", err.Error())
		return
	}

	tflog.Trace(ctx, "deleted annotation queue reviewer", map[string]interface{}{
		"queue_id":    data.QueueID.ValueString(),
		"identity_id": data.IdentityID.ValueString(),
	})
}

// ImportState expects an import ID of the form "queue_id/identity_id".
func (r *AnnotationQueueReviewerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	queueID, identityID, ok := parseAnnotationQueueReviewerImportID(req.ID)
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"Expected import ID in the format: queue_id/identity_id",
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("queue_id"), queueID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("identity_id"), identityID)...)
}

func parseAnnotationQueueReviewerImportID(id string) (queueID, identityID string, ok bool) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

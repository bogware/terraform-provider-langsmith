// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/bogware/terraform-provider-langsmith/internal/client"
)

// An invited member exists in one of two places until the invitation is
// accepted. LangSmith addresses the accepted form at
// `<base>/{identity_id}` and the unaccepted one at
// `<base>/{identity_id}/pending`, and each endpoint 404s for the other kind.
//
// State can be a step behind reality here in a way Terraform cannot prevent: an
// invitation accepted between the last refresh and the apply moves the member
// from one endpoint to the other. So rather than trusting the recorded state
// outright, these helpers try the endpoint it points at and fall back to the
// other on a 404.

func memberPaths(base, id string, pending bool) (first, second string) {
	active := base + "/" + id
	invited := active + "/pending"
	if pending {
		return invited, active
	}
	return active, invited
}

// deleteMember removes a member, whether or not the invitation was accepted.
// A 404 from both endpoints means the member is already gone, which is success
// for a delete.
func deleteMember(ctx context.Context, c *client.Client, base, id string, pending bool) error {
	first, second := memberPaths(base, id, pending)

	err := c.Delete(ctx, first)
	if err == nil || !client.IsNotFound(err) {
		return err
	}
	if err := c.Delete(ctx, second); err != nil && !client.IsNotFound(err) {
		return err
	}
	return nil
}

// patchMember updates a member, whether or not the invitation was accepted.
// Unlike deleteMember a 404 from both endpoints is a real failure: there is
// nothing to update.
func patchMember(ctx context.Context, c *client.Client, base, id string, pending bool, body, result interface{}) error {
	first, second := memberPaths(base, id, pending)

	err := c.Patch(ctx, first, body, result)
	if err == nil || !client.IsNotFound(err) {
		return err
	}
	return c.Patch(ctx, second, body, result)
}

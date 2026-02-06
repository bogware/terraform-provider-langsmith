// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

//go:build generate

package tools

import (
	_ "github.com/hashicorp/copywrite"
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)

// Generate copyright headers — branding the herd so everyone knows where they belong.
//go:generate go run github.com/hashicorp/copywrite headers -d .. --config ../.copywrite.hcl

// Format Terraform code for use in documentation — even outlaws keep their configs tidy.
//go:generate terraform fmt -recursive ../examples/

// Generate documentation — the official record, straight from the marshal's office.
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-dir .. -provider-name langsmith

// Copyright (c) Bogware, Inc. 2025
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/bogware/terraform-provider-langsmith/internal/provider"
)

// version is set at build time via ldflags. During development it rides under
// the "dev" brand — like a deputy who hasn't earned his badge yet.
var (
	version string = "dev"
)

// main fires up the Terraform provider server for LangSmith. Pass -debug to
// hitch it to a debugger like delve — handy for tracking down outlaws in your
// provider code.
func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/bogware/langsmith",
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), provider.New(version), opts)

	if err != nil {
		log.Fatal(err.Error())
	}
}

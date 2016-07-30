package main

import (
	"github.com/hashicorp/terraform/plugin"
	"github.com/sumerman/terraform-provider-awsx/awsx"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: awsx.Provider,
	})
}

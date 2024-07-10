package main

import (
	"context"
	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/wombatwisdom/wombat/internal/cli"

	// -- Import the free components from redpanda-connect
	_ "github.com/redpanda-data/connect/public/bundle/free/v4"
	_ "github.com/wombatwisdom/wombat/public/components/nats"
)

func main() {
	service.RunCLI(context.Background(),
		service.CLIOptSetBinaryName("wombat"),
		service.CLIOptSetProductName("Wombat"),
		service.CLIOptSetVersion(cli.Version, cli.DateBuilt))
}

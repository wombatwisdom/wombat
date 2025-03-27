package main

import (
    "context"
    "github.com/redpanda-data/benthos/v4/public/service"
    _ "github.com/wombatwisdom/wombat/components/legacy/all"
)

var Version = "0.0.1"
var DateBuilt = "1970-01-01T00:00:00Z"

func main() {
    service.RunCLI(context.Background(),
        service.CLIOptSetProductName("Wombat"),
        service.CLIOptSetBinaryName("wombat"),
        service.CLIOptSetVersion(Version, DateBuilt),
        service.CLIOptSetDocumentationURL("https://wombat.dev"),
    )
}

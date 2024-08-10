package main

import (
	"context"
	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/wombatwisdom/wombat/internal/cli"
	"os"
	"runtime"
	"runtime/pprof"

	// -- Import the free components from redpanda-connect
	_ "github.com/redpanda-data/connect/public/bundle/free/v4"
	_ "github.com/wombatwisdom/wombat/public/components/nats"
)

func main() {
	cpuf, err := os.Create("cpu.prof")
	if err != nil {
		panic(err)
	}
	defer cpuf.Close()

	memprof, err := os.Create("mem.prof")
	if err != nil {
		panic(err)
	}
	defer memprof.Close()

	runtime.GC()

	if err := pprof.StartCPUProfile(cpuf); err != nil {
		panic(err)
	}
	defer pprof.StopCPUProfile()

	service.RunCLI(context.Background(),
		service.CLIOptSetBinaryName("wombat"),
		service.CLIOptSetProductName("Wombat"),
		service.CLIOptSetVersion(cli.Version, cli.DateBuilt))

	if err := pprof.WriteHeapProfile(memprof); err != nil {
		panic(err)
	}
}

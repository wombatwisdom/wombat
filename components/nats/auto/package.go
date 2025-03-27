package auto

import (
    "errors"
    "github.com/redpanda-data/benthos/v4/public/service"
    "github.com/wombatwisdom/wombat/components/nats/core"
)

func init() {
    var errs error
    errs = errors.Join(errs, service.RegisterBatchInput(core.InputComponentName, core.InputConfig(), core.InputFromConfig))
    errs = errors.Join(errs, service.RegisterBatchOutput(core.OutputComponentName, core.OutputConfig(), core.OutputFromConfig))

    if errs != nil {
        panic(errs)
    }
}

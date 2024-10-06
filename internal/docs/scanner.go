package docs

import (
	"github.com/redpanda-data/benthos/v4/public/bloblang"
	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/sirupsen/logrus"
)

type Kind string

const (
	KindInput     Kind = "input"
	KindOutput    Kind = "output"
	KindCache     Kind = "cache"
	KindBuffer    Kind = "buffer"
	KindProcessor Kind = "processor"
	KindRateLimit Kind = "rate_limit"
	KindTracer    Kind = "tracer"
	KindScanner   Kind = "scanner"
	KindMetric    Kind = "metric"

	KindFunction Kind = "function"
	KindMethod   Kind = "method"
)

type Snapshot struct {
	Inputs     map[string]struct{}
	Outputs    map[string]struct{}
	Caches     map[string]struct{}
	Buffers    map[string]struct{}
	Processors map[string]struct{}
	RateLimits map[string]struct{}
	Tracers    map[string]struct{}
	Scanners   map[string]struct{}
	Metrics    map[string]struct{}

	Functions map[string]struct{}
	Methods   map[string]struct{}
}

func GlobalScanner() *Scanner {
	return &Scanner{
		benv: bloblang.GlobalEnvironment(),
		env:  service.GlobalEnvironment(),
	}
}

type Scanner struct {
	benv *bloblang.Environment
	env  *service.Environment
}

func (s *Scanner) Scan(fn func(doc *PluginDocView)) {
	s.benv.WalkFunctions(functionWalker(fn))
	s.benv.WalkMethods(methodWalker(fn))

	s.env.WalkBuffers(walker(fn))
	s.env.WalkCaches(walker(fn))
	s.env.WalkInputs(walker(fn))
	s.env.WalkOutputs(walker(fn))
	s.env.WalkProcessors(walker(fn))
	s.env.WalkRateLimits(walker(fn))
	s.env.WalkTracers(walker(fn))
	s.env.WalkScanners(walker(fn))
	s.env.WalkMetrics(walker(fn))
}

func (s *Scanner) ConfigSchema() *service.ConfigSchema {
	return s.env.FullConfigSchema("", "")
}

func methodWalker(fn func(doc *PluginDocView)) func(name string, spec *bloblang.MethodView) {
	return func(name string, spec *bloblang.MethodView) {
		dv, err := ParseDocViewFromMethodView(spec)
		if err != nil {
			logrus.Errorf("Failed to parse method %s: %v", name, err)
			return
		}

		fn(dv)
	}
}

func functionWalker(fn func(doc *PluginDocView)) func(name string, spec *bloblang.FunctionView) {
	return func(name string, spec *bloblang.FunctionView) {
		dv, err := ParseDocViewFromFunctionView(spec)
		if err != nil {
			logrus.Errorf("Failed to parse function %s: %v", name, err)
			return
		}

		fn(dv)
	}
}

func walker(fn func(doc *PluginDocView)) func(name string, config *service.ConfigView) {
	return func(name string, config *service.ConfigView) {
		dv, err := ParseDocViewFromConfigView(config)
		if err != nil {
			logrus.Errorf("Failed to parse component %s: %v", name, err)
			return
		}

		fn(dv)
	}
}

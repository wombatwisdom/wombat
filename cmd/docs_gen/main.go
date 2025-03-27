// Copyright 2024 Redpanda Data, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
    "bytes"
    "flag"
    "fmt"
    "os"
    "path"
    "path/filepath"
    "strings"
    "text/template"

    "github.com/redpanda-data/benthos/v4/public/bloblang"
    "github.com/redpanda-data/benthos/v4/public/service"

    _ "github.com/wombatwisdom/wombat/components/legacy/all"

    _ "embed"
)

var Version = "0.0.1"
var DateBuilt = "1970-01-01T00:00:00Z"

//go:embed templates/bloblang_functions.mdx.tmpl
var templateBloblFunctionsRaw string

//go:embed templates/bloblang_methods.mdx.tmpl
var templateBloblMethodsRaw string

//go:embed templates/plugin_fields.mdx.tmpl
var templatePluginFieldsRaw string

//go:embed templates/plugin.mdx.tmpl
var templatePluginRaw string

//go:embed templates/http.mdx.tmpl
var templateHTTPRaw string

//go:embed templates/logger.mdx.tmpl
var templateLoggerRaw string

//go:embed templates/tests.mdx.tmpl
var templateTestsRaw string

//go:embed templates/templates.mdx.tmpl
var templateTemplatesRaw string

var (
    templateBloblFunctions *template.Template
    templateBloblMethods   *template.Template
    templatePlugin         *template.Template
    templateHTTP           *template.Template
    templateLogger         *template.Template
    templateTests          *template.Template
    templateTemplates      *template.Template
)

func init() {
    templateBloblFunctions = template.Must(template.New("bloblang functions").Parse(templateBloblFunctionsRaw))
    templateBloblMethods = template.Must(template.New("bloblang methods").Parse(templateBloblMethodsRaw))
    templatePlugin = template.Must(template.New("plugin").Parse(templatePluginFieldsRaw + templatePluginRaw))
    templateHTTP = template.Must(template.New("http").Parse(templatePluginFieldsRaw + templateHTTPRaw))
    templateLogger = template.Must(template.New("logger").Parse(templatePluginFieldsRaw + templateLoggerRaw))
    templateTests = template.Must(template.New("tests").Parse(templatePluginFieldsRaw + templateTestsRaw))
    templateTemplates = template.Must(template.New("templates").Parse(templatePluginFieldsRaw + templateTemplatesRaw))
}

func create(t, path string, resBytes []byte) {
    if existing, err := os.ReadFile(path); err == nil {
        if bytes.Equal(existing, resBytes) {
            return
        }
    }

    if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
        panic(err)
    }

    if err := os.WriteFile(path, resBytes, 0o644); err != nil {
        panic(err)
    }
    fmt.Printf("Documentation for '%v' has changed, updating: %v\n", t, path)
}

func getSchema() *service.ConfigSchema {
    env := service.GlobalEnvironment()
    s := env.FullConfigSchema(Version, DateBuilt)

    s.SetFieldDefault(map[string]any{
        "@service": "wombat",
    }, "logger", "static_fields")

    return s
}

func main() {
    docsDir := "./website/src/content/docs"
    flag.StringVar(&docsDir, "dir", docsDir, "The directory to write docs to")
    flag.Parse()

    env := getSchema().Environment()

    refDir := path.Join(docsDir, "./reference")

    env.WalkInputs(viewForDir(path.Join(refDir, "./components/inputs")))
    env.WalkBuffers(viewForDir(path.Join(refDir, "./components/buffers")))
    env.WalkCaches(viewForDir(path.Join(refDir, "./components/caches")))
    env.WalkMetrics(viewForDir(path.Join(refDir, "./components/metrics")))
    env.WalkOutputs(viewForDir(path.Join(refDir, "./components/outputs")))
    env.WalkProcessors(viewForDir(path.Join(refDir, "./components/processors")))
    env.WalkRateLimits(viewForDir(path.Join(refDir, "./components/rate_limits")))
    env.WalkTracers(viewForDir(path.Join(refDir, "./components/tracers")))
    env.WalkScanners(viewForDir(path.Join(refDir, "./components/scanners")))

    // Bloblang stuff
    doBloblangMethods(refDir)
    doBloblangFunctions(refDir)

    // Unit test docs
    doTestDocs(refDir)

    // HTTP docs
    doHTTP(refDir)

    // Logger docs
    doLogger(refDir)

    // Template docs
    doTemplates(refDir)
}

func viewForDir(docsDir string) func(string, *service.ConfigView) {
    return func(name string, view *service.ConfigView) {
        data, err := view.TemplateData()
        if err != nil {
            panic(fmt.Sprintf("Failed to prepare docs for '%v': %v", name, err))
        }

        data = parsePluginTemplateData(data)

        var buf bytes.Buffer
        if err := templatePlugin.Execute(&buf, data); err != nil {
            panic(fmt.Sprintf("Failed to generate docs for '%v': %v", name, err))
        }

        if err := os.MkdirAll(docsDir, 0755); err != nil {
            panic(fmt.Sprintf("Failed to create docs directory path '%v': %v", docsDir, err))
        }

        create(name, path.Join(docsDir, name+".mdx"), buf.Bytes())
    }
}

type functionCategory struct {
    Name  string
    Specs []bloblang.TemplateFunctionData
}

type functionsContext struct {
    Categories []functionCategory
}

func doBloblangFunctions(dir string) {
    var specs []bloblang.TemplateFunctionData
    bloblang.GlobalEnvironment().WalkFunctions(func(name string, spec *bloblang.FunctionView) {
        tmpl := spec.TemplateData()
        prefixExamples(tmpl.Examples)
        specs = append(specs, parseTemplateFunctionData(tmpl))
    })

    ctx := functionsContext{}
    for _, cat := range []string{
        "General",
        "Message Info",
        "Environment",
        "Fake Data Generation",
        "Deprecated",
    } {
        functions := functionCategory{
            Name: cat,
        }
        for _, spec := range specs {
            if spec.Category == cat {
                functions.Specs = append(functions.Specs, spec)
            }
        }
        if len(functions.Specs) > 0 {
            ctx.Categories = append(ctx.Categories, functions)
        }
    }

    var buf bytes.Buffer
    if err := templateBloblFunctions.Execute(&buf, ctx); err != nil {
        panic(fmt.Sprintf("Failed to generate docs for bloblang functions: %v", err))
    }

    create("bloblang functions", filepath.Join(dir, "bloblang", "functions.mdx"), buf.Bytes())
}

type methodCategory struct {
    Name  string
    Specs []bloblang.TemplateMethodData
}

type methodsContext struct {
    Categories []methodCategory
    General    []bloblang.TemplateMethodData
}

func prefixExamples(s []bloblang.TemplateExampleData) {
    for _, spec := range s {
        for i := range spec.Results {
            spec.Results[i][0] = strings.ReplaceAll(
                strings.TrimSuffix(spec.Results[i][0], "\n"),
                "\n", "\n#      ",
            )
            spec.Results[i][1] = strings.ReplaceAll(
                strings.TrimSuffix(spec.Results[i][1], "\n"),
                "\n", "\n#      ",
            )
        }
    }
}

func methodForCat(s bloblang.TemplateMethodData, cat string) (bloblang.TemplateMethodData, bool) {
    for _, c := range s.Categories {
        if c.Category == cat {
            spec := s
            if c.Description != "" {
                spec.Description = strings.TrimSpace(c.Description)
            }
            if len(c.Examples) > 0 {
                spec.Examples = c.Examples
            }
            return spec, true
        }
    }
    return s, false
}

func doBloblangMethods(dir string) {
    var specs []bloblang.TemplateMethodData
    bloblang.GlobalEnvironment().WalkMethods(func(name string, spec *bloblang.MethodView) {
        tmpl := spec.TemplateData()
        prefixExamples(tmpl.Examples)
        for _, cat := range tmpl.Categories {
            prefixExamples(cat.Examples)
        }
        specs = append(specs, parseTemplateMethodData(tmpl))
    })

    ctx := methodsContext{}
    for _, cat := range []string{
        "String Manipulation",
        "Regular Expressions",
        "Number Manipulation",
        "Timestamp Manipulation",
        "Type Coercion",
        "Object & Array Manipulation",
        "Parsing",
        "Encoding and Encryption",
        "SQL",
        "JSON Web Tokens",
        "GeoIP",
        "Deprecated",
    } {
        methods := methodCategory{
            Name: cat,
        }
        for _, spec := range specs {
            var ok bool
            if spec, ok = methodForCat(spec, cat); ok {
                methods.Specs = append(methods.Specs, spec)
            }
        }
        if len(methods.Specs) > 0 {
            ctx.Categories = append(ctx.Categories, methods)
        }
    }

    for _, spec := range specs {
        if len(spec.Categories) == 0 && spec.Status != "hidden" {
            spec.Description = strings.TrimSpace(spec.Description)
            ctx.General = append(ctx.General, spec)
        }
    }

    var buf bytes.Buffer
    err := templateBloblMethods.Execute(&buf, ctx)
    if err != nil {
        panic(fmt.Sprintf("Failed to generate docs for bloblang methods: %v", err))
    }

    create("bloblang methods", filepath.Join(dir, "bloblang", "methods.mdx"), buf.Bytes())
}

func doTestDocs(dir string) {
    data, err := getSchema().TemplateData()
    if err != nil {
        panic(fmt.Sprintf("Failed to prepare tests docs: %v", err))
    }

    var newFields []service.TemplateDataPluginField
    for _, f := range data.Fields {
        if strings.HasPrefix(f.FullName, "tests") {
            newFields = append(newFields, f)
        }
    }
    data.Fields = newFields

    var buf bytes.Buffer
    if err := templateTests.Execute(&buf, data); err != nil {
        panic(fmt.Sprintf("Failed to generate tests docs: %v", err))
    }

    create("tests docs", filepath.Join(dir, "configuration", "unit_testing.mdx"), buf.Bytes())
}

func doHTTP(dir string) {
    data, err := getSchema().TemplateData("http")
    if err != nil {
        panic(fmt.Sprintf("Failed to prepare http docs: %v", err))
    }

    var buf bytes.Buffer
    if err := templateHTTP.Execute(&buf, data); err != nil {
        panic(fmt.Sprintf("Failed to generate http docs: %v", err))
    }

    create("http docs", filepath.Join(dir, "configuration", "http.mdx"), buf.Bytes())
}

func doLogger(dir string) {
    data, err := getSchema().TemplateData("logger")
    if err != nil {
        panic(fmt.Sprintf("Failed to prepare logger docs: %v", err))
    }

    var buf bytes.Buffer
    if err := templateLogger.Execute(&buf, data); err != nil {
        panic(fmt.Sprintf("Failed to generate logger docs: %v", err))
    }

    create("logger docs", filepath.Join(dir, "configuration", "logger.mdx"), buf.Bytes())
}

func doTemplates(dir string) {
    data, err := getSchema().Environment().TemplateSchema("", "").TemplateData()
    if err != nil {
        panic(fmt.Sprintf("Failed to prepare template docs: %v", err))
    }

    var buf bytes.Buffer
    if err := templateTemplates.Execute(&buf, data); err != nil {
        panic(fmt.Sprintf("Failed to generate template docs: %v", err))
    }

    create("tests docs", filepath.Join(dir, "configuration", "templating.mdx"), buf.Bytes())
}

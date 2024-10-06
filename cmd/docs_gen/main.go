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
	"github.com/wombatwisdom/wombat/internal/docs"
	"os"
	"path/filepath"
	"text/template"

	_ "github.com/wombatwisdom/wombat/public/components/all"

	_ "embed"
)

//go:embed templates/component.md.tmpl
var templateComponentRaw string

var (
	templateComponent *template.Template
)

func init() {
	templateComponent = template.Must(template.New("component").Parse(templateComponentRaw))
}

func main() {
	docsDir := "website/src/content/docs/generated"
	flag.StringVar(&docsDir, "dir", docsDir, "The directory to write docs to")
	flag.Parse()

	scanner := docs.GlobalScanner()
	scanner.Scan(func(doc *docs.PluginDocView) {
		w := &bytes.Buffer{}
		if err := templateComponent.Execute(w, doc); err != nil {
			panic(fmt.Sprintf("Failed to execute template: %v", err))
		}
		create(w.Bytes(), docsDir, doc.Kind, doc.Name)
	})

	//// Unit test docs
	//doTestDocs(scanner, docsDir)
	//
	//// HTTP docs
	//doHTTP(scanner, docsDir)
	//
	//// Logger docs
	//doLogger(scanner, docsDir)
	//
	//// Template docs
	//doTemplates(scanner, docsDir)
}

func create(content []byte, pathElements ...string) {
	path := fmt.Sprintf("%v.md", filepath.Join(pathElements...))

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		panic(fmt.Sprintf("Failed to create docs directory path '%v': %v", path, err))
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		panic(fmt.Sprintf("Failed to write doc '%v': %v", path, err))
	}
}

//func createTemplateData(td service.TemplateDataSchema, pathElements ...string) {
//
//}

//func doTestDocs(scanner *docs.Scanner, dir string) {
//	data, err := scanner.ConfigSchema().TemplateData()
//	if err != nil {
//		panic(fmt.Sprintf("Failed to prepare tests docs: %v", err))
//	}
//	create(data, dir, "concepts", "tests")
//}
//
//func doHTTP(scanner *docs.Scanner, dir string) {
//	data, err := scanner.ConfigSchema().TemplateData("http")
//	if err != nil {
//		panic(fmt.Sprintf("Failed to prepare http docs: %v", err))
//	}
//	create(data, dir, "concepts", "http")
//}
//
//func doLogger(scanner *docs.Scanner, dir string) {
//	data, err := scanner.ConfigSchema().TemplateData("logger")
//	if err != nil {
//		panic(fmt.Sprintf("Failed to prepare logger docs: %v", err))
//	}
//	create(data, dir, "concepts", "logging")
//}
//
//func doTemplates(scanner *docs.Scanner, dir string) {
//	data, err := scanner.ConfigSchema().Environment().TemplateSchema("", "").TemplateData()
//	if err != nil {
//		panic(fmt.Sprintf("Failed to prepare template docs: %v", err))
//	}
//
//	create(data, dir, "concepts", "templates")
//}

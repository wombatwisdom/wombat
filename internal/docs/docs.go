package docs

import (
	"github.com/redpanda-data/benthos/v4/public/bloblang"
	"github.com/redpanda-data/benthos/v4/public/service"
	"strings"
)

type PluginDocView struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Summary     string `json:"summary,omitempty"`
	Description string `json:"description,omitempty"`
	Footnotes   string `json:"footnotes,omitempty"`
	Version     string `json:"version,omitempty"`
	Status      string `json:"status"`

	Categories []string  `json:"categories,omitempty"`
	Examples   []Example `json:"examples,omitempty"`

	// Fields are only available on components, not on bloblang methods or functions.
	Fields []Field `json:"fields,omitempty"`

	// Params on the other hand are only available on bloblang methods and functions.
	Params             []Param `json:"params,omitempty"`
	VariadicParameters bool    `json:"variadic_parameters,omitempty"`
}

type Param struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	ValueType    string `json:"type"`
	IsOptional   bool   `json:"is_optional,omitempty"`
	DefaultValue string `json:"default,omitempty"`
}

type Field struct {
	// The description of the field.
	Description string `json:"description"`

	// Whether the field contains secrets.
	IsSecret bool `json:"isSecret"`

	// Whether the field is interpolated.
	IsInterpolated bool `json:"isInterpolated"`

	// The type information of the field.
	Type string `json:"type"`

	// The version in which this field was added.
	Version string `json:"version"`

	// An array of enum options accompanied by a description.
	AnnotatedOptions [][2]string `json:"annotatedOptions,omitempty"`

	// An array of enum options, without annotations.
	Options []string `json:"options,omitempty"`

	// An array of example values.
	Examples []any `json:"examples,omitempty"`

	// FullName describes the full dot path name of the field relative to
	// the root of the documented component.
	FullName string `json:"name"`

	// ExamplesMarshalled is a list of examples marshalled into YAML format.
	ExamplesMarshalled []string `json:"examplesMarshalled,omitempty"`

	// DefaultMarshalled is a marshalled string of the default value in JSON
	// format, if there is one.
	DefaultMarshalled string `json:"defaultMarshalled,omitempty"`
}

type Example struct {
	Summary string `json:"summary"`
	Content string `json:"content,omitempty"`

	// -- only filled in in case of a bloblang example
	Results [][2]string `json:"results,omitempty"`
}

func ParseDocViewFromConfigView(cv *service.ConfigView) (*PluginDocView, error) {
	td, err := cv.TemplateData()
	if err != nil {
		return nil, err
	}

	var examples []Example
	for _, ted := range td.Examples {
		examples = append(examples, exampleFromTemplateExampleData(ted))
	}

	var fields []Field
	for _, ted := range td.Fields {
		fields = append(fields, fieldFromTemplateData(ted))
	}

	return &PluginDocView{
		Kind:        td.Type,
		Name:        td.Name,
		Summary:     td.Summary,
		Description: sanitizeMarkdown(td.Description),
		Footnotes:   td.Footnotes,
		Version:     td.Version,
		Status:      td.Status,
		Categories:  strings.Split(td.Categories, ","),
		Examples:    examples,
		Fields:      fields,
	}, nil
}

func ParseDocViewFromMethodView(mv *bloblang.MethodView) (*PluginDocView, error) {
	td := mv.TemplateData()

	var categories []string
	for _, cat := range td.Categories {
		categories = append(categories, cat.Category)
	}

	var examples []Example
	for _, ted := range td.Examples {
		examples = append(examples, exampleFromBloblangTemplateData(ted))
	}

	var params []Param
	for _, p := range td.Params.Definitions {
		params = append(params, Param{
			Name:         p.Name,
			Description:  p.Description,
			ValueType:    p.ValueType,
			IsOptional:   p.IsOptional,
			DefaultValue: p.DefaultMarshalled,
		})
	}

	return &PluginDocView{
		Kind:               "method",
		Name:               td.Name,
		Description:        td.Description,
		Version:            td.Version,
		Status:             td.Status,
		Categories:         categories,
		Examples:           examples,
		Params:             params,
		VariadicParameters: false,
	}, nil
}

func ParseDocViewFromFunctionView(fv *bloblang.FunctionView) (*PluginDocView, error) {
	td := fv.TemplateData()

	var examples []Example
	for _, ted := range td.Examples {
		examples = append(examples, exampleFromBloblangTemplateData(ted))
	}

	var params []Param
	for _, p := range td.Params.Definitions {
		params = append(params, Param{
			Name:         p.Name,
			Description:  sanitizeMarkdown(p.Description),
			ValueType:    p.ValueType,
			IsOptional:   p.IsOptional,
			DefaultValue: p.DefaultMarshalled,
		})
	}

	return &PluginDocView{
		Kind:               "function",
		Name:               td.Name,
		Description:        sanitizeMarkdown(td.Description),
		Version:            td.Version,
		Status:             td.Status,
		Categories:         []string{td.Category},
		Examples:           examples,
		Params:             params,
		VariadicParameters: false,
	}, nil
}

func exampleFromTemplateExampleData(ted service.TemplatDataPluginExample) Example {
	return Example{
		Summary: ted.Summary,
		Content: ted.Config,
	}
}

func exampleFromBloblangTemplateData(ted bloblang.TemplateExampleData) Example {
	return Example{
		Summary: ted.Summary,
		Content: ted.Mapping,
		Results: ted.Results,
	}
}

func fieldFromTemplateData(ted service.TemplateDataPluginField) Field {
	return Field{
		Description:        ted.Description,
		IsSecret:           ted.IsSecret,
		IsInterpolated:     ted.IsInterpolated,
		Type:               ted.Type,
		Version:            ted.Version,
		AnnotatedOptions:   ted.AnnotatedOptions,
		Options:            ted.Options,
		Examples:           ted.Examples,
		FullName:           ted.FullName,
		ExamplesMarshalled: ted.ExamplesMarshalled,
		DefaultMarshalled:  ted.DefaultMarshalled,
	}
}

func sanitizeMarkdown(input string) string {
	return strings.ReplaceAll(input, "\n== ", "\n## ")
}

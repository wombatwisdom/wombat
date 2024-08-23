package library

import "time"

type Repository struct {
	Name      string   `json:"name"`
	Summary   string   `json:"summary"`
	SourceUrl string   `json:"source_url"`
	Latest    string   `json:"latest"`
	Versions  []string `json:"versions"`
}

type Version struct {
	Repository string   `json:"repository"`
	Name       string   `json:"name"`
	Summary    string   `json:"summary"`
	SourceRef  string   `json:"source_ref"`
	Packages   []string `json:"packages"`
}

type Package struct {
	Repository string              `json:"repository"`
	Version    string              `json:"version"`
	Name       string              `json:"name"`
	Summary    string              `json:"summary"`
	SourcePath string              `json:"source_path"`
	Components map[string][]string `json:"components"`
}

type Component struct {
	Repository string `json:"repository"`
	Version    string `json:"version"`
	Package    string `json:"package"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`

	Summary     string `json:"summary"`
	Description string `json:"description,omitempty"`
	Footnotes   string `json:"footnotes,omitempty"`
	Status      string `json:"status"`
	License     string `json:"license"`

	Categories []string  `json:"categories,omitempty"`
	Examples   []Example `json:"examples,omitempty"`

	// Fields are only available on components, not on bloblang methods or functions.
	Fields []Field `json:"fields,omitempty"`

	// Params on the other hand are only available on bloblang methods and functions.
	Params             Param `json:"params,omitempty"`
	VariadicParameters bool  `json:"variadic_parameters,omitempty"`
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

type IndexEntry struct {
	Repository string `json:"repository"`
	Version    string `json:"version"`
	Package    string `json:"package"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Summary    string `json:"summary"`
	Status     string `json:"status"`
	License    string `json:"license"`
}

type SearchResult struct {
	Total  uint64        `json:"total"`
	Took   time.Duration `json:"took"`
	Hits   []IndexEntry  `yaml:"hits"`
	Offset int           `json:"offset"`
	Size   int           `json:"size"`
}

package metadata

import (
    "github.com/redpanda-data/benthos/v4/public/service"
    "strings"
)

const (
    FilterField         string = "metadata"
    FilterPatternsField string = "patterns"
    FilterInvertField   string = "invert"
)

func MetadataFilterConfig() *service.ConfigField {
    return service.NewObjectField(FilterField,
        service.NewStringListField(FilterPatternsField).Description(strings.TrimSpace(`
A list of regular expressions to match metadata fields that should be included in the output. If a field matches any of`)),
        service.NewBoolField(FilterInvertField).Description(strings.TrimSpace(`
By default, the filter includes metadata fields that match the patterns. If this field is set to true, the filter will
exclude metadata fields that match the patterns.`)),
    ).
        Optional().
        Description(strings.TrimSpace(`
It isn't always desirable to pass all metadata fields on a message to an output. The metadata filter allows you to 
specify which fields to include or exclude from the output. You can use one or more regular expressions to match the
metadata fields you want to include.

By default all metadata fields are included in the output.
`))
}

func FilterFromConfig(cfg *service.ParsedConfig) (Filter, error) {
    if cfg.Contains(FilterField) {
        cfg = cfg.Namespace(FilterField)
    }

    var err error
    var patterns []string
    if patterns, err = cfg.FieldStringList(FilterPatternsField); err != nil {
        return nil, err
    }

    var result Filter
    if len(patterns) == 0 {
        return nil, nil
    } else if len(patterns) == 1 {
        result = NewRegexFilter(patterns[0])
    } else {
        filters := make([]Filter, len(patterns))
        for _, pattern := range patterns {
            filters = append(filters, NewRegexFilter(pattern))
        }

        result = NewCompositeFilter(filters...)
    }

    if invert, _ := cfg.FieldBool(FilterInvertField); invert {
        result = Invert(result)
    }

    return result, nil
}

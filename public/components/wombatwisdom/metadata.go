package wombatwisdom

import (
	"regexp"

	"github.com/wombatwisdom/components/framework/spec"
)

type MetadataFilterFactory struct {
}

func (m *MetadataFilterFactory) BuildMetadataFilter(patterns []string, invert bool) (spec.MetadataFilter, error) {
	result := &MetadataFilter{}

	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, err
		}
		result.exprs = append(result.exprs, *re)
	}

	return result, nil
}

type MetadataFilter struct {
	exprs []regexp.Regexp
}

func (m *MetadataFilter) Include(key string) bool {
	for _, re := range m.exprs {
		if re.MatchString(key) {
			return true
		}
	}
	return false
}

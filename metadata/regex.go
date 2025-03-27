package metadata

import (
    "regexp"
)

func NewRegexFilter(pattern string) Filter {
    return &regexFilter{
        pattern: regexp.MustCompile(pattern),
    }
}

type regexFilter struct {
    pattern *regexp.Regexp
}

func (r *regexFilter) Include(key string) bool {
    return r.pattern.MatchString(key)
}

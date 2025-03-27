package metadata

func NewCompositeFilter(filters ...Filter) Filter {
    return &compositeFilter{filters}
}

type compositeFilter struct {
    filters []Filter
}

func (c *compositeFilter) Include(key string) bool {
    for _, filter := range c.filters {
        if !filter.Include(key) {
            return false
        }
    }

    return true
}

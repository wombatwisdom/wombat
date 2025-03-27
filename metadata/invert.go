package metadata

func Invert(filter Filter) Filter {
    return &invertFilter{filter}
}

type invertFilter struct {
    filter Filter
}

func (n *invertFilter) Include(key string) bool {
    return !n.filter.Include(key)
}

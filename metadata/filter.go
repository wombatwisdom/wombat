package metadata

type Filter interface {
    Include(key string) bool
}

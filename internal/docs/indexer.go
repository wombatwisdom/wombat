package docs

type Index struct {
	Groups []IndexGroup `json:"groups"`
}

type IndexGroup struct {
	Name string `json:"name"`
}

type IndexItem struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
}

package builder

type PackageBuildSpec struct {
	PackageRef

	Module string `json:"module"`
	Path   string `json:"path"`

	Os   string `json:"os"`
	Arch string `json:"arch"`
}

type PackageRef struct {
	Library string `json:"library"`
	Version string `json:"version"`
	Package string `json:"package"`
}

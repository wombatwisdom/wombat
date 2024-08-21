package presets

import (
	_ "embed"
	"fmt"
)

//go:embed full.spec.yml
var fullPreset []byte

//go:embed slim.spec.yml
var slimPreset []byte

func Get(preset string) ([]byte, error) {
	switch preset {
	case "full":
		return fullPreset, nil
	case "slim":
		return slimPreset, nil
	default:
		return nil, fmt.Errorf("unknown preset %q", preset)
	}
}

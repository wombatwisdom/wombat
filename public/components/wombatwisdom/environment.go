package wombatwisdom

import (
	"github.com/wombatwisdom/components/framework/spec"
	"os"
	"strconv"
)

func NewEnvironment(logger spec.Logger) *Environment {
	return &Environment{
		Logger: logger,
	}
}

type Environment struct {
	spec.Logger
}

func (e *Environment) GetString(key string) string {
	return os.Getenv(key)
}

func (e *Environment) GetInt(key string) int {
	res, fnd := os.LookupEnv(key)
	if !fnd {
		return 0
	}

	i, _ := strconv.Atoi(res)
	return i
}

func (e *Environment) GetBool(key string) bool {
	res, fnd := os.LookupEnv(key)
	if !fnd {
		return false
	}

	b, _ := strconv.ParseBool(res)
	return b
}

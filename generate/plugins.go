package generate

import (
	"fmt"
	"plugin"
)

type FieldTagPlugin struct {
	Name      string
	ValueFunc func(PluginInput) (string, error)
}

type PluginInput struct {
	GraphQLName string
	Description string
}

func LoadFieldTagPlugins(name string, path string) (*FieldTagPlugin, error) {
	pl, err := plugin.Open(path)
	if err != nil {
		return nil, err
	}

	funcF, err := pl.Lookup("FieldTagPlugin")
	if err != nil {
		return nil, err
	}

	castedFunc, ok := funcF.(func(PluginInput) (string, error))
	if !ok {
		return nil, fmt.Errorf("expected 'func(PluginInput) (string, error)' function, got %T", funcF)
	}

	return &FieldTagPlugin{Name: name, ValueFunc: castedFunc}, nil
}

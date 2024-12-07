package stages

import (
	"fmt"
	"github.com/charmbracelet/huh"
)

type SelectStage struct{}

func init() {
	Register("select", func() StageRunner {
		return &SelectStage{}
	})
}

func (s *SelectStage) Run(input string, config map[string]interface{}) (string, error) {
	options, ok := config["options"].([]interface{})
	if !ok {
		return "", fmt.Errorf("select options missing")
	}

	var selected string
	opts := make([]string, len(options))
	for i, opt := range options {
		opts[i] = fmt.Sprint(opt)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Choose an option").
				Options(huh.NewOptions(opts...)...).
				Value(&selected),
		),
	)

	err := form.Run()
	if err != nil {
		return "", err
	}

	return selected, nil
}

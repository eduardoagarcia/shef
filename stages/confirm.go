package stages

import (
	"fmt"
	"github.com/charmbracelet/huh"
)

type ConfirmStage struct{}

func init() {
	Register("confirm", func() StageRunner {
		return &ConfirmStage{}
	})
}

func (c *ConfirmStage) Run(input string, config map[string]interface{}) (string, error) {
	message, ok := config["message"].(string)
	if !ok {
		message = "Continue?"
	}

	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(message).
				Value(&confirmed),
		),
	)

	err := form.Run()
	if err != nil {
		return "", err
	}

	if !confirmed {
		return input, fmt.Errorf("user declined")
	}

	return input, nil
}

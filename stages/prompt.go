package stages

import (
	"fmt"
	"github.com/charmbracelet/huh"
)

type PromptStage struct{}

func init() {
	Register("prompt", func() StageRunner {
		return &PromptStage{}
	})
}

func (p *PromptStage) Run(input string, config map[string]interface{}) (string, error) {
	message, ok := config["message"].(string)
	if !ok {
		return "", fmt.Errorf("prompt message missing")
	}

	var userInput string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(message).
				Value(&userInput),
		),
	)

	err := form.Run()
	if err != nil {
		return "", err
	}

	return userInput, nil
}

package stages

import (
	"fmt"
	"regexp"
)

type RegexStage struct{}

func init() {
	Register("regex", func() StageRunner {
		return &RegexStage{}
	})
}

func (r *RegexStage) Run(input string, config map[string]interface{}) (string, error) {
	pattern, ok := config["pattern"].(string)
	if !ok {
		return "", fmt.Errorf("regex pattern missing")
	}

	replacement, ok := config["replacement"].(string)
	if !ok {
		return "", fmt.Errorf("regex replacement missing")
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}

	return re.ReplaceAllString(input, replacement), nil
}

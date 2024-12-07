package stages

import "fmt"

type StageRunner interface {
	Run(input string, config map[string]interface{}) (string, error)
}

var registry = make(map[string]func() StageRunner)

func Register(stageType string, factory func() StageRunner) {
	registry[stageType] = factory
}

func GetStageRunner(stageType string) (StageRunner, error) {
	if factory, exists := registry[stageType]; exists {
		return factory(), nil
	}
	return nil, fmt.Errorf("unknown stage type: %s", stageType)
}

package internal

import (
	"sync"
	"text/template"
)

// File represents a collection of recipes
type File struct {
	Recipes []Recipe `yaml:"recipes"`
}

// Recipe defines a Shef recipe with its metadata and operations
type Recipe struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Category    string      `yaml:"category,omitempty"`
	Author      string      `yaml:"author,omitempty"`
	Help        string      `yaml:"help,omitempty"`
	Operations  []Operation `yaml:"operations"`
}

// Operation defines a single executable step in a recipe
type Operation struct {
	Name          string      `yaml:"name"`
	ID            string      `yaml:"id,omitempty"`
	Command       string      `yaml:"command,omitempty"`
	ControlFlow   interface{} `yaml:"control_flow,omitempty"`
	Operations    []Operation `yaml:"operations,omitempty"`
	ExecutionMode string      `yaml:"execution_mode,omitempty"`
	OutputFormat  string      `yaml:"output_format,omitempty"`
	Silent        bool        `yaml:"silent,omitempty"`
	Condition     string      `yaml:"condition,omitempty"`
	OnSuccess     string      `yaml:"on_success,omitempty"`
	OnFailure     string      `yaml:"on_failure,omitempty"`
	Transform     string      `yaml:"transform,omitempty"`
	Prompts       []Prompt    `yaml:"prompts,omitempty"`
	Break         bool        `yaml:"break,omitempty"`
	Exit          bool        `yaml:"exit,omitempty"`
}

// Prompt defines interactive user input required during recipe execution
type Prompt struct {
	Name            string            `yaml:"name"`
	ID              string            `yaml:"id,omitempty"`
	Type            string            `yaml:"type"`
	Message         string            `yaml:"message"`
	Default         string            `yaml:"default,omitempty"`
	Options         []string          `yaml:"options,omitempty"`
	Descriptions    map[string]string `yaml:"descriptions,omitempty"`
	SourceOp        string            `yaml:"source_operation,omitempty"`
	SourceTransform string            `yaml:"source_transform,omitempty"`
	MinValue        int               `yaml:"min_value,omitempty"`
	MaxValue        int               `yaml:"max_value,omitempty"`
	Required        bool              `yaml:"required,omitempty"`
	FileExtensions  []string          `yaml:"file_extensions,omitempty"`
	MultipleLimit   int               `yaml:"multiple_limit,omitempty"`
	EditorCmd       string            `yaml:"editor_cmd,omitempty"`
	HelpText        string            `yaml:"help_text,omitempty"`
	Validators      []PromptValidator `yaml:"validators,omitempty"`
}

// PromptValidator defines validation rules for prompt inputs
type PromptValidator struct {
	Type    string `yaml:"type"`
	Pattern string `yaml:"pattern,omitempty"`
	Message string `yaml:"message,omitempty"`
	Min     int    `yaml:"min,omitempty"`
	Max     int    `yaml:"max,omitempty"`
}

// BackgroundTaskStatus represents the current state of a background task
type BackgroundTaskStatus string

// Background task status constants.
const (
	TaskPending  BackgroundTaskStatus = "pending"
	TaskComplete BackgroundTaskStatus = "complete"
	TaskFailed   BackgroundTaskStatus = "failed"
	TaskUnknown  BackgroundTaskStatus = "unknown"
)

// BackgroundTask represents an asynchronous command execution
type BackgroundTask struct {
	ID      string
	Command string
	Status  BackgroundTaskStatus
	Output  string
	Error   string
}

// ExecutionContext maintains state during recipe execution
type ExecutionContext struct {
	Data             string
	Vars             map[string]interface{}
	OperationOutputs map[string]string
	OperationResults map[string]bool
	ProgressMode     bool
	templateFuncs    template.FuncMap
	BackgroundTasks  map[string]*BackgroundTask
	BackgroundMutex  sync.RWMutex
	BackgroundWg     sync.WaitGroup
}

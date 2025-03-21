package main

import (
	"errors"
	"fmt"
	"github.com/rogpeppe/go-internal/testscript"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockFileSystem is used to mock file system operations
type MockFileSystem struct {
	mock.Mock
}

func (m *MockFileSystem) ReadFile(filename string) ([]byte, error) {
	args := m.Called(filename)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockFileSystem) Stat(filename string) (os.FileInfo, error) {
	args := m.Called(filename)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(os.FileInfo), args.Error(1)
}

func (m *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	args := m.Called(path, perm)
	return args.Error(0)
}

func (m *MockFileSystem) Open(path string) (*os.File, error) {
	args := m.Called(path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*os.File), args.Error(1)
}

func (m *MockFileSystem) Create(path string) (*os.File, error) {
	args := m.Called(path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*os.File), args.Error(1)
}

// MockFileInfo is used to mock file info
type MockFileInfo struct {
	mock.Mock
	NameVal    string
	SizeVal    int64
	ModeVal    os.FileMode
	ModTimeVal time.Time
	IsDirVal   bool
}

func (m *MockFileInfo) Name() string {
	return m.NameVal
}

func (m *MockFileInfo) Size() int64 {
	return m.SizeVal
}

func (m *MockFileInfo) Mode() os.FileMode {
	return m.ModeVal
}

func (m *MockFileInfo) ModTime() time.Time {
	return m.ModTimeVal
}

func (m *MockFileInfo) IsDir() bool {
	return m.IsDirVal
}

func (m *MockFileInfo) Sys() interface{} {
	return nil
}

// NewMockFileInfo creates a new MockFileInfo
func NewMockFileInfo(name string, isDir bool) *MockFileInfo {
	return &MockFileInfo{
		NameVal:    name,
		SizeVal:    1024,
		ModeVal:    0644,
		ModTimeVal: time.Now(),
		IsDirVal:   isDir,
	}
}

// MockCommandExecutor is used to mock command execution
type MockCommandExecutor struct {
	mock.Mock
}

func (m *MockCommandExecutor) Execute(cmd string, input string, mode string, outputFormat string) (string, error) {
	args := m.Called(cmd, input, mode, outputFormat)
	return args.String(0), args.Error(1)
}

// Test fixtures
var testConfig = `
recipes:
  - name: test-recipe
    description: A test recipe
    category: test
    operations:
      - name: Test Operation
        id: test-op
        command: echo "Hello, World!"
  - name: another-recipe
    description: Another test recipe
    operations:
      - name: Operation 1
        command: echo "Operation 1"
      - name: Operation 2
        command: echo "Operation 2"
        condition: "$test == true"
`

var testRecipe = Recipe{
	Name:        "test-recipe",
	Description: "A test recipe",
	Category:    "test",
	Operations: []Operation{
		{
			Name:    "Test Operation",
			ID:      "test-op",
			Command: "echo \"Hello, World!\"",
		},
	},
}

// TestLoadConfig tests the loadConfig function
func TestLoadConfig(t *testing.T) {
	mockFS := new(MockFileSystem)
	mockFS.On("ReadFile", "test-config.yaml").Return([]byte(testConfig), nil).Maybe()
	mockFS.On("ReadFile", "non-existent.yaml").Return(nil, errors.New("file not found")).Maybe()
	mockFS.On("ReadFile", "invalid.yaml").Return([]byte("invalid: yaml: content"), nil).Maybe()

	patches := gomonkey.ApplyFunc(os.ReadFile, mockFS.ReadFile)
	defer patches.Reset()

	tests := []struct {
		name       string
		filename   string
		wantConfig *Config
		wantErr    bool
	}{
		{
			name:       "valid config",
			filename:   "test-config.yaml",
			wantConfig: &Config{Recipes: []Recipe{testRecipe, {Name: "another-recipe", Description: "Another test recipe", Operations: []Operation{{Name: "Operation 1", Command: "echo \"Operation 1\""}, {Name: "Operation 2", Command: "echo \"Operation 2\"", Condition: "$test == true"}}}}},
			wantErr:    false,
		},
		{
			name:       "file not found",
			filename:   "non-existent.yaml",
			wantConfig: nil,
			wantErr:    true,
		},
		{
			name:       "invalid yaml",
			filename:   "invalid.yaml",
			wantConfig: nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotConfig, err := loadConfig(tt.filename)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantConfig.Recipes[0].Name, gotConfig.Recipes[0].Name)
				assert.Equal(t, tt.wantConfig.Recipes[0].Description, gotConfig.Recipes[0].Description)
				assert.Equal(t, tt.wantConfig.Recipes[0].Category, gotConfig.Recipes[0].Category)
				assert.Len(t, gotConfig.Recipes[0].Operations, len(tt.wantConfig.Recipes[0].Operations))
			}
		})
	}
}

// TestLoadRecipes tests the loadRecipes function
func TestLoadRecipes(t *testing.T) {
	testConfigData := []byte(testConfig)
	patches := gomonkey.NewPatches()
	defer patches.Reset()

	patches.ApplyFunc(os.ReadFile, func(filename string) ([]byte, error) {
		if filename == "test-config.yaml" {
			return testConfigData, nil
		}
		return nil, fmt.Errorf("file not found: %s", filename)
	})

	t.Run("load all recipes", func(t *testing.T) {
		recipes, err := loadRecipes([]string{"test-config.yaml"}, "")
		assert.NoError(t, err)
		assert.Len(t, recipes, 2, "Expected 2 recipes, got %d", len(recipes))

		if len(recipes) >= 2 {
			assert.Equal(t, "test-recipe", recipes[0].Name)
			assert.Equal(t, "A test recipe", recipes[0].Description)
			assert.Equal(t, "test", recipes[0].Category)

			assert.Equal(t, "another-recipe", recipes[1].Name)
			assert.Equal(t, "Another test recipe", recipes[1].Description)
		}
	})

	t.Run("filter by category", func(t *testing.T) {
		recipes, err := loadRecipes([]string{"test-config.yaml"}, "test")
		assert.NoError(t, err)
		assert.Len(t, recipes, 1, "Expected 1 recipe, got %d", len(recipes))

		if len(recipes) >= 1 {
			assert.Equal(t, "test-recipe", recipes[0].Name)
		}
	})

	t.Run("no matching category", func(t *testing.T) {
		recipes, err := loadRecipes([]string{"test-config.yaml"}, "non-existent")
		assert.NoError(t, err)
		assert.Len(t, recipes, 0, "Expected 0 recipes, got %d", len(recipes))
	})
}

// TestFindRecipeSourcesByType tests the findRecipeSourcesByType function
func TestFindRecipeSourcesByType(t *testing.T) {
	mockFS := new(MockFileSystem)
	mockDirInfo := NewMockFileInfo(".shef", true)

	mockFS.On("Stat", mock.MatchedBy(func(path string) bool {
		return strings.HasPrefix(path, ".shef")
	})).Return(mockDirInfo, nil).Maybe()

	mockFS.On("Stat", mock.MatchedBy(func(path string) bool {
		return strings.HasPrefix(path, "/home/user/.shef")
	})).Return(mockDirInfo, nil).Maybe()

	mockFS.On("Stat", mock.MatchedBy(func(path string) bool {
		return strings.HasPrefix(path, "/home/user/.config/shef")
	})).Return(mockDirInfo, nil).Maybe()

	mockFS.On("Stat", mock.MatchedBy(func(path string) bool {
		return strings.HasPrefix(path, "/home/user/.local/share/shef")
	})).Return(mockDirInfo, nil).Maybe()

	patches := gomonkey.NewPatches()
	defer patches.Reset()

	patches.ApplyFunc(os.UserHomeDir, func() (string, error) {
		return "/home/user", nil
	})

	patches.ApplyFunc(getXDGConfigHome, func() string {
		return filepath.Join("/home/user", ".config")
	})

	patches.ApplyFunc(getXDGDataHome, func() string {
		return filepath.Join("/home/user", ".local", "share")
	})

	patches.ApplyFunc(os.ReadDir, func(dirname string) ([]os.DirEntry, error) {
		return []os.DirEntry{}, nil
	})

	patches.ApplyFunc(os.Stat, mockFS.Stat)

	tests := []struct {
		name       string
		localDir   bool
		userDir    bool
		publicRepo bool
		want       []string
		wantErr    bool
	}{
		{
			name:       "local directory only",
			localDir:   true,
			userDir:    false,
			publicRepo: false,
			want:       []string{},
			wantErr:    false,
		},
		{
			name:       "user directory only",
			localDir:   false,
			userDir:    true,
			publicRepo: false,
			want:       []string{},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findRecipeSourcesByType(tt.localDir, tt.userDir, tt.publicRepo)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, got, len(tt.want))
			}
		})
	}
}

// TestRenderTemplate tests the renderTemplate function
func TestRenderTemplate(t *testing.T) {
	tests := []struct {
		name    string
		tmplStr string
		vars    map[string]interface{}
		want    string
		wantErr bool
	}{
		{
			name:    "simple template",
			tmplStr: "Hello, {{.name}}!",
			vars:    map[string]interface{}{"name": "World"},
			want:    "Hello, World!",
			wantErr: false,
		},
		{
			name:    "template with function",
			tmplStr: "{{trim .text}}",
			vars:    map[string]interface{}{"text": "  trimmed  "},
			want:    "trimmed",
			wantErr: false,
		},
		{
			name:    "invalid template",
			tmplStr: "Hello, {{.name!",
			vars:    map[string]interface{}{"name": "World"},
			want:    "",
			wantErr: true,
		},
		{
			name:    "missing variable",
			tmplStr: "Hello, {{.missing}}!",
			vars:    map[string]interface{}{"name": "World"},
			want:    "Hello, false!",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderTemplate(tt.tmplStr, tt.vars)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestTransformOutput tests the transformOutput function with a simpler transform
func TestTransformOutput(t *testing.T) {
	ctx := &ExecutionContext{
		Vars:             map[string]interface{}{"var1": "value1"},
		OperationOutputs: map[string]string{"op1": "output1"},
		OperationResults: map[string]bool{"op1": true},
	}

	tests := []struct {
		name      string
		output    string
		transform string
		want      string
		wantErr   bool
	}{
		{
			name:      "simple transform",
			output:    "hello world",
			transform: `{{print "HELLO WORLD"}}`,
			want:      "HELLO WORLD",
			wantErr:   false,
		},
		{
			name:      "transform with variables",
			output:    "hello",
			transform: "{{.input}} {{.var1}}",
			want:      "hello value1",
			wantErr:   false,
		},
		{
			name:      "transform with operation output",
			output:    "hello",
			transform: "{{.input}} {{.op1}}",
			want:      "hello output1",
			wantErr:   false,
		},
		{
			name:      "invalid transform",
			output:    "hello",
			transform: "{{.input} {{.var1}}",
			want:      "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transformOutput(tt.output, tt.transform, ctx)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestExecuteCommand tests the executeCommand function
func TestExecuteCommand(t *testing.T) {
	t.Run("test output formatting", func(t *testing.T) {
		formatOutput := func(output, format string) string {
			switch format {
			case "trim":
				return strings.TrimSpace(output)
			case "lines":
				var lines []string
				for _, line := range strings.Split(output, "\n") {
					if trimmedLine := strings.TrimSpace(line); trimmedLine != "" {
						lines = append(lines, trimmedLine)
					}
				}
				return strings.Join(lines, "\n")
			case "raw", "":
				return output
			default:
				return output
			}
		}

		testCases := []struct {
			name         string
			rawOutput    string
			outputFormat string
			expected     string
		}{
			{
				name:         "raw format",
				rawOutput:    "test\n",
				outputFormat: "raw",
				expected:     "test\n",
			},
			{
				name:         "trimmed format",
				rawOutput:    "test\n",
				outputFormat: "trim",
				expected:     "test",
			},
			{
				name:         "lines format",
				rawOutput:    "test1\n  test2  \n\ntest3",
				outputFormat: "lines",
				expected:     "test1\ntest2\ntest3",
			},
			{
				name:         "default format (raw)",
				rawOutput:    "test\n",
				outputFormat: "",
				expected:     "test\n",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := formatOutput(tc.rawOutput, tc.outputFormat)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	mockCmd := new(MockCommandExecutor)
	patches := gomonkey.ApplyFunc(executeCommand, mockCmd.Execute)
	defer patches.Reset()

	tests := []struct {
		name          string
		cmd           string
		input         string
		executionMode string
		outputFormat  string
		mockOutput    string
		mockError     error
		wantErr       bool
	}{
		{
			name:          "standard execution",
			cmd:           "echo 'test'",
			input:         "",
			executionMode: "standard",
			outputFormat:  "raw",
			mockOutput:    "test\n",
			mockError:     nil,
			wantErr:       false,
		},
		{
			name:          "execution with input",
			cmd:           "cat",
			input:         "test input",
			executionMode: "standard",
			outputFormat:  "raw",
			mockOutput:    "test input",
			mockError:     nil,
			wantErr:       false,
		},
		{
			name:          "command error",
			cmd:           "invalid-command",
			input:         "",
			executionMode: "standard",
			outputFormat:  "raw",
			mockOutput:    "",
			mockError:     errors.New("command not found"),
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCmd.On("Execute", tt.cmd, tt.input, tt.executionMode, tt.outputFormat).Return(tt.mockOutput, tt.mockError).Once()

			got, err := executeCommand(tt.cmd, tt.input, tt.executionMode, tt.outputFormat)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.mockOutput, got)
			}
		})
	}
}

// TestEvaluateCondition tests the evaluateCondition function
func TestEvaluateCondition(t *testing.T) {
	ctx := &ExecutionContext{
		Vars:             map[string]interface{}{"test": true, "number": 42, "text": "value"},
		OperationOutputs: map[string]string{"op1": "output1"},
		OperationResults: map[string]bool{"op1": true, "op2": false},
	}

	tests := []struct {
		name      string
		condition string
		want      bool
		wantErr   bool
	}{
		{
			name:      "empty condition",
			condition: "",
			want:      true,
			wantErr:   false,
		},
		{
			name:      "true condition",
			condition: "true",
			want:      true,
			wantErr:   false,
		},
		{
			name:      "false condition",
			condition: "false",
			want:      false,
			wantErr:   false,
		},
		{
			name:      "variable equality (true)",
			condition: "$test == true",
			want:      true,
			wantErr:   false,
		},
		{
			name:      "variable equality (false)",
			condition: "$test == false",
			want:      false,
			wantErr:   false,
		},
		{
			name:      "variable inequality (true)",
			condition: "$test != false",
			want:      true,
			wantErr:   false,
		},
		{
			name:      "variable inequality (false)",
			condition: "$test != true",
			want:      false,
			wantErr:   false,
		},
		{
			name:      "operation success",
			condition: "op1.success",
			want:      true,
			wantErr:   false,
		},
		{
			name:      "operation failure",
			condition: "op2.failure",
			want:      true,
			wantErr:   false,
		},
		{
			name:      "numeric comparison (>)",
			condition: "$number > 10",
			want:      true,
			wantErr:   false,
		},
		{
			name:      "numeric comparison (<)",
			condition: "$number < 10",
			want:      false,
			wantErr:   false,
		},
		{
			name:      "numeric comparison (>=)",
			condition: "$number >= 42",
			want:      true,
			wantErr:   false,
		},
		{
			name:      "numeric comparison (<=)",
			condition: "$number <= 42",
			want:      true,
			wantErr:   false,
		},
		{
			name:      "and condition (true)",
			condition: "$test == true && $number > 10",
			want:      true,
			wantErr:   false,
		},
		{
			name:      "and condition (false)",
			condition: "$test == true && $number < 10",
			want:      false,
			wantErr:   false,
		},
		{
			name:      "or condition (true)",
			condition: "$test == false || $number > 10",
			want:      true,
			wantErr:   false,
		},
		{
			name:      "or condition (false)",
			condition: "$test == false || $number < 10",
			want:      false,
			wantErr:   false,
		},
		{
			name:      "not condition (true)",
			condition: "!$test == false",
			want:      true,
			wantErr:   false,
		},
		{
			name:      "not condition (false)",
			condition: "!$test == true",
			want:      false,
			wantErr:   false,
		},
		{
			name:      "template condition",
			condition: "{{if eq .test true}}true{{else}}false{{end}}",
			want:      true,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evaluateCondition(tt.condition, ctx)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestHandlePromptSimple is a minimal test for handlePrompt
func TestHandlePromptSimple(t *testing.T) {
	t.Run("prompts are defined correctly", func(t *testing.T) {
		inputPrompt := Prompt{
			Name:    "input_test",
			Type:    "input",
			Message: "Enter a value:",
			Default: "default",
		}

		assert.Equal(t, "input", inputPrompt.Type)
		assert.Equal(t, "Enter a value:", inputPrompt.Message)
	})
}

// TestGetPromptOptions tests the getPromptOptions function
func TestGetPromptOptions(t *testing.T) {
	ctx := &ExecutionContext{
		OperationOutputs: map[string]string{
			"sourceOp": "option1\noption2\noption3",
		},
	}

	tests := []struct {
		name    string
		prompt  Prompt
		want    []string
		wantErr bool
	}{
		{
			name: "options from prompt",
			prompt: Prompt{
				Options: []string{"option1", "option2", "option3"},
			},
			want:    []string{"option1", "option2", "option3"},
			wantErr: false,
		},
		{
			name: "options from source operation",
			prompt: Prompt{
				SourceOp: "sourceOp",
			},
			want:    []string{"option1", "option2", "option3"},
			wantErr: false,
		},
		{
			name: "source operation not found",
			prompt: Prompt{
				SourceOp: "nonExistentOp",
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getPromptOptions(tt.prompt, ctx)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestParseOptionsFromOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []string
	}{
		{
			name:   "simple options",
			output: "option1\noption2\noption3",
			want:   []string{"option1", "option2", "option3"},
		},
		{
			name:   "options with whitespace",
			output: "  option1  \n  option2  \n  option3  ",
			want:   []string{"option1", "option2", "option3"},
		},
		{
			name:   "empty lines",
			output: "option1\n\noption2\n\noption3",
			want:   []string{"option1", "option2", "option3"},
		},
		{
			name:   "empty output",
			output: "",
			want:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOptionsFromOutput(tt.output)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestFindRecipeByName tests the findRecipeByName function
func TestFindRecipeByName(t *testing.T) {
	recipes := []Recipe{
		{
			Name:        "recipe1",
			Description: "Recipe 1",
		},
		{
			Name:        "recipe2",
			Description: "Recipe 2",
		},
	}

	tests := []struct {
		name       string
		recipes    []Recipe
		recipeName string
		want       *Recipe
		wantErr    bool
	}{
		{
			name:       "recipe found",
			recipes:    recipes,
			recipeName: "recipe1",
			want:       &recipes[0],
			wantErr:    false,
		},
		{
			name:       "recipe not found",
			recipes:    recipes,
			recipeName: "nonexistent",
			want:       nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findRecipeByName(tt.recipes, tt.recipeName)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestFilterLines tests the filterLines function
func TestFilterLines(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		pattern string
		want    string
	}{
		{
			name:    "basic filter",
			input:   "line1\nline2\nline3\nline with pattern\nline5",
			pattern: "pattern",
			want:    "line with pattern",
		},
		{
			name:    "multiple matches",
			input:   "line1\nline2 match\nline3\nline4 match\nline5",
			pattern: "match",
			want:    "line2 match\nline4 match",
		},
		{
			name:    "no matches",
			input:   "line1\nline2\nline3",
			pattern: "pattern",
			want:    "",
		},
		{
			name:    "empty input",
			input:   "",
			pattern: "pattern",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterLines(tt.input, tt.pattern)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestCutFields tests the cutFields function
func TestCutFields(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		delimiter string
		field     int
		want      string
	}{
		{
			name:      "basic field cutting",
			input:     "field1,field2,field3\nvalueA,valueB,valueC",
			delimiter: ",",
			field:     1,
			want:      "field2\nvalueB",
		},
		{
			name:      "field index out of bounds",
			input:     "field1,field2\nvalueA,valueB",
			delimiter: ",",
			field:     5,
			want:      "",
		},
		{
			name:      "mixed field lengths",
			input:     "field1,field2,field3\nvalueA,valueB",
			delimiter: ",",
			field:     2,
			want:      "field3",
		},
		{
			name:      "tab delimiter",
			input:     "field1\tfield2\tfield3\nvalueA\tvalueB\tvalueC",
			delimiter: "\t",
			field:     0,
			want:      "field1\nvalueA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cutFields(tt.input, tt.delimiter, tt.field)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestJoinArray tests the JoinArray function
func TestJoinArray(t *testing.T) {
	tests := []struct {
		name string
		arr  interface{}
		sep  string
		want string
	}{
		{
			name: "string array",
			arr:  []string{"a", "b", "c"},
			sep:  ",",
			want: "a,b,c",
		},
		{
			name: "interface array",
			arr:  []interface{}{"a", 1, true},
			sep:  "-",
			want: "a-1-true",
		},
		{
			name: "non-array",
			arr:  42,
			sep:  ",",
			want: "42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JoinArray(tt.arr, tt.sep)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestExecuteRecipeSimple is a minimal test for executeRecipe
func TestExecuteRecipeSimple(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	defer func() {
		w.Close()
		os.Stdout = oldStdout
		io.Copy(io.Discard, r)
		r.Close()
	}()

	mockCmd := new(MockCommandExecutor)

	patches := gomonkey.ApplyFunc(executeCommand, mockCmd.Execute)
	defer patches.Reset()

	recipe := Recipe{
		Name:        "simple-recipe",
		Description: "A simple recipe",
		Operations: []Operation{
			{
				Name:    "Simple Operation",
				Command: "echo 'Hello'",
			},
		},
	}

	mockCmd.On("Execute", "echo 'Hello'", "", "", "").Return("Hello", nil).Maybe()

	err := executeRecipe(recipe, "", map[string]interface{}{}, false)
	assert.NoError(t, err)

	mockCmd.AssertCalled(t, "Execute", "echo 'Hello'", "", "", "")
}

// TestProcessRemainingArgs tests handling arguments and flags
func TestProcessRemainingArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantInput string
		wantVars  map[string]interface{}
	}{
		{
			name:      "no args",
			args:      []string{},
			wantInput: "",
			wantVars:  map[string]interface{}{},
		},
		{
			name:      "input only",
			args:      []string{"hello world"},
			wantInput: "hello world",
			wantVars:  map[string]interface{}{},
		},
		{
			name:      "short flag only",
			args:      []string{"-f"},
			wantInput: "",
			wantVars:  map[string]interface{}{"f": true},
		},
		{
			name:      "multiple short flags",
			args:      []string{"-abc"},
			wantInput: "",
			wantVars:  map[string]interface{}{"a": true, "b": true, "c": true},
		},
		{
			name:      "long flag only",
			args:      []string{"--name=John"},
			wantInput: "",
			wantVars:  map[string]interface{}{"name": "John"},
		},
		{
			name:      "long flag with dash",
			args:      []string{"--user-agent=Chrome"},
			wantInput: "",
			wantVars:  map[string]interface{}{"user_agent": "Chrome"},
		},
		{
			name:      "boolean long flag",
			args:      []string{"--verbose"},
			wantInput: "",
			wantVars:  map[string]interface{}{"verbose": true},
		},
		{
			name:      "input and flags",
			args:      []string{"hello world", "-f", "--name=John"},
			wantInput: "hello world",
			wantVars:  map[string]interface{}{"f": true, "name": "John"},
		},
		{
			name:      "complex case",
			args:      []string{"input text", "-v", "--count=5", "--max-retry=3"},
			wantInput: "input text",
			wantVars:  map[string]interface{}{"v": true, "count": "5", "max_retry": "3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotInput, gotVars := processRemainingArgs(tt.args)
			assert.Equal(t, tt.wantInput, gotInput)
			assert.Equal(t, tt.wantVars, gotVars)
		})
	}
}

// ExampleRenderTemplate demonstrates how to use the renderTemplate function
func Example_renderTemplate() {
	vars := map[string]interface{}{
		"name": "World",
	}
	result, err := renderTemplate("Hello, {{.name}}!", vars)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(result)
}

func TestMathTemplateFunctions(t *testing.T) {
	t.Run("mod function", func(t *testing.T) {
		tests := []struct {
			a    int
			b    int
			want int
		}{
			{10, 3, 1},
			{15, 5, 0},
			{7, 2, 1},
			{-10, 3, -1},
			{10, 0, 0},
		}

		modFunc := templateFuncs["mod"].(func(interface{}, interface{}) interface{})
		for _, tt := range tests {
			got := modFunc(tt.a, tt.b)
			intResult, ok := got.(int)
			assert.True(t, ok, "Expected int result from mod(%d, %d)", tt.a, tt.b)
			assert.Equal(t, tt.want, intResult, "mod(%d, %d)", tt.a, tt.b)
		}
	})

	t.Run("round function", func(t *testing.T) {
		tests := []struct {
			value float64
			want  int
		}{
			{3.2, 3},
			{3.5, 4},
			{3.7, 4},
			{-3.2, -3},
			{-3.7, -4},
			{0.0, 0},
		}

		roundFunc := templateFuncs["round"].(func(interface{}) int)
		for _, tt := range tests {
			got := roundFunc(tt.value)
			assert.Equal(t, tt.want, got, "round(%f)", tt.value)
		}
	})

	t.Run("ceil function", func(t *testing.T) {
		tests := []struct {
			value float64
			want  int
		}{
			{3.2, 4},
			{3.0, 3},
			{-3.2, -3},
			{0.1, 1},
		}

		ceilFunc := templateFuncs["ceil"].(func(interface{}) int)
		for _, tt := range tests {
			got := ceilFunc(tt.value)
			assert.Equal(t, tt.want, got, "ceil(%f)", tt.value)
		}
	})

	t.Run("floor function", func(t *testing.T) {
		tests := []struct {
			value float64
			want  int
		}{
			{3.2, 3},
			{3.0, 3},
			{-3.2, -4},
			{0.9, 0},
		}

		floorFunc := templateFuncs["floor"].(func(interface{}) int)
		for _, tt := range tests {
			got := floorFunc(tt.value)
			assert.Equal(t, tt.want, got, "floor(%f)", tt.value)
		}
	})

	t.Run("abs function", func(t *testing.T) {
		tests := []struct {
			value interface{}
			want  interface{}
		}{
			{3, 3},
			{0, 0},
			{-3, 3},
			{3.2, 3.2},
			{0.0, 0},
			{-3.2, 3.2},
		}

		absFunc := templateFuncs["abs"].(func(interface{}) interface{})
		for _, tt := range tests {
			got := absFunc(tt.value)
			assert.Equal(t, tt.want, got, "abs(%d)", tt.value)
		}
	})

	t.Run("max function", func(t *testing.T) {
		tests := []struct {
			a    interface{}
			b    interface{}
			want interface{}
		}{
			{0, 0, 0},
			{5, 10, 10},
			{10, 5, 10},
			{-5, -10, -5},
			{5.9, 10.2, 10.2},
			{10.2, 5.9, 10.2},
			{-3.2, -25.2, -3.2},
			{-25.2, -3.2, -3.2},
		}

		maxFunc := templateFuncs["max"].(func(interface{}, interface{}) interface{})
		for _, tt := range tests {
			got := maxFunc(tt.a, tt.b)
			intResult, ok := got.(interface{})
			assert.True(t, ok, "Expected int result from max(%d, %d)", tt.a, tt.b)
			assert.Equal(t, tt.want, intResult, "max(%d, %d)", tt.a, tt.b)
		}
	})

	t.Run("min function", func(t *testing.T) {
		tests := []struct {
			a    interface{}
			b    interface{}
			want interface{}
		}{
			{0, 0, 0},
			{5, 10, 5},
			{10, 5, 5},
			{-5, -10, -10},
			{5.9, 10.2, 5.9},
			{10.2, 5.9, 5.9},
			{-3.2, -25.2, -25.2},
			{-25.2, -3.2, -25.2},
		}

		minFunc := templateFuncs["min"].(func(interface{}, interface{}) interface{})
		for _, tt := range tests {
			got := minFunc(tt.a, tt.b)
			intResult, ok := got.(interface{})
			assert.True(t, ok, "Expected result from min(%d, %d)", tt.a, tt.b)
			assert.Equal(t, tt.want, intResult, "min(%d, %d)", tt.a, tt.b)
		}
	})

	t.Run("percent function", func(t *testing.T) {
		tests := []struct {
			part  float64
			total float64
			want  float64
		}{
			{50, 100, 50.0},
			{25, 50, 50.0},
			{0, 100, 0.0},
			{100, 0, 0.0},
		}

		percentFunc := templateFuncs["percent"].(func(interface{}, interface{}) interface{})
		for _, tt := range tests {
			got := percentFunc(tt.part, tt.total)
			floatResult, ok := got.(float64)
			if !ok {
				intResult, ok := got.(int)
				assert.True(t, ok, "Expected numeric result from percent(%f, %f)", tt.part, tt.total)
				floatResult = float64(intResult)
			}
			assert.Equal(t, tt.want, floatResult, "percent(%f, %f)", tt.part, tt.total)
		}
	})

	t.Run("formatPercent function", func(t *testing.T) {
		tests := []struct {
			value    float64
			decimals int
			want     string
		}{
			{50.0, 0, "50%"},
			{33.33333, 1, "33.3%"},
			{66.66666, 2, "66.67%"},
			{0.0, 0, "0%"},
		}

		formatPercentFunc := templateFuncs["formatPercent"].(func(interface{}, interface{}) string)
		for _, tt := range tests {
			got := formatPercentFunc(tt.value, tt.decimals)
			assert.Equal(t, tt.want, got, "formatPercent(%f, %d)", tt.value, tt.decimals)
		}
	})

	t.Run("pow function", func(t *testing.T) {
		tests := []struct {
			base     float64
			exponent float64
			want     float64
		}{
			{2.0, 3.0, 8.0},
			{10.0, 2.0, 100.0},
			{5.0, 0.0, 1.0},
			{0.0, 5.0, 0.0},
		}

		powFunc := templateFuncs["pow"].(func(interface{}, interface{}) interface{})
		for _, tt := range tests {
			got := powFunc(tt.base, tt.exponent)
			floatResult, ok := got.(float64)
			if !ok {
				intResult, ok := got.(int)
				assert.True(t, ok, "Expected numeric result from pow(%f, %f)", tt.base, tt.exponent)
				floatResult = float64(intResult)
			}
			assert.Equal(t, tt.want, floatResult, "pow(%f, %f)", tt.base, tt.exponent)
		}
	})

	t.Run("sqrt function", func(t *testing.T) {
		tests := []struct {
			value float64
			want  float64
		}{
			{4.0, 2.0},
			{9.0, 3.0},
			{0.0, 0.0},
			{2.0, 1.4142135623730951},
		}

		sqrtFunc := templateFuncs["sqrt"].(func(interface{}) interface{})
		for _, tt := range tests {
			got := sqrtFunc(tt.value)
			floatResult, ok := got.(float64)
			if !ok {
				intResult, ok := got.(int)
				assert.True(t, ok, "Expected numeric result from sqrt(%f)", tt.value)
				floatResult = float64(intResult)
			}
			assert.Equal(t, tt.want, floatResult, "sqrt(%f)", tt.value)
		}
	})

	t.Run("roundTo function", func(t *testing.T) {
		tests := []struct {
			value    float64
			decimals int
			want     float64
		}{
			{3.14159, 2, 3.14},
			{3.14159, 3, 3.142},
			{3.14159, 0, 3.0},
			{-3.14159, 2, -3.14},
		}

		roundToFunc := templateFuncs["roundTo"].(func(interface{}, interface{}) interface{})
		for _, tt := range tests {
			got := roundToFunc(tt.value, tt.decimals)
			floatResult, ok := got.(float64)
			if !ok {
				intResult, ok := got.(int)
				assert.True(t, ok, "Expected numeric result from roundTo(%f, %d)", tt.value, tt.decimals)
				floatResult = float64(intResult)
			}
			assert.Equal(t, tt.want, floatResult, "roundTo(%f, %d)", tt.value, tt.decimals)
		}
	})

	t.Run("rand function", func(t *testing.T) {
		randFunc := templateFuncs["rand"].(func(interface{}, interface{}) int)

		for i := 0; i < 100; i++ {
			minNum, maxNum := 1, 10
			got := randFunc(minNum, maxNum)
			assert.GreaterOrEqual(t, got, minNum, "rand(%d, %d) result below minimum", minNum, maxNum)
			assert.LessOrEqual(t, got, maxNum, "rand(%d, %d) result above maximum", minNum, maxNum)
		}

		for i := 0; i < 10; i++ {
			minNum, maxNum := 10, 1
			got := randFunc(minNum, maxNum)
			assert.GreaterOrEqual(t, got, maxNum, "rand(%d, %d) with swapped values", minNum, maxNum)
			assert.LessOrEqual(t, got, minNum, "rand(%d, %d) with swapped values", minNum, maxNum)
		}
	})

	t.Run("formatNumber function", func(t *testing.T) {
		tests := []struct {
			format string
			args   []interface{}
			want   string
		}{
			{"%d", []interface{}{42}, "42"},
			{"%.2f", []interface{}{3.14159}, "3.14"},
			{"%s=%d", []interface{}{"answer", 42}, "answer=42"},
		}

		formatNumberFunc := templateFuncs["formatNumber"].(func(string, ...interface{}) string)
		for _, tt := range tests {
			if tt.format == "%s=%d" {
				continue
			}
			got := formatNumberFunc(tt.format, tt.args...)
			assert.Equal(t, tt.want, got, "formatNumber(%s, %v)", tt.format, tt.args)
		}
	})

	t.Run("template rendering with math functions", func(t *testing.T) {
		vars := map[string]interface{}{
			"num1": 10,
			"num2": 3,
			"val":  3.14159,
		}

		tests := []struct {
			name     string
			template string
			want     string
		}{
			{"mod", "{{mod .num1 .num2}}", "1"},
			{"round", "{{round .val}}", "3"},
			{"ceil", "{{ceil .val}}", "4"},
			{"floor", "{{floor .val}}", "3"},
			{"abs", "{{abs (sub 5 10)}}", "5"},
			{"abs_float", "{{abs (sub 5.3 10.1)}}", "4.8"},
			{"percent", "{{percent 25 100}}", "25"},
			{"formatPercent", "{{formatPercent 33.333 1}}", "33.3%"},
			{"roundTo", "{{roundTo .val 2}}", "3.14"},
			{"multi-operation", "{{add (mul 2 3) (div 10 2)}}", "11"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := renderTemplate(tt.template, vars)
				assert.NoError(t, err)
				assert.Equal(t, tt.want, result)
			})
		}
	})
}

// TestShefEndToEnd runs all end-to-end tests within ./testdata
func TestShefEndToEnd(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
		Setup: func(e *testscript.Env) error {
			recipesDir := filepath.Join("testdata", "recipes")
			if _, err := os.Stat(recipesDir); err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}

			entries, err := os.ReadDir(recipesDir)
			if err != nil {
				return err
			}

			for _, entry := range entries {
				if filepath.Ext(entry.Name()) == ".yaml" {
					content, err := os.ReadFile(filepath.Join(recipesDir, entry.Name()))
					if err != nil {
						return err
					}

					err = os.WriteFile(filepath.Join(e.WorkDir, entry.Name()), content, 0644)
					if err != nil {
						return err
					}
				}
			}
			return nil
		},
	})
}

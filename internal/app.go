package internal

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/agnivade/levenshtein"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

func loadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func loadRecipes(sources []string, category string) ([]Recipe, error) {
	var allRecipes []Recipe
	lowerCategory := strings.ToLower(category)

	for _, source := range sources {
		config, err := loadConfig(source)
		if err != nil {
			fmt.Printf("Warning: Failed to load recipes from %s: %v\n", source, err)
			continue
		}

		if category == "" {
			allRecipes = append(allRecipes, config.Recipes...)
			continue
		}

		for _, recipe := range config.Recipes {
			if strings.ToLower(recipe.Category) == lowerCategory {
				allRecipes = append(allRecipes, recipe)
			}
		}
	}

	return allRecipes, nil
}

func findRecipeSourcesByType(localDir, userDir, publicRepo bool) ([]string, error) {
	var sources []string

	findYamlFiles := func(root string) ([]string, error) {
		var files []string
		visited := make(map[string]bool)

		var walkDir func(path string) error
		walkDir = func(path string) error {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return nil
			}

			if visited[absPath] {
				return nil
			}
			visited[absPath] = true

			// Get file info, will follow symlinks
			fileInfo, err := os.Stat(path)
			if err != nil {
				return nil
			}

			if fileInfo.IsDir() {
				entries, err := os.ReadDir(path)
				if err != nil {
					return nil
				}

				for _, entry := range entries {
					entryPath := filepath.Join(path, entry.Name())
					if err := walkDir(entryPath); err != nil {
						return err
					}
				}
			} else if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
				files = append(files, path)
			}

			return nil
		}

		err := walkDir(root)
		return files, err
	}

	if localDir {
		if _, err := os.Stat(".shef"); err == nil {
			if localFiles, err := findYamlFiles(".shef"); err == nil {
				sources = append(sources, localFiles...)
			}
		}
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		userRoot := filepath.Join(homeDir, ".shef")

		if userDir {
			userSpecificDir := filepath.Join(userRoot, "user")
			if _, err := os.Stat(userSpecificDir); err == nil {
				if userFiles, err := findYamlFiles(userSpecificDir); err == nil {
					sources = append(sources, userFiles...)
				}
			}
		}

		if publicRepo {
			publicSpecificDir := filepath.Join(userRoot, "public")
			if _, err := os.Stat(publicSpecificDir); err == nil {
				if publicFiles, err := findYamlFiles(publicSpecificDir); err == nil {
					sources = append(sources, publicFiles...)
				}
			}
		}

		if isLinux() && (userDir || publicRepo) {
			sources = addXDGRecipeSources(sources, userDir, publicRepo, findYamlFiles)
		}
	}

	return sources, nil
}

func formatOutput(output string, outputFormat string) (string, error) {
	switch outputFormat {
	case "trim":
		return strings.TrimSpace(output), nil
	case "lines":
		var lines []string
		for _, line := range strings.Split(output, "\n") {
			if trimmedLine := strings.TrimSpace(line); trimmedLine != "" {
				lines = append(lines, trimmedLine)
			}
		}
		return strings.Join(lines, "\n"), nil
	case "raw", "":
		return output, nil
	default:
		return output, nil
	}
}

func resolveValue(value string, ctx *ExecutionContext) (string, error) {
	if strings.Contains(value, "{{") && strings.Contains(value, "}}") {
		rendered, err := renderTemplate(value, ctx.templateVars())
		if err != nil {
			return "", err
		}
		return rendered, nil
	}

	if strings.HasPrefix(value, "$") || strings.HasPrefix(value, ".") {
		varName := value
		if strings.HasPrefix(varName, "$") {
			varName = varName[1:]
		}
		if strings.HasPrefix(varName, ".") {
			varName = varName[1:]
		}

		if val, exists := ctx.Vars[varName]; exists {
			return fmt.Sprintf("%v", val), nil
		}
		if val, exists := ctx.OperationOutputs[varName]; exists {
			return val, nil
		}
		return "false", nil
	}

	return value, nil
}

func executeRecipe(recipe Recipe, input string, vars map[string]interface{}, debug bool) error {
	ctx := ExecutionContext{
		Data:             "",
		Vars:             make(map[string]interface{}),
		OperationOutputs: make(map[string]string),
		OperationResults: make(map[string]bool),
	}

	ctx.templateFuncs = extendTemplateFuncs(templateFuncs, &ctx)
	vars["context"] = &ctx

	for k, v := range vars {
		ctx.Vars[k] = v
	}

	if input != "" {
		ctx.Vars["input"] = input
		ctx.Data = input
	}

	opMap := make(map[string]Operation)
	registerOperations(recipe.Operations, opMap)

	handlerIDs := make(map[string]bool)
	identifyHandlers(recipe.Operations, handlerIDs)

	if debug {
		fmt.Println("Registered operations:")
		for id := range opMap {
			handlerStatus := ""
			if handlerIDs[id] {
				handlerStatus = " (handler)"
			}
			fmt.Printf("  - %s%s\n", id, handlerStatus)
		}
	}

	var executeOp func(op Operation, depth int) (bool, error)
	executeOp = func(op Operation, depth int) (bool, error) {
		if depth > 50 {
			return false, fmt.Errorf("possible infinite loop detected (max depth reached)")
		}

		// 1. Check the condition first
		if op.Condition != "" {
			if debug {
				fmt.Printf("Evaluating condition: %s\n", op.Condition)
			}
			result, err := evaluateCondition(op.Condition, &ctx)
			if err != nil {
				return false, fmt.Errorf("condition evaluation failed: %w", err)
			}

			if !result {
				if debug {
					fmt.Printf("Skipping operation '%s' (condition not met)\n", op.Name)
				}
				return false, nil
			}
		}

		// 2. Run the prompts
		for _, prompt := range op.Prompts {
			value, err := handlePrompt(prompt, &ctx)
			if err != nil {
				return false, err
			}

			if value == ExitPrompt && (prompt.Type == "select" || prompt.Type == "autocomplete") {
				os.Exit(0)
			}

			varName := prompt.Name
			if prompt.ID != "" {
				varName = prompt.ID
			}
			ctx.Vars[varName] = value
		}

		// 3. Run the control flow if it exists
		var controlFlowExit bool
		var controlFlowErr error
		if op.ControlFlow != nil {
			flowMap, ok := op.ControlFlow.(map[string]interface{})
			if !ok {
				return false, fmt.Errorf("invalid control_flow structure")
			}

			typeVal, ok := flowMap["type"].(string)
			if !ok {
				return false, fmt.Errorf("control_flow requires a 'type' field")
			}

			switch typeVal {

			case "foreach":
				forEach, err := op.GetForEachFlow()
				if err != nil {
					return false, err
				}
				controlFlowExit, controlFlowErr = ExecuteForEach(op, forEach, &ctx, depth, executeOp, debug)

			case "while":
				whileFlow, err := op.GetWhileFlow()
				if err != nil {
					return false, err
				}
				controlFlowExit, controlFlowErr = ExecuteWhile(op, whileFlow, &ctx, depth, executeOp, debug)

			case "for":
				forFlow, err := op.GetForFlow()
				if err != nil {
					return false, err
				}
				controlFlowExit, controlFlowErr = ExecuteFor(op, forFlow, &ctx, depth, executeOp, debug)

			default:
				return false, fmt.Errorf("unknown control_flow type: %s", typeVal)
			}
		}

		if controlFlowErr != nil {
			return op.Exit, controlFlowErr
		}

		if controlFlowExit {
			if debug {
				fmt.Printf("Exiting recipe due to exit flag inside for control flow\n")
			}
			return true, nil
		}

		// 4. Run the command
		cmd, err := renderTemplate(op.Command, ctx.templateVars())
		if err != nil {
			return false, fmt.Errorf("failed to render command template: %w", err)
		}

		if debug {
			fmt.Printf("Running command: %s\n", cmd)
		}

		ctx.Vars["error"] = ""

		if op.ExecutionMode == "background" {
			if err := executeBackgroundCommand(op, &ctx, opMap, executeOp, depth, debug); err != nil {
				return false, err
			}
			return op.Exit, nil
		}

		output, err := executeCommand(cmd, ctx.Data, op.ExecutionMode, op.OutputFormat)
		operationSuccess := err == nil

		if op.ID != "" {
			ctx.OperationResults[op.ID] = operationSuccess
		}

		if err != nil {
			ctx.Vars["error"] = err.Error()

			if debug {
				fmt.Printf("Warning: command execution had errors: %v\n", err)
			}

			if op.OnFailure != "" {
				if debug {
					fmt.Printf("Executing on_failure handler: %s\n", op.OnFailure)
				}

				nextOp, exists := opMap[op.OnFailure]
				if !exists {
					return false, fmt.Errorf("on_failure operation %s not found", op.OnFailure)
				}
				shouldExit, err := executeOp(nextOp, depth+1)
				return shouldExit || op.Exit, err
			}

			fmt.Printf("Error in operation '%s': \n%v\n", op.Name, err)

			var continueExecution bool
			prompt := &survey.Confirm{
				Message: "Continue with recipe execution?",
				Default: false,
			}
			if err := survey.AskOne(prompt, &continueExecution); err != nil {
				return false, err
			}

			if !continueExecution {
				return true, fmt.Errorf("recipe execution aborted by user after command error")
			}
		}

		// 5. Run the transforms
		if op.Transform != "" {
			transformedOutput, err := transformOutput(output, op.Transform, &ctx)
			if err != nil {
				if debug {
					fmt.Printf("Warning: output transformation failed: %v\n", err)
				}
			} else {
				output = transformedOutput
			}
		}

		ctx.Data = output

		if op.ID != "" {
			ctx.OperationOutputs[op.ID] = strings.TrimSpace(output)
		}

		if output != "" && !op.Silent {
			if ctx.ProgressMode {
				firstLine := output
				if idx := strings.Index(output, "\n"); idx >= 0 {
					firstLine = output[:idx]
				}
				fmt.Print("\r" + firstLine + " " + "\033[K")
			} else {
				fmt.Println(output)
			}
		}

		// 7. Run the on_success handler
		if op.OnSuccess != "" && operationSuccess {
			nextOp, exists := opMap[op.OnSuccess]
			if !exists {
				return false, fmt.Errorf("on_success operation %s not found", op.OnSuccess)
			}
			shouldExit, err := executeOp(nextOp, depth+1)
			return shouldExit || op.Exit, err
		}

		if debug {
			fmt.Printf("Operation %s result: %v\n", op.ID, ctx.OperationResults[op.ID])
			fmt.Printf("Handler for on_success: '%s'\n", op.OnSuccess)
			fmt.Printf("Handler for on_failure: '%s'\n", op.OnFailure)
			if op.Exit {
				fmt.Printf("Exit flag is set. Will exit after this operation.\n")
			}
			if op.Break {
				fmt.Printf("Break flag is set. Will break out of control flow.\n")
			}
		}

		return op.Exit, nil
	}

	for i, op := range recipe.Operations {
		if op.ID != "" && handlerIDs[op.ID] {
			if debug {
				fmt.Printf("Skipping handler operation %d: %s (ID: %s)\n", i+1, op.Name, op.ID)
			}
			continue
		}

		if debug {
			fmt.Printf("Executing operation %d: %s\n", i+1, op.Name)
		}

		shouldExit, err := executeOp(op, 0)
		if err != nil {
			return err
		}

		if shouldExit {
			if debug {
				fmt.Printf("Exiting recipe execution after operation: %s\n", op.Name)
			}
			return nil
		}
	}

	ctx.BackgroundWg.Wait()
	ctx.BackgroundMutex.RLock()
	for id, task := range ctx.BackgroundTasks {
		if task.Status == TaskComplete {
			var op *Operation
			for _, recipeOp := range recipe.Operations {
				if recipeOp.ID == id {
					op = &recipeOp
					break
				}
			}
			if op == nil || !op.Silent {
				if task.Output != "" && debug {
					fmt.Printf("Background task %s output: %s\n", id, task.Output)
				}
			}
		} else if task.Status == TaskFailed && task.Error != "" && debug {
			fmt.Printf("Background task %s failed: %s\n", id, task.Error)
		}
	}
	ctx.BackgroundMutex.RUnlock()

	return nil
}

func registerOperations(operations []Operation, opMap map[string]Operation) {
	for _, op := range operations {
		if op.ID != "" {
			opMap[op.ID] = op
		}

		if op.ControlFlow != nil && len(op.Operations) > 0 {
			registerOperations(op.Operations, opMap)
		}
	}
}

func identifyHandlers(operations []Operation, handlerIDs map[string]bool) {
	for _, op := range operations {
		if op.OnSuccess != "" {
			handlerIDs[op.OnSuccess] = true
		}
		if op.OnFailure != "" {
			handlerIDs[op.OnFailure] = true
		}

		if op.ControlFlow != nil && len(op.Operations) > 0 {
			identifyHandlers(op.Operations, handlerIDs)
		}
	}
}

func listRecipes(recipes []Recipe) {
	if len(recipes) == 0 {
		fmt.Println(FormatText("No recipes found.", ColorYellow, StyleNone))
		return
	}

	fmt.Println("\nAvailable recipes:")

	categories := make(map[string][]Recipe)
	for _, recipe := range recipes {
		cat := recipe.Category
		if cat == "" {
			cat = "uncategorized"
		}
		categories[cat] = append(categories[cat], recipe)
	}

	var categoryNames []string
	for category := range categories {
		categoryNames = append(categoryNames, category)
	}
	sort.Strings(categoryNames)

	for _, category := range categoryNames {
		catRecipes := categories[category]

		sort.Slice(catRecipes, func(i, j int) bool {
			return catRecipes[i].Name < catRecipes[j].Name
		})

		fmt.Printf(
			"\n  %s%s%s\n",
			FormatText("[", ColorNone, StyleDim),
			FormatText(strings.ToLower(category), ColorMagenta, StyleNone),
			FormatText("]", ColorNone, StyleDim),
		)
		for _, recipe := range catRecipes {
			fmt.Printf(
				"    %s %s: %s\n",
				FormatText("•", ColorNone, StyleDim),
				FormatText(strings.ToLower(recipe.Name), ColorGreen, StyleBold),
				strings.ToLower(recipe.Description),
			)
		}
	}

	fmt.Printf("\n\n")
}

func findRecipeByName(recipes []Recipe, name string) (*Recipe, error) {
	lowerName := strings.ToLower(name)
	for _, recipe := range recipes {
		if strings.ToLower(recipe.Name) == lowerName {
			return &recipe, nil
		}
	}
	return nil, fmt.Errorf("recipe not found: %s", name)
}

func displayRecipeHelp(recipe *Recipe) {
	name := strings.ToLower(recipe.Name)
	category := strings.ToLower(recipe.Category)
	description := strings.ToLower(recipe.Description)

	fmt.Printf("%s:\n    %s - %s\n", "NAME", name, description)

	if recipe.Category != "" {
		fmt.Printf("\n%s:\n    %s\n", "CATEGORY", category)
	}

	if recipe.Author != "" {
		fmt.Printf("\n%s:\n    %s\n", "AUTHOR", recipe.Author)
	}

	fmt.Printf("\n%s:\n    shef %s [input] [options]\n", "USAGE", name)
	if recipe.Category != "" {
		fmt.Printf("    shef %s %s [input] [options]\n", category, name)
	}

	if recipe.Help != "" {
		indentedText := indentText(recipe.Help, 4)
		fmt.Printf("\n%s:\n%s\n", "OVERVIEW", indentedText)
	} else {
		fmt.Printf("\n%s:\n    %s\n", "OVERVIEW", "No detailed help available for this recipe.")
	}

	fmt.Println("")
}

func indentText(text string, spaces int) string {
	indent := strings.Repeat(" ", spaces)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = indent + line
		}
	}
	return strings.Join(lines, "\n")
}

func Run() {
	log.SetFlags(0)

	app := &cli.App{
		Name:    "shef",
		Usage:   "Shef is a powerful CLI tool for cooking up shell recipes.",
		Version: Version,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"d"},
				Usage:   "Enable debug output",
				Value:   false,
			},
			&cli.BoolFlag{
				Name:    "local",
				Aliases: []string{"L"},
				Usage:   "Force local recipes first",
			},
			&cli.BoolFlag{
				Name:    "user",
				Aliases: []string{"U"},
				Usage:   "Force user recipes first",
			},
			&cli.BoolFlag{
				Name:    "public",
				Aliases: []string{"P"},
				Usage:   "Force public recipes first",
			},
			&cli.StringFlag{
				Name:    "category",
				Aliases: []string{"c"},
				Usage:   "Filter by category",
			},
			&cli.PathFlag{
				Name:    "recipe-file",
				Aliases: []string{"r"},
				Usage:   "Path to the recipe file (note: additional recipe flags not supported)",
			},
		},
		Action: func(c *cli.Context) error {
			args := c.Args().Slice()
			if len(args) == 0 && !c.IsSet("recipe-file") {
				err := cli.ShowAppHelp(c)
				if err != nil {
					return err
				}
				return nil
			}

			sourcePriority := getSourcePriority(c)
			return handleRunCommand(c, args, sourcePriority)
		},
		Commands: []*cli.Command{
			{
				Name:    "list",
				Aliases: []string{"ls", "l"},
				Usage:   "List available recipes",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "local",
						Aliases: []string{"l"},
						Usage:   "Filter to local recipes only",
					},
					&cli.BoolFlag{
						Name:    "user",
						Aliases: []string{"u"},
						Usage:   "Filter to user recipes only",
					},
					&cli.BoolFlag{
						Name:    "public",
						Aliases: []string{"p"},
						Usage:   "Filter to public recipes only",
					},
					&cli.BoolFlag{
						Name:    "json",
						Aliases: []string{"j"},
						Usage:   "Output results in JSON format",
					},
					&cli.StringFlag{
						Name:    "category",
						Aliases: []string{"c"},
						Usage:   "Filter by category",
					},
				},
				Action: func(c *cli.Context) error {
					args := c.Args().Slice()
					sourcePriority := getSourcePriority(c)
					return handleListCommand(c, args, sourcePriority)
				},
			},
			{
				Name:    "sync",
				Aliases: []string{"s"},
				Usage:   "Sync public recipes locally",
				Action: func(c *cli.Context) error {
					return syncPublicRecipes()
				},
			},
			{
				Name:    "which",
				Aliases: []string{"w"},
				Usage:   "Show the location of a recipe file",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "local",
						Aliases: []string{"L"},
						Usage:   "Force local recipes first",
					},
					&cli.BoolFlag{
						Name:    "user",
						Aliases: []string{"U"},
						Usage:   "Force user recipes first",
					},
					&cli.BoolFlag{
						Name:    "public",
						Aliases: []string{"P"},
						Usage:   "Force public recipes first",
					},
				},
				Action: func(c *cli.Context) error {
					args := c.Args().Slice()
					sourcePriority := getSourcePriority(c)
					return handleWhichCommand(args, sourcePriority)
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		errorText := strings.ToLower(err.Error())
		formattedErr := fmt.Sprintf(
			"%s: %s",
			FormatText("Error", ColorRed, StyleBold),
			errorText,
		)
		log.Fatal(formattedErr)
	}
}

func getSourcePriority(c *cli.Context) []string {
	useLocal := c.Bool("local") || c.Bool("L")
	useUser := c.Bool("user") || c.Bool("U")
	usePublic := c.Bool("public") || c.Bool("P")

	if useLocal {
		return []string{"local", "user", "public"}
	} else if useUser {
		return []string{"user", "local", "public"}
	} else if usePublic {
		return []string{"public", "local", "user"}
	}

	return []string{"local", "user", "public"}
}

func handleListCommand(c *cli.Context, args []string, sourcePriority []string) error {
	category := c.String("category")
	if category == "" && len(args) >= 1 {
		category = args[0]
	}

	useLocal := c.Bool("local") || c.Bool("L")
	useUser := c.Bool("user") || c.Bool("U")
	usePublic := c.Bool("public") || c.Bool("P")

	if !useLocal && !useUser && !usePublic {
		useLocal = true
		useUser = true
		usePublic = true
	}

	var allRecipes []Recipe
	recipeMap := make(map[string]bool)

	for _, source := range sourcePriority {
		if (source == "local" && !useLocal) ||
			(source == "user" && !useUser) ||
			(source == "public" && !usePublic) {
			continue
		}

		sources, _ := findRecipeSourcesByType(source == "local", source == "user", source == "public")
		recipes, _ := loadRecipes(sources, category)

		for _, r := range recipes {
			if !recipeMap[r.Name] {
				allRecipes = append(allRecipes, r)
				recipeMap[r.Name] = true
			}
		}
	}

	if category == "" {
		var filteredRecipes []Recipe
		for _, recipe := range allRecipes {
			if recipe.Category != "demo" {
				filteredRecipes = append(filteredRecipes, recipe)
			}
		}
		allRecipes = filteredRecipes
	}

	if len(allRecipes) == 0 {
		if c.Bool("json") {
			fmt.Println("[]")
			return nil
		} else {
			fmt.Println("No recipes found.")
			return nil
		}
	}

	if c.Bool("json") {
		return outputRecipesAsJSON(allRecipes)
	}

	listRecipes(allRecipes)
	return nil
}

func outputRecipesAsJSON(recipes []Recipe) error {
	type recipeInfo struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Category    string `json:"category,omitempty"`
		Author      string `json:"author,omitempty"`
	}

	result := make([]recipeInfo, len(recipes))
	for i, r := range recipes {
		result[i] = recipeInfo{
			Name:        r.Name,
			Description: r.Description,
			Category:    r.Category,
			Author:      r.Author,
		}
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(jsonBytes))
	return nil
}

func findRecipeWithOptions(args []string, sourcePriority []string, debug bool) (*Recipe, []string, error) {
	var recipe *Recipe
	var err error

	// 1. try to match a recipe with the given name
	recipe, err = findRecipeInSources(args[0], "", sourcePriority, false)
	if err == nil {
		return recipe, args[1:], nil
	}

	// 2. try to match a recipe with the given category and name
	if len(args) > 1 {
		recipe, err = findRecipeInSources(args[1], args[0], sourcePriority, false)
		if err == nil {
			return recipe, args[2:], nil
		}
	}

	// 3. try to fuzzy match with the given category
	if len(args) > 1 {
		recipe, err = findRecipeInSources(args[1], args[0], sourcePriority, true)
		if err == nil {
			return recipe, args[2:], nil
		}
	}

	// 4. try to match category to prompt for selection
	recipe, err = handleCategorySelection(args[0], sourcePriority, debug)
	if err == nil {
		return recipe, args[1:], nil
	} else if err.Error() == "recipe selection aborted by user" {
		os.Exit(0)
	}

	// 5. try to fuzzy match without category
	recipe, err = findRecipeInSources(args[0], "", sourcePriority, true)
	if err == nil {
		return recipe, args[1:], nil
	}

	return nil, nil, fmt.Errorf("recipe not found: %s", args[0])
}

func handleCategorySelection(categoryName string, sourcePriority []string, debug bool) (*Recipe, error) {
	var allRecipes []Recipe
	recipeMap := make(map[string]Recipe)

	for _, source := range sourcePriority {
		useLocal := source == "local"
		useUser := source == "user"
		usePublic := source == "public"

		sources, _ := findRecipeSourcesByType(useLocal, useUser, usePublic)
		recipes, _ := loadRecipes(sources, categoryName)

		for _, recipe := range recipes {
			if _, exists := recipeMap[recipe.Name]; !exists {
				recipeMap[recipe.Name] = recipe
			}
		}
	}

	if len(recipeMap) == 0 {
		return nil, fmt.Errorf("no recipes found in category: %s", categoryName)
	}

	for _, recipe := range recipeMap {
		allRecipes = append(allRecipes, recipe)
	}
	sort.Slice(allRecipes, func(i, j int) bool {
		return allRecipes[i].Name < allRecipes[j].Name
	})

	options := make([]string, len(allRecipes)+1)
	for i, recipe := range allRecipes {
		options[i] = recipe.Name
	}
	options[len(allRecipes)] = ExitPrompt

	prompt := &survey.Select{
		Message: fmt.Sprintf("Choose a recipe from %s:", categoryName),
		Options: options,
	}

	var selected string
	if err := survey.AskOne(prompt, &selected); err != nil {
		return nil, err
	}

	if selected == ExitPrompt {
		return nil, fmt.Errorf("recipe selection aborted by user")
	}

	selectedRecipe := recipeMap[selected]
	return &selectedRecipe, nil
}

func handleRunCommand(c *cli.Context, args []string, sourcePriority []string) error {
	debug := c.Bool("debug")
	var recipes []Recipe
	var remainingArgs []string

	recipeFilePath := c.String("recipe-file")
	if recipeFilePath != "" {
		config, err := loadConfig(recipeFilePath)
		if err != nil {
			return fmt.Errorf("failed to load recipe file %s: %w", recipeFilePath, err)
		}
		recipes = config.Recipes
		remainingArgs = args
	} else {
		if len(args) == 0 {
			return fmt.Errorf("no recipe specified. Use shef ls to list available recipes")
		}

		var recipe *Recipe
		var err error

		recipe, remainingArgs, err = findRecipeWithOptions(args, sourcePriority, debug)
		if err != nil {
			return err
		}

		for _, arg := range remainingArgs {
			if arg == "-h" || arg == "--help" {
				displayRecipeHelp(recipe)
				return nil
			}
		}

		recipes = []Recipe{*recipe}
	}

	input, vars := processRemainingArgs(remainingArgs)

	if help, ok := vars["help"]; ok && help == true {
		displayRecipeHelp(&recipes[0])
		return nil
	}
	if h, ok := vars["h"]; ok && h == true {
		displayRecipeHelp(&recipes[0])
		return nil
	}

	for _, recipe := range recipes {
		if debug {
			fmt.Printf("Running recipe: %s\n", recipe.Name)
			fmt.Printf("With input: %s\n", input)
			fmt.Printf("With vars: %v\n", vars)
			fmt.Printf("Description: %s\n\n", recipe.Description)
		}

		if err := executeRecipe(recipe, input, vars, debug); err != nil {
			return err
		}
	}

	return nil
}

func processRemainingArgs(args []string) (string, map[string]interface{}) {
	vars := make(map[string]interface{})
	var input string

	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			if strings.HasPrefix(arg, "--") {
				arg = arg[2:] // Remove --
				if strings.Contains(arg, "=") {
					parts := strings.SplitN(arg, "=", 2)
					flagName := strings.ReplaceAll(parts[0], "-", "_")
					vars[flagName] = parts[1]
				} else {
					flagName := strings.ReplaceAll(arg, "-", "_")
					vars[flagName] = true
				}
			} else {
				arg = arg[1:] // Remove -
				if strings.Contains(arg, "=") {
					parts := strings.SplitN(arg, "=", 2)
					vars[parts[0]] = parts[1]
				} else {
					for _, c := range arg {
						vars[string(c)] = true
					}
				}
			}
		} else if input == "" {
			input = arg
		}
	}

	return input, vars
}

func findRecipeInSources(recipeName, category string, sourcePriority []string, fuzzyMatch bool) (*Recipe, error) {
	for _, source := range sourcePriority {
		useLocal := source == "local"
		useUser := source == "user"
		usePublic := source == "public"

		sources, _ := findRecipeSourcesByType(useLocal, useUser, usePublic)
		recipes, _ := loadRecipes(sources, category)

		recipe, err := findRecipeByName(recipes, recipeName)
		if err == nil {
			return recipe, nil
		}

		if category != "" {
			combinedName := fmt.Sprintf("%s-%s", category, recipeName)
			recipe, err = findRecipeByName(recipes, combinedName)
			if err == nil {
				return recipe, nil
			}
		}
	}

	if fuzzyMatch {
		var allRecipes []Recipe
		seenRecipeNames := make(map[string]bool)

		for _, source := range sourcePriority {
			useLocal := source == "local"
			useUser := source == "user"
			usePublic := source == "public"

			sources, _ := findRecipeSourcesByType(useLocal, useUser, usePublic)
			recipes, _ := loadRecipes(sources, "")

			for _, recipe := range recipes {
				if !seenRecipeNames[recipe.Name] {
					allRecipes = append(allRecipes, recipe)
					seenRecipeNames[recipe.Name] = true
				}
			}
		}

		if len(allRecipes) > 0 {
			recipeNames := make([]string, 0, len(allRecipes))
			recipeMap := make(map[string]Recipe)

			for _, recipe := range allRecipes {
				recipeNames = append(recipeNames, recipe.Name)
				recipeMap[recipe.Name] = recipe
			}

			if match, found := fuzzyMatchRecipe(recipeName, recipeNames, recipeMap); found {
				return match, nil
			}
		}
	}

	return nil, fmt.Errorf("recipe not found: %s", recipeName)
}

func fuzzyMatchRecipe(recipeName string, recipeNames []string, recipeMap map[string]Recipe) (*Recipe, bool) {
	if len(recipeNames) == 0 {
		return nil, false
	}

	type nameDistance struct {
		name     string
		distance int
	}
	var matches []nameDistance

	for _, name := range recipeNames {
		distance := levenshtein.ComputeDistance(recipeName, name)
		matches = append(matches, nameDistance{name: name, distance: distance})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].distance < matches[j].distance
	})

	if len(matches) > 0 {
		bestMatch := matches[0]
		recipe := recipeMap[bestMatch.name]

		var confirm bool
		var promptMessage string

		if recipe.Category != "" {
			promptMessage = fmt.Sprintf("Did you mean [%s] '%s'?", recipe.Category, bestMatch.name)
		} else {
			promptMessage = fmt.Sprintf("Did you mean '%s'?", bestMatch.name)
		}

		prompt := &survey.Confirm{
			Message: promptMessage,
			Default: true,
		}

		if err := survey.AskOne(prompt, &confirm); err == nil && confirm {
			return &recipe, true
		}
	}

	return nil, false
}

func handleWhichCommand(args []string, sourcePriority []string) error {
	if len(args) == 0 {
		return fmt.Errorf("you must specify a recipe name")
	}

	var category string
	var recipeName string

	if len(args) >= 2 {
		category = args[0]
		recipeName = args[1]
	} else {
		recipeName = args[0]
	}

	sourcePath, err := findRecipeSourceFile(recipeName, category, sourcePriority)
	if err != nil {
		return err
	}

	fmt.Println(sourcePath)
	return nil
}

func findRecipeSourceFile(recipeName, category string, sourcePriority []string) (string, error) {
	for _, source := range sourcePriority {
		useLocal := source == "local"
		useUser := source == "user"
		usePublic := source == "public"

		sources, _ := findRecipeSourcesByType(useLocal, useUser, usePublic)

		for _, sourceFile := range sources {
			config, err := loadConfig(sourceFile)
			if err != nil {
				continue
			}

			for _, recipe := range config.Recipes {
				if recipe.Name == recipeName {
					return sourceFile, nil
				}

				if category != "" {
					combinedName := fmt.Sprintf("%s-%s", category, recipeName)
					if recipe.Name == combinedName {
						return sourceFile, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("recipe not found: %s", recipeName)
}

func formatDuration(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func formatDurationWithMs(d time.Duration) string {
	baseFormat := formatDuration(d)
	milliseconds := int(d.Milliseconds()) % 1000

	return fmt.Sprintf("%s.%03d", baseFormat, milliseconds)
}

func updateDurationVars(ctx *ExecutionContext, startTime time.Time) {
	elapsed := time.Since(startTime)

	ctx.Vars["duration_ms"] = fmt.Sprintf("%d", elapsed.Milliseconds())
	ctx.Vars["duration_s"] = fmt.Sprintf("%d", int(elapsed.Seconds()))

	ctx.Vars["duration_fmt"] = formatDuration(elapsed)
	ctx.Vars["duration_ms_fmt"] = formatDurationWithMs(elapsed)
}

package internal

import (
	"fmt"
	"time"

	"github.com/schollz/progressbar/v3"
)

type ProgressBarOptions struct {
	Width              int     `yaml:"width,omitempty"`
	Description        string  `yaml:"description,omitempty"`
	Theme              *Theme  `yaml:"theme,omitempty"`
	ShowCount          bool    `yaml:"show_count,omitempty"`
	ShowPercentage     bool    `yaml:"show_percentage,omitempty"`
	ShowElapsedTime    bool    `yaml:"show_elapsed_time,omitempty"`
	ShowIterationSpeed bool    `yaml:"show_iteration_speed,omitempty"`
	RefreshRate        float64 `yaml:"refresh_rate,omitempty"`
	MessageTemplate    string  `yaml:"message_template,omitempty"`
}

type Theme struct {
	Saucer        string `yaml:"saucer,omitempty"`
	SaucerHead    string `yaml:"saucer_head,omitempty"`
	SaucerPadding string `yaml:"saucer_padding,omitempty"`
	BarStart      string `yaml:"bar_start,omitempty"`
	BarEnd        string `yaml:"bar_end,omitempty"`
}

type ProgressBar struct {
	bar *progressbar.ProgressBar
}

func CreateProgressBar(total int, opName string, opts *ProgressBarOptions) *ProgressBar {
	// Set default options if none provided
	if opts == nil {
		opts = &ProgressBarOptions{
			ShowCount:       true,
			ShowPercentage:  true,
			ShowElapsedTime: true,
			Theme: &Theme{
				Saucer:        "[green]=[reset]",
				SaucerHead:    "[green]>[reset]",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			},
		}
	}

	// Prepare progressbar options
	options := []progressbar.Option{
		progressbar.OptionEnableColorCodes(true),
	}

	// Apply custom width if specified
	if opts.Width > 0 {
		options = append(options, progressbar.OptionSetWidth(opts.Width))
	}

	// Apply description
	description := opts.Description
	if description == "" {
		description = opName
	}
	options = append(options, progressbar.OptionSetDescription(description))

	// Apply theme
	if opts.Theme != nil {
		theme := progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}

		if opts.Theme.Saucer != "" {
			theme.Saucer = opts.Theme.Saucer
		}
		if opts.Theme.SaucerHead != "" {
			theme.SaucerHead = opts.Theme.SaucerHead
		}
		if opts.Theme.SaucerPadding != "" {
			theme.SaucerPadding = opts.Theme.SaucerPadding
		}
		if opts.Theme.BarStart != "" {
			theme.BarStart = opts.Theme.BarStart
		}
		if opts.Theme.BarEnd != "" {
			theme.BarEnd = opts.Theme.BarEnd
		}

		options = append(options, progressbar.OptionSetTheme(theme))
	}

	// Apply additional display options
	if opts.ShowCount {
		options = append(options, progressbar.OptionShowCount())
	}
	if opts.ShowPercentage {
		options = append(options, progressbar.OptionShowBytes(false))
	}
	if opts.ShowElapsedTime {
		options = append(options, progressbar.OptionSetElapsedTime(true))
	}
	if opts.ShowIterationSpeed {
		options = append(options, progressbar.OptionShowIts())
	}
	if opts.RefreshRate > 0 {
		options = append(options, progressbar.OptionSetRenderBlankState(true))
		options = append(options, progressbar.OptionThrottle(time.Duration(float64(time.Second)*opts.RefreshRate)))
	}

	// Create the progress bar
	bar := progressbar.NewOptions(total, options...)
	return &ProgressBar{bar: bar}
}

func (p *ProgressBar) Increment() {
	p.bar.Add(1)
}

func (p *ProgressBar) Complete() {
	p.bar.Finish()
	fmt.Println()
}

func (p *ProgressBar) Update(message string) {
	p.bar.Describe(message)
}

// ParseProgressBarOptions converts a map of interface{} to a ProgressBarOptions struct
func ParseProgressBarOptions(optsMap map[string]interface{}) *ProgressBarOptions {
	opts := &ProgressBarOptions{}

	if msgTemplate, ok := optsMap["message_template"].(string); ok {
		opts.MessageTemplate = msgTemplate
	}

	if width, ok := optsMap["width"].(int); ok {
		opts.Width = width
	}

	if desc, ok := optsMap["description"].(string); ok {
		opts.Description = desc
	}

	if showCount, ok := optsMap["show_count"].(bool); ok {
		opts.ShowCount = showCount
	} else {
		opts.ShowCount = true // Default to true
	}

	if showPercentage, ok := optsMap["show_percentage"].(bool); ok {
		opts.ShowPercentage = showPercentage
	} else {
		opts.ShowPercentage = true // Default to true
	}

	if showElapsedTime, ok := optsMap["show_elapsed_time"].(bool); ok {
		opts.ShowElapsedTime = showElapsedTime
	} else {
		opts.ShowElapsedTime = true // Default to true
	}

	if showIterationSpeed, ok := optsMap["show_iteration_speed"].(bool); ok {
		opts.ShowIterationSpeed = showIterationSpeed
	}

	if refreshRate, ok := optsMap["refresh_rate"].(float64); ok {
		opts.RefreshRate = refreshRate
	}

	if themeVal, ok := optsMap["theme"].(map[string]interface{}); ok {
		theme := &Theme{}

		if saucer, ok := themeVal["saucer"].(string); ok {
			theme.Saucer = saucer
		}

		if saucerHead, ok := themeVal["saucer_head"].(string); ok {
			theme.SaucerHead = saucerHead
		}

		if saucerPadding, ok := themeVal["saucer_padding"].(string); ok {
			theme.SaucerPadding = saucerPadding
		}

		if barStart, ok := themeVal["bar_start"].(string); ok {
			theme.BarStart = barStart
		}

		if barEnd, ok := themeVal["bar_end"].(string); ok {
			theme.BarEnd = barEnd
		}

		opts.Theme = theme
	}

	return opts
}

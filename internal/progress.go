package internal

import (
	"fmt"
	"time"

	"github.com/schollz/progressbar/v3"
)

// defaultProgressTheme defines the default visual style for progress bars
var defaultProgressTheme = Theme{
	Saucer:        "[green]=[reset]",
	SaucerHead:    "[green]>[reset]",
	SaucerPadding: " ",
	BarStart:      "[",
	BarEnd:        "]",
}

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

// CreateProgressBar creates a new progress bar with the given total, operation name, and options
func CreateProgressBar(total int, opName string, opts *ProgressBarOptions) *ProgressBar {
	if opts == nil {
		opts = &ProgressBarOptions{
			ShowCount:       true,
			ShowPercentage:  true,
			ShowElapsedTime: true,
			Theme:           &Theme{},
		}
		*opts.Theme = defaultProgressTheme
	}

	options := []progressbar.Option{progressbar.OptionEnableColorCodes(true)}

	if opts.Width > 0 {
		options = append(options, progressbar.OptionSetWidth(opts.Width))
	}

	if opts.Description != "" {
		options = append(options, progressbar.OptionSetDescription(opts.Description))
	} else {
		options = append(options, progressbar.OptionSetDescription(opName))
	}

	if opts.Theme != nil {
		theme := progressbar.Theme{
			Saucer:        defaultProgressTheme.Saucer,
			SaucerHead:    defaultProgressTheme.SaucerHead,
			SaucerPadding: defaultProgressTheme.SaucerPadding,
			BarStart:      defaultProgressTheme.BarStart,
			BarEnd:        defaultProgressTheme.BarEnd,
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

	return &ProgressBar{bar: progressbar.NewOptions(total, options...)}
}

// Increment adds 1 to the progress bar
func (p *ProgressBar) Increment() {
	if err := p.bar.Add(1); err != nil {
		return
	}
}

// Complete marks the progress bar as finished
func (p *ProgressBar) Complete() {
	if err := p.bar.Finish(); err != nil {
		return
	}
	fmt.Println()
}

// Update changes the description of the progress bar
func (p *ProgressBar) Update(message string) {
	p.bar.Describe(message)
}

// ParseProgressBarOptions converts a map of interface{} to a ProgressBarOptions struct
func ParseProgressBarOptions(optsMap map[string]interface{}) *ProgressBarOptions {
	opts := &ProgressBarOptions{
		ShowCount:       true,
		ShowPercentage:  true,
		ShowElapsedTime: true,
		Theme:           &Theme{},
	}
	*opts.Theme = defaultProgressTheme

	if val, ok := optsMap["message_template"].(string); ok {
		opts.MessageTemplate = val
	}
	if val, ok := optsMap["width"].(int); ok {
		opts.Width = val
	}
	if val, ok := optsMap["description"].(string); ok {
		opts.Description = val
	}
	if val, ok := optsMap["show_count"].(bool); ok {
		opts.ShowCount = val
	}
	if val, ok := optsMap["show_percentage"].(bool); ok {
		opts.ShowPercentage = val
	}
	if val, ok := optsMap["show_elapsed_time"].(bool); ok {
		opts.ShowElapsedTime = val
	}
	if val, ok := optsMap["show_iteration_speed"].(bool); ok {
		opts.ShowIterationSpeed = val
	}
	if val, ok := optsMap["refresh_rate"].(float64); ok {
		opts.RefreshRate = val
	}

	if themeMap, ok := optsMap["theme"].(map[string]interface{}); ok {
		if val, ok := themeMap["saucer"].(string); ok {
			opts.Theme.Saucer = val
		}
		if val, ok := themeMap["saucer_head"].(string); ok {
			opts.Theme.SaucerHead = val
		}
		if val, ok := themeMap["saucer_padding"].(string); ok {
			opts.Theme.SaucerPadding = val
		}
		if val, ok := themeMap["bar_start"].(string); ok {
			opts.Theme.BarStart = val
		}
		if val, ok := themeMap["bar_end"].(string); ok {
			opts.Theme.BarEnd = val
		}
	}

	return opts
}

package internal

import (
	"os"
	"strings"
)

// Color represents terminal text colors
type Color string

const (
	ColorNone      Color = ""
	ColorBlack     Color = "black"
	ColorRed       Color = "red"
	ColorGreen     Color = "green"
	ColorYellow    Color = "yellow"
	ColorBlue      Color = "blue"
	ColorMagenta   Color = "magenta"
	ColorCyan      Color = "cyan"
	ColorWhite     Color = "white"
	BgColorBlack   Color = "bg-black"
	BgColorRed     Color = "bg-red"
	BgColorGreen   Color = "bg-green"
	BgColorYellow  Color = "bg-yellow"
	BgColorBlue    Color = "bg-blue"
	BgColorMagenta Color = "bg-magenta"
	BgColorCyan    Color = "bg-cyan"
	BgColorWhite   Color = "bg-white"
)

// colorCodes maps color names to ANSI escape sequences
var colorCodes = map[string]string{
	"black":      "\033[30m",
	"red":        "\033[31m",
	"green":      "\033[32m",
	"yellow":     "\033[33m",
	"blue":       "\033[34m",
	"magenta":    "\033[35m",
	"cyan":       "\033[36m",
	"white":      "\033[37m",
	"bg-black":   "\033[40m",
	"bg-red":     "\033[41m",
	"bg-green":   "\033[42m",
	"bg-yellow":  "\033[43m",
	"bg-blue":    "\033[44m",
	"bg-magenta": "\033[45m",
	"bg-cyan":    "\033[46m",
	"bg-white":   "\033[47m",
	"reset":      "\033[0m",
}

// Style represents terminal text styling options
type Style string

const (
	StyleNone      Style = ""
	StyleBold      Style = "bold"
	StyleDim       Style = "dim"
	StyleItalic    Style = "italic"
	StyleUnderline Style = "underline"
)

// styleCodes maps style names to ANSI escape sequences
var styleCodes = map[string]string{
	"bold":      "\033[1m",
	"dim":       "\033[2m",
	"italic":    "\033[3m",
	"underline": "\033[4m",
	"reset":     "\033[0m",
}

// FormatText applies color and style formatting to text for terminal output
func FormatText(text string, color Color, style Style) string {
	if os.Getenv("NO_COLOR") != "" {
		return text
	}

	if color == ColorNone && style == StyleNone {
		return text
	}

	if color != ColorNone && style != StyleNone {
		return applyColorAndStyle(text, color, style)
	}

	if color != ColorNone {
		return applyColor(text, color)
	}

	return applyStyle(text, style)
}

// applyColor adds color formatting to text
func applyColor(text string, color Color) string {
	if code, ok := colorCodes[string(color)]; ok {
		return code + text + colorCodes["reset"]
	}
	return text
}

// applyStyle adds style formatting to text
func applyStyle(text string, style Style) string {
	if code, ok := styleCodes[string(style)]; ok {
		return code + text + styleCodes["reset"]
	}
	return text
}

// applyColorAndStyle adds both color and style formatting to text
func applyColorAndStyle(text string, color Color, style Style) string {
	result := applyColor(text, color)

	pos := strings.Index(result, text)
	if pos == -1 {
		return result
	}

	styleCode, ok := styleCodes[string(style)]
	if !ok {
		return result
	}

	result = result[:pos] + styleCode + result[pos:]

	resetPos := strings.LastIndex(result, colorCodes["reset"])
	if resetPos != -1 {
		result = result[:resetPos] + styleCodes["reset"] + result[resetPos:]
	}

	return result
}

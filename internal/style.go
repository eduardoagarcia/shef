package internal

import (
	"os"
	"strings"
)

type Color string

type Style string

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

const (
	StyleNone      Style = ""
	StyleBold      Style = "bold"
	StyleDim       Style = "dim"
	StyleItalic    Style = "italic"
	StyleUnderline Style = "underline"
)

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

var styleCodes = map[string]string{
	"bold":      "\033[1m",
	"dim":       "\033[2m",
	"italic":    "\033[3m",
	"underline": "\033[4m",
	"reset":     "\033[0m",
}

func FormatText(text string, color Color, style Style) string {
	if os.Getenv("NO_COLOR") != "" {
		return text
	}

	var result string = text

	if color != ColorNone {
		if code, ok := colorCodes[string(color)]; ok {
			result = code + result + colorCodes["reset"]
		}
	}

	if style != StyleNone {
		if color != ColorNone {
			pos := strings.Index(result, text)
			if pos != -1 {
				if code, ok := styleCodes[string(style)]; ok {
					result = result[:pos] + code + result[pos:]
					resetPos := strings.LastIndex(result, colorCodes["reset"])
					if resetPos != -1 {
						result = result[:resetPos] + styleCodes["reset"] + result[resetPos:]
					}
				}
			}
		} else {
			if code, ok := styleCodes[string(style)]; ok {
				result = code + result + styleCodes["reset"]
			}
		}
	}

	return result
}

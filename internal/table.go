package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// renderTableFromJSON renders a table from JSON data
func renderTableFromJSON(jsonData string) string {
	var tableData map[string]interface{}
	err := json.Unmarshal([]byte(jsonData), &tableData)
	if err != nil {
		return fmt.Sprintf("Error parsing table data: %v", err)
	}

	t := table.NewWriter()
	t.SetOutputMirror(io.Discard)

	style := table.StyleRounded
	if val, ok := tableData["style"].(string); ok {
		switch strings.ToLower(val) {
		case "light":
			style = table.StyleLight
		case "double":
			style = table.StyleDouble
		case "bold":
			style = table.StyleBold
		case "ascii":
			style = table.StyleDefault
		}
	}

	t.SetStyle(style)

	if headers, ok := tableData["headers"].([]interface{}); ok {
		headerRow := table.Row{}
		for _, h := range headers {
			headerRow = append(headerRow, h)
		}
		t.AppendHeader(headerRow)
	}

	if rows, ok := tableData["rows"].([]interface{}); ok {
		for _, row := range rows {
			if rowData, ok := row.([]interface{}); ok {
				tableRow := table.Row{}
				for _, cell := range rowData {
					tableRow = append(tableRow, cell)
				}
				t.AppendRow(tableRow)
			}
		}
	}

	if footers, ok := tableData["footers"].([]interface{}); ok {
		footerRow := table.Row{}
		for _, f := range footers {
			footerRow = append(footerRow, f)
		}
		t.AppendFooter(footerRow)
	}

	if alignments, ok := tableData["align"].([]interface{}); ok {
		columnConfigs := make([]table.ColumnConfig, len(alignments))
		for i, align := range alignments {
			alignStr, ok := align.(string)
			if !ok {
				continue
			}

			columnConfig := table.ColumnConfig{
				Number: i + 1,
			}

			switch strings.ToLower(alignStr) {
			case "left":
				columnConfig.Align = text.AlignLeft
				columnConfig.AlignHeader = text.AlignLeft
				columnConfig.AlignFooter = text.AlignLeft
			case "center":
				columnConfig.Align = text.AlignCenter
				columnConfig.AlignHeader = text.AlignCenter
				columnConfig.AlignFooter = text.AlignCenter
			case "right":
				columnConfig.Align = text.AlignRight
				columnConfig.AlignHeader = text.AlignRight
				columnConfig.AlignFooter = text.AlignRight
			}

			columnConfigs[i] = columnConfig
		}

		if len(columnConfigs) > 0 {
			t.SetColumnConfigs(columnConfigs)
		}
	}

	return t.Render()
}

// renderSimpleTable is a helper for quick tables with minimal configuration
func renderSimpleTable(headers []string, rows [][]string, style string) string {
	t := table.NewWriter()
	t.SetOutputMirror(io.Discard)

	tableStyle := table.StyleRounded
	switch strings.ToLower(style) {
	case "light":
		tableStyle = table.StyleLight
	case "double":
		tableStyle = table.StyleDouble
	case "bold":
		tableStyle = table.StyleBold
	case "ascii":
		tableStyle = table.StyleDefault
	}

	t.SetStyle(tableStyle)

	if len(headers) > 0 {
		headerRow := table.Row{}
		for _, h := range headers {
			headerRow = append(headerRow, h)
		}
		t.AppendHeader(headerRow)
	}

	for _, r := range rows {
		tableRow := table.Row{}
		for _, cell := range r {
			tableRow = append(tableRow, cell)
		}
		t.AppendRow(tableRow)
	}

	return t.Render()
}

// TableFuncMap returns template functions for table rendering
func TableFuncMap() map[string]interface{} {
	funcs := make(map[string]interface{})

	funcs["tableJSON"] = func(jsonData string) string {
		return renderTableFromJSON(jsonData)
	}

	funcs["table"] = func(headers interface{}, rows interface{}, style interface{}, args ...interface{}) string {
		var headerStrs []string
		if headersArr, ok := headers.([]interface{}); ok {
			headerStrs = make([]string, len(headersArr))
			for i, h := range headersArr {
				headerStrs[i] = fmt.Sprintf("%v", h)
			}
		} else if headersArr, ok := headers.([]string); ok {
			headerStrs = headersArr
		}

		var rowStrs [][]string
		if rowsArr, ok := rows.([]interface{}); ok {
			rowStrs = make([][]string, len(rowsArr))
			for i, row := range rowsArr {
				if rowArr, ok := row.([]interface{}); ok {
					rowStrs[i] = make([]string, len(rowArr))
					for j, cell := range rowArr {
						rowStrs[i][j] = fmt.Sprintf("%v", cell)
					}
				}
			}
		}

		styleStr := "rounded"
		if s, ok := style.(string); ok {
			styleStr = s
		}

		if len(args) == 0 {
			return renderSimpleTable(headerStrs, rowStrs, styleStr)
		}

		t := table.NewWriter()
		t.SetOutputMirror(io.Discard)

		tableStyle := table.StyleRounded
		switch strings.ToLower(styleStr) {
		case "light":
			tableStyle = table.StyleLight
		case "double":
			tableStyle = table.StyleDouble
		case "bold":
			tableStyle = table.StyleBold
		case "ascii":
			tableStyle = table.StyleDefault
		}
		t.SetStyle(tableStyle)

		if len(headerStrs) > 0 {
			headerRow := table.Row{}
			for _, h := range headerStrs {
				headerRow = append(headerRow, h)
			}
			t.AppendHeader(headerRow)
		}

		for _, r := range rowStrs {
			tableRow := table.Row{}
			for _, cell := range r {
				tableRow = append(tableRow, cell)
			}
			t.AppendRow(tableRow)
		}

		if len(args) > 0 {
			if alignments, ok := args[0].([]interface{}); ok {
				columnConfigs := make([]table.ColumnConfig, len(alignments))
				for i, align := range alignments {
					alignStr, ok := align.(string)
					if !ok {
						continue
					}

					columnConfig := table.ColumnConfig{
						Number: i + 1,
					}

					switch strings.ToLower(alignStr) {
					case "left":
						columnConfig.Align = text.AlignLeft
						columnConfig.AlignHeader = text.AlignLeft
						columnConfig.AlignFooter = text.AlignLeft
					case "center":
						columnConfig.Align = text.AlignCenter
						columnConfig.AlignHeader = text.AlignCenter
						columnConfig.AlignFooter = text.AlignCenter
					case "right":
						columnConfig.Align = text.AlignRight
						columnConfig.AlignHeader = text.AlignRight
						columnConfig.AlignFooter = text.AlignRight
					}

					columnConfigs[i] = columnConfig
				}

				if len(columnConfigs) > 0 {
					t.SetColumnConfigs(columnConfigs)
				}
			}
		}

		return t.Render()
	}

	funcs["makeRow"] = func(cells ...interface{}) []interface{} {
		return cells
	}

	funcs["makeHeaders"] = func(headers ...interface{}) []interface{} {
		return headers
	}

	funcs["tableStyleRounded"] = func() string {
		return "rounded"
	}

	funcs["tableStyleLight"] = func() string {
		return "light"
	}

	funcs["tableStyleDouble"] = func() string {
		return "double"
	}

	funcs["tableStyleASCII"] = func() string {
		return "ascii"
	}

	return funcs
}

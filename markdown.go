/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 22:18:31
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 22:18:31
* @FilePath: \go-bot\markdown.go
* @Description: markdown 表格的公共识别与等宽对齐渲染，供各平台适配器复用
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// IsMarkdownTableLine 判断一行是否是 markdown 表格行
func IsMarkdownTableLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "|")
}

// IsMarkdownTableSeparator 判断一行是否是表格的分隔行（| --- | :---: 之类）
func IsMarkdownTableSeparator(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.Contains(trimmed, "-") {
		return false
	}
	for _, r := range trimmed {
		if r != '|' && r != '-' && r != ':' && r != ' ' {
			return false
		}
	}
	return true
}

// RenderMarkdownTableText 把表格块渲染为按显示宽度对齐的纯文本行：
// 去竖线后按列显示宽度对齐（中文等全角字符算 2 列，由 go-runewidth 给出，
// 等宽字体下即可对齐），分隔行仅用于解析对齐方式后丢弃；
// 列间以两个空格分隔，行尾空格修剪；仅分隔行构成时返回 nil。
// 输出为纯文本，HTML 转义等平台处理由调用方自行叠加
func RenderMarkdownTableText(lines []string) []string {
	var rows [][]string
	var align []string
	for _, line := range lines {
		if IsMarkdownTableSeparator(line) {
			if align == nil {
				align = parseTableAlign(line)
			}
			continue
		}
		rows = append(rows, splitTableRow(line))
	}
	if len(rows) == 0 {
		return nil
	}
	cols := 0
	for _, row := range rows {
		if len(row) > cols {
			cols = len(row)
		}
	}
	widths := make([]int, cols)
	for _, row := range rows {
		for i, cell := range row {
			if w := runewidth.StringWidth(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}
	columnAlign := make([]string, cols)
	for i := range columnAlign {
		columnAlign[i] = "left"
	}
	for i := range align {
		if i < cols {
			columnAlign[i] = align[i]
		}
	}
	rendered := make([]string, 0, len(rows))
	for _, row := range rows {
		var line strings.Builder
		for i := 0; i < cols; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			line.WriteString(alignCell(cell, widths[i], columnAlign[i]))
			if i < cols-1 {
				line.WriteString("  ")
			}
		}
		rendered = append(rendered, strings.TrimRight(line.String(), " "))
	}
	return rendered
}

// alignCell 按列宽与对齐方式在单元格两侧补空格
func alignCell(cell string, width int, align string) string {
	gap := width - runewidth.StringWidth(cell)
	if gap <= 0 {
		return cell
	}
	pad := strings.Repeat(" ", gap)
	switch align {
	case "right":
		return pad + cell
	case "center":
		left := gap / 2
		return strings.Repeat(" ", left) + cell + strings.Repeat(" ", gap-left)
	default:
		return cell + pad
	}
}

// splitTableRow 把一行表格拆为单元格：去首尾竖线后按竖线切分并修剪空白
func splitTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	cells := strings.Split(trimmed, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

// parseTableAlign 解析分隔行的对齐方式：:--- 左对齐、---: 右对齐、:---: 居中
func parseTableAlign(line string) []string {
	cells := splitTableRow(line)
	aligns := make([]string, len(cells))
	for i, cell := range cells {
		left := strings.HasPrefix(cell, ":")
		right := strings.HasSuffix(cell, ":")
		switch {
		case left && right:
			aligns[i] = "center"
		case right:
			aligns[i] = "right"
		default:
			aligns[i] = "left"
		}
	}
	return aligns
}

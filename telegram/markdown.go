/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 23:36:52
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 23:36:52
* @FilePath: \go-bot\telegram\markdown.go
* @Description: 通用 markdown 到 Telegram HTML 子集的转换
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package telegram

import (
	"html"
	"regexp"
	"strings"

	"github.com/mattn/go-runewidth"
)

// 通用 markdown 行内与标题语法的转换规则
var (
	reMDHeading = regexp.MustCompile(`^#{1,6}\s+(.+)$`)
	reMDLink    = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	reMDCode    = regexp.MustCompile("`([^`]+)`")
	reMDBold    = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reMDItalic  = regexp.MustCompile(`\*(\S(?:[^*]*\S)?)\*`)
	reMDStrike  = regexp.MustCompile(`~~(.+?)~~`)
)

// mdTable 是表格块的解析中间态：逐行收集单元格，
// 块结束后统一按列显示宽度对齐渲染
type mdTable struct {
	rows  [][]string
	align []string
}

// markdownToTelegramHTML 把通用 markdown 转换为 Telegram 支持的 HTML 子集：
// 围栏代码块按 <pre>（可带语言标注）、表格去竖线后按显示宽度对齐进 <pre> 等宽呈现，
// 行内支持标题、加粗、斜体、删除线、代码与链接，其余文本转义后原样保留
func markdownToTelegramHTML(md string) string {
	lines := strings.Split(md, "\n")
	var b strings.Builder
	var inCode, inTable bool
	var codeClose string
	var table mdTable
	closeTable := func() {
		if inTable {
			renderTable(&b, table)
			inTable = false
		}
	}
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "```"):
			closeTable()
			if inCode {
				inCode = false
				b.WriteString(codeClose)
				b.WriteString("\n")
				codeClose = ""
			} else {
				inCode = true
				if lang := strings.TrimSpace(strings.TrimPrefix(line, "```")); lang != "" {
					codeClose = "</code></pre>"
					b.WriteString(`<pre><code class="language-`)
					b.WriteString(html.EscapeString(lang))
					b.WriteString(`">`)
					b.WriteString("\n")
				} else {
					codeClose = "</pre>"
					b.WriteString("<pre>\n")
				}
			}
		case inCode:
			b.WriteString(html.EscapeString(line))
			b.WriteString("\n")
		case isTableSeparator(line):
			if inTable && table.align == nil {
				table.align = parseTableAlign(line)
			}
		case isTableLine(line):
			if !inTable {
				inTable = true
				table = mdTable{}
			}
			table.rows = append(table.rows, splitTableRow(line))
		default:
			closeTable()
			b.WriteString(convertMDInline(line))
			b.WriteString("\n")
		}
	}
	closeTable()
	if inCode {
		b.WriteString(codeClose)
		b.WriteString("\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// convertMDInline 转义一行文本并应用 markdown 行内语法
func convertMDInline(line string) string {
	line = html.EscapeString(line)
	if m := reMDHeading.FindStringSubmatch(line); m != nil {
		line = "<b>" + m[1] + "</b>"
	}
	line = reMDLink.ReplaceAllString(line, `<a href="$2">$1</a>`)
	line = reMDCode.ReplaceAllString(line, "<code>$1</code>")
	line = reMDBold.ReplaceAllString(line, "<b>$1</b>")
	line = reMDItalic.ReplaceAllString(line, "<i>$1</i>")
	line = reMDStrike.ReplaceAllString(line, "<s>$1</s>")
	return line
}

// renderTable 把表格块渲染为 pre 等宽块：去竖线、按列显示宽度对齐，
// 单元格内容转义；列宽按显示宽度计算（中文等全角字符算 2 列，
// 由 go-runewidth 给出，等宽字体下即可对齐），列间以两个空格分隔
func renderTable(b *strings.Builder, t mdTable) {
	cols := 0
	for _, row := range t.rows {
		if len(row) > cols {
			cols = len(row)
		}
	}
	widths := make([]int, cols)
	for _, row := range t.rows {
		for i, cell := range row {
			if w := runewidth.StringWidth(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}
	align := make([]string, cols)
	for i := range align {
		align[i] = "left"
	}
	for i := range t.align {
		if i < cols {
			align[i] = t.align[i]
		}
	}
	b.WriteString("<pre>\n")
	for _, row := range t.rows {
		var line strings.Builder
		for i := 0; i < cols; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			line.WriteString(html.EscapeString(alignCell(cell, widths[i], align[i])))
			if i < cols-1 {
				line.WriteString("  ")
			}
		}
		b.WriteString(strings.TrimRight(line.String(), " "))
		b.WriteString("\n")
	}
	b.WriteString("</pre>\n")
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

// isTableLine 判断一行是否是 markdown 表格行
func isTableLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "|")
}

// isTableSeparator 判断一行是否是表格的分隔行（| --- | :---: 之类）
func isTableSeparator(line string) bool {
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

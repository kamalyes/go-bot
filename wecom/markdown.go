/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-19 13:52:07
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-19 15:02:37
* @FilePath: \go-bot\wecom\markdown.go
* @Description: 通用 markdown 到企微语法子集的降级转换
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package wecom

import (
	"strings"

	gobot "github.com/kamalyes/go-bot"
)

// markdownToWecomMarkdown 把通用 markdown 降级为企微支持的语法子集：
// 标题/加粗/斜体/链接/行内代码/引用/字体颜色为原生语法直接透传；
// 企微无表格与围栏代码块语法，表格转按显示宽度对齐的纯文本行，
// 围栏代码块去掉围栏行保留内容行；
// 表格对齐由根包公共渲染，保证与 Telegram/Lark/钉钉端呈现一致
func markdownToWecomMarkdown(md string) string {
	lines := strings.Split(md, "\n")
	var b strings.Builder
	var inCode, inTable bool
	var codeLines, tableLines []string
	closeTable := func() {
		if inTable {
			renderTableRows(&b, tableLines)
			inTable = false
			tableLines = nil
		}
	}
	closeCode := func() {
		if !inCode {
			return
		}
		for _, line := range codeLines {
			b.WriteString(line)
			b.WriteString("\n")
		}
		inCode = false
		codeLines = nil
	}
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "```"):
			// 围栏行本身丢弃（企微不渲染代码块），内容行随块统一输出
			closeTable()
			if inCode {
				closeCode()
			} else {
				inCode = true
			}
		case inCode:
			codeLines = append(codeLines, line)
		case gobot.IsMarkdownTableSeparator(line):
			// 表格内的分隔行随块交给根包解析对齐；表格外的分隔行直接丢弃
			if inTable {
				tableLines = append(tableLines, line)
			}
		case gobot.IsMarkdownTableLine(line):
			if !inTable {
				inTable = true
				tableLines = nil
			}
			tableLines = append(tableLines, line)
		default:
			closeTable()
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	closeTable()
	closeCode()
	return strings.TrimSuffix(b.String(), "\n")
}

// renderTableRows 把表格块渲染为按显示宽度对齐的纯文本行：
// 企微无表格与代码块语法，对齐行直接以正文形式呈现表格效果
func renderTableRows(b *strings.Builder, tableLines []string) {
	rows := gobot.RenderMarkdownTableText(tableLines)
	if len(rows) == 0 {
		return
	}
	for _, row := range rows {
		b.WriteString(row)
		b.WriteString("\n")
	}
}

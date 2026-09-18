/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 12:56:06
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 18:00:36
* @FilePath: \go-bot\dingtalk\markdown.go
* @Description: 通用 markdown 到钉钉语法子集的降级转换
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package dingtalk

import (
	"strings"

	gobot "github.com/kamalyes/go-bot"
)

// markdownToDingtalkMarkdown 把通用 markdown 降级为钉钉支持的语法子集：
// 标题/加粗/斜体/链接/列表/引用/代码块为钉钉原生语法直接透传；
// 表格钉钉不支持 markdown 渲染，转按显示宽度对齐的代码块呈现，
// 对齐由根包公共渲染，保证与 Telegram/Lark 端呈现一致
func markdownToDingtalkMarkdown(md string) string {
	lines := strings.Split(md, "\n")
	var b strings.Builder
	var inCode, inTable bool
	var tableLines []string
	closeTable := func() {
		if inTable {
			renderTableBlock(&b, tableLines)
			inTable = false
			tableLines = nil
		}
	}
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "```"):
			closeTable()
			b.WriteString(line)
			b.WriteString("\n")
			inCode = !inCode
		case inCode:
			b.WriteString(line)
			b.WriteString("\n")
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
	return strings.TrimSuffix(b.String(), "\n")
}

// renderTableBlock 把表格块渲染为代码块围栏：钉钉不支持表格语法，
// 按显示宽度对齐的纯文本行在等宽代码块中呈现表格效果
func renderTableBlock(b *strings.Builder, tableLines []string) {
	rows := gobot.RenderMarkdownTableText(tableLines)
	if len(rows) == 0 {
		return
	}
	b.WriteString("```\n")
	for _, row := range rows {
		b.WriteString(row)
		b.WriteString("\n")
	}
	b.WriteString("```\n")
}

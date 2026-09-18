/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 22:18:31
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 22:18:31
* @FilePath: \go-bot\lark\markdown.go
* @Description: 通用 markdown 到 Lark 卡片富文本子集的降级转换
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lark

import (
	"regexp"
	"strings"

	gobot "github.com/kamalyes/go-bot"
)

// Lark 卡片富文本不支持的行内语法的降级规则
var (
	reLarkHeading = regexp.MustCompile(`^#{1,6}\s+(.+)$`)
	reLarkCode    = regexp.MustCompile("`([^`]+)`")
)

// markdownToCardMarkdown 把通用 markdown 降级为 Lark 卡片富文本支持的子集：
// 标题转加粗、行内代码去反引号、表格转按显示宽度对齐的代码块，
// 加粗/斜体/删除线/链接与围栏代码块为卡片原生语法直接透传；
// 表格对齐由根包公共渲染，保证与 Telegram 端呈现一致
func markdownToCardMarkdown(md string) string {
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
			b.WriteString(convertLarkInline(line))
			b.WriteString("\n")
		}
	}
	closeTable()
	return strings.TrimSuffix(b.String(), "\n")
}

// convertLarkInline 应用卡片不支持的行内语法的降级：标题转加粗、行内代码去反引号
func convertLarkInline(line string) string {
	if m := reLarkHeading.FindStringSubmatch(line); m != nil {
		line = "**" + m[1] + "**"
	}
	return reLarkCode.ReplaceAllString(line, "$1")
}

// renderTableBlock 把表格块渲染为代码块围栏：Lark 卡片无表格语法，
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

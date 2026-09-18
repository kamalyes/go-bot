/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 11:06:52
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

	gobot "github.com/kamalyes/go-bot"
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

// markdownToTelegramHTML 把通用 markdown 转换为 Telegram 支持的 HTML 子集：
// 围栏代码块按 <pre>（可带语言标注）、表格去竖线后按显示宽度对齐进 <pre> 等宽呈现，
// 行内支持标题、加粗、斜体、删除线、代码与链接，其余文本转义后原样保留
func markdownToTelegramHTML(md string) string {
	lines := strings.Split(md, "\n")
	var b strings.Builder
	var inCode, inTable bool
	var codeClose string
	var tableLines []string
	closeTable := func() {
		if inTable {
			renderTable(&b, tableLines)
			inTable = false
			tableLines = nil
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

// renderTable 把表格块渲染为 pre 等宽块：列宽与对齐由根包按显示宽度统一计算，
// 渲染后的整行做 HTML 转义（转义不影响空格对齐，实体在消息中仍显示为原字符）
func renderTable(b *strings.Builder, tableLines []string) {
	b.WriteString("<pre>\n")
	for _, line := range gobot.RenderMarkdownTableText(tableLines) {
		b.WriteString(html.EscapeString(line))
		b.WriteString("\n")
	}
	b.WriteString("</pre>\n")
}

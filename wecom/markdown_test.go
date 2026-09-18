/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-19 16:52:37
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-19 17:02:19
* @FilePath: \go-bot\wecom\markdown_test.go
* @Description: wecom markdown 降级转换测试：表格对齐行与代码块围栏
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package wecom

import (
	"strings"
	"testing"

	gobot "github.com/kamalyes/go-bot"
)

// TestMarkdownTableToAlignedRows 验证表格降级为根包对齐行（无围栏、无竖线）
func TestMarkdownTableToAlignedRows(t *testing.T) {
	tableLines := []string{
		"| 服务 | 实例数 | Q99 延迟 |",
		"| --- | ---: | ---: |",
		"| user-api | 12 | 87ms |",
		"| legacy-gateway | 1 | 512ms |",
	}
	got := markdownToWecomMarkdown(strings.Join(tableLines, "\n"))
	want := strings.Join(gobot.RenderMarkdownTableText(tableLines), "\n")
	if got != want {
		t.Fatalf("表格降级结果 = %q, 期望根包对齐行 %q", got, want)
	}
	if strings.ContainsAny(got, "|") {
		t.Fatalf("降级结果 = %q, 不应残留表格竖线", got)
	}
}

// TestMarkdownCodeFenceDropped 验证围栏行丢弃、代码内容保留
func TestMarkdownCodeFenceDropped(t *testing.T) {
	got := markdownToWecomMarkdown("```go\nretry: 3, backoff: 2s\ndropRate: 0.02%\n```")
	want := "retry: 3, backoff: 2s\ndropRate: 0.02%"
	if got != want {
		t.Fatalf("代码块降级结果 = %q, 期望 %q", got, want)
	}
}

// TestMarkdownHeadingPassthrough 验证标题/引用/行内代码/链接为原生语法直接透传
func TestMarkdownHeadingPassthrough(t *testing.T) {
	src := "### 服务概览\n> 引用文字\n`工具`: go-bot ｜ 详见 [值班手册](https://example.com/runbook)"
	if got := markdownToWecomMarkdown(src); got != src {
		t.Fatalf("原生语法应原样透传: got %q", got)
	}
}

// TestMarkdownTableInsideCodeIgnored 验证代码块内的表格行不参与降级
func TestMarkdownTableInsideCodeIgnored(t *testing.T) {
	got := markdownToWecomMarkdown("```\n| a | b |\n| --- | --- |\n| 1 | 2 |\n```")
	if got != "| a | b |\n| --- | --- |\n| 1 | 2 |" {
		t.Fatalf("代码块内表格应原样保留: got %q", got)
	}
}

// TestMarkdownMixedContent 验证生产级巡检报告的完整降级：
// 表格转对齐行、代码块去围栏，其余语法保留
func TestMarkdownMixedContent(t *testing.T) {
	src := strings.Join([]string{
		"**巡检范围**: cn-east-1 核心服务",
		"",
		"### 服务概览",
		"",
		"| 服务 | QPS | 状态 |",
		"| --- | ---: | :---: |",
		"| user-api | 12,400 | ✅ 健康 |",
		"",
		"```go",
		"retry: 3, backoff: 2s",
		"```",
		"",
		"规则详见 [值班手册](https://example.com/runbook?d=oncall&v=3)。",
	}, "\n")
	got := markdownToWecomMarkdown(src)

	if strings.Contains(got, "|") || strings.Contains(got, "---:") {
		t.Fatalf("降级结果 = %q, 表格应转对齐行", got)
	}
	if strings.Contains(got, "```") {
		t.Fatalf("降级结果 = %q, 围栏行应丢弃", got)
	}
	for _, want := range []string{"**巡检范围**", "### 服务概览", "user-api", "12,400", "retry: 3, backoff: 2s", "值班手册"} {
		if !strings.Contains(got, want) {
			t.Fatalf("降级结果 = %q, 应保留 %q", got, want)
		}
	}
}

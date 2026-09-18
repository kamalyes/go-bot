/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 21:18:31
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 22:18:31
* @FilePath: \go-bot\lark\markdown_test.go
* @Description: markdown 到 Lark 卡片富文本子集降级转换测试
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lark

import (
	"testing"

	gobot "github.com/kamalyes/go-bot"
)

// TestMarkdownToCardMarkdownHeadings 验证各级标题降级为加粗行
func TestMarkdownToCardMarkdownHeadings(t *testing.T) {
	md := "# 生产环境巡检报告\n## 服务概览\n###### 附录说明"
	want := "**生产环境巡检报告**\n**服务概览**\n**附录说明**"
	if got := markdownToCardMarkdown(md); got != want {
		t.Fatalf("markdownToCardMarkdown() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToCardMarkdownInlineCode 验证行内代码去反引号
func TestMarkdownToCardMarkdownInlineCode(t *testing.T) {
	md := "`工具`: go-bot v2.6.0 ｜ `巡检人`: kamalyes"
	want := "工具: go-bot v2.6.0 ｜ 巡检人: kamalyes"
	if got := markdownToCardMarkdown(md); got != want {
		t.Fatalf("markdownToCardMarkdown() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToCardMarkdownNativePassthrough 验证卡片原生语法原样保留
func TestMarkdownToCardMarkdownNativePassthrough(t *testing.T) {
	md := "**加粗** *斜体* ~~删除线~~ 规则详见 [值班手册](https://example.com/runbook?d=oncall&v=3)。"
	if got := markdownToCardMarkdown(md); got != md {
		t.Fatalf("markdownToCardMarkdown() = %q, 期望原样返回", got)
	}
}

// TestMarkdownToCardMarkdownCodeFencePassthrough 验证围栏代码块整块透传，
// 围栏内的反引号与 markdown 语法不做处理
func TestMarkdownToCardMarkdownCodeFencePassthrough(t *testing.T) {
	md := "```go\nretry: 3, backoff: 2s\ndropRate: 0.02% (含 `反引号` 与 *星号*)\n```"
	if got := markdownToCardMarkdown(md); got != md {
		t.Fatalf("markdownToCardMarkdown() = %q, 期望原样返回", got)
	}
}

// TestMarkdownToCardMarkdownUnclosedFence 验证文件结束仍未闭合的围栏原样保留
func TestMarkdownToCardMarkdownUnclosedFence(t *testing.T) {
	md := "```go\nunterminated"
	if got := markdownToCardMarkdown(md); got != md {
		t.Fatalf("markdownToCardMarkdown() = %q, 期望原样返回", got)
	}
}

// TestMarkdownToCardMarkdownTableToCodeBlock 验证表格降级为对齐代码块
func TestMarkdownToCardMarkdownTableToCodeBlock(t *testing.T) {
	md := "| 服务 | 状态 |\n| --- | --- |\n| user-api | 运行中 |\n| order-svc | 已重启 |"
	want := "```\n服务       状态\nuser-api   运行中\norder-svc  已重启\n```"
	if got := markdownToCardMarkdown(md); got != want {
		t.Fatalf("markdownToCardMarkdown() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToCardMarkdownTableAlignment 验证表格列对齐方式随降级保留
func TestMarkdownToCardMarkdownTableAlignment(t *testing.T) {
	md := "| 指标 | 当前值 | 阈值 | 趋势 |\n| --- | ---: | ---: | :---: |\n| CPU | 92% | 80% | 上升 |\n| 内存 | 78% | 85% | 持平 |"
	want := "```\n指标  当前值  阈值  趋势\nCPU      92%   80%  上升\n内存     78%   85%  持平\n```"
	if got := markdownToCardMarkdown(md); got != want {
		t.Fatalf("markdownToCardMarkdown() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToCardMarkdownTableAfterFence 验证代码块闭合后紧跟表格的正确衔接
func TestMarkdownToCardMarkdownTableAfterFence(t *testing.T) {
	md := "```\ncode\n```\n| a | b |\n| --- | --- |\n| 1 | 2 |"
	want := "```\ncode\n```\n```\na  b\n1  2\n```"
	if got := markdownToCardMarkdown(md); got != want {
		t.Fatalf("markdownToCardMarkdown() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToCardMarkdownComplexDocument 验证生产级巡检报告：标题降级、
// 行内代码去反引号、对齐表格代码块、有序列表透传、围栏代码块与行内语法的完整组合
func TestMarkdownToCardMarkdownComplexDocument(t *testing.T) {
	md := "### 服务概览\n\n" +
		"**巡检范围**: cn-east-1 核心服务，采样窗口 15 分钟\n" +
		"`工具`: go-bot v2.6.0 ｜ `巡检人`: kamalyes\n\n" +
		"| 服务 | 实例数 | QPS | P99 延迟 | 状态 |\n" +
		"| --- | ---: | ---: | ---: | :---: |\n" +
		"| user-api | 12 | 12,400 | 87ms | 健康 |\n" +
		"| order-svc | 3 | 3,200 | 231ms | 健康 |\n" +
		"| legacy-gateway | 1 | 800 | 512ms | 告警 |\n\n" +
		"### 关键事件\n\n" +
		"1. order-svc 超时熔断触发，dropRate 0.02%，15 分钟后自愈\n\n" +
		"```go\nretry: 3, backoff: 2s, budget: 30s\ndropRate: 0.02% (order-svc 超时熔断)\n```\n\n" +
		"~~旧告警通道~~ 已于本迭代下线，*新增* 双通道，规则详见 [值班手册](https://example.com/runbook?d=oncall&v=3)。"
	want := "**服务概览**\n\n" +
		"**巡检范围**: cn-east-1 核心服务，采样窗口 15 分钟\n" +
		"工具: go-bot v2.6.0 ｜ 巡检人: kamalyes\n\n" +
		"```\n" +
		"服务            实例数     QPS  P99 延迟  状态\n" +
		"user-api            12  12,400      87ms  健康\n" +
		"order-svc            3   3,200     231ms  健康\n" +
		"legacy-gateway       1     800     512ms  告警\n" +
		"```\n\n" +
		"**关键事件**\n\n" +
		"1. order-svc 超时熔断触发，dropRate 0.02%，15 分钟后自愈\n\n" +
		"```go\nretry: 3, backoff: 2s, budget: 30s\ndropRate: 0.02% (order-svc 超时熔断)\n```\n\n" +
		"~~旧告警通道~~ 已于本迭代下线，*新增* 双通道，规则详见 [值班手册](https://example.com/runbook?d=oncall&v=3)。"
	if got := markdownToCardMarkdown(md); got != want {
		t.Fatalf("markdownToCardMarkdown() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownCardDowngradesContent 验证卡片组装链路：正文先降级再进 markdown 元素
func TestMarkdownCardDowngradesContent(t *testing.T) {
	card := markdownCard(gobot.Markdown("生产环境巡检报告", "### 服务概览\n\n| a | b |\n| --- | --- |\n| 1 | 2 |"))
	elements, _ := card["elements"].([]any)
	elem, _ := elements[0].(map[string]any)
	want := "**服务概览**\n\n```\na  b\n1  2\n```"
	if elem["tag"] != "markdown" || elem["content"] != want {
		t.Fatalf("elements[0] = %v, 期望 content %q", elements[0], want)
	}
}

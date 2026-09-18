/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 13:15:07
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 13:26:31
* @FilePath: \go-bot\dingtalk\markdown_test.go
* @Description: markdown 到钉钉语法子集降级转换测试
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package dingtalk

import (
	"testing"
)

// TestMarkdownToDingtalkNativePassthrough 验证钉钉原生语法整体透传
func TestMarkdownToDingtalkNativePassthrough(t *testing.T) {
	md := "### 服务概览\n\n**巡检范围**: cn-east-1 核心服务，采样窗口 15 分钟\n\n" +
		"`工具`: go-bot v2.6.0 ｜ *巡检人*: ~~已下线~~ [值班手册](https://example.com/runbook?d=oncall&v=3)。"
	if got := markdownToDingtalkMarkdown(md); got != md {
		t.Fatalf("markdownToDingtalkMarkdown() = %q, 期望原样返回", got)
	}
}

// TestMarkdownToDingtalkCodeFencePassthrough 验证围栏代码块整块透传
func TestMarkdownToDingtalkCodeFencePassthrough(t *testing.T) {
	md := "```go\nretry: 3, backoff: 2s\ndropRate: 0.02% (含 `反引号` 与 | 竖线)\n```"
	if got := markdownToDingtalkMarkdown(md); got != md {
		t.Fatalf("markdownToDingtalkMarkdown() = %q, 期望原样返回", got)
	}
}

// TestMarkdownToDingtalkUnclosedFence 验证未闭合的围栏原样保留
func TestMarkdownToDingtalkUnclosedFence(t *testing.T) {
	md := "```go\nunterminated"
	if got := markdownToDingtalkMarkdown(md); got != md {
		t.Fatalf("markdownToDingtalkMarkdown() = %q, 期望原样返回", got)
	}
}

// TestMarkdownToDingtalkTableToCodeBlock 验证表格降级为对齐代码块
func TestMarkdownToDingtalkTableToCodeBlock(t *testing.T) {
	md := "| 服务 | 状态 |\n| --- | --- |\n| user-api | 运行中 |\n| order-svc | 已重启 |"
	want := "```\n服务       状态\nuser-api   运行中\norder-svc  已重启\n```"
	if got := markdownToDingtalkMarkdown(md); got != want {
		t.Fatalf("markdownToDingtalkMarkdown() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToDingtalkTableAlignment 验证表格列对齐方式随降级保留
func TestMarkdownToDingtalkTableAlignment(t *testing.T) {
	md := "| 指标 | 当前值 | 阈值 | 趋势 |\n| --- | ---: | ---: | :---: |\n| CPU | 92% | 80% | 上升 |\n| 内存 | 78% | 85% | 持平 |"
	want := "```\n指标  当前值  阈值  趋势\nCPU      92%   80%  上升\n内存     78%   85%  持平\n```"
	if got := markdownToDingtalkMarkdown(md); got != want {
		t.Fatalf("markdownToDingtalkMarkdown() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToDingtalkTableAfterFence 验证代码块闭合后紧跟表格的正确衔接
func TestMarkdownToDingtalkTableAfterFence(t *testing.T) {
	md := "```\ncode\n```\n| a | b |\n| --- | --- |\n| 1 | 2 |"
	want := "```\ncode\n```\n```\na  b\n1  2\n```"
	if got := markdownToDingtalkMarkdown(md); got != want {
		t.Fatalf("markdownToDingtalkMarkdown() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToDingtalkComplexDocument 验证生产级巡检报告：原生语法透传、
// 表格降级为对齐代码块、有序列表与围栏代码块的完整组合
func TestMarkdownToDingtalkComplexDocument(t *testing.T) {
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
	want := "### 服务概览\n\n" +
		"**巡检范围**: cn-east-1 核心服务，采样窗口 15 分钟\n" +
		"`工具`: go-bot v2.6.0 ｜ `巡检人`: kamalyes\n\n" +
		"```\n" +
		"服务            实例数     QPS  P99 延迟  状态\n" +
		"user-api            12  12,400      87ms  健康\n" +
		"order-svc            3   3,200     231ms  健康\n" +
		"legacy-gateway       1     800     512ms  告警\n" +
		"```\n\n" +
		"### 关键事件\n\n" +
		"1. order-svc 超时熔断触发，dropRate 0.02%，15 分钟后自愈\n\n" +
		"```go\nretry: 3, backoff: 2s, budget: 30s\ndropRate: 0.02% (order-svc 超时熔断)\n```\n\n" +
		"~~旧告警通道~~ 已于本迭代下线，*新增* 双通道，规则详见 [值班手册](https://example.com/runbook?d=oncall&v=3)。"
	if got := markdownToDingtalkMarkdown(md); got != want {
		t.Fatalf("markdownToDingtalkMarkdown() = %q, 期望 %q", got, want)
	}
}

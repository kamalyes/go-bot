/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 21:40:18
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 21:40:18
* @FilePath: \go-bot\telegram\markdown_test.go
* @Description: markdown 到 Telegram HTML 子集转换测试
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package telegram

import (
	"testing"
)

// TestMarkdownToHTMLHeadings 验证各级标题统一转为加粗行
func TestMarkdownToHTMLHeadings(t *testing.T) {
	md := "# 生产环境巡检报告\n## 服务概览\n###### 附录说明"
	want := "<b>生产环境巡检报告</b>\n<b>服务概览</b>\n<b>附录说明</b>"
	if got := markdownToTelegramHTML(md); got != want {
		t.Fatalf("markdownToTelegramHTML() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToHTMLInlineStyles 验证加粗、斜体、删除线与行内代码的行内组合
func TestMarkdownToHTMLInlineStyles(t *testing.T) {
	md := "**P0 告警** *灰度中* ~~旧通道已下线~~ `v2.6.0`"
	want := "<b>P0 告警</b> <i>灰度中</i> <s>旧通道已下线</s> <code>v2.6.0</code>"
	if got := markdownToTelegramHTML(md); got != want {
		t.Fatalf("markdownToTelegramHTML() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToHTMLLink 验证链接转换且 URL 中的 & 保持转义形态
func TestMarkdownToHTMLLink(t *testing.T) {
	md := "规则详见 [值班手册](https://example.com/runbook?d=oncall&v=3) 与 [告警策略](https://example.com/alert-policy)"
	want := "规则详见 <a href=\"https://example.com/runbook?d=oncall&amp;v=3\">值班手册</a> 与 <a href=\"https://example.com/alert-policy\">告警策略</a>"
	if got := markdownToTelegramHTML(md); got != want {
		t.Fatalf("markdownToTelegramHTML() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToHTMLCodeFenceWithLanguage 验证带语言标注的围栏代码块
func TestMarkdownToHTMLCodeFenceWithLanguage(t *testing.T) {
	md := "```go\nfmt.Println(\"hi <tag>\")\n```"
	want := "<pre><code class=\"language-go\">\nfmt.Println(&#34;hi &lt;tag&gt;&#34;)\n</code></pre>"
	if got := markdownToTelegramHTML(md); got != want {
		t.Fatalf("markdownToTelegramHTML() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToHTMLCodeFencePlain 验证无语言标注的围栏代码块与内容转义
func TestMarkdownToHTMLCodeFencePlain(t *testing.T) {
	md := "```\n纯文本 <b> 代码\n```"
	want := "<pre>\n纯文本 &lt;b&gt; 代码\n</pre>"
	if got := markdownToTelegramHTML(md); got != want {
		t.Fatalf("markdownToTelegramHTML() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToHTMLUnclosedFenceAtEOF 验证文件结束仍未闭合的代码块自动补齐闭合标签
func TestMarkdownToHTMLUnclosedFenceAtEOF(t *testing.T) {
	md := "```go\nunterminated"
	want := "<pre><code class=\"language-go\">\nunterminated\n</code></pre>"
	if got := markdownToTelegramHTML(md); got != want {
		t.Fatalf("markdownToTelegramHTML() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToHTMLCodeFenceThenTable 验证代码块闭合后紧跟表格的正确衔接
func TestMarkdownToHTMLCodeFenceThenTable(t *testing.T) {
	md := "```\ncode\n```\n| a | b |\n| --- | --- |\n| 1 | 2 |"
	want := "<pre>\ncode\n</pre>\n<pre>\na  b\n1  2\n</pre>"
	if got := markdownToTelegramHTML(md); got != want {
		t.Fatalf("markdownToTelegramHTML() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToHTMLTableAlignedColumns 验证表格去竖线、按显示宽度对齐且分隔行被丢弃
func TestMarkdownToHTMLTableAlignedColumns(t *testing.T) {
	md := "| 服务 | 状态 |\n| --- | --- |\n| user-api | 运行中 |\n| order-svc | 已重启 |"
	want := "<pre>\n服务       状态\nuser-api   运行中\norder-svc  已重启\n</pre>"
	if got := markdownToTelegramHTML(md); got != want {
		t.Fatalf("markdownToTelegramHTML() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToHTMLTableAlignment 验证分隔行语法决定列对齐方式：
// --- 左对齐、---: 右对齐、:---: 居中
func TestMarkdownToHTMLTableAlignment(t *testing.T) {
	md := "| 指标 | 当前值 | 阈值 | 趋势 |\n| --- | ---: | ---: | :---: |\n| CPU | 92% | 80% | 上升 |\n| 内存 | 78% | 85% | 持平 |"
	want := "<pre>\n指标  当前值  阈值  趋势\nCPU      92%   80%  上升\n内存     78%   85%  持平\n</pre>"
	if got := markdownToTelegramHTML(md); got != want {
		t.Fatalf("markdownToTelegramHTML() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToHTMLTableWithoutSeparator 验证无分隔行时表格仍对齐渲染且默认左对齐
func TestMarkdownToHTMLTableWithoutSeparator(t *testing.T) {
	md := "| a | b |\n| 1 | 2 |"
	want := "<pre>\na  b\n1  2\n</pre>"
	if got := markdownToTelegramHTML(md); got != want {
		t.Fatalf("markdownToTelegramHTML() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToHTMLTableEscapesCells 验证单元格内容转义且不破坏列宽对齐
func TestMarkdownToHTMLTableEscapesCells(t *testing.T) {
	md := "| 服务 | 错误率 | 备注 |\n| --- | ---: | --- |\n| user-api | 0.3% | 含 <历史> 峰值 & 抖动 |"
	want := "<pre>\n服务      错误率  备注\nuser-api    0.3%  含 &lt;历史&gt; 峰值 &amp; 抖动\n</pre>"
	if got := markdownToTelegramHTML(md); got != want {
		t.Fatalf("markdownToTelegramHTML() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToHTMLEscapesEntities 验证无 markdown 语法的特殊字符仅做 HTML 转义
func TestMarkdownToHTMLEscapesEntities(t *testing.T) {
	md := "5 < 6 & 7 > 2 \"引号\" '单引号'"
	want := "5 &lt; 6 &amp; 7 &gt; 2 &#34;引号&#34; &#39;单引号&#39;"
	if got := markdownToTelegramHTML(md); got != want {
		t.Fatalf("markdownToTelegramHTML() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToHTMLPlainPassthrough 验证普通文本行原样保留
func TestMarkdownToHTMLPlainPassthrough(t *testing.T) {
	md := "普通文本行 没有 markdown 语法"
	if got := markdownToTelegramHTML(md); got != md {
		t.Fatalf("markdownToTelegramHTML() = %q, 期望原样返回", got)
	}
}

// TestMarkdownToHTMLItalicNotForSpacedAsterisks 验证两侧带空格的星号不误判为斜体
func TestMarkdownToHTMLItalicNotForSpacedAsterisks(t *testing.T) {
	md := "a * b * c"
	if got := markdownToTelegramHTML(md); got != md {
		t.Fatalf("markdownToTelegramHTML() = %q, 期望原样返回", got)
	}
}

// TestMarkdownToHTMLHeadingWithInlineCode 验证标题内的行内代码继续转换
func TestMarkdownToHTMLHeadingWithInlineCode(t *testing.T) {
	md := "# 巡检报告 `v2.6`"
	want := "<b>巡检报告 <code>v2.6</code></b>"
	if got := markdownToTelegramHTML(md); got != want {
		t.Fatalf("markdownToTelegramHTML() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownToHTMLComplexDocument 验证生产级巡检报告：标题、元信息、
// 行内语法混合、对齐表格、带语言代码块与链接的完整转换
func TestMarkdownToHTMLComplexDocument(t *testing.T) {
	md := "# 生产环境巡检报告\n\n" +
		"**巡检范围**: cn-east-1 核心服务，采样窗口 15 分钟\n\n" +
		"`工具`: go-bot v2.6.0 ｜ `巡检人`: kamalyes\n\n" +
		"## 服务概览\n\n" +
		"| 服务 | 实例数 | QPS | P99 延迟 | 状态 |\n" +
		"| --- | ---: | ---: | ---: | :---: |\n" +
		"| user-api | 12 | 12,400 | 87ms | 健康 |\n" +
		"| order-svc | 3 | 3,200 | 231ms | 健康 |\n" +
		"| legacy-gateway | 1 | 800 | 512ms | 告警 |\n\n" +
		"## 关键事件\n\n" +
		"```go\nretry: 3, backoff: 2s, budget: 30s\ndropRate: 0.02% (order-svc 超时熔断)\n```\n\n" +
		"~~旧告警通道~~ 已于本迭代下线，*新增* Telegram/Lark 双通道，规则详见 [值班手册](https://example.com/runbook?d=oncall&v=3)。"
	want := "<b>生产环境巡检报告</b>\n\n" +
		"<b>巡检范围</b>: cn-east-1 核心服务，采样窗口 15 分钟\n\n" +
		"<code>工具</code>: go-bot v2.6.0 ｜ <code>巡检人</code>: kamalyes\n\n" +
		"<b>服务概览</b>\n\n" +
		"<pre>\n" +
		"服务            实例数     QPS  P99 延迟  状态\n" +
		"user-api            12  12,400      87ms  健康\n" +
		"order-svc            3   3,200     231ms  健康\n" +
		"legacy-gateway       1     800     512ms  告警\n" +
		"</pre>\n\n" +
		"<b>关键事件</b>\n\n" +
		"<pre><code class=\"language-go\">\n" +
		"retry: 3, backoff: 2s, budget: 30s\n" +
		"dropRate: 0.02% (order-svc 超时熔断)\n" +
		"</code></pre>\n\n" +
		"<s>旧告警通道</s> 已于本迭代下线，<i>新增</i> Telegram/Lark 双通道，规则详见 <a href=\"https://example.com/runbook?d=oncall&amp;v=3\">值班手册</a>。"
	if got := markdownToTelegramHTML(md); got != want {
		t.Fatalf("markdownToTelegramHTML() = %q, 期望 %q", got, want)
	}
}

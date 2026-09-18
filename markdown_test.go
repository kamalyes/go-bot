/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 22:18:31
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 22:18:31
* @FilePath: \go-bot\markdown_test.go
* @Description: markdown 表格识别与等宽对齐渲染测试
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import (
	"reflect"
	"testing"
)

// TestIsMarkdownTableLine 验证表格行的识别：以竖线开头（允许缩进）
func TestIsMarkdownTableLine(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"| 服务 | 状态 |", true},
		{"  | 缩进表格 | 也算 |", true},
		{"服务 | 状态", false},
		{"普通文本行", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsMarkdownTableLine(c.line); got != c.want {
			t.Fatalf("IsMarkdownTableLine(%q) = %v, 期望 %v", c.line, got, c.want)
		}
	}
}

// TestIsMarkdownTableSeparator 验证分隔行识别：仅含竖线/横线/冒号/空格且带横线
func TestIsMarkdownTableSeparator(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"| --- | ---: |", true},
		{"| :---: |", true},
		{"---", true},
		{"| -x- |", false},
		{"| abc |", false},
		{"|:|", false},
	}
	for _, c := range cases {
		if got := IsMarkdownTableSeparator(c.line); got != c.want {
			t.Fatalf("IsMarkdownTableSeparator(%q) = %v, 期望 %v", c.line, got, c.want)
		}
	}
}

// TestRenderMarkdownTableTextBasic 验证基础表格：去竖线、按显示宽度对齐、分隔行丢弃
func TestRenderMarkdownTableTextBasic(t *testing.T) {
	lines := []string{
		"| 服务 | 状态 |",
		"| --- | --- |",
		"| user-api | 运行中 |",
		"| order-svc | 已重启 |",
	}
	want := []string{"服务       状态", "user-api   运行中", "order-svc  已重启"}
	if got := RenderMarkdownTableText(lines); !reflect.DeepEqual(got, want) {
		t.Fatalf("RenderMarkdownTableText() = %q, 期望 %q", got, want)
	}
}

// TestRenderMarkdownTableTextAlignment 验证分隔行语法决定列对齐方式：
// --- 左对齐、---: 右对齐、:---: 居中
func TestRenderMarkdownTableTextAlignment(t *testing.T) {
	lines := []string{
		"| 指标 | 当前值 | 阈值 | 趋势 |",
		"| --- | ---: | ---: | :---: |",
		"| CPU | 92% | 80% | 上升 |",
		"| 内存 | 78% | 85% | 持平 |",
	}
	want := []string{"指标  当前值  阈值  趋势", "CPU      92%   80%  上升", "内存     78%   85%  持平"}
	if got := RenderMarkdownTableText(lines); !reflect.DeepEqual(got, want) {
		t.Fatalf("RenderMarkdownTableText() = %q, 期望 %q", got, want)
	}
}

// TestRenderMarkdownTableTextWithoutSeparator 验证无分隔行时仍对齐渲染且默认左对齐
func TestRenderMarkdownTableTextWithoutSeparator(t *testing.T) {
	lines := []string{"| a | b |", "| 1 | 2 |"}
	want := []string{"a  b", "1  2"}
	if got := RenderMarkdownTableText(lines); !reflect.DeepEqual(got, want) {
		t.Fatalf("RenderMarkdownTableText() = %q, 期望 %q", got, want)
	}
}

// TestRenderMarkdownTableTextSeparatorOnly 验证仅分隔行构成时返回 nil
func TestRenderMarkdownTableTextSeparatorOnly(t *testing.T) {
	if got := RenderMarkdownTableText([]string{"| --- | --- |"}); got != nil {
		t.Fatalf("RenderMarkdownTableText() = %q, 期望 nil", got)
	}
}

// TestRenderMarkdownTableTextProductionReport 验证生产级五列巡检表格：
// 混排中英文、右对齐数值列与全角字符按 2 列计算的宽度对齐
func TestRenderMarkdownTableTextProductionReport(t *testing.T) {
	lines := []string{
		"| 服务 | 实例数 | QPS | P99 延迟 | 状态 |",
		"| --- | ---: | ---: | ---: | :---: |",
		"| user-api | 12 | 12,400 | 87ms | 健康 |",
		"| order-svc | 3 | 3,200 | 231ms | 健康 |",
		"| legacy-gateway | 1 | 800 | 512ms | 告警 |",
	}
	want := []string{
		"服务            实例数     QPS  P99 延迟  状态",
		"user-api            12  12,400      87ms  健康",
		"order-svc            3   3,200     231ms  健康",
		"legacy-gateway       1     800     512ms  告警",
	}
	if got := RenderMarkdownTableText(lines); !reflect.DeepEqual(got, want) {
		t.Fatalf("RenderMarkdownTableText() = %q, 期望 %q", got, want)
	}
}

// TestRenderMarkdownTableTextKeepsRawText 验证输出保持纯文本：
// 单元格中的 HTML 特殊字符不做转义，转义由调用方按平台自行叠加
func TestRenderMarkdownTableTextKeepsRawText(t *testing.T) {
	lines := []string{
		"| 服务 | 错误率 | 备注 |",
		"| --- | ---: | --- |",
		"| user-api | 0.3% | 含 <历史> 峰值 & 抖动 |",
	}
	want := []string{"服务      错误率  备注", "user-api    0.3%  含 <历史> 峰值 & 抖动"}
	if got := RenderMarkdownTableText(lines); !reflect.DeepEqual(got, want) {
		t.Fatalf("RenderMarkdownTableText() = %q, 期望 %q", got, want)
	}
}

// TestRenderMarkdownTableTextRaggedRows 验证缺列的行补空单元格对齐、行尾空格修剪
func TestRenderMarkdownTableTextRaggedRows(t *testing.T) {
	lines := []string{
		"| 指标 | 值 | 备注 |",
		"| CPU | 92% |",
		"| 内存 | 78% | 高于基线 |",
	}
	want := []string{"指标  值   备注", "CPU   92%", "内存  78%  高于基线"}
	if got := RenderMarkdownTableText(lines); !reflect.DeepEqual(got, want) {
		t.Fatalf("RenderMarkdownTableText() = %q, 期望 %q", got, want)
	}
}

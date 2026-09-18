/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-19 10:52:09
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-19 11:02:57
* @FilePath: \go-bot\serverchan\payload_test.go
* @Description: serverchan 推送标题兜底测试
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package serverchan

import (
	"testing"

	gobot "github.com/kamalyes/go-bot"
)

// TestTitleOfTitlePreferred 验证 Title 非空时直接使用
func TestTitleOfTitlePreferred(t *testing.T) {
	if got := titleOf(gobot.Markdown("巡检报告", "**CPU 92%**")); got != "巡检报告" {
		t.Fatalf("titleOf() = %q, 期望消息 Title", got)
	}
}

// TestTitleOfFirstLineFallback 验证 Title 为空时取正文首个非空行兜底
func TestTitleOfFirstLineFallback(t *testing.T) {
	msg := gobot.Text("\n\n【告警】CPU 92%\n请立即处理")
	if got := titleOf(msg); got != "【告警】CPU 92%" {
		t.Fatalf("titleOf() = %q, 期望正文首个非空行", got)
	}
}

// TestTitleOfEmptyBodyFallback 验证正文全空时标题兜底为固定文案
func TestTitleOfEmptyBodyFallback(t *testing.T) {
	if got := titleOf(gobot.Text("\n \n")); got != "消息通知" {
		t.Fatalf("titleOf() = %q, 期望固定文案兜底", got)
	}
}

/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 15:29:56
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 21:00:22
* @FilePath: \go-bot\telegram\api_test.go
* @Description: Bot API 基础设施测试：平台码重试判定与 Retry-After 提取
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package telegram

import (
	"testing"
	"time"
)

// TestPlatformRetryable 验证平台业务码的重试白名单
func TestPlatformRetryable(t *testing.T) {
	for code, want := range map[int]bool{
		429: true,
		500: true,
		503: true,
		599: true,
		200: false,
		400: false,
		401: false,
		403: false,
		404: false,
		0:   false,
	} {
		if got := platformRetryable(code); got != want {
			t.Fatalf("platformRetryable(%d) = %v, 期望 %v", code, got, want)
		}
	}
}

// TestRetryAfterFrom 验证从错误响应体提取退避建议
func TestRetryAfterFrom(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want time.Duration
	}{
		{"携带退避建议", `{"ok":false,"error_code":429,"parameters":{"retry_after":7}}`, 7 * time.Second},
		{"无 parameters", `{"ok":false,"error_code":429}`, 0},
		{"retry_after 为零", `{"ok":false,"parameters":{"retry_after":0}}`, 0},
		{"非法 JSON", `<html>bad gateway</html>`, 0},
	} {
		if got := retryAfterFrom([]byte(tc.body)); got != tc.want {
			t.Fatalf("%s: retryAfterFrom() = %v, 期望 %v", tc.name, got, tc.want)
		}
	}
}

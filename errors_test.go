/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 22:03:17
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 22:03:17
* @FilePath: \go-bot\errors_test.go
* @Description: 结构化错误体系测试：构造、分类、可重试判定与截断
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

// sentinelTransportErr 模拟底层传输错误，用于 Unwrap 链验证
var sentinelTransportErr = errors.New("dial tcp 10.0.0.7:443: connection refused")

// TestErrorStringMinimal 验证最小字段的错误消息只含分类
func TestErrorStringMinimal(t *testing.T) {
	e := &Error{Kind: KindValidation}
	if got, want := e.Error(), "gobot: validation"; got != want {
		t.Fatalf("Error() = %q, 期望 %q", got, want)
	}
}

// TestErrorStringFullSegments 验证全字段的错误消息按序拼接各段
func TestErrorStringFullSegments(t *testing.T) {
	e := &Error{
		Platform:   PlatformTelegram,
		Operation:  "Send",
		Kind:       KindHTTP,
		HTTPStatus: 502,
		Code:       429,
		Message:    "upstream unhappy",
		Err:        errors.New("wrap me"),
	}
	want := "gobot/telegram Send: http (status 502) (code 429): upstream unhappy: wrap me"
	if got := e.Error(); got != want {
		t.Fatalf("Error() = %q, 期望 %q", got, want)
	}
}

// TestErrorStringChineseMessage 验证中文与 emoji 消息原样透传
func TestErrorStringChineseMessage(t *testing.T) {
	e := NewValidationError("Send", "目标会话不存在 🚨（生产环境）")
	if !strings.Contains(e.Error(), "目标会话不存在 🚨（生产环境）") {
		t.Fatalf("Error() = %q, 期望包含中文消息", e.Error())
	}
}

// TestErrorUnwrapChain 验证被包裹的原因对 errors.Is / errors.As 可见
func TestErrorUnwrapChain(t *testing.T) {
	e := NewTransportError(PlatformLark, "Send", sentinelTransportErr)
	if !errors.Is(e, sentinelTransportErr) {
		t.Fatal("errors.Is 应穿透到被包裹的原因")
	}
	if e.Unwrap() != sentinelTransportErr {
		t.Fatalf("Unwrap() = %v, 期望 %v", e.Unwrap(), sentinelTransportErr)
	}
	var target *Error
	if !errors.As(e, &target) || target != e {
		t.Fatal("errors.As 应命中自身")
	}
}

// TestNewValidationError 验证本地校验错误的字段填充
func TestNewValidationError(t *testing.T) {
	e := NewValidationError("Send", "text content is empty")
	if e.Kind != KindValidation || e.Operation != "Send" || e.Retryable {
		t.Fatalf("NewValidationError() = %+v, 期望 validation/Send/不可重试", e)
	}
}

// TestNewTransportError 验证取消类错误不可重试、其余传输错误可重试
func TestNewTransportError(t *testing.T) {
	for _, tc := range []struct {
		name       string
		err        error
		retryable  bool
	}{
		{"context canceled", context.Canceled, false},
		{"deadline exceeded", context.DeadlineExceeded, false},
		{"connection refused", sentinelTransportErr, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := NewTransportError(PlatformTelegram, "Send", tc.err)
			if e.Kind != KindTransport || e.Platform != PlatformTelegram {
				t.Fatalf("错误分类 = %s/%s, 期望 transport/telegram", e.Platform, e.Kind)
			}
			if e.Retryable != tc.retryable {
				t.Fatalf("Retryable = %v, 期望 %v", e.Retryable, tc.retryable)
			}
			if e.Err != tc.err {
				t.Fatalf("Err = %v, 期望原样保留", e.Err)
			}
		})
	}
}

// TestNewHTTPError 验证 HTTP 状态码驱动的可重试判定与字段填充
func TestNewHTTPError(t *testing.T) {
	for _, tc := range []struct {
		status     int
		retryable  bool
	}{
		{http.StatusRequestTimeout, true},
		{http.StatusTooEarly, true},
		{http.StatusTooManyRequests, true},
		{http.StatusInternalServerError, true},
		{http.StatusBadGateway, true},
		{599, true},
		{http.StatusBadRequest, false},
		{http.StatusUnauthorized, false},
		{http.StatusForbidden, false},
		{http.StatusNotFound, false},
		{http.StatusTeapot, false},
	} {
		e := NewHTTPError(PlatformTelegram, "Send", tc.status, "body", 0)
		if e.Kind != KindHTTP || e.HTTPStatus != tc.status {
			t.Fatalf("status=%d: 分类 = %s/%d, 期望 http/%d", tc.status, e.Kind, e.HTTPStatus, tc.status)
		}
		if e.Retryable != tc.retryable {
			t.Fatalf("status=%d: Retryable = %v, 期望 %v", tc.status, e.Retryable, tc.retryable)
		}
	}
}

// TestNewHTTPErrorRetryAfter 验证服务端退避建议原样透传
func TestNewHTTPErrorRetryAfter(t *testing.T) {
	e := NewHTTPError(PlatformLark, "Send", 429, "slow down", 7*time.Second)
	if e.RetryAfter != 7*time.Second {
		t.Fatalf("RetryAfter = %v, 期望 7s", e.RetryAfter)
	}
	if e.Message != "slow down" {
		t.Fatalf("Message = %q, 期望响应体短文本", e.Message)
	}
}

// TestNewPlatformError 验证平台错误的字段与调用方给定的可重试标记
func TestNewPlatformError(t *testing.T) {
	e := NewPlatformError(PlatformTelegram, "Send", 429, "Too Many Requests: retry after 3", true)
	if e.Kind != KindPlatform || e.Code != 429 || !e.Retryable {
		t.Fatalf("NewPlatformError() = %+v, 期望 platform/429/可重试", e)
	}
	permanent := NewPlatformError(PlatformTelegram, "Send", 400, "chat not found", false)
	if permanent.Retryable {
		t.Fatal("调用方标记不可重试时应保持不可重试")
	}
}

// TestNewDecodeError 验证解码错误的字段填充
func TestNewDecodeError(t *testing.T) {
	cause := errors.New("invalid character '<'")
	e := NewDecodeError(PlatformTelegram, "Send", cause)
	if e.Kind != KindDecode || e.Err != cause || e.Message != "decode response" {
		t.Fatalf("NewDecodeError() = %+v, 期望 decode 且保留原因", e)
	}
}

// TestRetryableFunc 验证可重试判定只认结构化 *Error 且可穿透包裹
func TestRetryableFunc(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"可重试平台错误", NewPlatformError(PlatformTelegram, "Send", 429, "rate limited", true), true},
		{"不可重试校验错误", NewValidationError("Send", "bad"), false},
		{"被 fmt 包裹的可重试错误", errors.Join(errors.New("x"), NewPlatformError(PlatformTelegram, "Send", 500, "boom", true)), true},
		{"普通错误", errors.New("plain"), false},
		{"nil", nil, false},
	} {
		if got := retryable(tc.err); got != tc.want {
			t.Fatalf("%s: retryable() = %v, 期望 %v", tc.name, got, tc.want)
		}
	}
}

// TestHTTPRetryable 验证状态码重试白名单的边界
func TestHTTPRetryable(t *testing.T) {
	for status, want := range map[int]bool{
		408: true, 425: true, 429: true,
		500: true, 503: true, 599: true,
		200: false, 301: false, 400: false, 404: false, 418: false,
	} {
		if got := httpRetryable(status); got != want {
			t.Fatalf("httpRetryable(%d) = %v, 期望 %v", status, got, want)
		}
	}
}

// TestTruncate 验证响应体按 rune 边界截断
func TestTruncate(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  string
	}{
		{"短文本原样返回", "部署完成 ✅", "部署完成 ✅"},
		{"恰好等于上限", strings.Repeat("a", maxBodyInError), strings.Repeat("a", maxBodyInError)},
		{"超限 ASCII 截断", strings.Repeat("a", maxBodyInError+1), strings.Repeat("a", maxBodyInError) + "…(truncated)"},
		{"多字节字符跨越切点", strings.Repeat("a", 510) + "中" + strings.Repeat("b", 10), strings.Repeat("a", 510) + "…(truncated)"},
		{"非法 UTF-8 字节回退", strings.Repeat("a", 511) + "\xff" + "tail", strings.Repeat("a", 511) + "…(truncated)"},
	} {
		if got := truncate(tc.input); got != tc.want {
			t.Fatalf("%s: truncate() = %q, 期望 %q", tc.name, got, tc.want)
		}
	}
}

// TestUTF8Valid 验证 UTF-8 合法性探测
func TestUTF8Valid(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  bool
	}{
		{"", true},
		{"纯 ASCII", true},
		{"中文与 emoji 🚀", true},
		{"坏字节 \xff", false},
	} {
		if got := utf8Valid(tc.input); got != tc.want {
			t.Fatalf("utf8Valid(%q) = %v, 期望 %v", tc.input, got, tc.want)
		}
	}
}

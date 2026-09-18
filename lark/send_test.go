/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 15:18:37
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 20:17:26
* @FilePath: \go-bot\lark\send_test.go
* @Description: lark webhook 推送测试：消息分支、签名、限流与错误路径
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lark

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	gobot "github.com/kamalyes/go-bot"
)

// captureRequest 返回一个捕获请求体并回写指定响应的 handler
func captureRequest(t *testing.T, captured *map[string]any, status int, respBody string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("读取请求体失败: %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("请求体不是合法 JSON: %v", err)
		}
		*captured = payload
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}
}

// TestSendTextSuccess 验证 text 消息的路径拼接与请求体结构
func TestSendTextSuccess(t *testing.T) {
	var payload map[string]any
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("请求体不是合法 JSON: %v", err)
		}
		_, _ = w.Write([]byte(`{"code":0,"msg":"success"}`))
	}))
	defer srv.Close()

	adapter, err := New(Config{Token: "hook-token", APIBase: srv.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	result, err := adapter.Send(context.Background(), gobot.Chat("ignored"), gobot.Text("部署完成"))
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if result == nil || result.MessageID != "" {
		t.Fatalf("Send() 结果 = %+v, 期望空 MessageID", result)
	}
	if gotPath != webhookPathPrefix+"hook-token" {
		t.Fatalf("请求路径 = %q, 期望 %q", gotPath, webhookPathPrefix+"hook-token")
	}
	if payload["msg_type"] != "text" {
		t.Fatalf("msg_type = %v, 期望 text", payload["msg_type"])
	}
	content, _ := payload["content"].(map[string]any)
	if content["text"] != "部署完成" {
		t.Fatalf("content.text = %v, 期望 部署完成", content["text"])
	}
}

// TestSendContentType 验证请求显式携带 Content-Type（与 telegram 的 SetBodyJSON 行为对齐）
func TestSendContentType(t *testing.T) {
	var gotContentType string
	adapter := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		_, _ = w.Write([]byte(`{"code":0}`))
	})
	if _, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Text("hi")); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q, 期望 application/json", gotContentType)
	}
}

// TestSendMarkdownSuccess 验证 markdown 消息组装为 interactive 卡片
func TestSendMarkdownSuccess(t *testing.T) {
	var payload map[string]any
	adapter := newTestAdapter(t, captureRequest(t, &payload, http.StatusOK, `{"code":0}`))
	if _, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Markdown("告警", "**CPU 90%**")); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if payload["msg_type"] != "interactive" {
		t.Fatalf("msg_type = %v, 期望 interactive", payload["msg_type"])
	}
	card, _ := payload["card"].(map[string]any)
	if card == nil {
		t.Fatal("card 缺失")
	}
	header, _ := card["header"].(map[string]any)
	title, _ := header["title"].(map[string]any)
	if title["content"] != "告警" {
		t.Fatalf("卡片标题 = %v, 期望 告警", title["content"])
	}
}

// TestSendWithSignature 验证开启签名后 payload 携带 timestamp 与 sign
func TestSendWithSignature(t *testing.T) {
	var payload map[string]any
	adapter := newTestAdapter(t, captureRequest(t, &payload, http.StatusOK, `{"code":0}`))
	adapter.cfg.Secret = "sign-secret"

	if _, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Text("hi")); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	tsStr, _ := payload["timestamp"].(string)
	sec, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		t.Fatalf("timestamp = %q, 不是合法秒级时间戳", tsStr)
	}
	wantTS, wantSign := sign("sign-secret", time.Unix(sec, 0))
	if tsStr != wantTS {
		t.Fatalf("timestamp = %q, 期望 sign 确定性输出 %q", tsStr, wantTS)
	}
	if payload["sign"] != wantSign {
		t.Fatalf("sign = %v, 期望 %v", payload["sign"], wantSign)
	}
}

// TestSendImageRejected 验证 webhook 形态不支持图片消息
func TestSendImageRejected(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("图片消息不应发出请求")
	})
	_, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Image("https://example.com/a.png"))
	assertValidationError(t, err, "image")
}

// TestSendUnsupportedType 验证未知消息类型被本地拒绝
func TestSendUnsupportedType(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("未知类型不应发出请求")
	})
	msg := &gobot.Message{Type: gobot.MessageType("sticker"), Text: "x"}
	_, err := adapter.Send(context.Background(), gobot.Chat(""), msg)
	assertValidationError(t, err, "unsupported")
}

// TestSendPayloadTooLarge 验证超出 20KB 上限时本地拦截
func TestSendPayloadTooLarge(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("超限消息不应发出请求")
	})
	_, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Text(strings.Repeat("a", maxPayloadSize)))
	assertValidationError(t, err, "20 KB")
}

// TestSendHTTPStatusError 验证非 2xx 响应转为 HTTP 错误
func TestSendHTTPStatusError(t *testing.T) {
	var payload map[string]any
	adapter := newTestAdapter(t, captureRequest(t, &payload, http.StatusInternalServerError, "server error"))
	_, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Text("hi"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindHTTP || e.HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("Send() 错误 = %v, 期望 http/500", err)
	}
}

// TestSendDecodeError 验证响应体无法解码时返回解码错误
func TestSendDecodeError(t *testing.T) {
	var payload map[string]any
	adapter := newTestAdapter(t, captureRequest(t, &payload, http.StatusOK, "<html>not json</html>"))
	_, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Text("hi"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindDecode {
		t.Fatalf("Send() 错误 = %v, 期望 decode", err)
	}
}

// TestSendRateLimited 验证限流业务码转为可重试的平台错误
func TestSendRateLimited(t *testing.T) {
	var payload map[string]any
	adapter := newTestAdapter(t, captureRequest(t, &payload, http.StatusOK,
		`{"code":11232,"msg":"too many request"}`))
	_, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Text("hi"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindPlatform || e.Code != CodeRateLimited {
		t.Fatalf("Send() 错误 = %v, 期望 platform/11232", err)
	}
	if !e.Retryable {
		t.Fatal("限流错误应可重试")
	}
}

// TestSendPlatformError 验证非限流业务码转为不可重试的平台错误
func TestSendPlatformError(t *testing.T) {
	var payload map[string]any
	adapter := newTestAdapter(t, captureRequest(t, &payload, http.StatusOK,
		`{"code":19021,"msg":"sign match fail"}`))
	_, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Text("hi"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindPlatform || e.Code != CodeSignInvalid || e.Retryable {
		t.Fatalf("Send() 错误 = %v, 期望 platform/19021 不可重试", err)
	}
}

// TestSendTransportError 验证网络不可达时返回传输错误
func TestSendTransportError(t *testing.T) {
	adapter, err := New(Config{Token: "t", APIBase: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = adapter.Send(context.Background(), gobot.Chat(""), gobot.Text("hi"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindTransport {
		t.Fatalf("Send() 错误 = %v, 期望 transport", err)
	}
}

// TestSendBodyReadError 验证响应体读取中断时返回传输错误
// 声明的 Content-Length 大于实际写入，客户端读到 unexpected EOF
func TestSendBodyReadError(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1024")
		_, _ = w.Write([]byte(`{"code":0`))
	})
	_, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Text("hi"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindTransport {
		t.Fatalf("Send() 错误 = %v, 期望 transport", err)
	}
}

// assertValidationError 断言 err 是携带关键字的校验错误
func assertValidationError(t *testing.T, err error, keyword string) {
	t.Helper()
	if err == nil {
		t.Fatal("期望返回错误, got nil")
	}
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindValidation {
		t.Fatalf("错误类型 = %T, 期望 validation", err)
	}
	if !strings.Contains(e.Message, keyword) {
		t.Fatalf("错误消息 %q 未包含关键字 %q", e.Message, keyword)
	}
}

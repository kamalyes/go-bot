/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 17:33:56
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 21:00:56
* @FilePath: \go-bot\telegram\send_test.go
* @Description: Telegram 消息发送测试：方法路由、请求体结构与四类错误映射
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	gobot "github.com/kamalyes/go-bot"
)

// captureRequest 返回一个捕获路径与请求体并回写指定响应的 handler
func captureRequest(t *testing.T, gotPath *string, captured *map[string]any, status int, respBody string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*gotPath = r.URL.Path
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

// okBody 是 sendMessage / sendPhoto 的成功响应包
const okBody = `{"ok":true,"result":{"message_id":42}}`

// TestSendTextSuccess 验证文本消息路由到 sendMessage 且请求体完整
func TestSendTextSuccess(t *testing.T) {
	var payload map[string]any
	var gotPath string
	adapter := newTestAdapter(t, captureRequest(t, &gotPath, &payload, http.StatusOK, okBody))

	result, err := adapter.Send(context.Background(), gobot.Chat("-100233"), gobot.Text("部署完成 ✅ CPU 92.5%"))
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if gotPath != "/bottest-token/sendMessage" {
		t.Fatalf("请求路径 = %q, 期望 /bottest-token/sendMessage", gotPath)
	}
	if result == nil || result.MessageID != "42" {
		t.Fatalf("Send() result = %+v, 期望 MessageID 42", result)
	}
	if payload["chat_id"] != "-100233" || payload["parse_mode"] != DefaultParseMode {
		t.Fatalf("payload = %v, 期望携带 chat_id 与默认 parse_mode", payload)
	}
	if payload["text"] != "部署完成 ✅ CPU 92.5%" {
		t.Fatalf("text = %v, 期望原样透传", payload["text"])
	}
}

// TestSendTextParseModeNone 验证 none 模式不向平台传 parse_mode
func TestSendTextParseModeNone(t *testing.T) {
	var payload map[string]any
	var gotPath string
	srv := captureRequest(t, &gotPath, &payload, http.StatusOK, okBody)
	adapter := newTestAdapter(t, srv)
	adapter.cfg.ParseMode = ""

	if _, err := adapter.Send(context.Background(), gobot.Chat("1"), gobot.Text("纯文本")); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if _, ok := payload["parse_mode"]; ok {
		t.Fatalf("payload = %v, 纯文本模式不应携带 parse_mode", payload)
	}
}

// TestSendMarkdownBoldTitle 验证 markdown 标题转义为 HTML 加粗行
func TestSendMarkdownBoldTitle(t *testing.T) {
	var payload map[string]any
	var gotPath string
	adapter := newTestAdapter(t, captureRequest(t, &gotPath, &payload, http.StatusOK, okBody))

	if _, err := adapter.Send(context.Background(), gobot.Chat("1"), gobot.Markdown("告警 <prod> & 紧急", "**CPU 92%**")); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if gotPath != "/bottest-token/sendMessage" {
		t.Fatalf("请求路径 = %q, 期望 sendMessage", gotPath)
	}
	want := "<b>告警 &lt;prod&gt; &amp; 紧急</b>\n<b>CPU 92%</b>"
	if payload["text"] != want {
		t.Fatalf("text = %v, 期望 %q", payload["text"], want)
	}
}

// TestSendImageSuccess 验证图片消息路由到 sendPhoto 且携带 caption
func TestSendImageSuccess(t *testing.T) {
	var payload map[string]any
	var gotPath string
	adapter := newTestAdapter(t, captureRequest(t, &gotPath, &payload, http.StatusOK, okBody))

	msg := gobot.Image("https://example.com/panel.png?tls=1&refresh=5s")
	msg.Text = "监控大盘截图 📊"
	if _, err := adapter.Send(context.Background(), gobot.Chat("1"), msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if gotPath != "/bottest-token/sendPhoto" {
		t.Fatalf("请求路径 = %q, 期望 sendPhoto", gotPath)
	}
	if payload["photo"] != "https://example.com/panel.png?tls=1&refresh=5s" {
		t.Fatalf("photo = %v, 期望原样透传", payload["photo"])
	}
	if payload["caption"] != "监控大盘截图 📊" {
		t.Fatalf("caption = %v, 期望 Text 转为 caption", payload["caption"])
	}
}

// TestSendImageWithoutCaption 验证空说明不产生 caption 字段
func TestSendImageWithoutCaption(t *testing.T) {
	var payload map[string]any
	var gotPath string
	adapter := newTestAdapter(t, captureRequest(t, &gotPath, &payload, http.StatusOK, okBody))

	if _, err := adapter.Send(context.Background(), gobot.Chat("1"), gobot.Image("https://example.com/a.png")); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if gotPath != "/bottest-token/sendPhoto" {
		t.Fatalf("请求路径 = %q, 期望 sendPhoto", gotPath)
	}
	if _, ok := payload["caption"]; ok {
		t.Fatalf("payload = %v, 空说明不应携带 caption", payload)
	}
}

// TestSendUnsupportedType 验证未知消息类型被本地拒绝
func TestSendUnsupportedType(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("未知类型不应发出请求")
	})
	msg := &gobot.Message{Type: gobot.MessageType("sticker"), Text: "x"}
	_, err := adapter.Send(context.Background(), gobot.Chat("1"), msg)
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindValidation {
		t.Fatalf("Send() 错误 = %v, 期望 validation", err)
	}
}

// TestSendLargeMessageID 验证大数值 message_id 的字符串化
func TestSendLargeMessageID(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":9876543210}}`))
	})
	result, err := adapter.Send(context.Background(), gobot.Chat("1"), gobot.Text("hi"))
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if result.MessageID != strconv.FormatInt(9876543210, 10) {
		t.Fatalf("MessageID = %q, 期望 9876543210", result.MessageID)
	}
}

// TestSendHTTPStatusError 验证非 2xx 响应转为携带正文的 HTTP 错误
func TestSendHTTPStatusError(t *testing.T) {
	adapter := newTestAdapter(t, captureRequest(t, new(string), new(map[string]any), http.StatusServiceUnavailable, "svc unavailable"))
	_, err := adapter.Send(context.Background(), gobot.Chat("1"), gobot.Text("hi"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindHTTP || e.HTTPStatus != http.StatusServiceUnavailable {
		t.Fatalf("Send() 错误 = %v, 期望 http/503", err)
	}
	if !e.Retryable {
		t.Fatal("5xx 应可重试")
	}
	if e.Message != "svc unavailable" {
		t.Fatalf("Message = %q, 期望携带响应体", e.Message)
	}
}

// TestSendHTTPClientError 验证 4xx 不重试
func TestSendHTTPClientError(t *testing.T) {
	adapter := newTestAdapter(t, captureRequest(t, new(string), new(map[string]any), http.StatusUnauthorized, "Unauthorized"))
	_, err := adapter.Send(context.Background(), gobot.Chat("1"), gobot.Text("hi"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindHTTP || e.HTTPStatus != http.StatusUnauthorized {
		t.Fatalf("Send() 错误 = %v, 期望 http/401", err)
	}
	if e.Retryable {
		t.Fatal("401 不应重试")
	}
}

// TestSendRateLimited 验证 429 平台错误携带退避建议且可重试
func TestSendRateLimited(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 3","parameters":{"retry_after":3}}`))
	})
	_, err := adapter.Send(context.Background(), gobot.Chat("1"), gobot.Text("hi"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindPlatform || e.Code != 429 {
		t.Fatalf("Send() 错误 = %v, 期望 platform/429", err)
	}
	if !e.Retryable {
		t.Fatal("限流错误应可重试")
	}
	if e.RetryAfter != 3*time.Second {
		t.Fatalf("RetryAfter = %v, 期望 3s", e.RetryAfter)
	}
}

// TestSendPlatformErrorNotRetryable 验证普通业务码不可重试且无退避建议
func TestSendPlatformErrorNotRetryable(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`))
	})
	_, err := adapter.Send(context.Background(), gobot.Chat("1"), gobot.Text("hi"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindPlatform || e.Code != 400 {
		t.Fatalf("Send() 错误 = %v, 期望 platform/400", err)
	}
	if e.Retryable || e.RetryAfter != 0 {
		t.Fatalf("400 应不可重试且无退避建议, got Retryable=%v RetryAfter=%v", e.Retryable, e.RetryAfter)
	}
	if e.Message != "Bad Request: chat not found" {
		t.Fatalf("Message = %q, 期望平台描述", e.Message)
	}
}

// TestSendDecodeError 验证响应体无法解码时返回解码错误
func TestSendDecodeError(t *testing.T) {
	adapter := newTestAdapter(t, captureRequest(t, new(string), new(map[string]any), http.StatusOK, "<html>not json</html>"))
	_, err := adapter.Send(context.Background(), gobot.Chat("1"), gobot.Text("hi"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindDecode {
		t.Fatalf("Send() 错误 = %v, 期望 decode", err)
	}
}

// TestSendTransportError 验证网络不可达时返回可重试的传输错误
func TestSendTransportError(t *testing.T) {
	adapter, err := New(Config{Token: "t", APIBase: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = adapter.Send(context.Background(), gobot.Chat("1"), gobot.Text("hi"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindTransport {
		t.Fatalf("Send() 错误 = %v, 期望 transport", err)
	}
	if !e.Retryable {
		t.Fatal("连接失败应可重试")
	}
}

/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-19 10:28:53
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-19 10:52:09
* @FilePath: \go-bot\serverchan\send_test.go
* @Description: serverchan 推送测试：表单投递、标题兜底与错误路径
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package serverchan

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"

	gobot "github.com/kamalyes/go-bot"
)

// captureForm 返回一个捕获路径、query 与表单请求体并回写指定响应的 handler
func captureForm(t *testing.T, gotPath *string, gotQuery *string, captured *url.Values, status int, respBody string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*gotPath = r.URL.Path
		*gotQuery = r.URL.RawQuery
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("读取请求体失败: %v", err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Errorf("请求体不是合法表单: %v", err)
		}
		*captured = form
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}
}

// TestSendTextSuccess 验证 text 消息的推送路径、title 兜底与 desp 透传
func TestSendTextSuccess(t *testing.T) {
	var form url.Values
	var gotPath, gotQuery string
	adapter := newTestAdapter(t, captureForm(t, &gotPath, &gotQuery, &form, http.StatusOK,
		`{"code":0,"message":"","data":{"pushid":"2026091909000016","readkey":"rk"}}`))

	text := "【生产发布通知】order-svc v2.6.0\n灰度 10% → 50% → 全量，错误率 > 0.5% 自动回滚"
	result, err := adapter.Send(context.Background(), gobot.Chat("ignored"), gobot.Text(text))
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if gotPath != "/test-key.send" {
		t.Fatalf("请求路径 = %q, 期望 /test-key.send", gotPath)
	}
	if gotQuery != "" {
		t.Fatalf("query = %q, 期望无 query 参数", gotQuery)
	}
	if form.Get("title") != "【生产发布通知】order-svc v2.6.0" {
		t.Fatalf("title = %q, 期望正文首个非空行兜底", form.Get("title"))
	}
	if form.Get("desp") != text {
		t.Fatalf("desp = %q, 期望正文原样透传", form.Get("desp"))
	}
	if result == nil || result.MessageID != "2026091909000016" {
		t.Fatalf("Send() 结果 = %+v, 期望回传 pushid", result)
	}
}

// TestSendContentType 验证请求显式携带表单 Content-Type（表单接口不接受 JSON 编码）
func TestSendContentType(t *testing.T) {
	var gotContentType string
	adapter := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		_, _ = w.Write([]byte(`{"code":0,"message":"","data":{"pushid":"p1"}}`))
	})
	if _, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Text("hi")); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if gotContentType != "application/x-www-form-urlencoded" {
		t.Fatalf("Content-Type = %q, 期望 application/x-www-form-urlencoded", gotContentType)
	}
}

// TestSendMarkdownSuccess 验证 markdown 消息 Title 优先作推送标题、desp 原文透传
func TestSendMarkdownSuccess(t *testing.T) {
	var form url.Values
	var gotPath, gotQuery string
	adapter := newTestAdapter(t, captureForm(t, &gotPath, &gotQuery, &form, http.StatusOK, `{"code":0}`))

	content := "| 指标 | 当前值 |\n| --- | ---: |\n| CPU | 92% |"
	if _, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Markdown("告警 <prod>", content)); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if form.Get("title") != "告警 <prod>" {
		t.Fatalf("title = %q, 期望消息 Title", form.Get("title"))
	}
	if form.Get("desp") != content {
		t.Fatalf("desp = %q, 期望 markdown 原文透传（平台侧渲染）", form.Get("desp"))
	}
}

// TestSendImageRejected 验证个人推送拒绝图片消息
func TestSendImageRejected(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("图片消息不应发出请求")
	})
	_, err := adapter.Send(context.Background(), gobot.Chat("1"), gobot.Image("https://example.com/a.png"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindValidation {
		t.Fatalf("Send() 错误 = %v, 期望 validation", err)
	}
}

// TestSendMentionRejected 验证个人推送拒绝 @ 提及
func TestSendMentionRejected(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("携带提及的消息不应发出请求")
	})
	_, err := adapter.Send(context.Background(), gobot.Chat("1"), gobot.Text("hi").AtAll())
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindValidation {
		t.Fatalf("Send() 错误 = %v, 期望 validation", err)
	}
}

// TestSendUnsupportedType 验证未知消息类型被本地拒绝
func TestSendUnsupportedType(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("未知类型不应发出请求")
	})
	msg := &gobot.Message{Type: gobot.MessageType("audio"), Text: "x"}
	_, err := adapter.Send(context.Background(), gobot.Chat("1"), msg)
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindValidation {
		t.Fatalf("Send() 错误 = %v, 期望 validation", err)
	}
}

// TestSendPlatformError 验证业务码失败转为不可重试的平台错误
func TestSendPlatformError(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":40001,"message":"bad sendkey","data":{}}`))
	})
	_, err := adapter.Send(context.Background(), gobot.Chat("1"), gobot.Text("hi"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindPlatform || e.Code != 40001 {
		t.Fatalf("Send() 错误 = %v, 期望 platform/40001", err)
	}
	if e.Retryable {
		t.Fatal("业务码失败应不可重试")
	}
	if e.Message != "bad sendkey" {
		t.Fatalf("Message = %q, 期望平台错误说明", e.Message)
	}
}

// TestSendHTTPStatusError 验证非 2xx 响应转为携带正文的 HTTP 错误
func TestSendHTTPStatusError(t *testing.T) {
	var form url.Values
	var gotPath, gotQuery string
	adapter := newTestAdapter(t, captureForm(t, &gotPath, &gotQuery, &form, http.StatusServiceUnavailable, "svc unavailable"))
	_, err := adapter.Send(context.Background(), gobot.Chat("1"), gobot.Text("hi"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindHTTP || e.HTTPStatus != http.StatusServiceUnavailable {
		t.Fatalf("Send() 错误 = %v, 期望 http/503", err)
	}
	if !e.Retryable {
		t.Fatal("5xx 应可重试")
	}
}

// TestSendDecodeError 验证响应体无法解码时返回解码错误
func TestSendDecodeError(t *testing.T) {
	var form url.Values
	var gotPath, gotQuery string
	adapter := newTestAdapter(t, captureForm(t, &gotPath, &gotQuery, &form, http.StatusOK, "<html>not json</html>"))
	_, err := adapter.Send(context.Background(), gobot.Chat("1"), gobot.Text("hi"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindDecode {
		t.Fatalf("Send() 错误 = %v, 期望 decode", err)
	}
}

// TestSendTransportError 验证网络不可达时返回可重试的传输错误
func TestSendTransportError(t *testing.T) {
	adapter, err := New(Config{SendKey: "SCTtest", APIBase: "http://127.0.0.1:1"})
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

/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 17:00:36
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 22:06:19
* @FilePath: \go-bot\dingtalk\send_test.go
* @Description: dingtalk webhook 推送测试：消息分支、加签 query 与错误路径
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package dingtalk

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	gobot "github.com/kamalyes/go-bot"
)

// captureRequest 返回一个捕获路径、query 与请求体并回写指定响应的 handler
func captureRequest(t *testing.T, gotPath *string, gotQuery *string, captured *map[string]any, status int, respBody string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*gotPath = r.URL.Path
		*gotQuery = r.URL.RawQuery
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

// TestSendTextSuccess 验证 text 消息的路径、access_token query 与请求体结构
func TestSendTextSuccess(t *testing.T) {
	var payload map[string]any
	var gotPath, gotQuery string
	adapter := newTestAdapter(t, captureRequest(t, &gotPath, &gotQuery, &payload, http.StatusOK, `{"errcode":0,"errmsg":"ok"}`))

	result, err := adapter.Send(context.Background(), gobot.Chat("ignored"), gobot.Text("【生产发布通知】order-svc v2.6.0 灰度 10% → 50% → 全量"))
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if result == nil || result.MessageID != "" {
		t.Fatalf("Send() 结果 = %+v, 期望空 MessageID", result)
	}
	if gotPath != webhookPath {
		t.Fatalf("请求路径 = %q, 期望 %q", gotPath, webhookPath)
	}
	if gotQuery != "access_token=test-token" {
		t.Fatalf("query = %q, 期望仅携带 access_token", gotQuery)
	}
	if payload["msgtype"] != "text" {
		t.Fatalf("msgtype = %v, 期望 text", payload["msgtype"])
	}
	text, _ := payload["text"].(map[string]any)
	if text["content"] != "【生产发布通知】order-svc v2.6.0 灰度 10% → 50% → 全量" {
		t.Fatalf("content = %v, 期望原样透传", text["content"])
	}
}

// TestSendContentType 验证请求显式携带 Content-Type（钉钉强校验，缺失返回 errcode 43004）
func TestSendContentType(t *testing.T) {
	var gotContentType string
	adapter := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	})
	if _, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Text("hi")); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q, 期望 application/json", gotContentType)
	}
}

// TestSendMarkdownSuccess 验证 markdown 消息降级组装与 title 映射
func TestSendMarkdownSuccess(t *testing.T) {
	var payload map[string]any
	var gotPath string
	var gotQuery string
	adapter := newTestAdapter(t, captureRequest(t, &gotPath, &gotQuery, &payload, http.StatusOK, `{"errcode":0}`))

	if _, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Markdown("告警 <prod>", "| 指标 | 当前值 |\n| --- | ---: |\n| CPU | 92% |")); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if payload["msgtype"] != "markdown" {
		t.Fatalf("msgtype = %v, 期望 markdown", payload["msgtype"])
	}
	md, _ := payload["markdown"].(map[string]any)
	if md["title"] != "告警 <prod>" {
		t.Fatalf("title = %v, 期望 消息 Title", md["title"])
	}
	wantText := "```\n指标  当前值\nCPU      92%\n```"
	if md["text"] != wantText {
		t.Fatalf("text = %q, 期望 %q", md["text"], wantText)
	}
}

// TestSendWithSecret 验证配置 Secret 时 query 追加 timestamp 与 sign
func TestSendWithSecret(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer srv.Close()
	adapter, err := New(Config{Token: "hook-token", Secret: "SECtest", APIBase: srv.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if _, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Text("带签投递")); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if !containsParam(gotQuery, "access_token", "hook-token") ||
		!containsParam(gotQuery, "timestamp", "") || !containsParam(gotQuery, "sign", "") {
		t.Fatalf("query = %q, 期望携带 access_token/timestamp/sign", gotQuery)
	}
}

// containsParam 判断 query 中指定参数存在且取值匹配（期望值为空串时仅判存在）
func containsParam(rawQuery, key, want string) bool {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return false
	}
	got, ok := values[key]
	if !ok || len(got) == 0 {
		return false
	}
	return want == "" || got[0] == want
}

// TestSendImageRejected 验证 webhook 形态拒绝图片消息
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
		_, _ = w.Write([]byte(`{"errcode":310000,"errmsg":"keywords not in content, ip is not in whitelist"}`))
	})
	_, err := adapter.Send(context.Background(), gobot.Chat("1"), gobot.Text("hi"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindPlatform || e.Code != 310000 {
		t.Fatalf("Send() 错误 = %v, 期望 platform/310000", err)
	}
	if e.Retryable {
		t.Fatal("业务码失败应不可重试")
	}
	if e.Message != "keywords not in content, ip is not in whitelist" {
		t.Fatalf("Message = %q, 期望平台错误说明", e.Message)
	}
}

// TestSendHTTPStatusError 验证非 2xx 响应转为携带正文的 HTTP 错误
func TestSendHTTPStatusError(t *testing.T) {
	var payload map[string]any
	var gotPath, gotQuery string
	adapter := newTestAdapter(t, captureRequest(t, &gotPath, &gotQuery, &payload, http.StatusServiceUnavailable, "svc unavailable"))
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
	var payload map[string]any
	var gotPath, gotQuery string
	adapter := newTestAdapter(t, captureRequest(t, &gotPath, &gotQuery, &payload, http.StatusOK, "<html>not json</html>"))
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

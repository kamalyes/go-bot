/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-19 15:52:09
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-19 16:28:31
* @FilePath: \go-bot\wecom\send_test.go
* @Description: wecom webhook 推送测试：消息分支、提及映射与错误路径
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package wecom

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

// TestSendTextSuccess 验证 text 消息的路径、key query 与请求体结构
func TestSendTextSuccess(t *testing.T) {
	var payload map[string]any
	var gotPath, gotQuery string
	adapter := newTestAdapter(t, captureRequest(t, &gotPath, &gotQuery, &payload, http.StatusOK, `{"errcode":0,"errmsg":"ok"}`))

	text := "【生产发布通知】order-svc v2.6.0 灰度 10% → 50% → 全量"
	result, err := adapter.Send(context.Background(), gobot.Chat("ignored"), gobot.Text(text))
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if result == nil || result.MessageID != "" {
		t.Fatalf("Send() 结果 = %+v, 期望空 MessageID", result)
	}
	if gotPath != webhookPath {
		t.Fatalf("请求路径 = %q, 期望 %q", gotPath, webhookPath)
	}
	if gotQuery != "key=test-key" {
		t.Fatalf("query = %q, 期望仅携带 key", gotQuery)
	}
	if payload["msgtype"] != "text" {
		t.Fatalf("msgtype = %v, 期望 text", payload["msgtype"])
	}
	textField, _ := payload["text"].(map[string]any)
	if textField["content"] != text {
		t.Fatalf("content = %v, 期望原样透传", textField["content"])
	}
	if _, ok := textField["mentioned_list"]; ok {
		t.Fatal("无提及时不应携带 mentioned_list")
	}
}

// TestSendContentType 验证请求显式携带 Content-Type（企微强校验该字段）
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

// TestSendTextWithMentions 验证 @ 提及映射为 mentioned_list（@ 全员为保留字 @all）
func TestSendTextWithMentions(t *testing.T) {
	var payload map[string]any
	var gotPath, gotQuery string
	adapter := newTestAdapter(t, captureRequest(t, &gotPath, &gotQuery, &payload, http.StatusOK, `{"errcode":0}`))

	msg := gobot.Text("磁盘清理完成，请相关同学确认").AtUsers("zhangsan", "lisi")
	if _, err := adapter.Send(context.Background(), gobot.Chat(""), msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	textField, _ := payload["text"].(map[string]any)
	mentioned, _ := textField["mentioned_list"].([]any)
	if len(mentioned) != 2 || mentioned[0] != "zhangsan" || mentioned[1] != "lisi" {
		t.Fatalf("mentioned_list = %v, 期望按 AtUserIDs 携带", mentioned)
	}
}

// TestSendTextMentionAll 验证 @ 全员映射为 mentioned_list 中的 @all 保留字
func TestSendTextMentionAll(t *testing.T) {
	var payload map[string]any
	var gotPath, gotQuery string
	adapter := newTestAdapter(t, captureRequest(t, &gotPath, &gotQuery, &payload, http.StatusOK, `{"errcode":0}`))

	if _, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Text("紧急告警").AtAll()); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	textField, _ := payload["text"].(map[string]any)
	mentioned, _ := textField["mentioned_list"].([]any)
	if len(mentioned) != 1 || mentioned[0] != "@all" {
		t.Fatalf("mentioned_list = %v, 期望 [@all]", mentioned)
	}
}

// TestSendMarkdownSuccess 验证 markdown 消息降级组装
func TestSendMarkdownSuccess(t *testing.T) {
	var payload map[string]any
	var gotPath, gotQuery string
	adapter := newTestAdapter(t, captureRequest(t, &gotPath, &gotQuery, &payload, http.StatusOK, `{"errcode":0}`))

	content := "### 告警 <prod>\n| 指标 | 当前值 |\n| --- | ---: |\n| CPU | 92% |"
	if _, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Markdown("告警", content)); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if payload["msgtype"] != "markdown" {
		t.Fatalf("msgtype = %v, 期望 markdown", payload["msgtype"])
	}
	md, _ := payload["markdown"].(map[string]any)
	got := md["content"].(string)
	if !strings.Contains(got, "**告警 <prod>**") && !strings.Contains(got, "告警 <prod>") {
		t.Fatalf("content = %q, 期望保留标题行", got)
	}
	if strings.Contains(got, "|") || strings.Contains(got, "---:") {
		t.Fatalf("content = %q, 期望表格降级为对齐行（不含竖线与分隔行）", got)
	}
	if !strings.Contains(got, "CPU") || !strings.Contains(got, "92%") {
		t.Fatalf("content = %q, 期望保留表格单元格内容", got)
	}
}

// TestSendMarkdownMentionRejected 验证 markdown 类型不支持 @ 提及
func TestSendMarkdownMentionRejected(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("携带提及的 markdown 消息不应发出请求")
	})
	_, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Markdown("告警", "**CPU 92%**").AtAll())
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindValidation {
		t.Fatalf("Send() 错误 = %v, 期望 validation", err)
	}
}

// TestSendImageSuccess 验证图片消息下载转 base64+md5 投递
func TestSendImageSuccess(t *testing.T) {
	png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0xFF, 0xEE, 0x11}
	mux := http.NewServeMux()
	mux.HandleFunc("/img", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(png)
	})
	var payload map[string]any
	var gotPath, gotQuery string
	mux.HandleFunc(webhookPath, captureRequest(t, &gotPath, &gotQuery, &payload, http.StatusOK, `{"errcode":0}`))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	adapter, err := New(Config{Key: "test-key", APIBase: srv.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Image(srv.URL+"/img")); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if payload["msgtype"] != "image" {
		t.Fatalf("msgtype = %v, 期望 image", payload["msgtype"])
	}
	image, _ := payload["image"].(map[string]any)
	if image["base64"] != base64.StdEncoding.EncodeToString(png) {
		t.Fatalf("base64 = %v, 期望图片内容编码", image["base64"])
	}
	if len(image["md5"].(string)) != 32 {
		t.Fatalf("md5 = %v, 期望 32 位十六进制摘要", image["md5"])
	}
}

// TestSendImageUnreachable 验证图片地址不可达时返回校验错误
func TestSendImageUnreachable(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("图片下载失败不应发出推送请求")
	})
	_, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Image("https://127.0.0.1:1/img.png"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindTransport {
		t.Fatalf("Send() 错误 = %v, 期望 transport", err)
	}
}

// TestSendImageHTTPError 验证图片地址返回非 2xx 时返回校验错误
func TestSendImageHTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/img", func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	})
	mux.HandleFunc(webhookPath, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("图片地址无效不应发出推送请求")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	adapter, err := New(Config{Key: "test-key", APIBase: srv.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = adapter.Send(context.Background(), gobot.Chat(""), gobot.Image(srv.URL+"/img"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindValidation {
		t.Fatalf("Send() 错误 = %v, 期望 validation", err)
	}
}

// TestSendImageOversize 验证图片 base64 编码超 2 MB 时本地拒绝
func TestSendImageOversize(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/img", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, 2<<20)) // 2 MB 原文，base64 后约 2.8 MB
	})
	mux.HandleFunc(webhookPath, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("超限图片不应发出推送请求")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	adapter, err := New(Config{Key: "test-key", APIBase: srv.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = adapter.Send(context.Background(), gobot.Chat(""), gobot.Image(srv.URL+"/img"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindValidation {
		t.Fatalf("Send() 错误 = %v, 期望 validation", err)
	}
}

// TestSendTextOversize 验证 text 正文超 2048 字节时本地拒绝
func TestSendTextOversize(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("超限文本不应发出请求")
	})
	_, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Text(strings.Repeat("告", 1025)))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindValidation {
		t.Fatalf("Send() 错误 = %v, 期望 validation", err)
	}
}

// TestSendMarkdownOversize 验证 markdown 正文超 4096 字节时本地拒绝
func TestSendMarkdownOversize(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		t.Error("超限 markdown 不应发出请求")
	})
	_, err := adapter.Send(context.Background(), gobot.Chat(""), gobot.Markdown("告警", strings.Repeat("a", 4097)))
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
		_, _ = w.Write([]byte(`{"errcode":93000,"errmsg":"invalid webhook key"}`))
	})
	_, err := adapter.Send(context.Background(), gobot.Chat("1"), gobot.Text("hi"))
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindPlatform || e.Code != 93000 {
		t.Fatalf("Send() 错误 = %v, 期望 platform/93000", err)
	}
	if e.Retryable {
		t.Fatal("业务码失败应不可重试")
	}
	if e.Message != "invalid webhook key" {
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
	adapter, err := New(Config{Key: "k", APIBase: "http://127.0.0.1:1"})
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

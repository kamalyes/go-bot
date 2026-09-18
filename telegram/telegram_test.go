/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 11:27:15
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 19:56:22
* @FilePath: \go-bot\telegram\telegram_test.go
* @Description: Telegram 适配器构造与配置测试
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package telegram

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gobot "github.com/kamalyes/go-bot"
	"github.com/kamalyes/go-toolbox/pkg/httpx"
)

// newTestAdapter 基于 httptest 服务器构造适配器，APIBase 指向测试服务器
func newTestAdapter(t *testing.T, handler http.HandlerFunc) *Adapter {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	adapter, err := New(Config{Token: "test-token", APIBase: srv.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return adapter
}

// TestNewTokenRequired 验证 Token 缺失时返回校验错误
func TestNewTokenRequired(t *testing.T) {
	for _, token := range []string{"", "   "} {
		_, err := New(Config{Token: token})
		if err == nil {
			t.Fatalf("New(token=%q) 期望返回错误", token)
		}
		var e *gobot.Error
		if !errors.As(err, &e) || e.Kind != gobot.KindValidation {
			t.Fatalf("New(token=%q) 错误类型 = %T, 期望 validation", token, err)
		}
	}
}

// TestNewDefaults 验证缺省值：官方 API 地址、HTML 渲染、自动建客户端
func TestNewDefaults(t *testing.T) {
	adapter, err := New(Config{Token: "123:ABC"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if adapter.cfg.APIBase != DefaultAPIBase {
		t.Fatalf("APIBase = %q, 期望 %q", adapter.cfg.APIBase, DefaultAPIBase)
	}
	if adapter.cfg.ParseMode != DefaultParseMode {
		t.Fatalf("ParseMode = %q, 期望 %q", adapter.cfg.ParseMode, DefaultParseMode)
	}
	if adapter.client == nil {
		t.Fatal("client 未自动构造")
	}
	if got := adapter.ListChats(); len(got) != 0 {
		t.Fatalf("ListChats() = %+v, 期望初始为空", got)
	}
}

// TestNewTrailingSlashTrimmed 验证 APIBase 尾部斜杠被修剪
func TestNewTrailingSlashTrimmed(t *testing.T) {
	adapter, err := New(Config{Token: "t", APIBase: "https://example.com///"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if adapter.cfg.APIBase != "https://example.com" {
		t.Fatalf("APIBase = %q, 期望修剪为 https://example.com", adapter.cfg.APIBase)
	}
}

// TestNewParseModeNone 验证 none 语义化为纯文本（不向平台传 parse_mode）
func TestNewParseModeNone(t *testing.T) {
	adapter, err := New(Config{Token: "t", ParseMode: "none"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if adapter.cfg.ParseMode != "" {
		t.Fatalf("ParseMode = %q, 期望 none 折叠为空串", adapter.cfg.ParseMode)
	}
}

// TestNewCustomParseMode 验证自定义渲染格式原样保留
func TestNewCustomParseMode(t *testing.T) {
	adapter, err := New(Config{Token: "t", ParseMode: "MarkdownV2"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if adapter.cfg.ParseMode != "MarkdownV2" {
		t.Fatalf("ParseMode = %q, 期望 MarkdownV2", adapter.cfg.ParseMode)
	}
}

// TestNewCustomHTTPClient 验证自定义客户端被复用而非重建
func TestNewCustomHTTPClient(t *testing.T) {
	client := httpx.NewClient(httpx.WithTimeout(3 * time.Second))
	adapter, err := New(Config{Token: "t", HTTPClient: client})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if adapter.client != client {
		t.Fatal("自定义 HTTPClient 应被原样复用")
	}
}

// TestPlatform 验证平台标识
func TestPlatform(t *testing.T) {
	adapter, err := New(Config{Token: "t"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if got := adapter.Platform(); got != gobot.PlatformTelegram {
		t.Fatalf("Platform() = %q, 期望 %q", got, gobot.PlatformTelegram)
	}
}

// TestClose 验证 Close 无副作用
func TestClose(t *testing.T) {
	adapter, err := New(Config{Token: "t"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := adapter.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

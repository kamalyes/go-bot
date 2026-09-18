/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-19 10:02:37
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-19 10:28:53
* @FilePath: \go-bot\serverchan\serverchan_test.go
* @Description: serverchan 适配器构造与配置测试
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package serverchan

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	gobot "github.com/kamalyes/go-bot"
)

// newTestAdapter 基于 httptest 服务器构造适配器，APIBase 指向测试服务器
func newTestAdapter(t *testing.T, handler http.HandlerFunc) *Adapter {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	adapter, err := New(Config{SendKey: "test-key", APIBase: srv.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return adapter
}

// TestNewSendKeyRequired 验证 SendKey 缺失时返回校验错误
func TestNewSendKeyRequired(t *testing.T) {
	for _, key := range []string{"", "   "} {
		if _, err := New(Config{SendKey: key}); err == nil {
			t.Fatalf("New(sendkey=%q) 期望返回错误", key)
		} else {
			var e *gobot.Error
			if !errors.As(err, &e) || e.Kind != gobot.KindValidation {
				t.Fatalf("New(sendkey=%q) 错误类型 = %T, 期望 validation", key, err)
			}
		}
	}
}

// TestNewDefaults 验证缺省值：APIBase 取官方地址、HTTPClient 自动构造
func TestNewDefaults(t *testing.T) {
	adapter, err := New(Config{SendKey: "SCTtest"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if adapter.cfg.APIBase != DefaultAPIBase {
		t.Fatalf("APIBase = %q, 期望 %q", adapter.cfg.APIBase, DefaultAPIBase)
	}
	if adapter.client == nil {
		t.Fatal("client 未自动构造")
	}
}

// TestNewTrailingSlash 验证 APIBase 尾部斜杠被修剪
func TestNewTrailingSlash(t *testing.T) {
	adapter, err := New(Config{SendKey: "SCTtest", APIBase: "https://example.com///"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if adapter.cfg.APIBase != "https://example.com" {
		t.Fatalf("APIBase = %q, 期望修剪为 https://example.com", adapter.cfg.APIBase)
	}
}

// TestPlatform 验证平台标识
func TestPlatform(t *testing.T) {
	adapter, err := New(Config{SendKey: "SCTtest"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if got := adapter.Platform(); got != gobot.PlatformServerchan {
		t.Fatalf("Platform() = %q, 期望 %q", got, gobot.PlatformServerchan)
	}
}

// TestClose 验证 Close 无副作用
func TestClose(t *testing.T) {
	adapter, err := New(Config{SendKey: "SCTtest"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := adapter.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

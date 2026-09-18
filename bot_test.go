/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 22:19:21
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 22:19:21
* @FilePath: \go-bot\bot_test.go
* @Description: Bot 门面与 Builder 测试：装配缺省值、校验短路、统计事件与共享开关
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// closeTrackingAdapter 在 fakeAdapter 基础上跟踪 Close 是否被调用
type closeTrackingAdapter struct {
	fakeAdapter
	closed atomic.Bool
}

// Close 覆写 fakeAdapter 的 Close，记录调用
func (c *closeTrackingAdapter) Close(context.Context) error {
	c.closed.Store(true)
	return nil
}

// TestBuildRequiresAdapter 验证缺失 adapter 时 Build 拒绝
func TestBuildRequiresAdapter(t *testing.T) {
	if _, err := NewBot(nil).Build(); err == nil {
		t.Fatal("NewBot(nil).Build() 期望返回错误")
	} else {
		var e *Error
		if !errors.As(err, &e) || e.Kind != KindValidation {
			t.Fatalf("Build() 错误 = %v, 期望 validation", err)
		}
	}
}

// TestBuildDefaults 验证缺省装配：botID 取平台名、统计可用、常量暴露一致
func TestBuildDefaults(t *testing.T) {
	bot := newTestBot(t, PlatformTelegram, okSend)

	if got := bot.Platform(); got != PlatformTelegram {
		t.Fatalf("Platform() = %q, 期望 telegram", got)
	}
	if got := bot.Name(); got != string(PlatformTelegram) {
		t.Fatalf("Name() = %q, 期望缺省取平台名", got)
	}
	if bot.Stats() == nil {
		t.Fatal("Stats() 应始终可用")
	}
	if got := bot.Stats().TotalSent() + bot.Stats().TotalError() + bot.Stats().TotalMuted(); got != 0 {
		t.Fatalf("初始计数应全为零, got %d", got)
	}
	if GetDefaultHTTPTimeout() != DefaultHTTPTimeout {
		t.Fatalf("GetDefaultHTTPTimeout() = %v, 期望 %v", GetDefaultHTTPTimeout(), DefaultHTTPTimeout)
	}
}

// TestBuildWithName 验证自定义 Bot 标识优先于平台名
func TestBuildWithName(t *testing.T) {
	bot, err := NewBot(&fakeAdapter{platform: PlatformLark, send: okSend}).
		WithName("order-ops-prod").
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got := bot.Name(); got != "order-ops-prod" {
		t.Fatalf("Name() = %q, 期望 order-ops-prod", got)
	}
}

// TestBotCloseDelegates 验证 Close 委托给底层 adapter
func TestBotCloseDelegates(t *testing.T) {
	adapter := &closeTrackingAdapter{fakeAdapter: fakeAdapter{platform: PlatformTelegram, send: okSend}}
	bot, err := NewBot(adapter).Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if err := bot.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !adapter.closed.Load() {
		t.Fatal("Close() 应传播到底层 adapter")
	}
}

// TestBotSendConvenienceMethods 验证便捷发送方法的消息形态
func TestBotSendConvenienceMethods(t *testing.T) {
	var gotType MessageType
	var gotTitle, gotText, gotURL string
	bot := newTestBot(t, PlatformTelegram, func(_ context.Context, _ Target, msg *Message) (*SendResult, error) {
		gotType, gotTitle, gotText, gotURL = msg.Type, msg.Title, msg.Text, msg.ImageURL
		return &SendResult{MessageID: "1"}, nil
	})

	if _, err := bot.SendMarkdown(context.Background(), "告警", "**CPU 92%**", Chat("g")); err != nil {
		t.Fatalf("SendMarkdown: error = %v", err)
	}
	if gotType != MsgTypeMarkdown || gotTitle != "告警" || gotText != "**CPU 92%**" {
		t.Fatalf("SendMarkdown 消息形态 = %s/%q/%q", gotType, gotTitle, gotText)
	}

	if _, err := bot.SendImage(context.Background(), "https://example.com/p.png", Chat("g")); err != nil {
		t.Fatalf("SendImage: error = %v", err)
	}
	if gotType != MsgTypeImage || gotURL != "https://example.com/p.png" {
		t.Fatalf("SendImage 消息形态 = %s/%q", gotType, gotURL)
	}
}

// TestBotSendValidatesTargetLocally 验证非法目标在校验阶段短路且计入错误
func TestBotSendValidatesTargetLocally(t *testing.T) {
	bot := newTestBot(t, PlatformTelegram, func(context.Context, Target, *Message) (*SendResult, error) {
		t.Error("校验失败不应触达 adapter")
		return nil, nil
	})

	_, err := bot.Send(context.Background(), Text("hi"), Target{Type: TargetChat})
	if err == nil {
		t.Fatal("空 ID 目标应返回错误")
	}
	var e *Error
	if !errors.As(err, &e) || e.Kind != KindValidation {
		t.Fatalf("错误 = %v, 期望 validation", err)
	}
	if got := bot.Stats().TotalError(); got != 1 {
		t.Fatalf("TotalError() = %d, 期望 1", got)
	}
	if got := bot.Stats().TotalSent(); got != 0 {
		t.Fatalf("TotalSent() = %d, 期望 0", got)
	}
}

// TestBotSendValidatesMessageLocally 验证空正文消息被本地拒绝
func TestBotSendValidatesMessageLocally(t *testing.T) {
	bot := newTestBot(t, PlatformLark, func(context.Context, Target, *Message) (*SendResult, error) {
		t.Error("校验失败不应触达 adapter")
		return nil, nil
	})

	_, err := bot.Send(context.Background(), Text(""), Chat("a"))
	var e *Error
	if !errors.As(err, &e) || e.Kind != KindValidation {
		t.Fatalf("错误 = %v, 期望 validation", err)
	}
}

// TestBotSendRecordsSuccessStat 验证成功发送的统计事件字段
func TestBotSendRecordsSuccessStat(t *testing.T) {
	var m captureMetrics
	bot, err := NewBot(&fakeAdapter{platform: PlatformTelegram, send: okSend}).
		WithName("metrics-bot").
		WithMetrics(&m).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	text := "部署完成 ✅"
	if _, err := bot.SendText(context.Background(), text, User("42")); err != nil {
		t.Fatalf("SendText: error = %v", err)
	}
	stats := m.snapshot()
	if len(stats) != 1 {
		t.Fatalf("收集 %d 条统计, 期望 1 条", len(stats))
	}
	stat := stats[0]
	if !stat.Success || stat.ErrorKind != "" || stat.ErrorCode != "" {
		t.Fatalf("成功事件 = %+v, 期望无错误字段", stat)
	}
	if stat.Platform != string(PlatformTelegram) || stat.BotID != "metrics-bot" || stat.TargetID != "42" {
		t.Fatalf("维度字段 = %s/%s/%s", stat.Platform, stat.BotID, stat.TargetID)
	}
	if stat.MsgType != string(MsgTypeText) || stat.ContentBytes != int64(len(text)) {
		t.Fatalf("消息字段 = %s/%d", stat.MsgType, stat.ContentBytes)
	}
	if stat.TS.IsZero() || stat.Latency < 0 {
		t.Fatalf("时间字段 = %v/%v", stat.TS, stat.Latency)
	}
}

// TestBotSendRecordsErrorStat 验证平台失败事件的错误维度
func TestBotSendRecordsErrorStat(t *testing.T) {
	var m captureMetrics
	bot, err := NewBot(&fakeAdapter{platform: PlatformLark, send: func(_ context.Context, _ Target, _ *Message) (*SendResult, error) {
		return nil, NewPlatformError(PlatformLark, "Send", 11232, "too many request", true)
	}}).WithMetrics(&m).Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if _, err := bot.SendText(context.Background(), "hi", Chat("c")); err == nil {
		t.Fatal("期望返回平台错误")
	}
	stats := m.snapshot()
	if len(stats) != 1 || stats[0].Success {
		t.Fatalf("失败事件 = %+v, 期望 Success=false", stats)
	}
	if stats[0].ErrorKind != string(KindPlatform) || stats[0].ErrorCode != "11232" {
		t.Fatalf("错误维度 = %s/%s, 期望 platform/11232", stats[0].ErrorKind, stats[0].ErrorCode)
	}
}

// TestBotWithRetryPolicy 验证重试策略接入 Bot 流水线
func TestBotWithRetryPolicy(t *testing.T) {
	var calls atomic.Int64
	bot, err := NewBot(&fakeAdapter{platform: PlatformTelegram, send: func(_ context.Context, _ Target, _ *Message) (*SendResult, error) {
		if calls.Add(1) == 1 {
			return nil, NewPlatformError(PlatformTelegram, "Send", 429, "rate limited", true)
		}
		return &SendResult{MessageID: "after-retry"}, nil
	}}).WithRetry(RetryPolicy{
		MaxRetries:   2,
		InitialDelay: time.Millisecond,
		MaxDelay:     2 * time.Millisecond,
	}).Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	results, err := bot.Send(context.Background(), Text("hi"), Chat("a"))
	if err != nil {
		t.Fatalf("Send: error = %v, 期望重试后成功", err)
	}
	if len(results) != 1 || results[0] == nil || results[0].MessageID != "after-retry" {
		t.Fatalf("Send: results = %+v, 期望 after-retry", results)
	}
	if calls.Load() != 2 {
		t.Fatalf("adapter 调用 %d 次, 期望 2 次", calls.Load())
	}
	// 重试多次但按发送任务计一次成功
	if got := bot.Stats().TotalSent(); got != 1 {
		t.Fatalf("TotalSent() = %d, 期望 1", got)
	}
}

// TestBotSharedSwitchMutesBoth 验证多个 Bot 共享开关联动静音
func TestBotSharedSwitchMutesBoth(t *testing.T) {
	sw := NewSwitch()
	mk := func() *Bot {
		bot, err := NewBot(&fakeAdapter{platform: PlatformLark, send: okSend}).WithSwitch(sw).Build()
		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		return bot
	}
	botA, botB := mk(), mk()
	sw.Disable()

	for i, bot := range []*Bot{botA, botB} {
		results, err := bot.Send(context.Background(), Text("hi"), Chat("x"))
		if err != nil {
			t.Fatalf("bot[%d] 静音发送: error = %v, 期望 nil", i, err)
		}
		if results[0] != nil {
			t.Fatalf("bot[%d] 静音结果应为 nil", i)
		}
		if got := bot.Stats().TotalMuted(); got != 1 {
			t.Fatalf("bot[%d] TotalMuted() = %d, 期望 1", i, got)
		}
	}
}

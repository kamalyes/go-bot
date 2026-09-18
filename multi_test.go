/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 21:12:26
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 21:31:52
* @FilePath: \go-bot\multi_test.go
* @Description: Bot/Multi 的 Send 多目标发送测试
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

// fakeAdapter 是根包测试用的最小 Adapter 实现，send 行为按用例注入
type fakeAdapter struct {
	platform Platform
	send     func(ctx context.Context, target Target, msg *Message) (*SendResult, error)
}

func (f *fakeAdapter) Platform() Platform { return f.platform }

func (f *fakeAdapter) Send(ctx context.Context, target Target, msg *Message) (*SendResult, error) {
	return f.send(ctx, target, msg)
}

func (f *fakeAdapter) Close(context.Context) error { return nil }

// newTestBot 构造一个挂载 fakeAdapter 的 Bot
func newTestBot(t *testing.T, platform Platform, send func(context.Context, Target, *Message) (*SendResult, error)) *Bot {
	t.Helper()
	bot, err := NewBot(&fakeAdapter{platform: platform, send: send}).Build()
	if err != nil {
		t.Fatalf("newTestBot: Build() error = %v", err)
	}
	return bot
}

// okSend 返回成功且携带目标标识的结果
func okSend(_ context.Context, target Target, _ *Message) (*SendResult, error) {
	return &SendResult{MessageID: "id-" + target.ID}, nil
}

func TestSendRequiresTargets(t *testing.T) {
	bot := newTestBot(t, PlatformTelegram, okSend)

	_, err := bot.Send(context.Background(), Text("hi"))
	if err == nil {
		t.Fatal("Send with no targets: error = nil, want validation error")
	}
	var e *Error
	if !errors.As(err, &e) || e.Kind != KindValidation {
		t.Fatalf("Send with no targets: error = %v, want KindValidation", err)
	}
}

func TestSendSingleTargetFastPath(t *testing.T) {
	bot := newTestBot(t, PlatformTelegram, okSend)

	results, err := bot.Send(context.Background(), Text("hi"), Chat("a"))
	if err != nil {
		t.Fatalf("Send: error = %v", err)
	}
	if len(results) != 1 || results[0] == nil || results[0].MessageID != "id-a" {
		t.Fatalf("Send: results = %+v, want single MessageID id-a", results)
	}
	if got := bot.Stats().TotalSent(); got != 1 {
		t.Fatalf("Send: TotalSent() = %d, want 1", got)
	}
}

func TestSendTextConvenience(t *testing.T) {
	bot := newTestBot(t, PlatformLark, okSend)

	results, err := bot.SendText(context.Background(), "hi", Chat("a"), Chat("b"))
	if err != nil {
		t.Fatalf("SendText: error = %v", err)
	}
	if len(results) != 2 || results[1].MessageID != "id-b" {
		t.Fatalf("SendText: results = %+v, want MessageID id-b at [1]", results)
	}
}

func TestSendSendsAllTargetsInOrder(t *testing.T) {
	bot := newTestBot(t, PlatformTelegram, okSend)
	targets := []Target{Chat("a"), Chat("b"), Chat("c")}

	results, err := bot.Send(context.Background(), Text("hi"), targets...)
	if err != nil {
		t.Fatalf("Send: error = %v", err)
	}
	if len(results) != len(targets) {
		t.Fatalf("Send: len(results) = %d, want %d", len(results), len(targets))
	}
	for i, target := range targets {
		if results[i] == nil || results[i].MessageID != "id-"+target.ID {
			t.Fatalf("Send: results[%d] = %+v, want MessageID id-%s", i, results[i], target.ID)
		}
	}
	if got := bot.Stats().TotalSent(); got != int64(len(targets)) {
		t.Fatalf("Send: TotalSent() = %d, want %d", got, len(targets))
	}
}

func TestSendAggregatesErrors(t *testing.T) {
	bot := newTestBot(t, PlatformTelegram, func(_ context.Context, target Target, _ *Message) (*SendResult, error) {
		if target.ID == "bad" {
			return nil, NewPlatformError(PlatformTelegram, "Send", 11232, "rate limited", true)
		}
		return okSend(nil, target, nil)
	})
	targets := []Target{Chat("ok1"), Chat("bad"), Chat("ok2")}

	results, err := bot.Send(context.Background(), Text("hi"), targets...)
	if err == nil {
		t.Fatal("Send: error = nil, want aggregated error")
	}
	if results[1] != nil {
		t.Fatalf("Send: results[1] = %+v, want nil", results[1])
	}
	var e *Error
	if !errors.As(err, &e) || e.Kind != KindPlatform {
		t.Fatalf("Send: error = %v, want KindPlatform", err)
	}
	if got := bot.Stats().TotalSent(); got != 2 {
		t.Fatalf("Send: TotalSent() = %d, want 2", got)
	}
	if got := bot.Stats().TotalError(); got != 1 {
		t.Fatalf("Send: TotalError() = %d, want 1", got)
	}
}

func TestSendLimitsConcurrency(t *testing.T) {
	var inFlight, peak atomic.Int64
	bot := newTestBot(t, PlatformTelegram, func(_ context.Context, target Target, _ *Message) (*SendResult, error) {
		cur := inFlight.Add(1)
		for {
			old := peak.Load()
			if cur <= old || peak.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		inFlight.Add(-1)
		return &SendResult{MessageID: target.ID}, nil
	})
	targets := make([]Target, 20)
	for i := range targets {
		targets[i] = Chat("t")
	}

	if _, err := bot.Send(context.Background(), Text("hi"), targets...); err != nil {
		t.Fatalf("Send: error = %v", err)
	}
	if got := peak.Load(); got > DefaultBatchConcurrency {
		t.Fatalf("Send: peak in-flight = %d, want <= %d", got, DefaultBatchConcurrency)
	}
}

func TestSendMutedDropsAll(t *testing.T) {
	sw := NewSwitch()
	bot, err := NewBot(&fakeAdapter{platform: PlatformLark, send: okSend}).WithSwitch(sw).Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	sw.Disable()

	results, sendErr := bot.Send(context.Background(), Text("hi"), Chat("a"), Chat("b"), Chat("c"))
	if sendErr != nil {
		t.Fatalf("Send muted: error = %v, want nil", sendErr)
	}
	for i, res := range results {
		if res != nil {
			t.Fatalf("Send muted: results[%d] = %+v, want nil", i, res)
		}
	}
	if got := bot.Stats().TotalMuted(); got != 3 {
		t.Fatalf("Send muted: TotalMuted() = %d, want 3", got)
	}
}

func TestMultiSendFanout(t *testing.T) {
	tgBot := newTestBot(t, PlatformTelegram, okSend)
	lkBot := newTestBot(t, PlatformLark, okSend)
	multi, err := NewMulti(tgBot, lkBot)
	if err != nil {
		t.Fatalf("NewMulti() error = %v", err)
	}
	targets := []Target{Chat("a"), Chat("b")}

	results, err := multi.Send(context.Background(), Text("hi"), targets...)
	if err != nil {
		t.Fatalf("Multi.Send: error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Multi.Send: len(results) = %d, want 2", len(results))
	}
	for bi, row := range results {
		if len(row) != len(targets) {
			t.Fatalf("Multi.Send: len(results[%d]) = %d, want %d", bi, len(row), len(targets))
		}
		for ti, target := range targets {
			if row[ti] == nil || row[ti].MessageID != "id-"+target.ID {
				t.Fatalf("Multi.Send: results[%d][%d] = %+v, want MessageID id-%s", bi, ti, row[ti], target.ID)
			}
		}
	}
}

func TestMultiSendSingleBotFastPath(t *testing.T) {
	bot := newTestBot(t, PlatformTelegram, okSend)
	multi, err := NewMulti(bot)
	if err != nil {
		t.Fatalf("NewMulti() error = %v", err)
	}

	results, err := multi.Send(context.Background(), Text("hi"), Chat("a"))
	if err != nil {
		t.Fatalf("Multi.Send: error = %v", err)
	}
	if len(results) != 1 || len(results[0]) != 1 || results[0][0].MessageID != "id-a" {
		t.Fatalf("Multi.Send: results = %+v, want single id-a", results)
	}
}

func TestMultiSendPropagatesValidation(t *testing.T) {
	multi, err := NewMulti(newTestBot(t, PlatformTelegram, okSend))
	if err != nil {
		t.Fatalf("NewMulti() error = %v", err)
	}

	_, err = multi.Send(context.Background(), Text("hi"))
	if err == nil {
		t.Fatal("Multi.Send with no targets: error = nil, want validation error")
	}
	var e *Error
	if !errors.As(err, &e) || e.Kind != KindValidation {
		t.Fatalf("Multi.Send with no targets: error = %v, want KindValidation", err)
	}
}

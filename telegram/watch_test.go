/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-10-20 10:10:26
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-10-20 11:02:53
* @FilePath: \go-bot\telegram\watch_test.go
* @Description: Telegram 常驻事件循环测试：起点探测、游标推进、退避重试与停机
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package telegram

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	gobot "github.com/kamalyes/go-bot"
)

// eventServer 记录每次 getUpdates 的请求游标与 handler 收到的事件批次，
// 按「成功请求序号」返回预设响应，供用例断言循环行为
type eventServer struct {
	mu        sync.Mutex
	t         *testing.T
	offsets   []float64
	batches   [][]Event
	failNext  int
	responses [][]string
}

// newEventServer 创建事件测试服务器
func newEventServer(t *testing.T, responses [][]string) *eventServer {
	t.Helper()
	return &eventServer{t: t, responses: responses}
}

// serve 是 http.HandlerFunc：failNext 耗尽前返回 409 平台错误，
// 否则记录游标并按成功请求序号返回预设批次
func (s *eventServer) serve(w http.ResponseWriter, req *http.Request) {
	s.mu.Lock()
	if s.failNext > 0 {
		s.failNext--
		s.mu.Unlock()
		_, _ = w.Write([]byte(`{"ok":false,"error_code":409,"description":"Conflict"}`))
		return
	}
	payload := decodeJSON(s.t, readAll(s.t, req))
	if offset, ok := payload["offset"].(float64); ok {
		s.offsets = append(s.offsets, offset)
	}
	index := len(s.offsets) - 1
	s.mu.Unlock()

	body := `{"ok":true,"result":[]}`
	if index >= 0 && index < len(s.responses) {
		body = `{"ok":true,"result":[` + joinStrings(s.responses[index]) + `]}`
	}
	_, _ = w.Write([]byte(body))
}

// handler 返回记录批次的 EventHandler
func (s *eventServer) handler() EventHandler {
	return func(_ context.Context, events []Event) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.batches = append(s.batches, events)
		return nil
	}
}

// batchCount 返回已记录的批次数
func (s *eventServer) batchCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.batches)
}

// joinStrings 用逗号拼接字符串切片
func joinStrings(items []string) string {
	joined := ""
	for i, item := range items {
		if i > 0 {
			joined += ","
		}
		joined += item
	}
	return joined
}

// runLoop 在后台起事件循环，返回停止函数
func runLoop(t *testing.T, adapter *Adapter, opts EventLoopOptions, handler EventHandler) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := adapter.RunEventLoop(ctx, opts, handler); err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("RunEventLoop() error = %v", err)
		}
	}()
	return cancel
}

// TestRunEventLoopHandlerRequired 验证 handler 缺失时返回校验错误
func TestRunEventLoopHandlerRequired(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
	})
	err := adapter.RunEventLoop(context.Background(), EventLoopOptions{}, nil)
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindValidation {
		t.Fatalf("RunEventLoop() 错误 = %v, 期望 validation", err)
	}
}

// TestRunEventLoopSkipsBacklog 验证零值起点先探测最新事件跳过积压：
// 探测请求携带 offset=-1，消费从最新事件之后开始
func TestRunEventLoopSkipsBacklog(t *testing.T) {
	server := newEventServer(t, [][]string{
		{`{"update_id":5000,"message":{"message_id":9,"chat":{"id":555,"type":"private"},"text":"旧事件","date":1758265000}}`},
		{`{"update_id":5001,"my_chat_member":{"chat":{"id":-100777,"type":"supergroup","title":"告警群"},"date":1758265001,` +
			`"old_chat_member":{"status":"left","user":{"id":999}},` +
			`"new_chat_member":{"status":"member","user":{"id":999}}}}`},
	})
	adapter := newTestAdapter(t, server.serve)

	stop := runLoop(t, adapter, EventLoopOptions{Backoff: 10 * time.Millisecond}, server.handler())
	waitFor(t, func() bool { return server.batchCount() >= 1 })
	stop()

	server.mu.Lock()
	defer server.mu.Unlock()
	if len(server.offsets) < 2 || server.offsets[0] != -1 {
		t.Fatalf("探测游标 = %v, 期望首个请求 offset=-1", server.offsets)
	}
	if server.offsets[1] != 5001 {
		t.Fatalf("消费游标 = %v, 期望探测后从 5001 开始", server.offsets)
	}
}

// TestRunEventLoopExplicitStartOffset 验证显式游标跳过探测直接消费
func TestRunEventLoopExplicitStartOffset(t *testing.T) {
	server := newEventServer(t, [][]string{
		{`{"update_id":5001,"message":{"message_id":10,"chat":{"id":556,"type":"group","title":"群"},"text":"新事件","date":1758265002}}`},
	})
	adapter := newTestAdapter(t, server.serve)

	stop := runLoop(t, adapter, EventLoopOptions{StartOffset: 5001, Backoff: 10 * time.Millisecond}, server.handler())
	waitFor(t, func() bool { return server.batchCount() >= 1 })
	stop()

	server.mu.Lock()
	defer server.mu.Unlock()
	if len(server.offsets) == 0 || server.offsets[0] != 5001 {
		t.Fatalf("消费游标 = %v, 期望直接从 5001 开始且无探测请求", server.offsets)
	}
}

// TestRunEventLoopHandlerErrorDoesNotStop 验证 handler 失败仅记日志，
// 循环继续消费后续批次并推进游标
func TestRunEventLoopHandlerErrorDoesNotStop(t *testing.T) {
	server := newEventServer(t, [][]string{
		{`{"update_id":6001,"message":{"message_id":11,"chat":{"id":557,"type":"private"},"text":"a","date":1758265003}}`},
		{`{"update_id":6002,"message":{"message_id":12,"chat":{"id":557,"type":"private"},"text":"b","date":1758265004}}`},
	})
	adapter := newTestAdapter(t, server.serve)

	failed := make(chan struct{})
	recovered := make(chan struct{})
	stop := runLoop(t, adapter, EventLoopOptions{StartOffset: 6001, Backoff: 10 * time.Millisecond}, func(ctx context.Context, events []Event) error {
		_ = server.handler()(ctx, events)
		select {
		case <-failed:
			close(recovered)
		default:
			close(failed)
			return errors.New("handler boom")
		}
		return nil
	})
	defer stop()

	select {
	case <-recovered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler 失败后循环未继续消费")
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if len(server.offsets) < 2 || server.offsets[1] != 6002 {
		t.Fatalf("游标推进 = %v, 期望第二批从 6002 开始", server.offsets)
	}
}

// TestRunEventLoopRetriesAfterPollFailure 验证拉取失败退避后重试恢复消费
func TestRunEventLoopRetriesAfterPollFailure(t *testing.T) {
	server := newEventServer(t, [][]string{
		{`{"update_id":7001,"message":{"message_id":13,"chat":{"id":558,"type":"private"},"text":"recovered","date":1758265005}}`},
	})
	server.mu.Lock()
	server.failNext = 1
	server.mu.Unlock()
	adapter := newTestAdapter(t, server.serve)

	stop := runLoop(t, adapter, EventLoopOptions{StartOffset: 7001, Backoff: 10 * time.Millisecond}, server.handler())
	defer stop()
	waitFor(t, func() bool { return server.batchCount() >= 1 })

	server.mu.Lock()
	defer server.mu.Unlock()
	if len(server.batches) != 1 || len(server.batches[0]) != 1 || server.batches[0][0].EventID != 7001 {
		t.Fatalf("恢复后的事件 = %+v, 期望 7001", server.batches)
	}
}

// TestRunEventLoopStopsOnCancel 验证 ctx 取消后循环退出并返回 ctx.Err
func TestRunEventLoopStopsOnCancel(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- adapter.RunEventLoop(ctx, EventLoopOptions{StartOffset: 1}, func(_ context.Context, _ []Event) error {
			return nil
		})
	}()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RunEventLoop() 返回 = %v, 期望 context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ctx 取消后循环未退出")
	}
}

// waitFor 轮询断言条件成立，超时终止用例
func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("等待条件超时")
}

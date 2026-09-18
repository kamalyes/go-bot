/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-19 15:00:08
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-19 17:51:17
* @FilePath: \go-bot\queued_test.go
* @Description: QueuedBot 削峰填谷测试：入队拆分、节流消费与重投传导
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// memQueue 是内存 Queue 实现：Publish 后通知消费循环；
// handler 失败的任务重回队尾等待重投，失败次数达到上限后丢弃
type memQueue struct {
	mu          sync.Mutex
	pending     []*QueueTask
	attempts    map[string]int
	acked       map[string]bool
	dropped     []string
	maxAttempts int
	notify      chan struct{}
	publishErr  atomic.Value
}

func newMemQueue(maxAttempts int) *memQueue {
	return &memQueue{
		attempts:    make(map[string]int),
		acked:       make(map[string]bool),
		maxAttempts: maxAttempts,
		notify:      make(chan struct{}, 1),
	}
}

func (q *memQueue) Publish(_ context.Context, task *QueueTask) error {
	if err, ok := q.publishErr.Load().(error); ok && err != nil {
		return err
	}
	q.mu.Lock()
	q.pending = append(q.pending, task)
	q.mu.Unlock()
	select {
	case q.notify <- struct{}{}:
	default:
	}
	return nil
}

func (q *memQueue) Consume(ctx context.Context, handler QueueHandler) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-q.notify:
		}
		for {
			task := q.next()
			if task == nil {
				break
			}
			if err := handler(ctx, task); err != nil {
				q.requeue(task)
				continue
			}
			q.mu.Lock()
			q.acked[task.ID] = true
			q.mu.Unlock()
		}
	}
}

func (q *memQueue) next() *QueueTask {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) == 0 {
		return nil
	}
	task := q.pending[0]
	q.pending = q.pending[1:]
	return task
}

func (q *memQueue) requeue(task *QueueTask) {
	q.mu.Lock()
	q.attempts[task.ID]++
	exhausted := q.attempts[task.ID] >= q.maxAttempts
	if !exhausted {
		q.pending = append(q.pending, task)
	} else {
		q.dropped = append(q.dropped, task.ID)
	}
	q.mu.Unlock()
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

func (q *memQueue) Close(context.Context) error { return nil }

// recordedSend 构造记录每次调用的 adapter 发送函数，前 failFirst 次返回失败
func recordedSend(mu *sync.Mutex, calls *[]Target, failFirst *int32) func(context.Context, Target, *Message) (*SendResult, error) {
	return func(_ context.Context, target Target, _ *Message) (*SendResult, error) {
		n := atomic.AddInt32(failFirst, -1)
		if n >= 0 {
			return nil, NewPlatformError(PlatformTelegram, "Send", 429, "Too Many Requests", false)
		}
		mu.Lock()
		*calls = append(*calls, target)
		mu.Unlock()
		return &SendResult{MessageID: "id-" + target.ID}, nil
	}
}

// runQueuedBot 启动消费循环，返回停止函数
func runQueuedBot(t *testing.T, qb *QueuedBot) context.CancelFunc {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = qb.Run(ctx) }()
	t.Cleanup(cancel)
	return cancel
}

// TestQueuedBotBuildValidation 验证 bot / queue 缺失与 interval 缺省
func TestQueuedBotBuildValidation(t *testing.T) {
	bot := newTestBot(t, PlatformTelegram, okSend)
	if _, err := NewQueuedBot(nil, newMemQueue(3)).Build(); err == nil {
		t.Fatal("bot 缺失时 Build() 应返回错误")
	}
	if _, err := NewQueuedBot(bot, nil).Build(); err == nil {
		t.Fatal("queue 缺失时 Build() 应返回错误")
	}
	qb, err := NewQueuedBot(bot, newMemQueue(3)).Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if qb.interval != DefaultQueueInterval {
		t.Fatalf("interval = %v, 期望缺省 %v", qb.interval, DefaultQueueInterval)
	}
}

// TestQueuedBotSendSplitsTargets 验证一次 Send 的多目标按目标粒度拆分入队且 FIFO 保序
func TestQueuedBotSendSplitsTargets(t *testing.T) {
	var mu sync.Mutex
	var calls []Target
	var failFirst int32 = -1 // -1 表示全部成功
	bot := newTestBot(t, PlatformTelegram, recordedSend(&mu, &calls, &failFirst))
	q := newMemQueue(3)
	qb, err := NewQueuedBot(bot, q).Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	msg := Markdown("【生产发布通知】order-svc v2.6.0", "灰度 10% → 50% → 全量，P99 120ms")
	targets := []Target{Chat("-1001234567890"), Chat("-1009876543210"), User("123456789")}
	if _, err := qb.Send(context.Background(), msg, targets...); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	q.mu.Lock()
	enqueued := len(q.pending)
	q.mu.Unlock()
	if enqueued != 3 {
		t.Fatalf("入队任务数 = %d, 期望按目标拆分为 3", enqueued)
	}

	cancel := runQueuedBot(t, qb)
	deadline := time.Now().Add(DefaultQueueInterval * 10)
	for time.Now().Before(deadline) {
		q.mu.Lock()
		done := len(q.acked) == 3
		q.mu.Unlock()
		if done {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	cancel()

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 3 {
		t.Fatalf("adapter 调用数 = %d, 期望 3", len(calls))
	}
	want := []string{"-1001234567890", "-1009876543210", "123456789"}
	for i, id := range want {
		if calls[i].ID != id {
			t.Fatalf("第 %d 次投递目标 = %q, 期望 %q（FIFO 保序）", i, calls[i].ID, id)
		}
	}
}

// TestQueuedBotSendValidatesLocal 验证坏输入在入队前被本地拒绝，不产生任何任务
func TestQueuedBotSendValidatesLocal(t *testing.T) {
	bot := newTestBot(t, PlatformTelegram, okSend)
	q := newMemQueue(3)
	qb, _ := NewQueuedBot(bot, q).Build()

	cases := []struct {
		name    string
		msg     *Message
		targets []Target
		keyword string
	}{
		{"无目标", Text("hi"), nil, "targets are required"},
		{"空正文", Text(""), []Target{Chat("a")}, "empty"},
		{"坏消息类型", &Message{Type: MessageType("sticker")}, []Target{Chat("a")}, "unsupported"},
		{"空目标 ID", Text("hi"), []Target{Chat("")}, "target id is empty"},
	}
	for _, tc := range cases {
		_, err := qb.Send(context.Background(), tc.msg, tc.targets...)
		var e *Error
		if !errors.As(err, &e) || e.Kind != KindValidation {
			t.Fatalf("%s: 错误 = %v, 期望 validation", tc.name, err)
		}
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) != 0 {
		t.Fatalf("坏输入不应入队，队列长度 = %d", len(q.pending))
	}
}

// TestQueuedBotRunRedelivers 验证投递失败的任务被队列重投，最终成功投递
func TestQueuedBotRunRedelivers(t *testing.T) {
	var mu sync.Mutex
	var calls []Target
	var failFirst int32 = 1 // 首次失败，重投成功
	bot := newTestBot(t, PlatformTelegram, recordedSend(&mu, &calls, &failFirst))
	q := newMemQueue(3)
	qb, _ := NewQueuedBot(bot, q).WithInterval(time.Millisecond).Build()

	if _, err := qb.Send(context.Background(), Text("网络抖动后重投"), Chat("chat-1")); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	runQueuedBot(t, qb)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		q.mu.Lock()
		done := q.acked[""] == false && len(q.acked) == 1
		q.mu.Unlock()
		if done {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.acked) != 1 {
		t.Fatalf("确认任务数 = %d, 期望 1", len(q.acked))
	}
	if len(q.dropped) != 0 {
		t.Fatalf("不应有任务被丢弃, dropped = %v", q.dropped)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 1 {
		t.Fatalf("成功投递次数 = %d, 期望重投后成功 1 次", len(calls))
	}
	if calls[0].ID != "chat-1" {
		t.Fatalf("投递目标 = %q, 期望 chat-1", calls[0].ID)
	}
	if got := bot.Stats().TotalSent(); got != 1 {
		t.Fatalf("TotalSent() = %d, 期望 1", got)
	}
	if got := bot.Stats().TotalError(); got != 1 {
		t.Fatalf("TotalError() = %d, 期望首次失败计入 1", got)
	}
}

// TestQueuedBotRunDropsExhausted 验证持续失败的任务达到投递上限后被队列丢弃
func TestQueuedBotRunDropsExhausted(t *testing.T) {
	bot := newTestBot(t, PlatformTelegram, func(context.Context, Target, *Message) (*SendResult, error) {
		return nil, NewPlatformError(PlatformTelegram, "Send", 400, "Bad Request: chat not found", false)
	})
	q := newMemQueue(2)
	qb, _ := NewQueuedBot(bot, q).WithInterval(time.Millisecond).Build()

	if _, err := qb.Send(context.Background(), Text("僵尸会话"), Chat("dead-chat")); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	runQueuedBot(t, qb)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		q.mu.Lock()
		dropped := len(q.dropped)
		q.mu.Unlock()
		if dropped == 1 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.dropped) != 1 || q.dropped[0] == "" {
		t.Fatalf("丢弃列表 = %v, 期望 1 条已耗尽任务", q.dropped)
	}
	if len(q.acked) != 0 {
		t.Fatalf("失败任务不应被确认, acked = %v", q.acked)
	}
}

// TestQueuedBotRunThrottles 验证消费按 interval 节流：3 条任务的总投递耗时不小于 2 个间隔
func TestQueuedBotRunThrottles(t *testing.T) {
	var mu sync.Mutex
	var calls []Target
	var failFirst int32 = -1
	bot := newTestBot(t, PlatformTelegram, recordedSend(&mu, &calls, &failFirst))
	q := newMemQueue(3)
	interval := 30 * time.Millisecond
	qb, _ := NewQueuedBot(bot, q).WithInterval(interval).Build()

	for _, id := range []string{"c-1", "c-2", "c-3"} {
		if _, err := qb.Send(context.Background(), Text("突发流量"), Chat(id)); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
	}

	start := time.Now()
	runQueuedBot(t, qb)
	deadline := time.Now().Add(interval * 20)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := len(calls) == 3
		mu.Unlock()
		if done {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	elapsed := time.Since(start)

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 3 {
		t.Fatalf("投递数 = %d, 期望 3", len(calls))
	}
	// 每条任务投递前等待一个 tick，第 3 条之前至少经过 2 个间隔
	if min := interval * 2; elapsed < min {
		t.Fatalf("总投递耗时 = %v, 期望节流后不小于 %v", elapsed, min)
	}
}

// TestQueuedBotRunStopsOnCancel 验证 ctx 取消后消费循环退出并返回 ctx.Err
func TestQueuedBotRunStopsOnCancel(t *testing.T) {
	bot := newTestBot(t, PlatformTelegram, okSend)
	q := newMemQueue(3)
	qb, _ := NewQueuedBot(bot, q).WithInterval(5 * time.Millisecond).Build()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- qb.Run(ctx) }()
	// 空队列阻塞等待 ctx 取消
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() 返回 %v, 期望 context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() 未随 ctx 取消退出")
	}
}

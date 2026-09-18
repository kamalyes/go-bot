/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 22:16:53
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 22:16:53
* @FilePath: \go-bot\metrics_test.go
* @Description: 统计后端测试：空实现与捕获型实现
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import (
	"context"
	"sync"
	"testing"
	"time"
)

// 接口实现断言：EmptyMetrics 与捕获型实现都满足 Metrics SPI
var (
	_ Metrics = EmptyMetrics{}
	_ Metrics = (*captureMetrics)(nil)
)

// captureMetrics 是测试用 Metrics 实现：并发安全地收集全部 SendStat
type captureMetrics struct {
	mu    sync.Mutex
	stats []SendStat
}

// RecordSend 实现 Metrics 接口，追加一条统计事件
func (c *captureMetrics) RecordSend(_ context.Context, stat SendStat) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats = append(c.stats, stat)
}

// snapshot 返回已收集事件的副本，避免用例间共享底层数组
func (c *captureMetrics) snapshot() []SendStat {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]SendStat(nil), c.stats...)
}

// TestEmptyMetricsNoOp 验证空实现丢弃事件且不产生副作用
func TestEmptyMetricsNoOp(t *testing.T) {
	m := EmptyMetrics{}
	m.RecordSend(context.Background(), SendStat{
		TS:       time.Now(),
		Platform: "telegram",
		BotID:    "any",
		Success:  true,
	})
	// 无 panic、无状态即通过
}

// TestCaptureMetricsCollectsInOrder 验证捕获实现按序收集
func TestCaptureMetricsCollectsInOrder(t *testing.T) {
	var m captureMetrics
	want := []string{"a", "b", "c"}
	for _, id := range want {
		m.RecordSend(context.Background(), SendStat{TargetID: id, TS: time.Now()})
	}
	got := m.snapshot()
	if len(got) != len(want) {
		t.Fatalf("收集 %d 条, 期望 %d 条", len(got), len(want))
	}
	for i, id := range want {
		if got[i].TargetID != id {
			t.Fatalf("snapshot()[%d].TargetID = %q, 期望 %q", i, got[i].TargetID, id)
		}
	}
}

// TestCaptureMetricsConcurrentRecord 验证并发记录不丢不乱（配合 -race 检测）
func TestCaptureMetricsConcurrentRecord(t *testing.T) {
	var m captureMetrics
	var wg sync.WaitGroup
	const goroutines, perGoroutine = 8, 100
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				m.RecordSend(context.Background(), SendStat{TargetID: "t", TS: time.Now()})
			}
		}()
	}
	wg.Wait()
	if got := len(m.snapshot()); got != goroutines*perGoroutine {
		t.Fatalf("收集 %d 条, 期望 %d 条", got, goroutines*perGoroutine)
	}
}

/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 22:11:07
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 22:11:07
* @FilePath: \go-bot\stats_test.go
* @Description: 进程内发送统计测试：计数、成功率与并发安全
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import (
	"sync"
	"testing"
)

// TestStatsCountersIndependent 验证三类计数互不干扰
func TestStatsCountersIndependent(t *testing.T) {
	var s Stats
	s.IncSent()
	s.IncSent()
	s.IncSent()
	s.IncError()
	s.IncMuted()
	s.IncMuted()

	if got := s.TotalSent(); got != 3 {
		t.Fatalf("TotalSent() = %d, 期望 3", got)
	}
	if got := s.TotalError(); got != 1 {
		t.Fatalf("TotalError() = %d, 期望 1", got)
	}
	if got := s.TotalMuted(); got != 2 {
		t.Fatalf("TotalMuted() = %d, 期望 2", got)
	}
}

// TestStatsSuccessRate 验证成功率计算与静音不污染样本
func TestStatsSuccessRate(t *testing.T) {
	for _, tc := range []struct {
		name       string
		sent       int64
		errs       int64
		muted      int64
		wantRate   float64
	}{
		{"无样本返回零", 0, 0, 0, 0},
		{"部分成功", 3, 1, 0, 0.75},
		{"全部失败", 0, 2, 0, 0},
		{"全部成功", 5, 0, 0, 1},
		{"静音不计入样本", 3, 1, 97, 0.75},
	} {
		var s Stats
		for i := int64(0); i < tc.sent; i++ {
			s.IncSent()
		}
		for i := int64(0); i < tc.errs; i++ {
			s.IncError()
		}
		for i := int64(0); i < tc.muted; i++ {
			s.IncMuted()
		}
		if got := s.SuccessRate(); got != tc.wantRate {
			t.Fatalf("%s: SuccessRate() = %v, 期望 %v", tc.name, got, tc.wantRate)
		}
	}
}

// TestStatsConcurrentIncrement 验证并发递增不丢计数（配合 -race 检测）
func TestStatsConcurrentIncrement(t *testing.T) {
	const goroutines, perGoroutine = 16, 500

	var s Stats
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				s.IncSent()
				s.IncError()
				s.IncMuted()
			}
		}()
	}
	wg.Wait()

	want := int64(goroutines * perGoroutine)
	if got := s.TotalSent(); got != want {
		t.Fatalf("TotalSent() = %d, 期望 %d", got, want)
	}
	if got := s.TotalError(); got != want {
		t.Fatalf("TotalError() = %d, 期望 %d", got, want)
	}
	if got := s.TotalMuted(); got != want {
		t.Fatalf("TotalMuted() = %d, 期望 %d", got, want)
	}
	if got := s.SuccessRate(); got != 0.5 {
		t.Fatalf("SuccessRate() = %v, 期望 0.5", got)
	}
}

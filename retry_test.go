/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 22:13:29
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 22:13:29
* @FilePath: \go-bot\retry_test.go
* @Description: 发送重试策略测试：禁用、退避耗尽、非重试短路、取消与抖动路径
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

// errRateLimited 模拟可重试的平台限流错误
var errRateLimited = NewPlatformError(PlatformTelegram, "Send", 429, "Too Many Requests: retry after 3", true)

// errPermanent 模拟不可重试的本地校验错误
var errPermanent = NewValidationError("Send", "text content is empty")

// fastPolicy 返回退避极快的重试策略，保证测试耗时可控
func fastPolicy(maxRetries int, jitter bool) RetryPolicy {
	return RetryPolicy{
		MaxRetries:   maxRetries,
		InitialDelay: time.Millisecond,
		MaxDelay:     2 * time.Millisecond,
		Jitter:       jitter,
	}
}

// TestRetryZeroValueDisablesRetry 验证零值策略单次尝试语义
func TestRetryZeroValueDisablesRetry(t *testing.T) {
	var calls atomic.Int64
	err := RetryPolicy{}.do(context.Background(), func(context.Context) error {
		calls.Add(1)
		return errRateLimited
	})
	if calls.Load() != 1 {
		t.Fatalf("零值策略调用 fn %d 次, 期望 1 次", calls.Load())
	}
	if !errors.Is(err, errRateLimited) && err != errRateLimited {
		t.Fatalf("do() = %v, 期望原样返回错误", err)
	}
}

// TestRetryNegativeMaxRetriesDisablesRetry 验证负值按禁用处理
func TestRetryNegativeMaxRetriesDisablesRetry(t *testing.T) {
	var calls atomic.Int64
	p := RetryPolicy{MaxRetries: -3}
	if err := p.do(context.Background(), func(context.Context) error {
		calls.Add(1)
		return nil
	}); err != nil {
		t.Fatalf("do() = %v, 期望成功", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("负值策略调用 fn %d 次, 期望 1 次", calls.Load())
	}
}

// TestRetrySucceedsAfterTransientFailure 验证瞬时失败后重试成功
func TestRetrySucceedsAfterTransientFailure(t *testing.T) {
	var calls atomic.Int64
	p := fastPolicy(3, false)
	err := p.do(context.Background(), func(context.Context) error {
		if calls.Add(1) == 1 {
			return errRateLimited
		}
		return nil
	})
	if err != nil {
		t.Fatalf("do() = %v, 期望重试后成功", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("调用 fn %d 次, 期望 2 次（首次 + 一次重试）", calls.Load())
	}
}

// TestRetryExhaustsAllAttempts 验证持续可重试错误耗尽全部尝试次数
func TestRetryExhaustsAllAttempts(t *testing.T) {
	var calls atomic.Int64
	p := fastPolicy(3, false)
	err := p.do(context.Background(), func(context.Context) error {
		calls.Add(1)
		return errRateLimited
	})
	if calls.Load() != 4 {
		t.Fatalf("调用 fn %d 次, 期望 4 次（1 次首试 + 3 次重试）", calls.Load())
	}
	var e *Error
	if !errors.As(err, &e) || e.Code != 429 {
		t.Fatalf("do() = %v, 期望返回最后的平台错误", err)
	}
}

// TestRetryStopsOnNonRetryable 验证不可重试错误立即短路
func TestRetryStopsOnNonRetryable(t *testing.T) {
	var calls atomic.Int64
	p := fastPolicy(5, false)
	err := p.do(context.Background(), func(context.Context) error {
		calls.Add(1)
		return errPermanent
	})
	if calls.Load() != 1 {
		t.Fatalf("调用 fn %d 次, 期望 1 次（校验错误不重试）", calls.Load())
	}
	var e *Error
	if !errors.As(err, &e) || e.Kind != KindValidation {
		t.Fatalf("do() = %v, 期望返回校验错误", err)
	}
}

// TestRetryJitterPathCompletes 验证开启抖动后流程可正常收敛
func TestRetryJitterPathCompletes(t *testing.T) {
	var calls atomic.Int64
	p := fastPolicy(2, true)
	err := p.do(context.Background(), func(context.Context) error {
		if calls.Add(1) == 1 {
			return errRateLimited
		}
		return nil
	})
	if err != nil {
		t.Fatalf("do() = %v, 期望抖动路径下重试成功", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("调用 fn %d 次, 期望 2 次", calls.Load())
	}
}

// TestRetryDefaultDelaysApplied 验证零值退避时长落到缺省值后仍可完成重试
func TestRetryDefaultDelaysApplied(t *testing.T) {
	var calls atomic.Int64
	p := RetryPolicy{MaxRetries: 1} // InitialDelay/MaxDelay 取缺省 200ms/3s
	start := time.Now()
	err := p.do(context.Background(), func(context.Context) error {
		if calls.Add(1) == 1 {
			return errRateLimited
		}
		return nil
	})
	if err != nil {
		t.Fatalf("do() = %v, 期望缺省退避下重试成功", err)
	}
	if elapsed := time.Since(start); elapsed < DefaultRetryInitialDelay {
		t.Fatalf("耗时 %v, 期望至少经历一次缺省首次退避 %v", elapsed, DefaultRetryInitialDelay)
	}
}

// TestRetryContextCancelled 验证 ctx 取消后不再继续退避循环
func TestRetryContextCancelled(t *testing.T) {
	var calls atomic.Int64
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	p := RetryPolicy{MaxRetries: 50, InitialDelay: 50 * time.Millisecond, MaxDelay: 100 * time.Millisecond}
	start := time.Now()
	err := p.do(ctx, func(context.Context) error {
		calls.Add(1)
		return errRateLimited
	})
	if err == nil {
		t.Fatal("ctx 取消后不应报告成功")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("耗时 %v, 期望取消后快速返回", elapsed)
	}
}

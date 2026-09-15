/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-15 15:38:21
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-15 20:26:53
* @FilePath: \go-bot\retry.go
* @Description: 发送重试策略，go-toolbox retry 驱动
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import (
	"context"
	"time"

	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"github.com/kamalyes/go-toolbox/pkg/retry"
)

// RetryPolicy 控制发送失败后如何重试，底层由 go-toolbox 的 retry 包驱动
// （指数退避 + 可选 jitter + ctx 取消控制）零值禁用重试（单次尝试语义）
// 启用重试意味着 at-least-once：请求可能在客户端观察到失败前已在平台端
// 成功，一次重试可能产生重复消息；仅在可接受重复或下游去重时启用
type RetryPolicy struct {
	// MaxRetries 是首次尝试之后的额外重试次数，0 禁用，负值按 0 处理
	MaxRetries int
	// InitialDelay 是首次重试前的基础退避时长，零值取 DefaultRetryInitialDelay
	InitialDelay time.Duration
	// MaxDelay 限制本地指数退避的单次上限，零值取 DefaultRetryMaxDelay
	MaxDelay time.Duration
	// Jitter 为退避加入随机抖动，避免 thundering herd
	Jitter bool
}

// do 按策略带重试地执行 fn只有标记为可重试的结构化 *Error 才会重试；
// 校验错误、401/403、不可重试的平台码、解码错误会立即返回
func (p RetryPolicy) do(ctx context.Context, fn func(context.Context) error) error {
	if p.MaxRetries <= 0 {
		return fn(ctx)
	}
	initial := mathx.IF(p.InitialDelay <= 0, DefaultRetryInitialDelay, p.InitialDelay)
	maxDelay := mathx.IF(p.MaxDelay <= 0, DefaultRetryMaxDelay, p.MaxDelay)
	return retry.NewRetryWithCtx(ctx).
		SetAttemptCount(p.MaxRetries + 1). // toolbox 语义：attemptCount 为总尝试次数
		SetInterval(initial).
		SetMaxInterval(maxDelay).
		SetBackoffMultiplier(2.0).
		SetJitter(p.Jitter).
		SetConditionFunc(retryable).
		Do(func() error { return fn(ctx) })
}

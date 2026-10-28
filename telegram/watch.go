/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-10-20 09:15:36
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-10-20 10:21:07
* @FilePath: \go-bot\telegram\watch.go
* @Description: Telegram 常驻事件循环：自动推进游标的长轮询消费，驱动群组实时感知
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package telegram

import (
	"context"
	"time"

	gobot "github.com/kamalyes/go-bot"
	gologger "github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
)

// DefaultEventPollTimeout 是单次长轮询窗口的缺省秒数，
// 需小于适配器 HTTP 客户端超时（gobot.DefaultHTTPTimeout），
// 否则窗口未结束请求先被客户端切断
const DefaultEventPollTimeout = 8

// eventRetryBackoff 是拉取失败后的固定退避，避免故障期间持续压测平台
const eventRetryBackoff = 3 * time.Second

// EventLoopOptions 控制常驻事件循环的行为
type EventLoopOptions struct {
	// StartOffset 是起始游标（已消费的最大 EventID 加一），
	// 零值时先探测平台最新事件、跳过历史积压，只感知启动之后发生的变化
	StartOffset int64
	// PollTimeout 是单次长轮询窗口秒数，缺省 DefaultEventPollTimeout，
	// 必须小于适配器 HTTP 客户端超时
	PollTimeout int64
	// Limit 是单次拉取的事件上限，零值取平台默认
	Limit int64
	// Backoff 是拉取失败后的退避时长，缺省 3s
	Backoff time.Duration
	// Logger 可选，缺省 go-logger 的 EmptyLogger
	Logger gologger.ILogger
}

// EventHandler 批量处理一批事件：
// 返回 error 只会记入日志，不会中断循环也不会重投（游标已推进），
// 需要重试语义时由实现方自行兜底
type EventHandler func(ctx context.Context, events []Event) error

// RunEventLoop 起常驻长轮询循环，阻塞直到 ctx 取消，返回 ctx.Err() 表示正常停机：
// 每轮按游标拉取一批事件交给 handler（GetEvents 会顺带实时维护已知会话列表），
// 拉取失败退避重试，handler 失败仅记日志；多实例部署时 getUpdates 互斥，
// 同一 token 的事件循环只能存在一个
func (a *Adapter) RunEventLoop(ctx context.Context, opts EventLoopOptions, handler EventHandler) error {
	const op = "RunEventLoop"
	if handler == nil {
		return gobot.NewValidationError(op, "handler is required")
	}
	logger := mathx.IF(opts.Logger == nil, gologger.ILogger(&gologger.EmptyLogger{}), opts.Logger)

	offset, err := a.resolveStartOffset(ctx, opts)
	if err != nil {
		return err
	}
	limit := mathx.IF(opts.Limit > 0, opts.Limit, int64(0))
	pollTimeout := mathx.IF(opts.PollTimeout > 0, opts.PollTimeout, int64(DefaultEventPollTimeout))
	backoff := mathx.IF(opts.Backoff > 0, opts.Backoff, eventRetryBackoff)

	logger.InfoContext(ctx, "telegram: event loop started",
		"offset", offset, "poll_timeout_s", pollTimeout)
	for ctx.Err() == nil {
		events, err := a.GetEvents(ctx, EventOptions{
			Offset:  offset,
			Limit:   limit,
			Timeout: pollTimeout,
		})
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			logger.WarnContext(ctx, "telegram: event poll failed, backing off",
				"offset", offset, "error", err)
			if !sleepCtx(ctx, backoff) {
				break
			}
			continue
		}
		if len(events) == 0 {
			continue
		}
		if hErr := handler(ctx, events); hErr != nil {
			logger.ErrorContext(ctx, "telegram: event handler failed, events confirmed anyway",
				"events", len(events), "error", hErr)
		}
		offset = events[len(events)-1].EventID + 1
	}
	logger.InfoContext(ctx, "telegram: event loop stopped", "offset", offset)
	return ctx.Err()
}

// resolveStartOffset 确定消费起点：显式正游标直接采用，
// 零值时探测平台最新事件（offset=-1 取最新一条）后跳过积压，
// 平台无积压事件时从零开始
func (a *Adapter) resolveStartOffset(ctx context.Context, opts EventLoopOptions) (int64, error) {
	if opts.StartOffset > 0 {
		return opts.StartOffset, nil
	}
	latest, err := a.GetEvents(ctx, EventOptions{Offset: -1, Limit: 1})
	if err != nil {
		return 0, gobot.NewTransportError(gobot.PlatformTelegram, "resolveStartOffset", err)
	}
	if len(latest) == 0 {
		return 0, nil
	}
	return latest[len(latest)-1].EventID + 1, nil
}

// sleepCtx 等待指定时长，ctx 先行取消时返回 false
func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

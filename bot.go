/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-16 15:07:28
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 21:15:07
* @FilePath: \go-bot\bot.go
* @Description: Bot 统一门面，Builder 链式装配通用能力链
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	gologger "github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-toolbox/pkg/breaker"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
)

// Bot 是消息发送的统一入口：把一个 Adapter 包装上通用能力
// （静音开关、重试、熔断、统计），对外暴露完全平台无关的 API
// Bot 可安全并发使用；其零值不可用，需通过 Builder 构造
type Bot struct {
	adapter Adapter
	logger  gologger.ILogger
	retry   RetryPolicy
	sw      *Switch
	metrics Metrics
	stats   Stats
	botID   string
	breaker *breaker.Circuit // 可选熔断器，nil 表示不启用
}

// BotBuilder 是 Bot 的链式构造器，Builder 模式与 go-logger / go-cachex 保持一致
type BotBuilder struct {
	adapter Adapter
	logger  gologger.ILogger
	retry   RetryPolicy
	sw      *Switch
	metrics Metrics
	name    string
	breaker *breaker.Circuit
}

// NewBot 以 adapter 为底座开始链式配置，最终 Build 产出 Bot
func NewBot(adapter Adapter) *BotBuilder {
	return &BotBuilder{adapter: adapter}
}

// WithLogger 设置日志实现；缺省使用 go-logger 的 EmptyLogger
func (b *BotBuilder) WithLogger(l gologger.ILogger) *BotBuilder {
	b.logger = l
	return b
}

// WithRetry 设置重试策略；缺省零值（不重试）
func (b *BotBuilder) WithRetry(p RetryPolicy) *BotBuilder {
	b.retry = p
	return b
}

// WithSwitch 设置运行期静音开关，多个 Bot 可共享同一实例实现联动静音
func (b *BotBuilder) WithSwitch(s *Switch) *BotBuilder {
	b.sw = s
	return b
}

// WithMetrics 设置统计后端；缺省 EmptyMetrics（不持久化，进程内 Stats 始终可用）
func (b *BotBuilder) WithMetrics(m Metrics) *BotBuilder {
	b.metrics = m
	return b
}

// WithName 设置 Bot 标识，作为 SendStat.BotID 维度；缺省用平台名
// 同平台多个 Bot（如多个 TG 机器人）时用它区分
func (b *BotBuilder) WithName(name string) *BotBuilder {
	b.name = name
	return b
}

// WithBreaker 设置熔断器（go-toolbox breaker.Circuit）：连续失败达到阈值后
// 快速失败，避免对故障平台持续压测；静音丢弃不计入熔断缺省不启用
func (b *BotBuilder) WithBreaker(c *breaker.Circuit) *BotBuilder {
	b.breaker = c
	return b
}

// Build 校验并冻结配置，产出 Botadapter 为 nil 时返回错误
func (b *BotBuilder) Build() (*Bot, error) {
	if b.adapter == nil {
		return nil, NewValidationError("Build", "adapter is required")
	}
	logger := mathx.IF(b.logger == nil, gologger.ILogger(&gologger.EmptyLogger{}), b.logger)
	metrics := mathx.IF(b.metrics == nil, Metrics(EmptyMetrics{}), b.metrics)
	botID := mathx.IF(b.name == "", string(b.adapter.Platform()), b.name)
	return &Bot{
		adapter: b.adapter,
		logger:  logger,
		retry:   b.retry,
		sw:      b.sw,
		metrics: metrics,
		botID:   botID,
		breaker: b.breaker,
	}, nil
}

// Platform 返回底层 adapter 的平台标识
func (b *Bot) Platform() Platform { return b.adapter.Platform() }

// Name 返回 Bot 标识（统计维度）
func (b *Bot) Name() string { return b.botID }

// Stats 返回进程内统计（sent/error/muted 与成功率），始终可用
func (b *Bot) Stats() *Stats { return &b.stats }

// Close 释放底层 adapter 的资源
func (b *Bot) Close(ctx context.Context) error { return b.adapter.Close(ctx) }

// SendText 发送一条纯文本消息到一个或多个目标（便捷形式）
func (b *Bot) SendText(ctx context.Context, text string, targets ...Target) ([]*SendResult, error) {
	return b.Send(ctx, Text(text), targets...)
}

// SendMarkdown 发送一条 markdown 消息到一个或多个目标（便捷形式）
func (b *Bot) SendMarkdown(ctx context.Context, title, content string, targets ...Target) ([]*SendResult, error) {
	return b.Send(ctx, Markdown(title, content), targets...)
}

// SendImage 发送一条图片消息到一个或多个目标（便捷形式）
func (b *Bot) SendImage(ctx context.Context, url string, targets ...Target) ([]*SendResult, error) {
	return b.Send(ctx, Image(url), targets...)
}

// Send 把一条消息发送到一个或多个目标，是所有通用能力的收口：
// 单目标走快路径不引入 goroutine；多目标并发扇出，
// 在途请求受 DefaultBatchConcurrency 限制，限流退避交给重试链消化
// 每个目标独立走完整流水线（校验 → 静音 → 重试/熔断包裹 adapter.Send → 统计），
// 结果按 targets 顺序返回，失败位置为 nil；错误用 errors.Join 聚合，全部成功时为 nil
// 静音时目标返回 nil 结果且不计入聚合错误（消息被丢弃并计入 muted，
// 调用方可用 Stats().TotalMuted 观测，与成功不可区分是刻意设计）
func (b *Bot) Send(ctx context.Context, msg *Message, targets ...Target) ([]*SendResult, error) {
	if len(targets) == 0 {
		return nil, NewValidationError("Send", "targets are required")
	}
	if len(targets) == 1 {
		res, err := b.sendOne(ctx, targets[0], msg)
		return []*SendResult{res}, err
	}

	results := make([]*SendResult, len(targets))
	errs := make([]error, len(targets))
	sem := make(chan struct{}, min(len(targets), DefaultBatchConcurrency))
	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i], errs[i] = b.sendOne(ctx, target, msg)
		})
	}
	wg.Wait()
	return results, errors.Join(errs...)
}

// sendOne 是单目标的完整发送流水线：校验 → 静音 → 重试/熔断 → 统计
func (b *Bot) sendOne(ctx context.Context, target Target, msg *Message) (*SendResult, error) {
	start := time.Now()

	if err := b.validate(target, msg); err != nil {
		b.record(ctx, target, msg, nil, err, start)
		return nil, err
	}

	// 静音：不计 sent/error，单独计 muted，避免污染成功率
	if !b.sw.Enabled() {
		b.stats.IncMuted()
		b.logger.DebugContext(ctx, "gobot: muted by switch, message dropped",
			"platform", b.adapter.Platform(), "target", target.ID)
		return nil, nil
	}

	var (
		result *SendResult
		err    error
	)
	// 重试（toolbox retry）+ 可选熔断（toolbox breaker）包裹 adapter 发送；
	// 熔断打开时快速失败，不触碰平台
	send := func() error {
		return b.retry.do(ctx, func(ctx context.Context) error {
			var sendErr error
			result, sendErr = b.adapter.Send(ctx, target, msg)
			return sendErr
		})
	}
	if b.breaker != nil {
		err = b.breaker.Execute(send)
	} else {
		err = send()
	}

	b.record(ctx, target, msg, result, err, start)
	return result, err
}

// validate 统一做本地校验，任何一方失败都记为一次发送（validation 错误）
func (b *Bot) validate(target Target, msg *Message) error {
	if err := target.validate(); err != nil {
		return err
	}
	return msg.validate()
}

// record 记录一次发送的统计：进程内 atomic 计数 + 可选 Metrics 后端
// 统计异常绝不影响发送结果
func (b *Bot) record(ctx context.Context, target Target, msg *Message, result *SendResult, err error, start time.Time) {
	stat := SendStat{
		TS:           time.Now(),
		Platform:     string(b.adapter.Platform()),
		BotID:        b.botID,
		TargetID:     target.ID,
		MsgType:      string(msg.Type),
		Success:      err == nil,
		Latency:      time.Since(start),
		ContentBytes: msg.ContentSize(),
	}
	if err != nil {
		b.stats.IncError()
		var e *Error
		if errors.As(err, &e) {
			stat.ErrorKind = string(e.Kind)
			stat.ErrorCode = strconv.Itoa(e.Code)
		}
		b.logger.ErrorContext(ctx, "gobot: send failed",
			"platform", stat.Platform, "bot", stat.BotID, "target", stat.TargetID, "error", err)
	} else {
		b.stats.IncSent()
		if result != nil {
			b.logger.DebugContext(ctx, "gobot: message sent",
				"platform", stat.Platform, "bot", stat.BotID,
				"target", stat.TargetID, "message_id", result.MessageID,
				"latency_ms", stat.Latency.Milliseconds())
		}
	}
	b.metrics.RecordSend(ctx, stat)
}

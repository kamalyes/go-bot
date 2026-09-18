/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-19 10:00:08
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-19 13:00:56
* @FilePath: \go-bot\queued.go
* @Description: QueuedBot 削峰填谷门面：入队直返 + 节流消费
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import (
	"context"
	"time"

	gologger "github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
)

// QueuedBot 是 Bot 的削峰填谷门面：Send 不再直连平台，
// 而是校验后把任务写入 Queue 立即返回（生产端）；
// Run 起消费循环按 Interval 节奏逐条排干队列、
// 调用底层 Bot.Send 真实投递（消费端），避免瞬时调用打爆平台限流
//
// 分布式部署时每个实例各自 Run：队列层保证一条任务只被一个实例投递，
// 单实例的 Interval 节流限制单实例吞吐，集群总吞吐 = 实例数 × 单实例速率
type QueuedBot struct {
	bot      *Bot
	queue    Queue
	interval time.Duration
	logger   gologger.ILogger
}

// QueuedBotBuilder 是 QueuedBot 的链式构造器，与 BotBuilder 风格一致
type QueuedBotBuilder struct {
	bot      *Bot
	queue    Queue
	interval time.Duration
	logger   gologger.ILogger
}

// NewQueuedBot 以 bot 与 queue 为底座开始链式配置
func NewQueuedBot(bot *Bot, queue Queue) *QueuedBotBuilder {
	return &QueuedBotBuilder{bot: bot, queue: queue}
}

// WithInterval 设置消费端的两条任务投递间隔（削峰节奏），
// 如钉钉 20 条/分钟的风控对应 3s；缺省 DefaultQueueInterval
func (b *QueuedBotBuilder) WithInterval(d time.Duration) *QueuedBotBuilder {
	b.interval = d
	return b
}

// WithLogger 设置日志实现；缺省使用 go-logger 的 EmptyLogger
func (b *QueuedBotBuilder) WithLogger(l gologger.ILogger) *QueuedBotBuilder {
	b.logger = l
	return b
}

// Build 校验并冻结配置，产出 QueuedBot，bot 或 queue 缺失时返回错误
func (b *QueuedBotBuilder) Build() (*QueuedBot, error) {
	const op = "Build"
	if b.bot == nil {
		return nil, NewValidationError(op, "bot is required")
	}
	if b.queue == nil {
		return nil, NewValidationError(op, "queue is required")
	}
	interval := b.interval
	if interval <= 0 {
		interval = DefaultQueueInterval
	}
	logger := mathx.IF(b.logger == nil, gologger.ILogger(&gologger.EmptyLogger{}), b.logger)
	return &QueuedBot{bot: b.bot, queue: b.queue, interval: interval, logger: logger}, nil
}

// Queue 返回底层队列，供运行期探活或运维操作
func (q *QueuedBot) Queue() Queue { return q.queue }

// Send 把一条消息转为投递任务写入队列，入队成功立即返回：
// 返回 nil error 仅代表已入队，不代表已投递；
// 真实投递结果由消费端 Run 记录进底层 Bot 的 Stats 与日志。
// 一次 Send 的多个目标按目标粒度拆分为多条任务（FIFO 保序），
// 失败重投时不会重复轰炸已成功的目标
func (q *QueuedBot) Send(ctx context.Context, msg *Message, targets ...Target) ([]*SendResult, error) {
	const op = "Send"
	if len(targets) == 0 {
		return nil, NewValidationError(op, "targets are required")
	}
	if err := msg.validate(); err != nil {
		return nil, err
	}
	for _, target := range targets {
		if err := target.validate(); err != nil {
			return nil, err
		}
	}
	now := time.Now()
	for _, target := range targets {
		task := &QueueTask{
			ID:          newTaskID(),
			Message:     *msg,
			Targets:     []Target{target},
			PublishedAt: now,
		}
		if err := q.queue.Publish(ctx, task); err != nil {
			return nil, NewTransportError(q.bot.Platform(), op, err)
		}
	}
	q.logger.DebugContext(ctx, "gobot: task enqueued",
		"platform", q.bot.Platform(), "targets", len(targets))
	return nil, nil
}

// Run 起消费循环，阻塞直到 ctx 取消：每条任务投递前等待
// 一个 Interval（削峰节奏），投递交给底层 Bot.Send（静音/重试/
// 熔断/统计等通用能力全部生效）；投递失败的任务由队列侧重投，
// 投递次数耗尽后丢弃。返回 ctx.Err() 表示正常停机
func (q *QueuedBot) Run(ctx context.Context) error {
	q.logger.InfoContext(ctx, "gobot: queue consumer started",
		"platform", q.bot.Platform(), "bot", q.bot.Name(), "interval", q.interval.String())
	ticker := time.NewTicker(q.interval)
	defer ticker.Stop()
	err := q.queue.Consume(ctx, func(ctx context.Context, task *QueueTask) error {
		// 固定节奏排干队列：每条任务前等一个 tick，消费慢于 tick 时直接放行
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}
		latency := time.Since(task.PublishedAt)
		_, err := q.bot.Send(ctx, &task.Message, task.Targets...)
		if err != nil {
			// 返回错误交给队列重投；次数耗尽由队列侧丢弃并记录
			q.logger.ErrorContext(ctx, "gobot: deliver failed, will redeliver",
				"platform", q.bot.Platform(), "task", task.ID, "error", err)
			return err
		}
		q.logger.DebugContext(ctx, "gobot: task delivered",
			"platform", q.bot.Platform(), "task", task.ID,
			"queue_latency_ms", latency.Milliseconds())
		return nil
	})
	q.logger.InfoContext(ctx, "gobot: queue consumer stopped",
		"platform", q.bot.Platform(), "error", err)
	return err
}

// Close 释放底层队列的资源；底层 Bot 与 Switch 的生命周期由调用方管理
func (q *QueuedBot) Close(ctx context.Context) error { return q.queue.Close(ctx) }

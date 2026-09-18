/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-21 09:51:08
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-21 10:06:08
* @FilePath: \go-bot\queue.go
* @Description: 削峰填谷队列 SPI 与投递任务模型
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"
)

// Queue 是削峰填谷队列的 SPI：把投递任务可靠地中转，供独立 adapter 库
// （如 go-bot-queue-redis / go-bot-queue-nats）对接具体 MQ。
// 实现面向分布式部署设计：多个进程实例共享同一队列消费，
// 每条任务只会被一个实例投递，实例崩溃后其任务由其余实例接管
// 实现必须可被多个 goroutine 安全并发使用
type Queue interface {
	// Publish 把一条任务入队，入队成功不等于投递成功，
	// 真实投递发生在消费端（见 QueuedBot.Run）
	Publish(ctx context.Context, task *QueueTask) error
	// Consume 阻塞消费：循环回调 handler 直到 ctx 取消才返回，
	// 返回 ctx.Err() 表示正常停机；handler 返回 nil 表示任务完成
	// （队列侧确认删除），返回 error 表示投递失败，
	// 由队列按自身的投递语义决定重投或投递次数耗尽后丢弃
	Consume(ctx context.Context, handler QueueHandler) error
	// Close 释放队列持有的资源
	Close(ctx context.Context) error
}

// QueueHandler 处理一条投递任务，由 QueuedBot 提供：
// 返回 nil 确认任务，返回 error 触发队列侧重投
type QueueHandler func(ctx context.Context, task *QueueTask) error

// QueueTask 是队列中的一次投递任务：单条消息与单批目标的可序列化载体。
// 按目标粒度入队（一次 Send 的多个目标拆为多条任务），
// 保证失败重投时不会重复轰炸已成功的目标
type QueueTask struct {
	// ID 是任务标识，由入队方生成，用于日志追踪
	ID string `json:"id"`
	// Message 是待投递的消息快照
	Message Message `json:"message"`
	// Targets 是本次任务的目标集合，通常为单目标
	Targets []Target `json:"targets"`
	// PublishedAt 是入队时间，供消费端观测排队延迟
	PublishedAt time.Time `json:"published_at"`
}

// taskSeq 是进程内任务序号，配合纳秒时间戳生成全局足够唯一的任务标识
var taskSeq atomic.Uint64

// newTaskID 生成任务标识：纳秒时间戳与进程内序号的 36 进制拼接
func newTaskID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + strconv.FormatUint(taskSeq.Add(1), 36)
}

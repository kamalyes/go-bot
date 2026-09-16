/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-16 08:27:19
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-16 09:53:32
* @FilePath: \go-bot\metrics.go
* @Description: 统计后端接口与发送事件模型
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import (
	"context"
	"time"
)

// SendStat 是一次发送的完整统计事件，由 Bot 在每次发送（成功或失败）后交给 Metrics它是持久化统计的唯一事实来源，字段与
// metrics/clickhouse 的 bot_send_events 表一一对应
type SendStat struct {
	TS           time.Time     // 发送完成时间
	Platform     string        // 平台标识
	BotID        string        // Bot 标识（WithName 设置，默认为平台名）
	TargetID     string        // 发送目标标识
	MsgType      string        // 消息类型（text/markdown/image）
	Success      bool          // 是否成功（重试后成功仍记成功）
	Latency      time.Duration // 总耗时（含重试）
	ContentBytes int64         // 消息正文字节量
	ErrorKind    string        // 失败分类，成功时为空
	ErrorCode    string        // 平台错误码（字符串化），成功时为空
}

// Metrics 是可选取接的统计后端接口RecordSend 必须非阻塞或快速返回——
// 统计失败绝不影响发送主流程；实现自行决定异步批量落盘策略
// 默认使用 EmptyMetrics（不落任何盘）；ClickHouse 实现见 metrics/clickhouse
type Metrics interface {
	RecordSend(ctx context.Context, stat SendStat)
}

// EmptyMetrics 是 Metrics 的空实现：丢弃所有事件，零开销
type EmptyMetrics struct{}

// RecordSend 实现 Metrics 接口，什么都不做
func (EmptyMetrics) RecordSend(context.Context, SendStat) {}

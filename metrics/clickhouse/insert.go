/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 08:28:13
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 09:02:36
* @FilePath: \go-bot\metrics\clickhouse\insert.go
* @Description: ClickHouse 批量写入：事件模型与参数化 INSERT
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package clickhouse

import (
	"context"
	"strings"
	"time"

	gobot "github.com/kamalyes/go-bot"
)

// insertColumns 是 bot_send_events 的写入列清单，列序与参数绑定顺序一致
const insertColumns = `(ts, platform, bot_id, target_id, msg_type, success, latency_ms, content_bytes, error_kind, error_code)`

// insertPlaceholder 是单行参数占位，10 列与 insertColumns 一一对应
const insertPlaceholder = `(?,?,?,?,?,?,?,?,?,?)`

// SendEvent 是 bot_send_events 表单行的写入模型：
// 字段与 gobot.SendStat 一一对应，TS 写入前统一折算 UTC，
// 与连接的 session_timezone=UTC 保持同一时区口径
type SendEvent struct {
	// TS 是发送完成时间，对应 DateTime64(3) 列
	TS time.Time
	// Platform 是平台标识
	Platform string
	// BotID 是 Bot 标识
	BotID string
	// TargetID 是发送目标标识
	TargetID string
	// MsgType 是消息类型（text/markdown/image）
	MsgType string
	// Success 是否成功，绑定前折算 0/1
	Success bool
	// LatencyMS 是总耗时毫秒（含重试）
	LatencyMS int64
	// ContentBytes 是消息正文字节量
	ContentBytes int64
	// ErrorKind 是失败分类，成功时为空
	ErrorKind string
	// ErrorCode 是平台错误码（字符串化），成功时为空
	ErrorCode string
}

// TableName 返回缺省表名，供 AutoMigrate 或文档参照
func (SendEvent) TableName() string { return DefaultTable }

// toSendEvent 把公开统计事件转为写入模型；成功事件 ErrorKind/ErrorCode 为空串
func toSendEvent(stat gobot.SendStat) *SendEvent {
	return &SendEvent{
		TS:           stat.TS.UTC(),
		Platform:     stat.Platform,
		BotID:        stat.BotID,
		TargetID:     stat.TargetID,
		MsgType:      stat.MsgType,
		Success:      stat.Success,
		LatencyMS:    stat.Latency.Milliseconds(),
		ContentBytes: stat.ContentBytes,
		ErrorKind:    stat.ErrorKind,
		ErrorCode:    stat.ErrorCode,
	}
}

// flushBatch 是聚合器的落盘回调：整批一次写入；
// 失败时丢弃该批并累计失败计数，不重试、不阻塞 worker
func (m *Metrics) flushBatch(stats []gobot.SendStat) {
	if len(stats) == 0 {
		return
	}
	events := make([]*SendEvent, len(stats))
	for i := range stats {
		events[i] = toSendEvent(stats[i])
	}
	// 聚合跨多个请求周期，此处无法继承单次发送的 ctx，统一使用 Background
	if err := m.insertEvents(context.Background(), events); err != nil {
		m.failedBatches.Add(1)
	}
}

// insertEvents 执行一次参数化批量 INSERT
//
// 不用 gorm.CreateInBatches：gorm-clickhouse v0.7.0 的 Create callback 走 PrepareContext，
// clickhouse-go native 协议连接对 PrepareContext 支持有限，会触发
// "code: 101, Unexpected packet Query received from client"；
// 改用 INSERT INTO ... VALUES 参数化批量绑定（走 ExecContext，不经 PrepareContext）
//
// success（bool）→ Bool 列：显式转 int(0/1) 绑定，不依赖驱动对 bool 的支持，
// 避免类型不匹配
func (m *Metrics) insertEvents(ctx context.Context, events []*SendEvent) error {
	placeholders := make([]string, len(events))
	args := make([]any, 0, len(events)*10)
	for i, e := range events {
		placeholders[i] = insertPlaceholder
		success := 0
		if e.Success {
			success = 1
		}
		args = append(args,
			e.TS, e.Platform, e.BotID, e.TargetID, e.MsgType,
			success, e.LatencyMS, e.ContentBytes, e.ErrorKind, e.ErrorCode,
		)
	}
	var b strings.Builder
	b.WriteString("INSERT INTO ")
	b.WriteString(m.cfg.Table)
	b.WriteString(" ")
	b.WriteString(insertColumns)
	b.WriteString(" VALUES ")
	b.WriteString(strings.Join(placeholders, ","))
	return m.db.WithContext(ctx).Exec(b.String(), args...).Error
}

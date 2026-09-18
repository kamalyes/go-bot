/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 08:12:36
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 09:52:18
* @FilePath: \go-bot\metrics\clickhouse\clickhouse.go
* @Description: ClickHouse 统计后端的构造与配置
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// Package clickhouse 提供 gobot.Metrics 的 ClickHouse 实现：
// 发送事件先进进程内队列，由后台聚合器按批或定时组装 INSERT 落盘；
// gorm 连接（含连接池）由接入方创建并管理生命周期，本包只做聚合与写入；
// 队列满或落盘失败时丢弃该批，统计路径绝不阻塞、绝不影响发送主流程
package clickhouse

import (
	"context"
	"sync/atomic"
	"time"

	gobot "github.com/kamalyes/go-bot"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
	"gorm.io/gorm"
)

// DefaultTable 是缺省表名，与 schema.sql 中 bot_send_events 对应
const DefaultTable = "bot_send_events"

// queueSizeRatio 是内部队列容量相对批大小的倍数，为突发流量留缓冲
const queueSizeRatio = 8

// Config 配置统计后端的聚合行为，不含连接参数（连接由接入方提供）
type Config struct {
	// Table 缺省 bot_send_events
	Table string
	// BatchSize 是单批最大事件数，缺省 gobot.DefaultMetricsBatchSize
	BatchSize int
	// FlushInterval 是最长落盘间隔，缺省 gobot.DefaultMetricsFlushInterval
	FlushInterval time.Duration
}

// Metrics 实现 gobot.Metrics：把 SendStat 批量聚合后异步写入 ClickHouse
// RecordSend 非阻塞；关停时调用 Stop 落盘剩余事件
// Metrics 可被多个 goroutine 安全并发使用
type Metrics struct {
	cfg           Config
	db            *gorm.DB
	processor     *syncx.BatchProcessor[gobot.SendStat]
	failedBatches atomic.Int64
}

// New 基于接入方提供的 gorm 连接创建统计后端并启动后台聚合，
// 连接池的建连与关闭由接入方管理；db 为 nil 时返回校验错误
func New(db *gorm.DB, cfg Config) (*Metrics, error) {
	const op = "New"
	if db == nil {
		return nil, gobot.NewValidationError(op, "gorm db is required")
	}
	cfg.Table = mathx.IfNotEmpty(cfg.Table, DefaultTable)
	cfg.BatchSize = mathx.IF(cfg.BatchSize <= 0, gobot.DefaultMetricsBatchSize, cfg.BatchSize)
	cfg.FlushInterval = mathx.IF(cfg.FlushInterval <= 0, gobot.DefaultMetricsFlushInterval, cfg.FlushInterval)
	m := &Metrics{cfg: cfg, db: db}
	m.processor = syncx.NewBatchProcessor(
		cfg.BatchSize*queueSizeRatio, cfg.BatchSize, cfg.FlushInterval, m.flushBatch,
		syncx.WithBatchProcessorName[gobot.SendStat]("gobot-metrics-clickhouse"),
	)
	return m, nil
}

// RecordSend 实现 gobot.Metrics：非阻塞入队，队列满时该事件被丢弃
func (m *Metrics) RecordSend(_ context.Context, stat gobot.SendStat) {
	m.processor.Submit(stat)
}

// Stop 停止后台聚合并落盘剩余事件；连接池归接入方管理，此处不关闭
func (m *Metrics) Stop() {
	m.processor.Stop()
}

// FailedBatches 返回落盘失败的批次累计数（网络故障或表不存在等），
// 用于观测统计路径的健康状况，失败批次已被丢弃且不会重试
func (m *Metrics) FailedBatches() int64 {
	return m.failedBatches.Load()
}

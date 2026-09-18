/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 08:12:36
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 08:53:21
* @FilePath: \go-bot\metrics\clickhouse\clickhouse.go
* @Description: ClickHouse 统计后端的构造与配置
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// Package clickhouse 提供 gobot.Metrics 的 ClickHouse 实现：
// 发送事件先进进程内队列，由后台聚合器按批或定时组装 INSERT 落盘；
// 底层连接复用 gorm + clickhouse-go 的连接池（与 dashboard 服务同一模式）；
// 队列满或落盘失败时丢弃该批，统计路径绝不阻塞、绝不影响发送主流程
package clickhouse

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	gobot "github.com/kamalyes/go-bot"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
	"gorm.io/gorm"
)

// DefaultDatabase 是缺省库名
const DefaultDatabase = "default"

// DefaultTable 是缺省表名，与 schema.sql 中 bot_send_events 对应
const DefaultTable = "bot_send_events"

// queueSizeRatio 是内部队列容量相对批大小的倍数，为突发流量留缓冲
const queueSizeRatio = 8

// Config 配置 ClickHouse 统计后端，Addr 必填
type Config struct {
	// Addr 是 ClickHouse native TCP 地址列表（形如 host:9000），必填，
	// 多地址时驱动自动负载均衡
	Addr []string
	// Database 缺省 default
	Database string
	// Table 缺省 bot_send_events
	Table string
	// Username 缺省 default
	Username string
	// Password 可空
	Password string
	// Secure 启用 TLS 连接
	Secure bool
	// DialTimeout 是建连超时，缺省 gobot.DefaultHTTPTimeout
	DialTimeout time.Duration
	// BatchSize 是单批最大事件数，缺省 gobot.DefaultMetricsBatchSize
	BatchSize int
	// FlushInterval 是最长落盘间隔，缺省 gobot.DefaultMetricsFlushInterval
	FlushInterval time.Duration
	// MaxOpenConns 是连接池上限，零值用驱动默认
	MaxOpenConns int
	// MaxIdleConns 是空闲连接上限，零值用驱动默认
	MaxIdleConns int
	// ConnMaxLifetime 是连接最长复用时长，零值用驱动默认
	ConnMaxLifetime time.Duration
}

// Metrics 实现 gobot.Metrics：把 SendStat 批量聚合后异步写入 ClickHouse
// RecordSend 非阻塞；关停时调用 Close 落盘剩余事件并归还连接池
// Metrics 可被多个 goroutine 安全并发使用
type Metrics struct {
	cfg           Config
	db            *gorm.DB
	processor     *syncx.BatchProcessor[gobot.SendStat]
	failedBatches atomic.Int64
}

// New 按配置创建连接池并启动后台聚合，Addr 为空或建连失败时返回错误
func New(cfg Config) (*Metrics, error) {
	const op = "New"
	if len(cfg.Addr) == 0 || strings.TrimSpace(cfg.Addr[0]) == "" {
		return nil, gobot.NewValidationError(op, "clickhouse addr is required")
	}
	cfg.Database = mathx.IfNotEmpty(cfg.Database, DefaultDatabase)
	cfg.Table = mathx.IfNotEmpty(cfg.Table, DefaultTable)
	cfg.Username = mathx.IfNotEmpty(cfg.Username, "default")
	cfg.DialTimeout = mathx.IF(cfg.DialTimeout <= 0, gobot.DefaultHTTPTimeout, cfg.DialTimeout)
	cfg.BatchSize = mathx.IF(cfg.BatchSize <= 0, gobot.DefaultMetricsBatchSize, cfg.BatchSize)
	cfg.FlushInterval = mathx.IF(cfg.FlushInterval <= 0, gobot.DefaultMetricsFlushInterval, cfg.FlushInterval)
	db, err := openGorm(cfg)
	if err != nil {
		return nil, err
	}
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

// Close 停止后台聚合并落盘剩余事件，随后关闭连接池
func (m *Metrics) Close() {
	m.processor.Stop()
	if sqlDB, err := m.db.DB(); err == nil {
		sqlDB.Close()
	}
}

// FailedBatches 返回落盘失败的批次累计数（网络故障或表不存在等），
// 用于观测统计路径的健康状况，失败批次已被丢弃且不会重试
func (m *Metrics) FailedBatches() int64 {
	return m.failedBatches.Load()
}

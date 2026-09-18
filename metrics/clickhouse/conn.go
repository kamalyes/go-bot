/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 08:21:52
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 08:58:07
* @FilePath: \go-bot\metrics\clickhouse\conn.go
* @Description: ClickHouse 连接池构建：clickhouse-go 原生连接注入 gorm 驱动
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package clickhouse

import (
	"crypto/tls"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	gormch "gorm.io/driver/clickhouse"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// openGorm 建立带连接池的 gorm 连接：
// clickhouse.OpenDB 基于 Options 直接构建 *sql.DB（连接池参数随 Options 生效），
// 再注入 gorm clickhouse 驱动，与 dashboard 服务同一连接模式
func openGorm(cfg Config) (*gorm.DB, error) {
	sqlDB := ch.OpenDB(buildOptions(cfg))
	return gorm.Open(gormch.New(gormch.Config{Conn: sqlDB}), &gorm.Config{
		// SDK 不外泄 SQL 日志，接入方的日志体系由其自身负责
		Logger: gormlogger.Discard,
	})
}

// buildOptions 把 Config 翻译为 clickhouse-go 的原生连接选项
func buildOptions(cfg Config) *ch.Options {
	opts := &ch.Options{
		Addr: cfg.Addr,
		Auth: ch.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Compression:     &ch.Compression{Method: ch.CompressionLZ4},
		DialTimeout:     cfg.DialTimeout,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
		// 固定会话时区为 UTC：ts 列是 naive DateTime64(3)（无时区），
		// 驱动以会话时区解释该列，不显式设置会取服务器默认值，
		// 导致写入与查询发生时区错位；setting 名必须是 session_timezone（CH 22.x+）
		Settings: ch.Settings{
			"session_timezone": "UTC",
		},
	}
	if cfg.Secure {
		opts.TLS = &tls.Config{InsecureSkipVerify: true}
	}
	return opts
}

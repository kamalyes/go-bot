/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-15 08:23:17
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 22:56:36
* @FilePath: \go-bot\constants.go
* @Description: 平台常量与默认值定义
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// Package gobot 提供统一的多平台机器人消息发送能力，
// 支持 Telegram、Lark（飞书）与钉钉，并可通过实现 Adapter 接入更多平台
// 所有暴露能力均为平台无关的通用 API；Bot 可安全并发使用
package gobot

import "time"

// Platform 标识一个受支持的消息平台，由各 adapter 通过 Adapter.Platform 返回
type Platform string

const (
	// PlatformTelegram 标识 telegram 包提供的 Telegram Bot API 适配器
	PlatformTelegram Platform = "telegram"
	// PlatformLark 标识 lark 包提供的 Lark（飞书）应用机器人适配器
	PlatformLark Platform = "lark"
	// PlatformDingtalk 标识 dingtalk 包提供的钉钉自定义机器人适配器
	PlatformDingtalk Platform = "dingtalk"
)

const (
	// DefaultHTTPTimeout 是 adapter 未自定义 HTTP 客户端时的单次请求超时
	DefaultHTTPTimeout = 10 * time.Second
	// DefaultUploadTimeout 是图片上传/下载等大流量请求的默认超时
	DefaultUploadTimeout = 30 * time.Second
	// DefaultRetryInitialDelay 是首次重试的基础退避时长
	DefaultRetryInitialDelay = 200 * time.Millisecond
	// DefaultRetryMaxDelay 限制本地指数退避的单次上限
	DefaultRetryMaxDelay = 3 * time.Second
	// DefaultRetryAfterLimit 是尊重服务端 Retry-After 时的安全上限，
	// 防止异常巨大的 Retry-After 在无 deadline 的 ctx 下造成近乎永久的阻塞
	DefaultRetryAfterLimit = 30 * time.Second
	// DefaultMetricsBatchSize 是 ClickHouse 统计的默认批量条数
	DefaultMetricsBatchSize = 500
	// DefaultMetricsFlushInterval 是 ClickHouse 统计的默认批量落盘间隔
	DefaultMetricsFlushInterval = 5 * time.Second
	// DefaultBatchConcurrency 是 SendBatch 批量发送的并发上限
	// 一次向大量目标投递时限制在途请求数，限流交给重试链消化
	DefaultBatchConcurrency = 8
)

// GetDefaultHTTPTimeout 等默认值以函数形式暴露，便于测试与运行期统一调整
func GetDefaultHTTPTimeout() time.Duration { return DefaultHTTPTimeout }

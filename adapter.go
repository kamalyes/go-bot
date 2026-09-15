/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-15 15:12:37
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-15 19:38:52
* @FilePath: \go-bot\adapter.go
* @Description: 平台适配器 SPI 与发送结果定义
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import "context"

// SendResult 是一次成功发送的平台侧结果
type SendResult struct {
	// MessageID 是平台返回的消息标识，可为空（平台未提供时）
	MessageID string
}

// Adapter 是平台适配器 SPI：只负责把通用消息翻译为平台协议，
// 重试/限流/静音/统计等通用能力由核心层的 Bot 统一包装，adapter 无需关心
// 实现必须可被多个 goroutine 安全并发使用
type Adapter interface {
	// Platform 返回该 adapter 的平台标识
	Platform() Platform
	// Send 向 target 投递一条消息，返回平台侧结果
	// 失败时返回 *gobot.Error 并正确标记 Retryable，供核心层决定是否重试
	Send(ctx context.Context, target Target, msg *Message) (*SendResult, error)
	// Close 释放 adapter 持有的资源；无资源时返回 nil
	Close(ctx context.Context) error
}

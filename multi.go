/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-16 15:23:51
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 21:05:37
* @FilePath: \go-bot\multi.go
* @Description: 多 Bot 并发广播器，Send 支持多目标，聚合错误
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import (
	"context"
	"errors"
	"sync"
)

// Multi 把消息并发广播到多个 Bot（各自保留完整的通用能力链）
// 单个 Bot 快速路径不引入 goroutine；多个 Bot 时聚合所有错误
type Multi struct {
	bots []*Bot
}

// NewMulti 创建一个多 Bot 分发器，至少需要一个 Bot
func NewMulti(bots ...*Bot) (*Multi, error) {
	if len(bots) == 0 {
		return nil, NewValidationError("NewMulti", "at least one bot is required")
	}
	snapshot := make([]*Bot, len(bots))
	for i, bot := range bots {
		if bot == nil {
			return nil, NewValidationError("NewMulti", "bot at index is nil")
		}
		snapshot[i] = bot
	}
	return &Multi{bots: snapshot}, nil
}

// Send 把一条消息发送到所有 Bot 的一到多个目标，用于跨平台通知与批量告警
// Bot 之间并发扇出，单个 Bot 内部多目标再按 DefaultBatchConcurrency 并发；
// 结果按 Bot 顺序返回二维切片：results[botIndex][targetIndex]，失败位置为 nil
// 空目标列表返回校验错误；错误用 errors.Join 聚合，全部成功时为 nil
func (m *Multi) Send(ctx context.Context, msg *Message, targets ...Target) ([][]*SendResult, error) {
	if len(m.bots) == 1 {
		res, err := m.bots[0].Send(ctx, msg, targets...)
		return [][]*SendResult{res}, err
	}

	results := make([][]*SendResult, len(m.bots))
	errs := make([]error, len(m.bots))
	var wg sync.WaitGroup
	for i, bot := range m.bots {
		wg.Go(func() {
			results[i], errs[i] = bot.Send(ctx, msg, targets...)
		})
	}
	wg.Wait()
	return results, errors.Join(errs...)
}

// SendText 是 Send 的纯文本便捷形式
func (m *Multi) SendText(ctx context.Context, text string, targets ...Target) ([][]*SendResult, error) {
	return m.Send(ctx, Text(text), targets...)
}

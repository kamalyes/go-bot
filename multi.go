/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-16 15:23:51
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-16 20:38:07
* @FilePath: \go-bot\multi.go
* @Description: 多 Bot 并发广播器，聚合各 Bot 的通用能力链与错误
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

// Send 把一条消息并发发送到所有 Bot，返回按 Bot 顺序排列的结果与聚合错误
// 结果切片中失败位置为 nil；全部成功时错误为 nil
func (m *Multi) Send(ctx context.Context, target Target, msg *Message) ([]*SendResult, error) {
	if len(m.bots) == 1 {
		res, err := m.bots[0].Send(ctx, target, msg)
		return []*SendResult{res}, err
	}

	results := make([]*SendResult, len(m.bots))
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	for i, bot := range m.bots {
		wg.Go(func() {
			res, err := bot.Send(ctx, target, msg)
			results[i] = res
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return results, errors.Join(errs...)
}

// SendText 是 Send 的纯文本便捷形式
func (m *Multi) SendText(ctx context.Context, target Target, text string) ([]*SendResult, error) {
	return m.Send(ctx, target, Text(text))
}

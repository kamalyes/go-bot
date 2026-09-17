/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-17 20:21:36
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-17 20:58:52
* @FilePath: \go-bot\telegram\send.go
* @Description: Telegram 消息发送流程
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
*/

package telegram

import (
	"context"
	"strconv"

	gobot "github.com/kamalyes/go-bot"
)

// Send 实现 gobot.Adapter，按消息类型选择 sendMessage 或 sendPhoto
func (a *Adapter) Send(ctx context.Context, target gobot.Target, msg *gobot.Message) (*gobot.SendResult, error) {
	const op = "Send"
	switch msg.Type {
	case gobot.MsgTypeText:
		return a.sendMessage(ctx, op, target.ID, a.renderText(msg))
	case gobot.MsgTypeMarkdown:
		return a.sendMessage(ctx, op, target.ID, a.renderMarkdown(msg))
	case gobot.MsgTypeImage:
		return a.sendPhoto(ctx, op, target, msg)
	default:
		return nil, gobot.NewValidationError(op, "unsupported message type: "+string(msg.Type))
	}
}

// sendMessage 调用 sendMessage 投递文本形态的消息
func (a *Adapter) sendMessage(ctx context.Context, op, chatID, text string) (*gobot.SendResult, error) {
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	if a.cfg.ParseMode != "" {
		payload["parse_mode"] = a.cfg.ParseMode
	}
	result, err := callAPI[messageResult](a, ctx, op, "sendMessage", payload)
	if err != nil {
		return nil, err
	}
	return &gobot.SendResult{MessageID: strconv.FormatInt(result.MessageID, 10)}, nil
}

// sendPhoto 调用 sendPhoto 按 URL 投递图片，Text 作为图片说明
func (a *Adapter) sendPhoto(ctx context.Context, op string, target gobot.Target, msg *gobot.Message) (*gobot.SendResult, error) {
	payload := map[string]any{
		"chat_id": target.ID,
		"photo":   msg.ImageURL,
	}
	if msg.Text != "" {
		payload["caption"] = msg.Text
	}
	result, err := callAPI[messageResult](a, ctx, op, "sendPhoto", payload)
	if err != nil {
		return nil, err
	}
	return &gobot.SendResult{MessageID: strconv.FormatInt(result.MessageID, 10)}, nil
}

// messageResult 是 sendMessage / sendPhoto 返回的消息标识，仅用于解码
type messageResult struct {
	MessageID int64 `json:"message_id"`
}

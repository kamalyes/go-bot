/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-10-19 00:00:00
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-10-19 21:36:00
* @FilePath: \go-bot\lark\app_send.go
* @Description: Lark 应用形态消息发送：经 im/v1/messages 按会话或用户投递
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lark

import (
	"context"

	gobot "github.com/kamalyes/go-bot"
)

// Send 实现 gobot.Adapter：text 直发、markdown 组装 interactive 卡片；
// 应用形态不支持图片消息，收到 image 消息时返回校验错误；
// chat 目标映射 chat_id、user 目标映射 open_id
func (a *AppAdapter) Send(ctx context.Context, target gobot.Target, msg *gobot.Message) (*gobot.SendResult, error) {
	const op = "Send"
	var receiveIDType string
	switch target.Type {
	case gobot.TargetChat:
		receiveIDType = "chat_id"
	case gobot.TargetUser:
		receiveIDType = "open_id"
	default:
		return nil, gobot.NewValidationError(op, "unsupported target type: "+string(target.Type))
	}
	var content map[string]any
	var msgType string
	switch msg.Type {
	case gobot.MsgTypeText:
		msgType = "text"
		content = map[string]any{"text": renderMentions(msg.Text, msg)}
	case gobot.MsgTypeMarkdown:
		msgType = "interactive"
		content = markdownCard(msg)
	case gobot.MsgTypeImage:
		return nil, gobot.NewValidationError(op, "app bot cannot send image messages")
	default:
		return nil, gobot.NewValidationError(op, "unsupported message type: "+string(msg.Type))
	}
	var result struct {
		MessageID string `json:"message_id"`
	}
	err := a.callApp(ctx, op, httpMethodPost, "/open-apis/im/v1/messages",
		map[string]string{"receive_id_type": receiveIDType},
		map[string]any{
			"receive_id": target.ID,
			"msg_type":   msgType,
			"content":    content,
		}, &result)
	if err != nil {
		return nil, err
	}
	return &gobot.SendResult{MessageID: result.MessageID}, nil
}

// httpMethodPost 标识 callApp 按 POST 请求
const httpMethodPost = "POST"

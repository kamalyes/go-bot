/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-17 20:35:12
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-17 20:55:38
* @FilePath: \go-bot\lark\send.go
* @Description: Lark webhook 推送：消息发送与响应解码
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lark

import (
	"context"
	"encoding/json"
	"time"

	gobot "github.com/kamalyes/go-bot"
)

// Send 实现 gobot.Adapter：text 直发、markdown 组装 interactive 卡片；
// webhook 形态不支持图片消息，收到 image 消息时返回校验错误；
// 平台不回传消息标识，成功时 MessageID 为空
func (a *Adapter) Send(ctx context.Context, _ gobot.Target, msg *gobot.Message) (*gobot.SendResult, error) {
	const op = "Send"
	var payload map[string]any
	switch msg.Type {
	case gobot.MsgTypeText:
		payload = map[string]any{
			"msg_type": "text",
			"content":  map[string]string{"text": renderMentions(msg.Text, msg)},
		}
	case gobot.MsgTypeMarkdown:
		payload = map[string]any{"msg_type": "interactive", "card": markdownCard(msg)}
	case gobot.MsgTypeImage:
		return nil, gobot.NewValidationError(op, "webhook bot cannot send image messages")
	default:
		return nil, gobot.NewValidationError(op, "unsupported message type: "+string(msg.Type))
	}
	if a.cfg.Secret != "" {
		timestamp, signature := sign(a.cfg.Secret, time.Now())
		payload["timestamp"] = timestamp
		payload["sign"] = signature
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, gobot.NewValidationError(op, "encode payload failed")
	}
	if len(data) > maxPayloadSize {
		return nil, gobot.NewValidationError(op, "payload exceeds 20 KB limit")
	}
	resp, err := a.client.Post(a.cfg.APIBase + webhookPathPrefix + a.cfg.Token).
		WithContext(ctx).
		SetBodyRaw(data).
		Send()
	if err != nil {
		return nil, gobot.NewTransportError(gobot.PlatformLark, op, err)
	}
	body, err := resp.Bytes()
	if err != nil {
		return nil, gobot.NewTransportError(gobot.PlatformLark, op, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, gobot.NewHTTPError(gobot.PlatformLark, op, resp.StatusCode, string(body), 0)
	}
	var result webhookResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, gobot.NewDecodeError(gobot.PlatformLark, op, err)
	}
	if result.Code != 0 {
		return nil, gobot.NewPlatformError(gobot.PlatformLark, op, result.Code, result.Msg, result.Code == CodeRateLimited)
	}
	return &gobot.SendResult{}, nil
}

// webhookResult 是自定义机器人响应的原始 JSON 结构，仅用于解码：
// 成功 Code 为 0；19021 签名失败，9499 token 失效或参数错误，
// 限流时 Msg 固定为 too many request
type webhookResult struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

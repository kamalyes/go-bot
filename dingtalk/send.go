/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 10:26:17
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 18:00:16
* @FilePath: \go-bot\dingtalk\send.go
* @Description: 钉钉 webhook 推送：消息发送与响应解码
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package dingtalk

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	gobot "github.com/kamalyes/go-bot"
	"github.com/kamalyes/go-toolbox/pkg/httpx"
)

// Send 实现 gobot.Adapter：text 直发、markdown 降级后发送；
// webhook 形态不支持图片消息，收到 image 消息时返回校验错误；
// 平台不回传消息标识，成功时 MessageID 为空
func (a *Adapter) Send(ctx context.Context, _ gobot.Target, msg *gobot.Message) (*gobot.SendResult, error) {
	const op = "Send"
	var payload map[string]any
	switch msg.Type {
	case gobot.MsgTypeText:
		payload = textPayload(msg)
	case gobot.MsgTypeMarkdown:
		payload = markdownPayload(msg)
	case gobot.MsgTypeImage:
		return nil, gobot.NewValidationError(op, "webhook bot cannot send image messages")
	default:
		return nil, gobot.NewValidationError(op, "unsupported message type: "+string(msg.Type))
	}
	// payload 的值均为 string 与 map 的组合，Marshal 不存在失败路径
	data, _ := json.Marshal(payload)
	if len(data) > maxPayloadSize {
		return nil, gobot.NewValidationError(op, "payload exceeds 20 KB limit")
	}
	query := url.Values{}
	query.Set("access_token", a.cfg.Token)
	if a.cfg.Secret != "" {
		timestamp, signature := sign(a.cfg.Secret, time.Now())
		query.Set("timestamp", timestamp)
		query.Set("sign", signature)
	}
	// 钉钉强校验 Content-Type 为 application/json，缺失时返回 errcode 43004
	resp, err := a.client.Post(a.cfg.APIBase + webhookPath + "?" + query.Encode()).
		WithContext(ctx).
		SetContentType(httpx.ContentTypeApplicationJSON).
		SetBodyRaw(data).
		Send()
	if err != nil {
		return nil, gobot.NewTransportError(gobot.PlatformDingtalk, op, err)
	}
	body, err := resp.Bytes()
	if err != nil {
		return nil, gobot.NewTransportError(gobot.PlatformDingtalk, op, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, gobot.NewHTTPError(gobot.PlatformDingtalk, op, resp.StatusCode, string(body), 0)
	}
	var result webhookResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, gobot.NewDecodeError(gobot.PlatformDingtalk, op, err)
	}
	// 平台风控限流为每分钟 20 条、超限封禁 10 分钟，本地立即重试无意义，
	// 所有业务码失败统一按不可重试处理
	if result.ErrCode != 0 {
		return nil, gobot.NewPlatformError(gobot.PlatformDingtalk, op, result.ErrCode, result.ErrMsg, false)
	}
	return &gobot.SendResult{}, nil
}

// webhookResult 是自定义机器人响应的原始 JSON 结构，仅用于解码：
// 成功 ErrCode 为 0，非 0 时 ErrMsg 携带平台侧错误说明
type webhookResult struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

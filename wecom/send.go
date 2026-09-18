/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-19 15:02:37
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-19 15:28:53
* @FilePath: \go-bot\wecom\send.go
* @Description: 企微 webhook 推送：消息发送与响应解码
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package wecom

import (
	"context"
	"encoding/json"
	"net/url"

	gobot "github.com/kamalyes/go-bot"
	"github.com/kamalyes/go-toolbox/pkg/httpx"
)

// Send 实现 gobot.Adapter：text 直发（@ 进 mentioned_list）、
// markdown 降级后发送（平台 markdown 类型不支持 @，携带提及时返回校验错误）、
// image 下载转 base64+md5 后发送；
// 平台不回传消息标识，成功时 MessageID 为空
func (a *Adapter) Send(ctx context.Context, _ gobot.Target, msg *gobot.Message) (*gobot.SendResult, error) {
	const op = "Send"
	var payload map[string]any
	switch msg.Type {
	case gobot.MsgTypeText:
		if len(msg.Text) > maxTextBytes {
			return nil, gobot.NewValidationError(op, "text content exceeds 2048-byte limit")
		}
		payload = textPayload(msg)
	case gobot.MsgTypeMarkdown:
		if msg.HasAt() {
			return nil, gobot.NewValidationError(op, "wecom markdown messages cannot mention users")
		}
		body := markdownToWecomMarkdown(msg.Text)
		if len(body) > maxMarkdownBytes {
			return nil, gobot.NewValidationError(op, "markdown content exceeds 4096-byte limit")
		}
		payload = markdownPayload(body)
	case gobot.MsgTypeImage:
		var err error
		if payload, err = a.imagePayload(ctx, op, msg); err != nil {
			return nil, err
		}
	default:
		return nil, gobot.NewValidationError(op, "unsupported message type: "+string(msg.Type))
	}
	// payload 的值均为 string/map/[]string 的组合，Marshal 不存在失败路径
	data, _ := json.Marshal(payload)
	query := url.Values{}
	query.Set("key", a.cfg.Key)
	// 企微强校验 Content-Type 为 application/json
	resp, err := a.client.Post(a.cfg.APIBase + webhookPath + "?" + query.Encode()).
		WithContext(ctx).
		SetContentType(httpx.ContentTypeApplicationJSON).
		SetBodyRaw(data).
		Send()
	if err != nil {
		return nil, gobot.NewTransportError(gobot.PlatformWecom, op, err)
	}
	body, err := resp.Bytes()
	if err != nil {
		return nil, gobot.NewTransportError(gobot.PlatformWecom, op, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, gobot.NewHTTPError(gobot.PlatformWecom, op, resp.StatusCode, string(body), 0)
	}
	var result webhookResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, gobot.NewDecodeError(gobot.PlatformWecom, op, err)
	}
	// 平台风控限流为每分钟 20 条、超限封禁 10 分钟，本地立即重试无意义，
	// 所有业务码失败统一按不可重试处理
	if result.ErrCode != 0 {
		return nil, gobot.NewPlatformError(gobot.PlatformWecom, op, result.ErrCode, result.ErrMsg, false)
	}
	return &gobot.SendResult{}, nil
}

// webhookResult 是群机器人响应的原始 JSON 结构，仅用于解码：
// 成功 ErrCode 为 0，非 0 时 ErrMsg 携带平台侧错误说明
type webhookResult struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

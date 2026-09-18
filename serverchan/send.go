/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-19 09:52:19
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-19 10:02:37
* @FilePath: \go-bot\serverchan\send.go
* @Description: Server酱推送：表单投递与响应解码
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package serverchan

import (
	"context"
	"encoding/json"
	"net/url"

	gobot "github.com/kamalyes/go-bot"
)

// Send 实现 gobot.Adapter：text/markdown 均以 title + desp 表单透传
// （desp 由平台以 markdown 渲染，无需本地降级）；
// 个人推送无图片与 @ 概念，收到 image 消息或携带提及时返回校验错误；
// 成功时把平台回传的 pushid 作为 MessageID 返回
func (a *Adapter) Send(ctx context.Context, _ gobot.Target, msg *gobot.Message) (*gobot.SendResult, error) {
	const op = "Send"
	if msg.HasAt() {
		return nil, gobot.NewValidationError(op, "serverchan push cannot mention users")
	}
	switch msg.Type {
	case gobot.MsgTypeText, gobot.MsgTypeMarkdown:
	case gobot.MsgTypeImage:
		return nil, gobot.NewValidationError(op, "serverchan push cannot send image messages")
	default:
		return nil, gobot.NewValidationError(op, "unsupported message type: "+string(msg.Type))
	}
	form := url.Values{}
	form.Set("title", titleOf(msg))
	form.Set("desp", msg.Text)
	// 推送接口按表单编码提交，SetBodyForm 自动携带 application/x-www-form-urlencoded
	resp, err := a.client.Post(a.cfg.APIBase + "/" + a.cfg.SendKey + ".send").
		WithContext(ctx).
		SetBodyForm(form).
		Send()
	if err != nil {
		return nil, gobot.NewTransportError(gobot.PlatformServerchan, op, err)
	}
	body, err := resp.Bytes()
	if err != nil {
		return nil, gobot.NewTransportError(gobot.PlatformServerchan, op, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, gobot.NewHTTPError(gobot.PlatformServerchan, op, resp.StatusCode, string(body), 0)
	}
	var result pushResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, gobot.NewDecodeError(gobot.PlatformServerchan, op, err)
	}
	// 免费额度每日 5 条，超限当日不恢复，本地立即重试无意义，
	// 业务码失败统一按不可重试处理
	if result.Code != 0 {
		return nil, gobot.NewPlatformError(gobot.PlatformServerchan, op, result.Code, result.Message, false)
	}
	return &gobot.SendResult{MessageID: result.Data.PushID}, nil
}

// pushResult 是推送响应的原始 JSON 结构，仅用于解码：
// 成功 Code 为 0 且 Data.PushID 为平台侧推送标识，非 0 时 Message 携带错误说明
type pushResult struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		PushID string `json:"pushid"`
	} `json:"data"`
}

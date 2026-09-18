/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-19 13:28:51
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-19 13:52:07
* @FilePath: \go-bot\wecom\payload.go
* @Description: 企微消息体构造：@ 提及映射、markdown 组装与图片转码
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package wecom

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"

	gobot "github.com/kamalyes/go-bot"
)

// textPayload 组装 text 消息体：正文进 text.content，
// @ 提及映射为 mentioned_list（@ 全员为平台保留字 @all，其余按 AtUserIDs 逐个携带）；
// 无任何提及时省略 mentioned_list 字段
func textPayload(msg *gobot.Message) map[string]any {
	text := map[string]any{"content": msg.Text}
	if msg.HasAt() {
		mentioned := make([]string, 0, len(msg.AtUserIDs)+1)
		if msg.MentionAll {
			mentioned = append(mentioned, "@all")
		}
		mentioned = append(mentioned, msg.AtUserIDs...)
		if len(mentioned) > 0 {
			text["mentioned_list"] = mentioned
		}
	}
	return map[string]any{"msgtype": "text", "text": text}
}

// markdownPayload 组装 markdown 消息体：正文先降级为企微支持的语法子集；
// 平台的 markdown 类型不提供 @ 字段，携带提及的消息在 Send 中提前拒绝
func markdownPayload(body string) map[string]any {
	return map[string]any{
		"msgtype":  "markdown",
		"markdown": map[string]string{"content": body},
	}
}

// imagePayload 下载图片地址并组装 image 消息体：
// 企微 webhook 仅接受 base64 + md5 的二进制形态，URL 由适配器代为拉取；
// 下载失败返回校验错误（地址不可达属调用方输入问题），网络异常返回传输错误
func (a *Adapter) imagePayload(ctx context.Context, op string, msg *gobot.Message) (map[string]any, error) {
	resp, err := a.client.Get(msg.ImageURL).WithContext(ctx).Send()
	if err != nil {
		return nil, gobot.NewTransportError(gobot.PlatformWecom, op, err)
	}
	body, err := resp.Bytes()
	if err != nil {
		return nil, gobot.NewTransportError(gobot.PlatformWecom, op, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 || len(body) == 0 {
		return nil, gobot.NewValidationError(op, "image url is not reachable: "+msg.ImageURL)
	}
	encoded := base64.StdEncoding.EncodeToString(body)
	if len(encoded) > maxImageBase64Bytes {
		return nil, gobot.NewValidationError(op, "image exceeds 2 MB base64 limit")
	}
	sum := md5.Sum(body)
	return map[string]any{
		"msgtype": "image",
		"image": map[string]string{
			"base64": encoded,
			"md5":    hex.EncodeToString(sum[:]),
		},
	}, nil
}

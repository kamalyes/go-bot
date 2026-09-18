/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 13:56:36
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 17:00:36
* @FilePath: \go-bot\dingtalk\payload.go
* @Description: 钉钉消息体构造：@ 提及字段与 text/markdown 消息组装
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package dingtalk

import (
	"strings"

	gobot "github.com/kamalyes/go-bot"
)

// atField 构造 @ 字段：AtUserIDs 映射为 atUserIds（平台侧限制最多 @ 50 人），
// MentionAll 映射为 isAtAll；无任何提及时返回 nil，省略 at 字段
func atField(msg *gobot.Message) map[string]any {
	if !msg.HasAt() {
		return nil
	}
	at := map[string]any{"isAtAll": msg.MentionAll}
	if len(msg.AtUserIDs) > 0 {
		at["atUserIds"] = msg.AtUserIDs
	}
	return at
}

// textPayload 组装 text 消息体：正文进 text.content，@ 进 at 字段
func textPayload(msg *gobot.Message) map[string]any {
	payload := map[string]any{
		"msgtype": "text",
		"text":    map[string]string{"content": msg.Text},
	}
	if at := atField(msg); at != nil {
		payload["at"] = at
	}
	return payload
}

// markdownPayload 组装 markdown 消息体：正文先降级为钉钉支持的语法子集，
// 会话列表标题取消息 Title，为空时截取正文首个非空行兜底
func markdownPayload(msg *gobot.Message) map[string]any {
	body := markdownToDingtalkMarkdown(msg.Text)
	payload := map[string]any{
		"msgtype":  "markdown",
		"markdown": map[string]string{"title": markdownTitle(msg, body), "text": body},
	}
	if at := atField(msg); at != nil {
		payload["at"] = at
	}
	return payload
}

// markdownTitle 取 markdown 消息的会话列表标题：
// Title 非空直接使用，否则取正文首个非空行（标题语法符号按原文保留），
// 正文全空时兜底为固定文案
func markdownTitle(msg *gobot.Message, body string) string {
	if msg.Title != "" {
		return msg.Title
	}
	for _, line := range strings.Split(body, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return "消息通知"
}

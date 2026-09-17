/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-17 15:21:53
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-17 20:26:38
* @FilePath: \go-bot\lark\payload.go
* @Description: Lark 消息体构造：@ 提及渲染与 markdown 卡片组装
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lark

import (
	"strings"

	gobot "github.com/kamalyes/go-bot"
)

// renderMentions 把 @ 提及以 Lark 的 <at> 标签形式追加到文本尾部：
// MentionAll 使用平台保留字 all，其余按 AtUserIDs 逐个生成
func renderMentions(text string, msg *gobot.Message) string {
	if !msg.HasAt() {
		return text
	}
	var b strings.Builder
	b.WriteString(text)
	if msg.MentionAll {
		b.WriteString("\n<at user_id=\"all\"></at>")
	}
	for _, id := range msg.AtUserIDs {
		b.WriteString("\n<at user_id=\"")
		b.WriteString(id)
		b.WriteString("\"></at>")
	}
	return b.String()
}

// markdownCard 把 markdown 消息组装为 interactive 卡片：
// 标题进卡片 header（仅非空时），正文进 markdown 元素，@ 提及追加在正文尾部
func markdownCard(msg *gobot.Message) map[string]any {
	card := map[string]any{
		"elements": []any{
			map[string]any{"tag": "markdown", "content": renderMentions(msg.Text, msg)},
		},
	}
	if msg.Title != "" {
		card["config"] = map[string]any{"wide_screen_mode": true}
		card["header"] = map[string]any{
			"title": map[string]any{"tag": "plain_text", "content": msg.Title},
		}
	}
	return card
}

/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-17 20:26:11
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-17 21:01:23
* @FilePath: \go-bot\telegram\payload.go
* @Description: Telegram 消息体渲染：正文组装与 @ 提及构造
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
*/

package telegram

import (
	"html"
	"strings"

	gobot "github.com/kamalyes/go-bot"
)

// renderText 渲染纯文本消息：正文加 @ 提及
func (a *Adapter) renderText(msg *gobot.Message) string {
	return a.renderMentions(msg.Text, msg)
}

// renderMarkdown 渲染 markdown 消息：HTML 模式下标题转为加粗行，正文原样投递
func (a *Adapter) renderMarkdown(msg *gobot.Message) string {
	text := msg.Text
	if msg.Title != "" && a.cfg.ParseMode == DefaultParseMode {
		text = "<b>" + html.EscapeString(msg.Title) + "</b>\n" + text
	} else if msg.Title != "" {
		text = msg.Title + "\n" + text
	}
	return a.renderMentions(text, msg)
}

// renderMentions 把 @ 提及追加到文本尾部：HTML 模式下生成可跳转的提及链接，
// 其他模式退化为普通 @ 文本；Telegram 无原生 @所有人，
// MentionAll 统一降级为字面 @everyone，不产生平台级提及
func (a *Adapter) renderMentions(text string, msg *gobot.Message) string {
	if !msg.HasAt() {
		return text
	}
	var b strings.Builder
	b.WriteString(text)
	if msg.MentionAll {
		b.WriteString("\n@everyone")
	}
	for _, id := range msg.AtUserIDs {
		b.WriteString("\n")
		if a.cfg.ParseMode != DefaultParseMode {
			b.WriteString("@")
			b.WriteString(id)
			continue
		}
		b.WriteString(`<a href="tg://user?id=`)
		b.WriteString(id)
		b.WriteString(`">@`)
		b.WriteString(id)
		b.WriteString(`</a>`)
	}
	return b.String()
}

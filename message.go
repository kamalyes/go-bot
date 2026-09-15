/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-15 08:35:26
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-15 09:38:12
* @FilePath: \go-bot\message.go
* @Description: 平台无关的通用消息模型与链式构造
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

// MessageType 标识通用消息模型支持的消息类型
type MessageType string

const (
	// MsgTypeText 纯文本消息
	MsgTypeText MessageType = "text"
	// MsgTypeMarkdown markdown 消息（可携带标题）
	MsgTypeMarkdown MessageType = "markdown"
	// MsgTypeImage 图片消息（当前以可公开访问的 URL 表示）
	MsgTypeImage MessageType = "image"
)

// Message 是平台无关的消息模型通过 Text/Markdown/Image 构造，
// 再用链式方法补充 @ 提及等修饰，最后交给 Bot.Send 发送
// Message 构造完成后可复用与并发读取，但 Send 不会修改它
type Message struct {
	// Type 是消息类型，决定 adapter 采用的平台消息形态
	Type MessageType
	// Title 是 markdown 消息的标题；平台不支持标题时并入正文
	Title string
	// Text 是 text 消息的正文，或 markdown 消息的内容
	Text string
	// ImageURL 是 image 消息的图片地址
	ImageURL string
	// MentionAll 指示是否 @所有人；仅群聊场景有效，不支持的平台降级为纯文本
	MentionAll bool
	// AtUserIDs 是需要 @ 的平台用户标识（TG 为数字 user id，Lark 为 open_id）
	AtUserIDs []string
}

// Text 构造一条纯文本消息
func Text(text string) *Message {
	return &Message{Type: MsgTypeText, Text: text}
}

// Markdown 构造一条 markdown 消息，title 可为空
func Markdown(title, content string) *Message {
	return &Message{Type: MsgTypeMarkdown, Title: title, Text: content}
}

// Image 构造一条图片消息，url 需可被对应平台访问
func Image(url string) *Message {
	return &Message{Type: MsgTypeImage, ImageURL: url}
}

// AtAll 链式启用 @所有人
func (m *Message) AtAll() *Message {
	m.MentionAll = true
	return m
}

// AtUsers 链式追加需要 @ 的用户标识
func (m *Message) AtUsers(ids ...string) *Message {
	m.AtUserIDs = append(m.AtUserIDs, ids...)
	return m
}

// HasAt 报告消息是否携带任何 @ 提及
func (m *Message) HasAt() bool { return m.MentionAll || len(m.AtUserIDs) > 0 }

// ContentSize 返回消息正文的字节量，供统计发送内容量使用
// 按类型取 text 或 image URL 的 UTF-8 字节数，@ 提及不计入
func (m *Message) ContentSize() int64 {
	switch m.Type {
	case MsgTypeImage:
		return int64(len(m.ImageURL))
	default:
		return int64(len(m.Text))
	}
}

// validate 在发送前对消息做本地校验，返回结构化校验错误
func (m *Message) validate() error {
	switch m.Type {
	case MsgTypeText:
		if m.Text == "" {
			return NewValidationError("", "text content is empty")
		}
	case MsgTypeMarkdown:
		if m.Text == "" {
			return NewValidationError("", "markdown content is empty")
		}
	case MsgTypeImage:
		if m.ImageURL == "" {
			return NewValidationError("", "image url is empty")
		}
	default:
		return NewValidationError("", "unsupported message type: "+string(m.Type))
	}
	return nil
}

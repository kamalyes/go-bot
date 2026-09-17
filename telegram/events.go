/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-17 08:26:53
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-17 09:50:31
* @FilePath: \go-bot\telegram\events.go
* @Description: Telegram 事件接收：getUpdates 长轮询与事件模型
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package telegram

import (
	"context"
)

// Event 是从 Telegram 平台收到的一条事件
// 事件不等于消息：编辑消息、成员变动、被移出群聊等同样是事件；
// 当前只展开消息类事件，其余类型的 Message 为 nil，由调用方按需跳过
// 订阅码等业务指令的匹配发生在 Message.Text
type Event struct {
	// EventID 是事件的唯一递增标识，消费后以最大值加一作为下次拉取的 Offset 游标
	EventID int64
	// Message 是事件携带的消息，非消息类事件为 nil
	Message *ChatMessage
}

// ChatMessage 是一条已收到消息的公开快照
type ChatMessage struct {
	// MessageID 是消息在会话内的标识
	MessageID int64
	// From 是发送者，频道发言等场景为 nil
	From *User
	// Chat 是消息所在会话的快照
	Chat Chat
	// Text 是消息文本，业务在此匹配订阅码等指令
	Text string
	// Date 是消息发送时间的 Unix 秒
	Date int64
}

// User 是消息发送者的公开信息
type User struct {
	// ID 是发送者的数值 user id，@ 提及时对应 gobot.Message.AtUserIDs 的字符串形态
	ID int64
	// FirstName 是名字
	FirstName string
	// LastName 是姓氏，可为空
	LastName string
	// Username 是公开用户名，可为空
	Username string
}

// EventOptions 控制一次事件拉取的行为
type EventOptions struct {
	// Offset 是已消费的最大 EventID 加一，零值表示从最旧未消费事件开始
	Offset int64
	// Limit 是单次拉取的事件上限，零值取平台默认（100）
	Limit int64
	// Timeout 是长轮询窗口秒数，零值表示立即返回；
	// 大于零时调用方需保证 ctx 的超时覆盖整个轮询窗口
	Timeout int64
}

// GetEvents 长轮询拉取一批平台事件，同时把出现过的会话记入已知列表
// 平台在轮询窗口内无新事件时返回空列表而非错误
func (a *Adapter) GetEvents(ctx context.Context, opts EventOptions) ([]Event, error) {
	const op = "GetEvents"
	payload := map[string]any{}
	if opts.Offset > 0 {
		payload["offset"] = opts.Offset
	}
	if opts.Limit > 0 {
		payload["limit"] = opts.Limit
	}
	if opts.Timeout > 0 {
		payload["timeout"] = opts.Timeout
	}
	results, err := callAPI[[]eventResult](a, ctx, op, "getUpdates", payload)
	if err != nil {
		return nil, err
	}
	events := make([]Event, 0, len(results))
	for _, r := range results {
		events = append(events, r.toEvent(a.chats))
	}
	return events, nil
}

// eventResult 是 getUpdates 响应 result 数组内单个事件的原始 JSON 结构，仅用于解码：
// 平台键名是 update_id，此处对齐公开模型命名为 EventID；
// Message 为 nil 表示当前未展开的事件类型，toEvent 会原样跳过
type eventResult struct {
	EventID int64              `json:"update_id"`
	Message *chatMessageResult `json:"message"`
}

// chatMessageResult 是事件内 message 字段的原始 JSON 结构，仅用于解码：
// 其中 Chat.ID 是数值型 chat_id，经 toChat 转为公开模型 Chat.ID 的字符串形态，
// 与 gobot.Target.ID 取同一表示，屏蔽两套 ID 形态
type chatMessageResult struct {
	MessageID int64      `json:"message_id"`
	From      *User      `json:"from"`
	Chat      chatResult `json:"chat"`
	Text      string     `json:"text"`
	Date      int64      `json:"date"`
}

// toEvent 把原始事件解码结构转为公开模型，并顺带把会话快照记入已知列表
func (r eventResult) toEvent(store *chatStore) Event {
	e := Event{EventID: r.EventID}
	if r.Message == nil {
		return e
	}
	chat := r.Message.Chat.toChat()
	e.Message = &ChatMessage{
		MessageID: r.Message.MessageID,
		From:      r.Message.From,
		Chat:      chat,
		Text:      r.Message.Text,
		Date:      r.Message.Date,
	}
	store.remember(chat)
	return e
}

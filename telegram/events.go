/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-17 08:26:53
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-17 09:50:31
* @FilePath: \go-bot\telegram\events.go
* @Description: Telegram 事件接收：getUpdates 长轮询、事件模型与机器人成员状态变化
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package telegram

import (
	"context"
)

// Event 是从 Telegram 平台收到的一条事件
// 事件不等于消息：编辑消息、成员变动、被移出群聊等同样是事件；
// 当前展开消息与机器人成员状态变化两类事件，其余类型 Message 与 MyChatMember 均为 nil，由调用方按需跳过
// 订阅码等业务指令的匹配发生在 Message.Text
type Event struct {
	// EventID 是事件的唯一递增标识，消费后以最大值加一作为下次拉取的 Offset 游标
	EventID int64
	// Message 是事件携带的消息，非消息类事件为 nil
	Message *ChatMessage
	// MyChatMember 是机器人自身在会话中的成员状态变化，非该类事件为 nil
	MyChatMember *ChatMemberUpdate
}

// ChatMemberStatus 标识机器人在会话中的成员状态，取值对齐平台
type ChatMemberStatus string

const (
	// ChatMemberStatusCreator 是群主
	ChatMemberStatusCreator ChatMemberStatus = "creator"
	// ChatMemberStatusAdministrator 是管理员
	ChatMemberStatusAdministrator ChatMemberStatus = "administrator"
	// ChatMemberStatusMember 是普通成员
	ChatMemberStatusMember ChatMemberStatus = "member"
	// ChatMemberStatusRestricted 是受限成员（仍在会话内）
	ChatMemberStatusRestricted ChatMemberStatus = "restricted"
	// ChatMemberStatusLeft 是已离开会话
	ChatMemberStatusLeft ChatMemberStatus = "left"
	// ChatMemberStatusKicked 是被移出会话
	ChatMemberStatusKicked ChatMemberStatus = "kicked"
)

// ChatMemberUpdate 是机器人自身在会话中的成员状态变化：
// 被拉入群、被提权/降权、被移出群或主动退群均触发，是群组实时感知的信号源
type ChatMemberUpdate struct {
	// Chat 是状态变化所在的会话快照
	Chat Chat
	// From 是触发状态变化的操作者，可为 nil
	From *User
	// Date 是状态变化的 Unix 秒
	Date int64
	// OldStatus 是变化前的成员状态
	OldStatus ChatMemberStatus
	// NewStatus 是变化后的成员状态
	NewStatus ChatMemberStatus
}

// InGroup 报告变化后机器人是否仍在会话内：
// creator/administrator/member/restricted 视为在会话内，left/kicked 视为不在
func (u *ChatMemberUpdate) InGroup() bool {
	switch u.NewStatus {
	case ChatMemberStatusCreator, ChatMemberStatusAdministrator, ChatMemberStatusMember, ChatMemberStatusRestricted:
		return true
	default:
		return false
	}
}

// Joined 报告本次变化是否为加入会话（此前不在、现在在），如被拉入新群
func (u *ChatMemberUpdate) Joined() bool { return !inGroupStatus(u.OldStatus) && u.InGroup() }

// Left 报告本次变化是否为离开会话（此前在、现在不在），如被移出群或主动退群
func (u *ChatMemberUpdate) Left() bool { return inGroupStatus(u.OldStatus) && !u.InGroup() }

// inGroupStatus 报告状态是否属于会话内
func inGroupStatus(s ChatMemberStatus) bool {
	u := ChatMemberUpdate{NewStatus: s}
	return u.InGroup()
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
	ID int64 `json:"id"`
	// FirstName 是名字
	FirstName string `json:"first_name"`
	// LastName 是姓氏，可为空
	LastName string `json:"last_name"`
	// Username 是公开用户名，可为空
	Username string `json:"username"`
}

// EventOptions 控制一次事件拉取的行为
type EventOptions struct {
	// Offset 是已消费的最大 EventID 加一，零值表示从最旧未消费事件开始；
	// 负值原样透传平台（-1 配 Limit 取最新一条，用于探测消费起点）
	Offset int64
	// Limit 是单次拉取的事件上限，零值取平台默认（100）
	Limit int64
	// Timeout 是长轮询窗口秒数，零值表示立即返回；
	// 大于零时调用方需保证 ctx 的超时覆盖整个轮询窗口
	Timeout int64
}

// GetEvents 长轮询拉取一批平台事件，同时把出现过的会话记入已知列表；
// 机器人成员状态变化会实时维护已知列表（加入记入、离开移除）
// 平台在轮询窗口内无新事件时返回空列表而非错误
func (a *Adapter) GetEvents(ctx context.Context, opts EventOptions) ([]Event, error) {
	const op = "GetEvents"
	payload := map[string]any{}
	if opts.Offset != 0 {
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
// Message 与 MyChatMember 均为 nil 表示当前未展开的事件类型，toEvent 会原样跳过
type eventResult struct {
	EventID      int64                   `json:"update_id"`
	Message      *chatMessageResult      `json:"message"`
	MyChatMember *chatMemberUpdateResult `json:"my_chat_member"`
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

// toEvent 把原始事件解码结构转为公开模型，并顺带维护已知会话列表：
// 消息事件记入其所在会话，机器人成员状态变化按加入记入、离开移除
func (r eventResult) toEvent(store *chatStore) Event {
	e := Event{EventID: r.EventID}
	if r.Message != nil {
		chat := r.Message.Chat.toChat()
		e.Message = &ChatMessage{
			MessageID: r.Message.MessageID,
			From:      r.Message.From,
			Chat:      chat,
			Text:      r.Message.Text,
			Date:      r.Message.Date,
		}
		store.remember(chat)
	}
	if r.MyChatMember != nil {
		update := r.MyChatMember.toChatMemberUpdate()
		e.MyChatMember = &update
		switch {
		case update.Left():
			store.remove(update.Chat.ID)
		case update.InGroup():
			store.remember(update.Chat)
		}
	}
	return e
}

// chatMemberUpdateResult 是事件内 my_chat_member 字段的原始 JSON 结构，仅用于解码：
// 机器人自身在新旧会话中的成员状态变化，Chat.ID 数值型经 toChat 转字符串形态
type chatMemberUpdateResult struct {
	Chat          chatResult       `json:"chat"`
	From          *User            `json:"from"`
	Date          int64            `json:"date"`
	OldChatMember chatMemberResult `json:"old_chat_member"`
	NewChatMember chatMemberResult `json:"new_chat_member"`
}

// chatMemberResult 是平台 ChatMember 的原始 JSON 结构，仅用于解码
type chatMemberResult struct {
	Status string `json:"status"`
	User   User   `json:"user"`
}

// toChatMemberUpdate 把原始结构转为公开模型
func (r chatMemberUpdateResult) toChatMemberUpdate() ChatMemberUpdate {
	return ChatMemberUpdate{
		Chat:      r.Chat.toChat(),
		From:      r.From,
		Date:      r.Date,
		OldStatus: ChatMemberStatus(r.OldChatMember.Status),
		NewStatus: ChatMemberStatus(r.NewChatMember.Status),
	}
}

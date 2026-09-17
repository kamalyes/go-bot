/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-16 21:18:23
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-17 09:52:17
* @FilePath: \go-bot\telegram\chat.go
* @Description: Telegram 会话模型与群组基础能力（列表/详情/退群）
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package telegram

import (
	"context"
	"sort"
	"strconv"
	"strings"

	gobot "github.com/kamalyes/go-bot"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// ChatType 标识 Telegram 会话的类型
type ChatType string

const (
	// ChatTypePrivate 是与单个用户的私聊
	ChatTypePrivate ChatType = "private"
	// ChatTypeGroup 是普通群聊
	ChatTypeGroup ChatType = "group"
	// ChatTypeSuperGroup 是超级群组
	ChatTypeSuperGroup ChatType = "supergroup"
	// ChatTypeChannel 是频道
	ChatTypeChannel ChatType = "channel"
)

// Chat 是 SDK 跟踪的 Telegram 会话快照，只携带平台侧信息；
// 绑定、订阅等业务状态由调用方在业务层维护
type Chat struct {
	// ID 是 TG chat_id，与 gobot.Target.ID 同一取值空间
	ID string
	// Type 是会话类型
	Type ChatType
	// Title 是群组/频道标题，私聊为空
	Title string
	// Username 是会话的公开用户名，可为空
	Username string
}

// chatStore 并发维护已知会话快照，syncx.Map 驱动
type chatStore struct {
	m *syncx.Map[string, Chat]
}

// newChatStore 创建会话存储
func newChatStore() *chatStore {
	return &chatStore{m: syncx.NewMap[string, Chat]()}
}

// remember 记录一个会话快照，同 ID 直接覆盖为新快照
func (s *chatStore) remember(chat Chat) { s.m.Store(chat.ID, chat) }

// list 按 ID 稳定排序返回全部已知会话
func (s *chatStore) list() []Chat {
	chats := s.m.Values()
	sort.Slice(chats, func(i, j int) bool { return chats[i].ID < chats[j].ID })
	return chats
}

// remove 删除一个已知会话
func (s *chatStore) remove(chatID string) { s.m.Delete(chatID) }

// ListChats 返回机器人已知会话快照，按 ID 升序；
// 数据来源是 GetUpdates / GetChat 过程中出现过的会话，
// Bot API 不提供直接列出全部会话的接口
func (a *Adapter) ListChats() []Chat { return a.chats.list() }

// GetChat 调用 getChat 查询单个会话的最新详情并记入已知列表
func (a *Adapter) GetChat(ctx context.Context, chatID string) (*Chat, error) {
	const op = "GetChat"
	if strings.TrimSpace(chatID) == "" {
		return nil, gobot.NewValidationError(op, "chat id is required")
	}
	result, err := callAPI[chatResult](a, ctx, op, "getChat", map[string]string{"chat_id": chatID})
	if err != nil {
		return nil, err
	}
	chat := result.toChat()
	a.chats.remember(chat)
	return &chat, nil
}

// LeaveChat 调用 leaveChat 退出群聊或频道，并从已知列表移除该会话
func (a *Adapter) LeaveChat(ctx context.Context, chatID string) error {
	const op = "LeaveChat"
	if strings.TrimSpace(chatID) == "" {
		return gobot.NewValidationError(op, "chat id is required")
	}
	if _, err := callAPI[bool](a, ctx, op, "leaveChat", map[string]string{"chat_id": chatID}); err != nil {
		return err
	}
	a.chats.remove(chatID)
	return nil
}

// chatResult 是 getChat 返回的会话详情
type chatResult struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Username string `json:"username"`
}

// toChat 把平台返回的会话详情转为公开模型
func (r chatResult) toChat() Chat {
	return Chat{
		ID:       strconv.FormatInt(r.ID, 10),
		Type:     ChatType(r.Type),
		Title:    r.Title,
		Username: r.Username,
	}
}

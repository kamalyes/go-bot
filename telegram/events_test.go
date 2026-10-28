/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 00:15:56
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 22:56:36
* @FilePath: \go-bot\telegram\events_test.go
* @Description: Telegram 事件接收测试：getUpdates 解码、游标参数、会话记忆与成员状态变化
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	gobot "github.com/kamalyes/go-bot"
)

// readAll 读取请求体并在失败时终止用例
func readAll(t *testing.T, r *http.Request) []byte {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("读取请求体失败: %v", err)
	}
	return body
}

// decodeJSON 解析请求体为 map，失败时终止用例
func decodeJSON(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("请求体不是合法 JSON: %v", err)
	}
	return payload
}

// TestGetEventsEmpty 验证无事件时返回空列表而非错误
func TestGetEventsEmpty(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
	})
	events, err := adapter.GetEvents(context.Background(), EventOptions{})
	if err != nil {
		t.Fatalf("GetEvents() error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("events = %+v, 期望空列表", events)
	}
}

// TestGetEventsDecodesMessagesAndSkipsOthers 验证消息事件完整解码、非消息事件跳过
func TestGetEventsDecodesMessagesAndSkipsOthers(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":[` +
			`{"update_id":1001,"message":{"message_id":77,` +
			`"from":{"id":42,"first_name":"小明","last_name":"王","username":"xiaoming"},` +
			`"chat":{"id":555,"type":"private","username":"xiaoming"},` +
			`"text":"订阅 k8s 告警 🚨","date":1758265000}},` +
			`{"update_id":1002,"edited_message":{"message_id":78,"chat":{"id":556,"type":"group","title":"运维群"}}}]}`))
	})

	events, err := adapter.GetEvents(context.Background(), EventOptions{})
	if err != nil {
		t.Fatalf("GetEvents() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events 数量 = %d, 期望 2", len(events))
	}

	first := events[0]
	if first.EventID != 1001 || first.Message == nil {
		t.Fatalf("events[0] = %+v, 期望 1001 且携带消息", events[0])
	}
	msg := first.Message
	if msg.MessageID != 77 || msg.Text != "订阅 k8s 告警 🚨" || msg.Date != 1758265000 {
		t.Fatalf("消息快照 = %+v, 期望完整字段", msg)
	}
	if msg.From == nil || msg.From.ID != 42 || msg.From.FirstName != "小明" || msg.From.Username != "xiaoming" {
		t.Fatalf("发送者 = %+v, 期望完整用户信息", msg.From)
	}
	if msg.Chat.ID != "555" || msg.Chat.Type != ChatTypePrivate {
		t.Fatalf("会话快照 = %+v, 期望 555/private", msg.Chat)
	}

	second := events[1]
	if second.EventID != 1002 || second.Message != nil {
		t.Fatalf("events[1] = %+v, 期望 1002 且 Message 为 nil（未展开的事件类型）", events[1])
	}
}

// TestGetEventsRemembersChats 验证事件流中出现过的会话进入已知列表
func TestGetEventsRemembersChats(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":1,"message":` +
			`{"message_id":5,"chat":{"id":888,"type":"group","title":"告警群"},"text":"hi","date":1758265000}}]}`))
	})
	if _, err := adapter.GetEvents(context.Background(), EventOptions{}); err != nil {
		t.Fatalf("GetEvents() error = %v", err)
	}
	known := adapter.ListChats()
	if len(known) != 1 || known[0].ID != "888" || known[0].Title != "告警群" {
		t.Fatalf("ListChats() = %+v, 期望记录事件中的会话", known)
	}
}

// TestGetEventsPayloadOptions 验证游标参数按需携带
func TestGetEventsPayloadOptions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		opts    EventOptions
		present []string
		absent  []string
	}{
		{"零值全部省略", EventOptions{}, nil, []string{"offset", "limit", "timeout"}},
		{"正值全部携带", EventOptions{Offset: 11, Limit: 5, Timeout: 25}, []string{"offset", "limit", "timeout"}, nil},
		{"仅 Offset", EventOptions{Offset: 3}, []string{"offset"}, []string{"limit", "timeout"}},
	} {
		adapter := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
			payload := decodeJSON(t, readAll(t, r))
			for _, key := range tc.present {
				if _, ok := payload[key]; !ok {
					t.Errorf("%s: payload 缺少 %s: %v", tc.name, key, payload)
				}
			}
			for _, key := range tc.absent {
				if _, ok := payload[key]; ok {
					t.Errorf("%s: payload 不应携带 %s: %v", tc.name, key, payload)
				}
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
		})
		if _, err := adapter.GetEvents(context.Background(), tc.opts); err != nil {
			t.Fatalf("%s: GetEvents() error = %v", tc.name, err)
		}
	}
}

// TestGetEventsPropagatesError 验证平台错误透传
func TestGetEventsPropagatesError(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"error_code":409,"description":"Conflict: terminated by other getUpdates request"}`))
	})
	_, err := adapter.GetEvents(context.Background(), EventOptions{})
	var e *gobot.Error
	if !errors.As(err, &e) || e.Kind != gobot.KindPlatform || e.Code != 409 {
		t.Fatalf("GetEvents() 错误 = %v, 期望 platform/409", err)
	}
}

// TestGetEventsDecodesMyChatMember 验证机器人成员状态变化事件完整解码并实时维护已知列表：
// 加入记入会话、被移出移除会话、群内提权刷新会话
func TestGetEventsDecodesMyChatMember(t *testing.T) {
	for _, tc := range []struct {
		name      string
		oldStatus string
		newStatus string
		inList    bool
	}{
		{"被拉入群", "left", "member", true},
		{"群内提权", "member", "administrator", true},
		{"被移出群", "member", "kicked", false},
		{"主动退群", "member", "left", false},
	} {
		adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":2001,"my_chat_member":{` +
				`"chat":{"id":-100777,"type":"supergroup","title":"告警群"},` +
				`"from":{"id":42,"first_name":"小明","username":"xiaoming"},"date":1758265000,` +
				`"old_chat_member":{"status":"` + tc.oldStatus + `","user":{"id":999,"is_bot":true}},` +
				`"new_chat_member":{"status":"` + tc.newStatus + `","user":{"id":999,"is_bot":true}}}}]}`))
		})
		events, err := adapter.GetEvents(context.Background(), EventOptions{})
		if err != nil {
			t.Fatalf("%s: GetEvents() error = %v", tc.name, err)
		}
		if len(events) != 1 {
			t.Fatalf("%s: events 数量 = %d, 期望 1", tc.name, len(events))
		}
		event := events[0]
		if event.MyChatMember == nil || event.Message != nil {
			t.Fatalf("%s: event = %+v, 期望仅携带 MyChatMember", tc.name, event)
		}
		update := event.MyChatMember
		if update.Chat.ID != "-100777" || update.Chat.Type != ChatTypeSuperGroup || update.Chat.Title != "告警群" {
			t.Fatalf("%s: 会话快照 = %+v, 期望 -100777/supergroup/告警群", tc.name, update.Chat)
		}
		if update.From == nil || update.From.ID != 42 || update.Date != 1758265000 {
			t.Fatalf("%s: 变化元数据 = %+v, 期望操作者与时间完整", tc.name, update)
		}
		if update.OldStatus != ChatMemberStatus(tc.oldStatus) || update.NewStatus != ChatMemberStatus(tc.newStatus) {
			t.Fatalf("%s: 状态变化 = %s -> %s, 期望 %s -> %s",
				tc.name, update.OldStatus, update.NewStatus, tc.oldStatus, tc.newStatus)
		}
		known := adapter.ListChats()
		if tc.inList && (len(known) != 1 || known[0].ID != "-100777") {
			t.Fatalf("%s: ListChats() = %+v, 期望保留 -100777", tc.name, known)
		}
		if !tc.inList && len(known) != 0 {
			t.Fatalf("%s: ListChats() = %+v, 期望已移除 -100777", tc.name, known)
		}
	}
}

// TestChatMemberUpdatePredicates 验证成员状态判断语义
func TestChatMemberUpdatePredicates(t *testing.T) {
	for _, tc := range []struct {
		oldStatus, newStatus  ChatMemberStatus
		inGroup, joined, left bool
	}{
		{ChatMemberStatusLeft, ChatMemberStatusMember, true, true, false},
		{ChatMemberStatusKicked, ChatMemberStatusAdministrator, true, true, false},
		{ChatMemberStatusMember, ChatMemberStatusAdministrator, true, false, false},
		{ChatMemberStatusMember, ChatMemberStatusRestricted, true, false, false},
		{ChatMemberStatusMember, ChatMemberStatusKicked, false, false, true},
		{ChatMemberStatusAdministrator, ChatMemberStatusLeft, false, false, true},
	} {
		update := ChatMemberUpdate{OldStatus: tc.oldStatus, NewStatus: tc.newStatus}
		if update.InGroup() != tc.inGroup || update.Joined() != tc.joined || update.Left() != tc.left {
			t.Errorf("%s -> %s: InGroup/Joined/Left = %v/%v/%v, 期望 %v/%v/%v",
				tc.oldStatus, tc.newStatus,
				update.InGroup(), update.Joined(), update.Left(),
				tc.inGroup, tc.joined, tc.left)
		}
	}
}

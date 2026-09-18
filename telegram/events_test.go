/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 00:15:56
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 22:56:36
* @FilePath: \go-bot\telegram\events_test.go
* @Description: Telegram 事件接收测试：getUpdates 解码、游标参数与会话记忆
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

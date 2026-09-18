/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 15:00:16
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 19:36:56
* @FilePath: \go-bot\telegram\chat_test.go
* @Description: Telegram 会话能力测试：快照存储、getChat 详情与 leaveChat 退群
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package telegram

import (
	"context"
	"errors"
	"net/http"
	"testing"

	gobot "github.com/kamalyes/go-bot"
)

// TestChatStoreListSortedByID 验证快照列表按 ID 稳定排序
func TestChatStoreListSortedByID(t *testing.T) {
	store := newChatStore()
	store.remember(Chat{ID: "3", Type: ChatTypeGroup, Title: "C"})
	store.remember(Chat{ID: "1", Type: ChatTypePrivate})
	store.remember(Chat{ID: "2", Type: ChatTypeSuperGroup, Title: "B"})

	got := store.list()
	want := []string{"1", "2", "3"}
	if len(got) != len(want) {
		t.Fatalf("list() 数量 = %d, 期望 %d", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("list()[%d].ID = %q, 期望 %q", i, got[i].ID, id)
		}
	}
}

// TestChatStoreRememberOverwrites 验证同 ID 快照直接覆盖
func TestChatStoreRememberOverwrites(t *testing.T) {
	store := newChatStore()
	store.remember(Chat{ID: "7", Title: "旧标题"})
	store.remember(Chat{ID: "7", Title: "新标题 🚨"})

	got := store.list()
	if len(got) != 1 || got[0].Title != "新标题 🚨" {
		t.Fatalf("list() = %+v, 期望覆盖为最新快照", got)
	}
}

// TestChatStoreRemove 验证删除已知会话
func TestChatStoreRemove(t *testing.T) {
	store := newChatStore()
	store.remember(Chat{ID: "9"})
	store.remove("9")
	if got := store.list(); len(got) != 0 {
		t.Fatalf("remove 后 list() = %+v, 期望为空", got)
	}
	store.remove("不存在") // 幂等
	if got := store.list(); len(got) != 0 {
		t.Fatalf("重复 remove 后 list() = %+v, 期望为空", got)
	}
}

// TestGetChatRequiresID 验证空 chat id 被本地拒绝
func TestGetChatRequiresID(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("校验失败不应发出请求")
	})
	for _, id := range []string{"", "   "} {
		if _, err := adapter.GetChat(context.Background(), id); err == nil {
			t.Fatalf("GetChat(%q) 期望返回错误", id)
		} else {
			var e *gobot.Error
			if !errors.As(err, &e) || e.Kind != gobot.KindValidation {
				t.Fatalf("GetChat(%q) 错误 = %v, 期望 validation", id, err)
			}
		}
	}
}

// TestGetChatSuccessRemembers 验证 getChat 详情解析并记入已知列表
func TestGetChatSuccessRemembers(t *testing.T) {
	var payload map[string]any
	adapter := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bottest-token/getChat" {
			t.Errorf("请求路径 = %q, 期望 getChat", r.URL.Path)
		}
		body := readAll(t, r)
		payload = decodeJSON(t, body)
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":123456,"type":"supergroup","title":"K8s 告警群","username":"k8s_alerts"}}`))
	})

	chat, err := adapter.GetChat(context.Background(), "123456")
	if err != nil {
		t.Fatalf("GetChat() error = %v", err)
	}
	if chat.ID != "123456" || chat.Type != ChatTypeSuperGroup || chat.Title != "K8s 告警群" || chat.Username != "k8s_alerts" {
		t.Fatalf("GetChat() = %+v, 期望完整会话快照", chat)
	}
	if payload["chat_id"] != "123456" {
		t.Fatalf("payload = %v, 期望携带 chat_id", payload)
	}
	known := adapter.ListChats()
	if len(known) != 1 || known[0].ID != "123456" {
		t.Fatalf("ListChats() = %+v, 期望记住刚查询的会话", known)
	}
}

// TestGetChatErrorNotRemembered 验证查询失败不污染已知列表
func TestGetChatErrorNotRemembered(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`))
	})
	if _, err := adapter.GetChat(context.Background(), "999"); err == nil {
		t.Fatal("GetChat() 期望返回平台错误")
	}
	if got := adapter.ListChats(); len(got) != 0 {
		t.Fatalf("ListChats() = %+v, 期望失败不记录", got)
	}
}

// TestLeaveChatRequiresID 验证空 chat id 被本地拒绝
func TestLeaveChatRequiresID(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("校验失败不应发出请求")
	})
	if err := adapter.LeaveChat(context.Background(), "  "); err == nil {
		t.Fatal("LeaveChat(空白) 期望返回错误")
	}
}

// TestLeaveChatSuccessRemoves 验证退群成功后移除已知会话
func TestLeaveChatSuccessRemoves(t *testing.T) {
	var gotPath string
	adapter := newTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	})
	// 先通过事件流记录会话，再退群
	adapter.chats.remember(Chat{ID: "555", Type: ChatTypeGroup, Title: "运维群"})

	if err := adapter.LeaveChat(context.Background(), "555"); err != nil {
		t.Fatalf("LeaveChat() error = %v", err)
	}
	if gotPath != "/bottest-token/leaveChat" {
		t.Fatalf("请求路径 = %q, 期望 leaveChat", gotPath)
	}
	if got := adapter.ListChats(); len(got) != 0 {
		t.Fatalf("ListChats() = %+v, 期望退群后移除", got)
	}
}

// TestLeaveChatErrorKeepsChat 验证退群失败保留已知会话
func TestLeaveChatErrorKeepsChat(t *testing.T) {
	adapter := newTestAdapter(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	})
	adapter.chats.remember(Chat{ID: "555"})

	if err := adapter.LeaveChat(context.Background(), "555"); err == nil {
		t.Fatal("LeaveChat() 期望返回 HTTP 错误")
	}
	if got := adapter.ListChats(); len(got) != 1 || got[0].ID != "555" {
		t.Fatalf("ListChats() = %+v, 期望失败时保留会话", got)
	}
}

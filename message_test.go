/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 22:05:36
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 22:05:36
* @FilePath: \go-bot\message_test.go
* @Description: 通用消息模型测试：构造、链式修饰、体积统计与校验
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import (
	"errors"
	"strings"
	"testing"
)

// TestTextConstructor 验证纯文本消息构造
func TestTextConstructor(t *testing.T) {
	msg := Text("部署完成 ✅ CPU 92.5% & 内存 <81%>（生产环境）")
	if msg.Type != MsgTypeText || msg.Title != "" || msg.ImageURL != "" {
		t.Fatalf("Text() = %+v, 期望仅 Type/Text 有值", msg)
	}
}

// TestMarkdownConstructor 验证 markdown 消息构造
func TestMarkdownConstructor(t *testing.T) {
	msg := Markdown("告警汇总", "| 指标 | 值 |\n| --- | --- |\n| CPU | 92% |")
	if msg.Type != MsgTypeMarkdown || msg.Title != "告警汇总" {
		t.Fatalf("Markdown() = %+v, 期望携带标题", msg)
	}
}

// TestImageConstructor 验证图片消息构造
func TestImageConstructor(t *testing.T) {
	msg := Image("https://example.com/panel.png?tls=1&refresh=5s")
	if msg.Type != MsgTypeImage || msg.Text != "" {
		t.Fatalf("Image() = %+v, 期望仅 Type/ImageURL 有值", msg)
	}
}

// TestAtAllChain 验证 AtAll 链式启用并返回自身
func TestAtAllChain(t *testing.T) {
	msg := Text("notice").AtAll()
	if !msg.MentionAll {
		t.Fatal("AtAll() 后 MentionAll 应为 true")
	}
	if msg.AtAll() != msg {
		t.Fatal("AtAll() 应返回自身以支持链式调用")
	}
}

// TestAtUsersChain 验证 AtUsers 多次链式追加累积
func TestAtUsersChain(t *testing.T) {
	msg := Text("review").AtUsers("111", "222").AtUsers("333")
	want := []string{"111", "222", "333"}
	if len(msg.AtUserIDs) != len(want) {
		t.Fatalf("AtUserIDs = %v, 期望 %v", msg.AtUserIDs, want)
	}
	for i, id := range want {
		if msg.AtUserIDs[i] != id {
			t.Fatalf("AtUserIDs[%d] = %q, 期望 %q", i, msg.AtUserIDs[i], id)
		}
	}
}

// TestHasAt 验证 @ 提及探测
func TestHasAt(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  *Message
		want bool
	}{
		{"无提及", Text("plain"), false},
		{"仅 @所有人", Text("plain").AtAll(), true},
		{"仅逐 id 提及", Text("plain").AtUsers("111"), true},
		{"两者并存", Text("plain").AtAll().AtUsers("111"), true},
	} {
		if got := tc.msg.HasAt(); got != tc.want {
			t.Fatalf("%s: HasAt() = %v, 期望 %v", tc.name, got, tc.want)
		}
	}
}

// TestContentSize 验证按消息类型统计正文字节量
func TestContentSize(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  *Message
		want int64
	}{
		{"中文与 emoji 按字节计", Text("你好🚀"), int64(len("你好🚀"))},
		{"markdown 只计正文不计标题", Markdown("很长的标题", "正文"), int64(len("正文"))},
		{"image 计 URL 长度", Image("https://example.com/x.png"), int64(len("https://example.com/x.png"))},
		{"@ 提及不计入体积", Text("abc").AtAll().AtUsers("111", "222"), 3},
		{"空文本为零", Text(""), 0},
	} {
		if got := tc.msg.ContentSize(); got != tc.want {
			t.Fatalf("%s: ContentSize() = %d, 期望 %d", tc.name, got, tc.want)
		}
	}
}

// TestMessageValidate 验证发送前的本地校验
func TestMessageValidate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		msg     *Message
		wantErr string
	}{
		{"text 合法（复杂内容）", Text("混合内容：中文 ✅ emoji 🚀 & <html> \"quotes\" '单引号'"), ""},
		{"text 空", Text(""), "text content is empty"},
		{"markdown 合法", Markdown("标题", "```go\nfmt.Println(42)\n```"), ""},
		{"markdown 空", Markdown("只有标题", ""), "markdown content is empty"},
		{"image 合法", Image("https://example.com/a.png"), ""},
		{"image 空 URL", Image(""), "image url is empty"},
		{"未知类型", &Message{Type: MessageType("sticker"), Text: "x"}, "unsupported message type: sticker"},
	} {
		err := tc.msg.validate()
		if tc.wantErr == "" {
			if err != nil {
				t.Fatalf("%s: validate() = %v, 期望通过", tc.name, err)
			}
			continue
		}
		if err == nil {
			t.Fatalf("%s: validate() = nil, 期望返回错误", tc.name)
		}
		var e *Error
		if !errors.As(err, &e) || e.Kind != KindValidation || !strings.Contains(e.Message, tc.wantErr) {
			t.Fatalf("%s: validate() = %v, 期望包含 %q 的校验错误", tc.name, err, tc.wantErr)
		}
	}
}

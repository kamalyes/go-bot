/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-19 16:28:31
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-19 16:52:37
* @FilePath: \go-bot\wecom\payload_test.go
* @Description: wecom 消息体构造测试：@ 提及映射与 markdown 组装
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package wecom

import (
	"testing"

	gobot "github.com/kamalyes/go-bot"
)

// TestTextPayloadPlain 验证无提及时省略 mentioned_list 字段
func TestTextPayloadPlain(t *testing.T) {
	payload := textPayload(gobot.Text("磁盘清理完成"))
	text, _ := payload["text"].(map[string]any)
	if text["content"] != "磁盘清理完成" {
		t.Fatalf("content = %v, 期望原样透传", text["content"])
	}
	if _, ok := text["mentioned_list"]; ok {
		t.Fatal("无提及时不应携带 mentioned_list")
	}
}

// TestTextPayloadMentionAll 验证 @ 全员映射为 @all 保留字
func TestTextPayloadMentionAll(t *testing.T) {
	payload := textPayload(gobot.Text("紧急告警").AtAll())
	text, _ := payload["text"].(map[string]any)
	mentioned := text["mentioned_list"].([]string)
	if len(mentioned) != 1 || mentioned[0] != "@all" {
		t.Fatalf("mentioned_list = %v, 期望 [@all]", mentioned)
	}
}

// TestTextPayloadMentionUsers 验证 AtUserIDs 逐个映射，@ 全员排在首位
func TestTextPayloadMentionUsers(t *testing.T) {
	payload := textPayload(gobot.Text("请确认").AtAll().AtUsers("zhangsan", "lisi"))
	text, _ := payload["text"].(map[string]any)
	mentioned := text["mentioned_list"].([]string)
	want := []string{"@all", "zhangsan", "lisi"}
	if len(mentioned) != len(want) {
		t.Fatalf("mentioned_list = %v, 期望 %v", mentioned, want)
	}
	for i := range want {
		if mentioned[i] != want[i] {
			t.Fatalf("mentioned_list = %v, 期望 %v", mentioned, want)
		}
	}
}

// TestMarkdownPayload 验证 markdown 消息体结构
func TestMarkdownPayload(t *testing.T) {
	payload := markdownPayload("**CPU 92%**")
	if payload["msgtype"] != "markdown" {
		t.Fatalf("msgtype = %v, 期望 markdown", payload["msgtype"])
	}
	md, _ := payload["markdown"].(map[string]string)
	if md["content"] != "**CPU 92%**" {
		t.Fatalf("content = %q, 期望降级后正文", md["content"])
	}
}

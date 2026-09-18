/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 15:20:19
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 15:56:36
* @FilePath: \go-bot\dingtalk\payload_test.go
* @Description: dingtalk 消息体构造测试：@ 字段映射与 text/markdown 组装
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package dingtalk

import (
	"testing"

	gobot "github.com/kamalyes/go-bot"
)

// TestAtFieldNilWithoutMention 验证无提及时省略 at 字段
func TestAtFieldNilWithoutMention(t *testing.T) {
	if got := atField(gobot.Text("plain")); got != nil {
		t.Fatalf("atField() = %v, 期望 nil", got)
	}
}

// TestAtFieldUsers 验证 AtUserIDs 映射为 atUserIds
func TestAtFieldUsers(t *testing.T) {
	msg := gobot.Text("hi").AtUsers("user001", "user002")
	at := atField(msg)
	ids, _ := at["atUserIds"].([]string)
	if len(ids) != 2 || ids[0] != "user001" || ids[1] != "user002" {
		t.Fatalf("atUserIds = %v, 期望 [user001 user002]", at["atUserIds"])
	}
	if isAtAll, _ := at["isAtAll"].(bool); isAtAll {
		t.Fatal("isAtAll 应为 false")
	}
}

// TestAtFieldAll 验证 @所有人 映射为 isAtAll 且不携带 atUserIds
func TestAtFieldAll(t *testing.T) {
	at := atField(gobot.Text("notice").AtAll())
	if isAtAll, _ := at["isAtAll"].(bool); !isAtAll {
		t.Fatalf("isAtAll = %v, 期望 true", at["isAtAll"])
	}
	if _, ok := at["atUserIds"]; ok {
		t.Fatalf("at = %v, 纯 @所有人 不应携带 atUserIds", at)
	}
}

// TestTextPayload 验证 text 消息体结构
func TestTextPayload(t *testing.T) {
	payload := textPayload(gobot.Text("【生产发布通知】order-svc v2.6.0 灰度 10% → 50% → 全量"))
	if payload["msgtype"] != "text" {
		t.Fatalf("msgtype = %v, 期望 text", payload["msgtype"])
	}
	text, _ := payload["text"].(map[string]string)
	if text["content"] != "【生产发布通知】order-svc v2.6.0 灰度 10% → 50% → 全量" {
		t.Fatalf("content = %v, 期望原样透传", text["content"])
	}
	if _, ok := payload["at"]; ok {
		t.Fatalf("payload = %v, 无提及时不应携带 at", payload)
	}
}

// TestTextPayloadWithMention 验证带 @ 的 text 消息体
func TestTextPayloadWithMention(t *testing.T) {
	msg := gobot.Text("值班同学请关注").AtUsers("manager7675")
	payload := textPayload(msg)
	at, _ := payload["at"].(map[string]any)
	if at == nil {
		t.Fatal("payload 缺少 at 字段")
	}
	ids, _ := at["atUserIds"].([]string)
	if len(ids) != 1 || ids[0] != "manager7675" {
		t.Fatalf("atUserIds = %v, 期望 [manager7675]", at["atUserIds"])
	}
}

// TestMarkdownPayloadWithTitle 验证 markdown 消息体：Title 优先作会话列表标题
func TestMarkdownPayloadWithTitle(t *testing.T) {
	payload := markdownPayload(gobot.Markdown("生产环境巡检报告", "**CPU 92%**"))
	if payload["msgtype"] != "markdown" {
		t.Fatalf("msgtype = %v, 期望 markdown", payload["msgtype"])
	}
	md, _ := payload["markdown"].(map[string]string)
	if md["title"] != "生产环境巡检报告" {
		t.Fatalf("title = %v, 期望消息 Title", md["title"])
	}
	if md["text"] != "**CPU 92%**" {
		t.Fatalf("text = %v, 期望原样透传", md["text"])
	}
}

// TestMarkdownPayloadTitleFallback 验证 Title 为空时取正文首个非空行兜底
func TestMarkdownPayloadTitleFallback(t *testing.T) {
	payload := markdownPayload(gobot.Markdown("", "\n### 服务概览\n\n**CPU 92%**"))
	md, _ := payload["markdown"].(map[string]string)
	if md["title"] != "### 服务概览" {
		t.Fatalf("title = %q, 期望正文首个非空行 ### 服务概览", md["title"])
	}
}

// TestMarkdownPayloadTitleFallbackEmptyBody 验证正文全空时标题兜底为固定文案
func TestMarkdownPayloadTitleFallbackEmptyBody(t *testing.T) {
	payload := markdownPayload(gobot.Markdown("", " \n\n"))
	md, _ := payload["markdown"].(map[string]string)
	if md["title"] != "消息通知" {
		t.Fatalf("title = %q, 期望兜底文案 消息通知", md["title"])
	}
}

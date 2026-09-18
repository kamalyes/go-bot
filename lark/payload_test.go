/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 15:28:51
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 20:26:37
* @FilePath: \go-bot\lark\payload_test.go
* @Description: lark 消息体构造测试：@ 提及渲染与 markdown 卡片组装
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lark

import (
	"testing"

	gobot "github.com/kamalyes/go-bot"
)

// TestRenderMentionsNoAt 验证无提及时原样返回
func TestRenderMentionsNoAt(t *testing.T) {
	msg := gobot.Text("plain")
	if got := renderMentions("plain", msg); got != "plain" {
		t.Fatalf("renderMentions() = %q, 期望原样返回", got)
	}
}

// TestRenderMentionsAll 验证 @所有人 使用平台保留字 all
func TestRenderMentionsAll(t *testing.T) {
	msg := gobot.Text("notice").AtAll()
	got := renderMentions("notice", msg)
	want := "notice\n<at user_id=\"all\"></at>"
	if got != want {
		t.Fatalf("renderMentions() = %q, 期望 %q", got, want)
	}
}

// TestRenderMentionsUsers 验证逐 id 生成 <at> 标签
func TestRenderMentionsUsers(t *testing.T) {
	msg := gobot.Text("hi").AtUsers("ou_aaa", "ou_bbb")
	got := renderMentions("hi", msg)
	want := "hi\n<at user_id=\"ou_aaa\"></at>\n<at user_id=\"ou_bbb\"></at>"
	if got != want {
		t.Fatalf("renderMentions() = %q, 期望 %q", got, want)
	}
}

// TestRenderMentionsAllAndUsers 验证 @所有人 与逐 id 提及并存
func TestRenderMentionsAllAndUsers(t *testing.T) {
	msg := gobot.Text("go").AtAll().AtUsers("ou_x")
	got := renderMentions("go", msg)
	want := "go\n<at user_id=\"all\"></at>\n<at user_id=\"ou_x\"></at>"
	if got != want {
		t.Fatalf("renderMentions() = %q, 期望 %q", got, want)
	}
}

// TestMarkdownCardWithTitle 验证非空标题进卡片 header 并开启宽屏
func TestMarkdownCardWithTitle(t *testing.T) {
	card := markdownCard(gobot.Markdown("标题", "**正文**"))
	if _, ok := card["config"]; !ok {
		t.Fatal("卡片缺少 config")
	}
	header, _ := card["header"].(map[string]any)
	if header == nil {
		t.Fatal("卡片缺少 header")
	}
	title, _ := header["title"].(map[string]any)
	if title["content"] != "标题" {
		t.Fatalf("header 标题 = %v, 期望 标题", title["content"])
	}
	elements, _ := card["elements"].([]any)
	elem, _ := elements[0].(map[string]any)
	if elem["tag"] != "markdown" || elem["content"] != "**正文**" {
		t.Fatalf("elements[0] = %v, 期望 markdown 正文", elements[0])
	}
}

// TestMarkdownCardWithoutTitle 验证空标题不生成 header
func TestMarkdownCardWithoutTitle(t *testing.T) {
	card := markdownCard(gobot.Markdown("", "**正文**"))
	if _, ok := card["header"]; ok {
		t.Fatal("空标题不应生成 header")
	}
	if _, ok := card["config"]; ok {
		t.Fatal("空标题不应生成 config")
	}
}

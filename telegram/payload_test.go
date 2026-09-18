/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 19:26:28
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 21:00:56
* @FilePath: \go-bot\telegram\payload_test.go
* @Description: Telegram 消息体渲染测试：标题转义、@ 提及与模式降级
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package telegram

import (
	"testing"

	gobot "github.com/kamalyes/go-bot"
)

// newRenderAdapter 按指定渲染格式构造适配器（不发出任何请求）
func newRenderAdapter(t *testing.T, parseMode string) *Adapter {
	t.Helper()
	adapter, err := New(Config{Token: "t", ParseMode: parseMode})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return adapter
}

// TestRenderTextAsIs 验证非 HTML 模式下复杂字符原样透传
func TestRenderTextAsIs(t *testing.T) {
	adapter := newRenderAdapter(t, "none")
	text := "部署完成 ✅ CPU 92.5% & 内存 <81%>（生产环境 🚨）"
	if got := adapter.renderText(gobot.Text(text)); got != text {
		t.Fatalf("renderText() = %q, 期望原样返回", got)
	}
}

// TestRenderTextEscapesHTMLEntities 验证 HTML 模式下正文特殊字符被转义
func TestRenderTextEscapesHTMLEntities(t *testing.T) {
	adapter := newRenderAdapter(t, DefaultParseMode)
	got := adapter.renderText(gobot.Text("特殊 <b> & \"引号\" '单引号' ✅"))
	want := "特殊 &lt;b&gt; &amp; &#34;引号&#34; &#39;单引号&#39; ✅"
	if got != want {
		t.Fatalf("renderText() = %q, 期望 %q", got, want)
	}
}

// TestRenderMarkdownHTMLBoldTitle 验证 HTML 模式下标题转义为加粗行、
// 正文 markdown 转换为 Telegram HTML 子集
func TestRenderMarkdownHTMLBoldTitle(t *testing.T) {
	adapter := newRenderAdapter(t, DefaultParseMode)
	msg := gobot.Markdown("告警 <prod> & 紧急", "**CPU 92%**")
	want := "<b>告警 &lt;prod&gt; &amp; 紧急</b>\n<b>CPU 92%</b>"
	if got := adapter.renderMarkdown(msg); got != want {
		t.Fatalf("renderMarkdown() = %q, 期望 %q", got, want)
	}
}

// TestRenderMarkdownNonHTMLKeepsTitlePlain 验证非 HTML 模式下标题作为普通行
func TestRenderMarkdownNonHTMLKeepsTitlePlain(t *testing.T) {
	adapter := newRenderAdapter(t, "MarkdownV2")
	msg := gobot.Markdown("告警 & <prod>", "正文")
	want := "告警 & <prod>\n正文"
	if got := adapter.renderMarkdown(msg); got != want {
		t.Fatalf("renderMarkdown() = %q, 期望 %q", got, want)
	}
}

// TestRenderMarkdownNoTitle 验证空标题不注入任何前缀
func TestRenderMarkdownNoTitle(t *testing.T) {
	adapter := newRenderAdapter(t, DefaultParseMode)
	if got := adapter.renderMarkdown(gobot.Markdown("", "only body")); got != "only body" {
		t.Fatalf("renderMarkdown() = %q, 期望原样返回", got)
	}
}

// TestRenderMentionsNoAt 验证无提及时原样返回
func TestRenderMentionsNoAt(t *testing.T) {
	adapter := newRenderAdapter(t, DefaultParseMode)
	if got := adapter.renderMentions("plain", gobot.Text("plain")); got != "plain" {
		t.Fatalf("renderMentions() = %q, 期望原样返回", got)
	}
}

// TestRenderMentionAllDegraded 验证 @所有人 降级为字面 @everyone
func TestRenderMentionAllDegraded(t *testing.T) {
	adapter := newRenderAdapter(t, DefaultParseMode)
	got := adapter.renderMentions("通知", gobot.Text("通知").AtAll())
	want := "通知\n@everyone"
	if got != want {
		t.Fatalf("renderMentions() = %q, 期望 %q", got, want)
	}
}

// TestRenderMentionsHTMLMode 验证 HTML 模式逐 id 生成可跳转提及链接
func TestRenderMentionsHTMLMode(t *testing.T) {
	adapter := newRenderAdapter(t, DefaultParseMode)
	msg := gobot.Text("deploy ok").AtUsers("111", "222")
	want := "deploy ok\n<a href=\"tg://user?id=111\">@111</a>\n<a href=\"tg://user?id=222\">@222</a>"
	if got := adapter.renderMentions("deploy ok", msg); got != want {
		t.Fatalf("renderMentions() = %q, 期望 %q", got, want)
	}
}

// TestRenderMentionsPlainMode 验证非 HTML 模式退化为普通 @ 文本
func TestRenderMentionsPlainMode(t *testing.T) {
	adapter := newRenderAdapter(t, "none")
	msg := gobot.Text("hi").AtUsers("111")
	if got := adapter.renderMentions("hi", msg); got != "hi\n@111" {
		t.Fatalf("renderMentions() = %q, 期望 hi\\n@111", got)
	}
}

// TestRenderMentionsAllAndUsersCombined 验证 @所有人 与逐 id 提及并存
func TestRenderMentionsAllAndUsersCombined(t *testing.T) {
	adapter := newRenderAdapter(t, "none")
	msg := gobot.Text("go").AtAll().AtUsers("111")
	if got := adapter.renderMentions("go", msg); got != "go\n@everyone\n@111" {
		t.Fatalf("renderMentions() = %q, 期望 go\\n@everyone\\n@111", got)
	}
}

// TestRenderTextWithMentions 验证完整文本渲染管线含提及
func TestRenderTextWithMentions(t *testing.T) {
	adapter := newRenderAdapter(t, DefaultParseMode)
	msg := gobot.Text("发布完成 🚀").AtUsers("42")
	want := "发布完成 🚀\n<a href=\"tg://user?id=42\">@42</a>"
	if got := adapter.renderText(msg); got != want {
		t.Fatalf("renderText() = %q, 期望 %q", got, want)
	}
}

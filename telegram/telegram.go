/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-16 20:12:36
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-17 09:38:52
* @FilePath: \go-bot\telegram\telegram.go
* @Description: Telegram Bot API 适配器：通用消息到平台协议的翻译层
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// Package telegram 提供 Telegram Bot API 的平台适配器：
// 发送（sendMessage / sendPhoto）、接收（getUpdates）与群组基础能力
// （已知会话列表 / getChat / leaveChat）；
// 订阅码关联、绑定解绑等业务语义由调用方基于这些基础能力实现
package telegram

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	gobot "github.com/kamalyes/go-bot"
	"github.com/kamalyes/go-toolbox/pkg/httpx"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
)

// DefaultAPIBase 是 Telegram Bot API 官方地址，测试时可替换为本地实例
const DefaultAPIBase = "https://api.telegram.org"

// DefaultParseMode 是缺省的文本渲染格式，Telegram 原生 HTML
const DefaultParseMode = "HTML"

// Config 配置 Telegram 适配器，零值不可用，Token 必填
type Config struct {
	// Token 是机器人 token，来自 @BotFather
	Token string
	// APIBase 是 API 根地址，缺省官方地址
	APIBase string
	// ParseMode 是文本渲染格式：HTML（默认）/ MarkdownV2 / none（纯文本）
	ParseMode string
	// HTTPClient 可选，nil 时按 gobot.DefaultHTTPTimeout 构造
	HTTPClient *httpx.Client
}

// Adapter 实现 gobot.Adapter，把通用消息翻译为 Telegram Bot API 调用
// 同时维护一份已知会话快照（ListChats 的数据来源）
// Adapter 可被多个 goroutine 安全并发使用
type Adapter struct {
	cfg    Config
	client *httpx.Client
	chats  *chatStore
}

// New 按配置创建 Telegram 适配器，Token 为空时返回校验错误
func New(cfg Config) (*Adapter, error) {
	const op = "New"
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, gobot.NewValidationError(op, "token is required")
	}
	cfg.APIBase = strings.TrimRight(cfg.APIBase, "/")
	cfg.APIBase = mathx.IfNotEmpty(cfg.APIBase, DefaultAPIBase)
	cfg.ParseMode = mathx.IfNotEmpty(cfg.ParseMode, DefaultParseMode)
	if cfg.ParseMode == "none" {
		cfg.ParseMode = "" // none 语义化为纯文本，不向平台传 parse_mode
	}
	client := cfg.HTTPClient
	if client == nil {
		client = httpx.NewClient(httpx.WithTimeout(gobot.DefaultHTTPTimeout))
	}
	return &Adapter{cfg: cfg, client: client, chats: newChatStore()}, nil
}

// Platform 返回平台标识
func (a *Adapter) Platform() gobot.Platform { return gobot.PlatformTelegram }

// Close 释放资源；Telegram 适配器不持有需回收的资源
func (a *Adapter) Close(context.Context) error { return nil }

// Send 实现 gobot.Adapter，按消息类型选择 sendMessage 或 sendPhoto
func (a *Adapter) Send(ctx context.Context, target gobot.Target, msg *gobot.Message) (*gobot.SendResult, error) {
	const op = "Send"
	switch msg.Type {
	case gobot.MsgTypeText:
		return a.sendMessage(ctx, op, target.ID, a.renderText(msg))
	case gobot.MsgTypeMarkdown:
		return a.sendMessage(ctx, op, target.ID, a.renderMarkdown(msg))
	case gobot.MsgTypeImage:
		return a.sendPhoto(ctx, op, target, msg)
	default:
		return nil, gobot.NewValidationError(op, "unsupported message type: "+string(msg.Type))
	}
}

// sendMessage 调用 sendMessage 投递文本形态的消息
func (a *Adapter) sendMessage(ctx context.Context, op, chatID, text string) (*gobot.SendResult, error) {
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	if a.cfg.ParseMode != "" {
		payload["parse_mode"] = a.cfg.ParseMode
	}
	result, err := callAPI[messageResult](a, ctx, op, "sendMessage", payload)
	if err != nil {
		return nil, err
	}
	return &gobot.SendResult{MessageID: strconv.FormatInt(result.MessageID, 10)}, nil
}

// sendPhoto 调用 sendPhoto 按 URL 投递图片，Text 作为图片说明
func (a *Adapter) sendPhoto(ctx context.Context, op string, target gobot.Target, msg *gobot.Message) (*gobot.SendResult, error) {
	payload := map[string]any{
		"chat_id": target.ID,
		"photo":   msg.ImageURL,
	}
	if msg.Text != "" {
		payload["caption"] = msg.Text
	}
	result, err := callAPI[messageResult](a, ctx, op, "sendPhoto", payload)
	if err != nil {
		return nil, err
	}
	return &gobot.SendResult{MessageID: strconv.FormatInt(result.MessageID, 10)}, nil
}

// renderText 渲染纯文本消息：正文加 @ 提及
func (a *Adapter) renderText(msg *gobot.Message) string {
	return a.renderMentions(msg.Text, msg)
}

// renderMarkdown 渲染 markdown 消息：HTML 模式下标题转为加粗行，正文原样投递
func (a *Adapter) renderMarkdown(msg *gobot.Message) string {
	text := msg.Text
	if msg.Title != "" && a.cfg.ParseMode == DefaultParseMode {
		text = "<b>" + html.EscapeString(msg.Title) + "</b>\n" + text
	} else if msg.Title != "" {
		text = msg.Title + "\n" + text
	}
	return a.renderMentions(text, msg)
}

// renderMentions 把 @ 提及追加到文本尾部：HTML 模式下生成可跳转的提及链接，
// 其他模式退化为普通 @ 文本；Telegram 无原生 @所有人，
// MentionAll 统一降级为字面 @everyone，不产生平台级提及
func (a *Adapter) renderMentions(text string, msg *gobot.Message) string {
	if !msg.HasAt() {
		return text
	}
	var b strings.Builder
	b.WriteString(text)
	if msg.MentionAll {
		b.WriteString("\n@everyone")
	}
	for _, id := range msg.AtUserIDs {
		b.WriteString("\n")
		if a.cfg.ParseMode == DefaultParseMode {
			b.WriteString(`<a href="tg://user?id=` + id + `">@` + id + `</a>`)
		} else {
			b.WriteString("@" + id)
		}
	}
	return b.String()
}

// messageResult 是 sendMessage / sendPhoto 返回的消息标识
type messageResult struct {
	MessageID int64 `json:"message_id"`
}

// apiResult 是 Telegram Bot API 的统一响应包，成功时 Result 携带业务数据
type apiResult[T any] struct {
	OK          bool             `json:"ok"`
	Result      T                `json:"result"`
	ErrorCode   int              `json:"error_code"`
	Description string           `json:"description"`
	Parameters  *retryParameters `json:"parameters"`
}

// retryParameters 是平台在 429 等场景下附带的退避建议
type retryParameters struct {
	RetryAfter int `json:"retry_after"`
}

// callAPI 向 Bot API 发送 JSON POST 并解码统一响应包，
// 把传输/HTTP/平台/解码四类失败分别映射为结构化 *gobot.Error
func callAPI[T any](a *Adapter, ctx context.Context, op, method string, payload any) (T, error) {
	var zero T
	resp, err := a.client.Post(a.cfg.APIBase + "/bot" + a.cfg.Token + "/" + method).
		WithContext(ctx).
		SetBodyJSON(payload).
		Send()
	if err != nil {
		return zero, gobot.NewTransportError(gobot.PlatformTelegram, op, err)
	}
	body, err := resp.Bytes()
	if err != nil {
		return zero, gobot.NewTransportError(gobot.PlatformTelegram, op, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return zero, gobot.NewHTTPError(gobot.PlatformTelegram, op, resp.StatusCode, string(body), retryAfterFrom(body))
	}
	var result apiResult[T]
	if err := json.Unmarshal(body, &result); err != nil {
		return zero, gobot.NewDecodeError(gobot.PlatformTelegram, op, err)
	}
	if !result.OK {
		e := gobot.NewPlatformError(gobot.PlatformTelegram, op, result.ErrorCode, result.Description, platformRetryable(result.ErrorCode))
		if result.Parameters != nil {
			e.RetryAfter = time.Duration(result.Parameters.RetryAfter) * time.Second
		}
		return zero, e
	}
	return result.Result, nil
}

// retryAfterFrom 从 429 响应体中提取 parameters.retry_after
func retryAfterFrom(body []byte) time.Duration {
	var errResp apiResult[json.RawMessage]
	if err := json.Unmarshal(body, &errResp); err != nil || errResp.Parameters == nil {
		return 0
	}
	return time.Duration(errResp.Parameters.RetryAfter) * time.Second
}

// platformRetryable 报告平台业务码是否值得重试：429 限流或 5xx 服务端错误
func platformRetryable(code int) bool {
	if code == http.StatusTooManyRequests {
		return true
	}
	return code >= 500 && code <= 599
}

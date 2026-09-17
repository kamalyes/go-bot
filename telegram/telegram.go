/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-16 20:12:36
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-17 20:56:31
* @FilePath: \go-bot\telegram\telegram.go
* @Description: Telegram 适配器结构与构造
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
	"strings"

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

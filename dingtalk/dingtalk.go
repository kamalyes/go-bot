/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 13:00:00
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 21:00:36
* @FilePath: \go-bot\dingtalk\dingtalk.go
* @Description: 钉钉自定义机器人 webhook 适配器的构造与配置
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// Package dingtalk 提供钉钉群自定义机器人（webhook 形态）的平台适配器：
// 凭 webhook 的 access_token 把消息推送到机器人所在群，支持可选加签；
// 消息内容受机器人安全设置（关键词/IP 白名单）约束，由平台侧校验
package dingtalk

import (
	"context"
	"strings"

	gobot "github.com/kamalyes/go-bot"
	"github.com/kamalyes/go-toolbox/pkg/httpx"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
)

// DefaultAPIBase 是钉钉开放平台官方地址
const DefaultAPIBase = "https://oapi.dingtalk.com"

// webhookPath 是自定义机器人推送地址的路径，token 与签名以 query 参数携带
const webhookPath = "/robot/send"

// maxPayloadSize 是单次请求体的大小上限（20 KB），超出本地直接拒绝
const maxPayloadSize = 20 << 10

// Config 配置钉钉自定义机器人适配器，Token 必填
type Config struct {
	// Token 是自定义机器人 webhook 地址中的 access_token
	Token string
	// Secret 可选，安全设置选择加签时必填
	Secret string
	// APIBase 是开放平台根地址，缺省官方地址
	APIBase string
	// HTTPClient 可选，nil 时按 gobot.DefaultHTTPTimeout 构造
	HTTPClient *httpx.Client
}

// Adapter 实现 gobot.Adapter，把通用消息翻译为自定义机器人的 webhook 推送；
// webhook 形态消息固定投递到机器人所在群，Target 不参与路由，
// 仅为满足 Adapter SPI 保留参数
// Adapter 可被多个 goroutine 安全并发使用
type Adapter struct {
	cfg    Config
	client *httpx.Client
}

// New 按配置创建适配器，Token 为空时返回校验错误
func New(cfg Config) (*Adapter, error) {
	const op = "New"
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, gobot.NewValidationError(op, "webhook access_token is required")
	}
	cfg.APIBase = strings.TrimRight(cfg.APIBase, "/")
	cfg.APIBase = mathx.IfNotEmpty(cfg.APIBase, DefaultAPIBase)
	client := cfg.HTTPClient
	if client == nil {
		client = httpx.NewClient(httpx.WithTimeout(gobot.DefaultHTTPTimeout))
	}
	return &Adapter{cfg: cfg, client: client}, nil
}

// Platform 返回平台标识
func (a *Adapter) Platform() gobot.Platform { return gobot.PlatformDingtalk }

// Close 释放资源；适配器不持有需回收的资源
func (a *Adapter) Close(context.Context) error { return nil }

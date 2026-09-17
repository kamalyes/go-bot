/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-17 15:07:31
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-17 20:52:26
* @FilePath: \go-bot\lark\lark.go
* @Description: Lark（飞书）自定义机器人 webhook 适配器的构造与配置
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// Package lark 提供 Lark（飞书）自定义机器人（webhook 形态）的平台适配器：
// 凭 webhook token 把消息推送到机器人所在会话，支持可选签名校验；
// 订阅、绑定等业务语义由调用方基于 gobot 核心能力实现
package lark

import (
	"context"
	"strings"

	gobot "github.com/kamalyes/go-bot"
	"github.com/kamalyes/go-toolbox/pkg/httpx"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
)

// DefaultAPIBase 是 Lark 开放平台官方地址
const DefaultAPIBase = "https://open.feishu.cn"

// webhookPathPrefix 是自定义机器人推送地址的路径前缀，Token 拼接在其后
const webhookPathPrefix = "/open-apis/bot/v2/hook/"

// CodeSignInvalid 是签名校验失败的业务码（密钥不符或时间戳距当前超过一小时）
const CodeSignInvalid = 19021

// CodeRateLimited 是触发频率控制的业务码（单机器人 100 次/分钟、5 次/秒）
const CodeRateLimited = 11232

// maxPayloadSize 是单次请求体的大小上限（20 KB），超出本地直接拒绝
const maxPayloadSize = 20 << 10

// Config 配置 Lark 自定义机器人适配器，Token 必填
type Config struct {
	// Token 是自定义机器人的 webhook token，即推送地址的最后一段
	Token string
	// Secret 可选，开启签名校验后必填
	Secret string
	// APIBase 是开放平台根地址，缺省官方地址
	APIBase string
	// HTTPClient 可选，nil 时按 gobot.DefaultHTTPTimeout 构造
	HTTPClient *httpx.Client
}

// Adapter 实现 gobot.Adapter，把通用消息翻译为自定义机器人的 webhook 推送；
// webhook 形态消息固定投递到机器人所在会话，Target 不参与路由，
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
		return nil, gobot.NewValidationError(op, "webhook token is required")
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
func (a *Adapter) Platform() gobot.Platform { return gobot.PlatformLark }

// Close 释放资源；适配器不持有需回收的资源
func (a *Adapter) Close(context.Context) error { return nil }

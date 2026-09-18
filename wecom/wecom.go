/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-19 13:02:19
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-19 13:28:51
* @FilePath: \go-bot\wecom\wecom.go
* @Description: 企业微信群机器人 webhook 适配器的构造与配置
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// Package wecom 提供企业微信群机器人（webhook 形态）的平台适配器：
// 凭 webhook key 把消息推送到机器人所在群，支持 text（含 @ 提及）、
// markdown 与图片（自动下载转 base64+md5）；
// 订阅、绑定等业务语义由调用方基于 gobot 核心能力实现
package wecom

import (
	"context"
	"strings"

	gobot "github.com/kamalyes/go-bot"
	"github.com/kamalyes/go-toolbox/pkg/httpx"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
)

// DefaultAPIBase 是企业微信开放接口官方地址
const DefaultAPIBase = "https://qyapi.weixin.qq.com"

// webhookPath 是群机器人推送地址的路径，key 以 query 参数携带
const webhookPath = "/cgi-bin/webhook/send"

// 平台侧单条消息的字节上限，超出本地直接拒绝：
// text 正文 2048 字节、markdown 正文 4096 字节、图片 base64 编码后 2 MB
const (
	maxTextBytes        = 2048
	maxMarkdownBytes    = 4096
	maxImageBase64Bytes = 2 << 20
)

// Config 配置企业微信群机器人适配器，Key 必填
type Config struct {
	// Key 是群机器人 webhook 地址中的 key 参数
	Key string
	// APIBase 是开放接口根地址，缺省官方地址
	APIBase string
	// HTTPClient 可选，nil 时按 gobot.DefaultHTTPTimeout 构造
	HTTPClient *httpx.Client
}

// Adapter 实现 gobot.Adapter，把通用消息翻译为群机器人的 webhook 推送；
// webhook 形态消息固定投递到机器人所在群，Target 不参与路由，
// 仅为满足 Adapter SPI 保留参数
// Adapter 可被多个 goroutine 安全并发使用
type Adapter struct {
	cfg    Config
	client *httpx.Client
}

// New 按配置创建适配器，Key 为空时返回校验错误
func New(cfg Config) (*Adapter, error) {
	const op = "New"
	if strings.TrimSpace(cfg.Key) == "" {
		return nil, gobot.NewValidationError(op, "webhook key is required")
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
func (a *Adapter) Platform() gobot.Platform { return gobot.PlatformWecom }

// Close 释放资源；适配器不持有需回收的资源
func (a *Adapter) Close(context.Context) error { return nil }

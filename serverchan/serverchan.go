/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-19 09:12:36
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-19 09:28:17
* @FilePath: \go-bot\serverchan\serverchan.go
* @Description: Server酱（ServerChan Turbo）个人推送适配器的构造与配置
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// Package serverchan 提供 Server酱（ServerChan Turbo）个人推送的平台适配器：
// 凭 SendKey 把消息推送到绑定者的微信，消息内容以 markdown 渲染；
// 属个人推送形态，无群组与 @ 概念，订阅、绑定等业务语义由调用方基于 gobot 核心能力实现
package serverchan

import (
	"context"
	"strings"

	gobot "github.com/kamalyes/go-bot"
	"github.com/kamalyes/go-toolbox/pkg/httpx"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
)

// DefaultAPIBase 是 Server酱 Turbo 官方推送地址
const DefaultAPIBase = "https://sctapi.ftqq.com"

// Config 配置 Server酱适配器，SendKey 必填
type Config struct {
	// SendKey 是推送地址 https://sctapi.ftqq.com/<SendKey>.send 中的 SendKey
	SendKey string
	// APIBase 是推送服务根地址，缺省官方地址
	APIBase string
	// HTTPClient 可选，nil 时按 gobot.DefaultHTTPTimeout 构造
	HTTPClient *httpx.Client
}

// Adapter 实现 gobot.Adapter，把通用消息翻译为 Server酱的微信推送；
// 推送固定送达 SendKey 绑定者，Target 不参与路由，仅为满足 Adapter SPI 保留参数
// Adapter 可被多个 goroutine 安全并发使用
type Adapter struct {
	cfg    Config
	client *httpx.Client
}

// New 按配置创建适配器，SendKey 为空时返回校验错误
func New(cfg Config) (*Adapter, error) {
	const op = "New"
	if strings.TrimSpace(cfg.SendKey) == "" {
		return nil, gobot.NewValidationError(op, "sendkey is required")
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
func (a *Adapter) Platform() gobot.Platform { return gobot.PlatformServerchan }

// Close 释放资源；适配器不持有需回收的资源
func (a *Adapter) Close(context.Context) error { return nil }

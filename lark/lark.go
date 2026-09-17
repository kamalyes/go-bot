/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-17 15:07:31
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-17 20:12:07
* @FilePath: \go-bot\lark\lark.go
* @Description: Lark（飞书）自定义机器人 webhook 适配器
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// Package lark 提供 Lark（飞书）自定义机器人（webhook 形态）的平台适配器：
// 凭 webhook token 把消息推送到机器人所在会话，支持可选签名校验；
// 订阅、绑定等业务语义由调用方基于 gobot 核心能力实现
package lark

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"time"

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

// Send 实现 gobot.Adapter：text 直发、markdown 组装 interactive 卡片；
// webhook 形态不支持图片消息，收到 image 消息时返回校验错误；
// 平台不回传消息标识，成功时 MessageID 为空
func (a *Adapter) Send(ctx context.Context, _ gobot.Target, msg *gobot.Message) (*gobot.SendResult, error) {
	const op = "Send"
	var payload map[string]any
	switch msg.Type {
	case gobot.MsgTypeText:
		payload = map[string]any{
			"msg_type": "text",
			"content":  map[string]string{"text": renderMentions(msg.Text, msg)},
		}
	case gobot.MsgTypeMarkdown:
		payload = map[string]any{"msg_type": "interactive", "card": markdownCard(msg)}
	case gobot.MsgTypeImage:
		return nil, gobot.NewValidationError(op, "webhook bot cannot send image messages")
	default:
		return nil, gobot.NewValidationError(op, "unsupported message type: "+string(msg.Type))
	}
	if a.cfg.Secret != "" {
		timestamp, signature := sign(a.cfg.Secret, time.Now())
		payload["timestamp"] = timestamp
		payload["sign"] = signature
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, gobot.NewValidationError(op, "encode payload failed")
	}
	if len(data) > maxPayloadSize {
		return nil, gobot.NewValidationError(op, "payload exceeds 20 KB limit")
	}
	resp, err := a.client.Post(a.cfg.APIBase + webhookPathPrefix + a.cfg.Token).
		WithContext(ctx).
		SetBodyRaw(data).
		Send()
	if err != nil {
		return nil, gobot.NewTransportError(gobot.PlatformLark, op, err)
	}
	body, err := resp.Bytes()
	if err != nil {
		return nil, gobot.NewTransportError(gobot.PlatformLark, op, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, gobot.NewHTTPError(gobot.PlatformLark, op, resp.StatusCode, string(body), 0)
	}
	var result webhookResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, gobot.NewDecodeError(gobot.PlatformLark, op, err)
	}
	if result.Code != 0 {
		return nil, gobot.NewPlatformError(gobot.PlatformLark, op, result.Code, result.Msg, result.Code == CodeRateLimited)
	}
	return &gobot.SendResult{}, nil
}

// sign 生成自定义机器人的请求签名：
// 以 timestamp + 换行 + secret 作为 HMAC-SHA256 的密钥对空串签名，base64 后随请求携带
func sign(secret string, now time.Time) (timestamp, signature string) {
	timestamp = strconv.FormatInt(now.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(timestamp+"\n"+secret))
	return timestamp, base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// webhookResult 是自定义机器人响应的原始 JSON 结构，仅用于解码：
// 成功 Code 为 0；19021 签名失败，9499 token 失效或参数错误，
// 限流时 Msg 固定为 too many request
type webhookResult struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

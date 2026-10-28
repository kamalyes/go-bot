/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-10-19 00:00:00
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-10-19 21:36:00
* @FilePath: \go-bot\lark\app.go
* @Description: Lark 应用机器人形态适配器构造与配置：经应用凭证调用开放平台 API
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lark

import (
	"context"
	"encoding/json"
	"strings"

	gobot "github.com/kamalyes/go-bot"
	"github.com/kamalyes/go-toolbox/pkg/httpx"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
)

// AppConfig 配置 Lark 应用机器人（自建应用形态），AppID 与 AppSecret 必填
type AppConfig struct {
	// AppID 是开放平台应用的 app_id
	AppID string
	// AppSecret 是开放平台应用的 app_secret
	AppSecret string
	// APIBase 是开放平台根地址，缺省官方地址
	APIBase string
	// HTTPClient 可选，nil 时按 gobot.DefaultHTTPTimeout 构造
	HTTPClient *httpx.Client
}

// AppAdapter 实现 gobot.Adapter，把通用消息翻译为开放平台 im/v1/messages 调用：
// 应用形态经 tenant_access_token 鉴权，可向机器人所在的任意会话与用户投递，
// 同时提供群列表发现与机器人身份查询能力（见 app_chat.go）
// AppAdapter 可被多个 goroutine 安全并发使用
type AppAdapter struct {
	cfg    AppConfig
	client *httpx.Client
	tokens *tokenCache
}

// NewApp 按配置创建应用机器人适配器，AppID 或 AppSecret 为空时返回校验错误
func NewApp(cfg AppConfig) (*AppAdapter, error) {
	const op = "NewApp"
	if strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.AppSecret) == "" {
		return nil, gobot.NewValidationError(op, "app_id and app_secret are required")
	}
	cfg.APIBase = strings.TrimRight(cfg.APIBase, "/")
	cfg.APIBase = mathx.IfNotEmpty(cfg.APIBase, DefaultAPIBase)
	client := cfg.HTTPClient
	if client == nil {
		client = httpx.NewClient(httpx.WithTimeout(gobot.DefaultHTTPTimeout))
	}
	return &AppAdapter{cfg: cfg, client: client, tokens: newTokenCache()}, nil
}

// Platform 返回平台标识
func (a *AppAdapter) Platform() gobot.Platform { return gobot.PlatformLark }

// Close 释放资源；适配器不持有需回收的资源
func (a *AppAdapter) Close(context.Context) error { return nil }

// appResult 是开放平台 API 响应的公共外壳，data 由各接口自行解码
type appResult struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// callApp 调用开放平台 API：自动附带 tenant_access_token 鉴权头，
// method 为空时按 GET 请求，query 为查询参数，body 为 POST 请求体，
// out 非 nil 时把响应的 data 段解码到 out
func (a *AppAdapter) callApp(ctx context.Context, op, method, path string, query map[string]string, body any, out any) error {
	token, err := a.tokens.tenantAccessToken(ctx, a)
	if err != nil {
		return err
	}
	var req *httpx.Request
	if method == "" {
		req = a.client.Get(a.cfg.APIBase + path)
		for key, value := range query {
			req.AddQuery(key, value)
		}
	} else {
		req = a.client.Post(a.cfg.APIBase + path)
		for key, value := range query {
			req.AddQuery(key, value)
		}
		if body != nil {
			req.SetBodyJSON(body)
		}
	}
	req.WithContext(ctx).
		SetContentType(httpx.ContentTypeApplicationJSON).
		SetHeader("Authorization", "Bearer "+token)
	resp, err := req.Send()
	if err != nil {
		return gobot.NewTransportError(gobot.PlatformLark, op, err)
	}
	respBody, err := resp.Bytes()
	if err != nil {
		return gobot.NewTransportError(gobot.PlatformLark, op, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return gobot.NewHTTPError(gobot.PlatformLark, op, resp.StatusCode, string(respBody), 0)
	}
	var envelope struct {
		appResult
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return gobot.NewDecodeError(gobot.PlatformLark, op, err)
	}
	if envelope.Code != 0 {
		return gobot.NewPlatformError(gobot.PlatformLark, op, envelope.Code, envelope.Msg, envelope.Code == CodeRateLimited)
	}
	if out == nil || len(envelope.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return gobot.NewDecodeError(gobot.PlatformLark, op, err)
	}
	return nil
}

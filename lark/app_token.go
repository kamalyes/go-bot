/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-10-19 00:00:00
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-10-19 21:36:00
* @FilePath: \go-bot\lark\app_token.go
* @Description: Lark 应用形态 tenant_access_token 获取与缓存，到期前提前刷新
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lark

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	gobot "github.com/kamalyes/go-bot"
)

// tokenRefreshAhead 是令牌到期前的提前刷新量，避免边界时刻请求失败
const tokenRefreshAhead = 5 * time.Minute

// tokenResult 是 auth/v3/tenant_access_token/internal 响应的原始结构，仅用于解码
type tokenResult struct {
	//nolint:tagliatelle // 平台返回字段为 snake_case 命名
	TenantAccessToken string `json:"tenant_access_token"`
	// Expire 是令牌有效期秒数
	Expire int64 `json:"expire"`
}

// tokenCache 并发维护单个应用的 tenant_access_token，过期前提前刷新
type tokenCache struct {
	mu       sync.Mutex
	token    string
	expireAt time.Time
}

// newTokenCache 创建令牌缓存
func newTokenCache() *tokenCache {
	return &tokenCache{}
}

// tenantAccessToken 返回有效的 tenant_access_token，缓存未到期直接复用，
// 到期前 5 分钟或已过期时重新获取；获取失败时清空缓存，下次调用重试
func (c *tokenCache) tenantAccessToken(ctx context.Context, a *AppAdapter) (string, error) {
	const op = "tenantAccessToken"
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Add(tokenRefreshAhead).Before(c.expireAt) {
		return c.token, nil
	}
	var result tokenResult
	if err := a.callAppRaw(ctx, op, tokenPath, map[string]string{
		"app_id":     a.cfg.AppID,
		"app_secret": a.cfg.AppSecret,
	}, &result); err != nil {
		c.token = ""
		return "", err
	}
	c.token = result.TenantAccessToken
	c.expireAt = time.Now().Add(time.Duration(result.Expire) * time.Second)
	return c.token, nil
}

// tokenPath 是获取 tenant_access_token 的接口路径
const tokenPath = "/open-apis/auth/v3/tenant_access_token/internal"

// callAppRaw 调用无需鉴权的开放平台接口（仅获取令牌自身使用），
// POST 请求体为 JSON，响应的 data 段解码到 out
func (a *AppAdapter) callAppRaw(ctx context.Context, op, path string, body any, out any) error {
	resp, err := a.client.Post(a.cfg.APIBase + path).
		WithContext(ctx).
		SetBodyJSON(body).
		Send()
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
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return gobot.NewDecodeError(gobot.PlatformLark, op, err)
	}
	return nil
}

/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-17 20:31:37
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-17 21:03:58
* @FilePath: \go-bot\telegram\api.go
* @Description: Telegram Bot API 调用基础设施与统一响应解码
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
*/

package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	gobot "github.com/kamalyes/go-bot"
)

// callAPI 向 Bot API 发送 JSON POST 并解码统一响应包，
// 把传输/HTTP/平台/解码四类失败分别映射为结构化 *gobot.Error
func callAPI[T any](a *Adapter, ctx context.Context, op, method string, payload any) (T, error) {
	var zero T
	resp, err := a.client.Post(a.cfg.APIBase+"/bot"+a.cfg.Token+"/"+method).
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

// apiResult 是 Bot API 的统一响应包，仅用于解码：
// 成功时 OK 为 true 且 Result 携带业务数据，失败时 ErrorCode / Description 是错误详情
type apiResult[T any] struct {
	OK          bool             `json:"ok"`
	Result      T                `json:"result"`
	ErrorCode   int              `json:"error_code"`
	Description string           `json:"description"`
	Parameters  *retryParameters `json:"parameters"`
}

// retryParameters 是平台在 429 等场景下附带的退避建议，仅用于解码
type retryParameters struct {
	RetryAfter int `json:"retry_after"`
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

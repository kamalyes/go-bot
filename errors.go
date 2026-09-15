/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-15 15:26:08
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-15 19:52:16
* @FilePath: \go-bot\errors.go
* @Description: 结构化错误体系与可重试判定
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ErrorKind 对失败进行分类，调用方无需匹配错误字符串即可分支处理
type ErrorKind string

const (
	// KindValidation 表示本地输入在发出任何请求前就被拒绝，永不重试
	KindValidation ErrorKind = "validation"
	// KindTransport 表示网络/传输层失败（拨号、连接重置、超时、取消）
	KindTransport ErrorKind = "transport"
	// KindHTTP 表示收到了非 2xx 的 HTTP 响应
	KindHTTP ErrorKind = "http"
	// KindPlatform 表示平台返回了业务错误码
	KindPlatform ErrorKind = "platform"
	// KindDecode 表示响应体无法解码
	KindDecode ErrorKind = "decode"
)

// Error 是所有出站操作返回的结构化错误用 errors.As(err, &e) 检查它，
// 再按 Kind / HTTPStatus / Code / Retryable 分支处理；被包裹的原因对
// errors.Is / errors.As 保持可见
//
// Error 的消息中永远不包含 token、secret 等凭据
type Error struct {
	Platform   Platform      // 来源平台，核心层填充
	Operation  string        // 逻辑操作，例如 "Send"
	Kind       ErrorKind     // 失败分类
	HTTPStatus int           // KindHTTP 时的 HTTP 状态码，否则为 0
	Code       int           // KindPlatform 时的平台业务码，否则为 0
	Message    string        // 不含敏感信息的可读详情
	RetryAfter time.Duration // 服务端建议的退避时长，无则为 0
	Retryable  bool          // 重试该操作是否可能成功
	Err        error         // 被包裹的原因，可能为 nil
}

// Error 实现 error 接口
func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("gobot")
	if e.Platform != "" {
		b.WriteString("/")
		b.WriteString(string(e.Platform))
	}
	if e.Operation != "" {
		b.WriteString(" ")
		b.WriteString(e.Operation)
	}
	b.WriteString(": ")
	b.WriteString(string(e.Kind))
	if e.HTTPStatus != 0 {
		b.WriteString(" (status ")
		b.WriteString(strconv.Itoa(e.HTTPStatus))
		b.WriteString(")")
	}
	if e.Code != 0 {
		b.WriteString(" (code ")
		b.WriteString(strconv.Itoa(e.Code))
		b.WriteString(")")
	}
	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}
	if e.Err != nil {
		b.WriteString(": ")
		b.WriteString(e.Err.Error())
	}
	return b.String()
}

// Unwrap 返回被包裹的原因，供 errors.Is / errors.As 使用
func (e *Error) Unwrap() error { return e.Err }

// NewValidationError 构造本地校验失败的 *Error
func NewValidationError(op, msg string) *Error {
	return &Error{Operation: op, Kind: KindValidation, Message: msg}
}

// NewTransportError 构造传输失败的 *Error
func NewTransportError(platform Platform, op string, err error) *Error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &Error{Platform: platform, Operation: op, Kind: KindTransport, Retryable: false, Err: err}
	}
	return &Error{Platform: platform, Operation: op, Kind: KindTransport, Retryable: true, Err: err}
}

// NewHTTPError 构造非 2xx HTTP 响应的 *Error，并基于状态码决定是否可重试
func NewHTTPError(platform Platform, op string, status int, body string, retryAfter time.Duration) *Error {
	return &Error{
		Platform:   platform,
		Operation:  op,
		Kind:       KindHTTP,
		HTTPStatus: status,
		Message:    truncate(body),
		RetryAfter: retryAfter,
		Retryable:  httpRetryable(status),
	}
}

// NewPlatformError 构造平台业务错误的 *Errorretryable 由调用方
// （adapter，持有平台限流码知识）显式给出
func NewPlatformError(platform Platform, op string, code int, msg string, retryable bool) *Error {
	return &Error{Platform: platform, Operation: op, Kind: KindPlatform, Code: code, Message: msg, Retryable: retryable}
}

// NewDecodeError 构造响应解码失败的 *Error
func NewDecodeError(platform Platform, op string, err error) *Error {
	return &Error{Platform: platform, Operation: op, Kind: KindDecode, Message: "decode response", Err: err}
}

// retryable 报告 err 是否为标记了可重试的结构化 *Error
func retryable(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Retryable
	}
	return false
}

// httpRetryable 报告某个 HTTP 状态码是否值得重试：408、425、429，或任意 5xx
func httpRetryable(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusTooEarly, http.StatusTooManyRequests:
		return true
	}
	return status >= 500 && status <= 599
}

// maxBodyInError 限制错误消息中携带的响应体长度
const maxBodyInError = 512

// truncate 按 rune 边界截断过长文本
func truncate(s string) string {
	if len(s) <= maxBodyInError {
		return s
	}
	t := s[:maxBodyInError]
	for len(t) > 0 && !utf8Valid(t) {
		t = t[:len(t)-1]
	}
	return t + "…(truncated)"
}

func utf8Valid(s string) bool { return strings.ToValidUTF8(s, "") == s }

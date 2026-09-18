/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 15:23:08
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 20:21:53
* @FilePath: \go-bot\lark\sign_test.go
* @Description: lark 请求签名测试
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lark

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"testing"
	"time"
)

// TestSign 验证签名算法：以 timestamp + 换行 + secret 为密钥对空串做 HMAC-SHA256
func TestSign(t *testing.T) {
	now := time.Unix(1758265000, 0)
	timestamp, signature := sign("test-secret", now)

	if timestamp != strconv.FormatInt(now.Unix(), 10) {
		t.Fatalf("timestamp = %q, 期望 %d", timestamp, now.Unix())
	}
	mac := hmac.New(sha256.New, []byte(timestamp+"\n"+"test-secret"))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if signature != want {
		t.Fatalf("signature = %q, 期望独立重算结果 %q", signature, want)
	}
}

// TestSignDeterministic 验证同一时间戳与密钥的签名可确定性复现
func TestSignDeterministic(t *testing.T) {
	now := time.Unix(1758265000, 0)
	ts1, sig1 := sign("abc", now)
	ts2, sig2 := sign("abc", now)
	if ts1 != ts2 || sig1 != sig2 {
		t.Fatalf("签名不确定: (%q,%q) vs (%q,%q)", ts1, sig1, ts2, sig2)
	}
}

// TestSignVariesBySecret 验证不同密钥产生不同签名
func TestSignVariesBySecret(t *testing.T) {
	now := time.Unix(1758265000, 0)
	_, sig1 := sign("secret-a", now)
	_, sig2 := sign("secret-b", now)
	if sig1 == sig2 {
		t.Fatal("不同密钥的签名不应相同")
	}
}

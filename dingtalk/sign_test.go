/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 15:00:00
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 15:20:36
* @FilePath: \go-bot\dingtalk\sign_test.go
* @Description: 钉钉 webhook 请求加签测试
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package dingtalk

import (
	"testing"
	"time"
)

// TestSignDeterministic 验证同一时间戳与密钥产出确定签名，
// 且签名为 URL 编码后的 base64 形态（不含裸 + / 字符）
func TestSignDeterministic(t *testing.T) {
	now := time.Date(2026, 9, 18, 22, 40, 36, 0, time.Local)
	firstTS, firstSign := sign("SECe66b1f2c0a0d1f2b3c4d5e6f708192a", now)
	secondTS, secondSign := sign("SECe66b1f2c0a0d1f2b3c4d5e6f708192a", now)
	if firstTS != secondTS || firstSign != secondSign {
		t.Fatalf("sign() 期望确定输出, got %q/%q 与 %q/%q", firstTS, firstSign, secondTS, secondSign)
	}
	if firstTS != "1789742436000" {
		t.Fatalf("timestamp = %q, 期望毫秒时间戳 1789742436000", firstTS)
	}
	for _, r := range firstSign {
		if (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && r != '%' && r != '=' && r != '_' && r != '-' {
			t.Fatalf("sign = %q, 期望 URL 编码形态", firstSign)
		}
	}
}

// TestSignVariesWithSecret 验证不同密钥产出不同签名
func TestSignVariesWithSecret(t *testing.T) {
	now := time.Date(2026, 9, 18, 22, 40, 36, 0, time.Local)
	_, signA := sign("secret-a", now)
	_, signB := sign("secret-b", now)
	if signA == signB {
		t.Fatalf("不同密钥不应产出相同签名: %q", signA)
	}
}

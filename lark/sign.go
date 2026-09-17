/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-17 20:37:29
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-17 20:58:16
* @FilePath: \go-bot\lark\sign.go
* @Description: Lark 自定义机器人的 webhook 请求签名
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lark

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"time"
)

// sign 生成自定义机器人的请求签名：
// 以 timestamp + 换行 + secret 作为 HMAC-SHA256 的密钥对空串签名，base64 后随请求携带
func sign(secret string, now time.Time) (timestamp, signature string) {
	timestamp = strconv.FormatInt(now.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(timestamp+"\n"+secret))
	return timestamp, base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

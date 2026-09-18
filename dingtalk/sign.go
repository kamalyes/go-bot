/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 11:26:27
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 15:00:16
* @FilePath: \go-bot\dingtalk\sign.go
* @Description: 钉钉自定义机器人的 webhook 请求加签
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package dingtalk

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strconv"
	"time"
)

// sign 生成自定义机器人的请求加签：
// 以毫秒时间戳 + 换行 + secret 作为待签内容，secret 同时作为 HMAC-SHA256 密钥，
// 结果 base64 后再 URL 编码，随请求以 timestamp 与 sign query 参数携带
func sign(secret string, now time.Time) (timestamp, signature string) {
	timestamp = strconv.FormatInt(now.UnixMilli(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "\n" + secret))
	return timestamp, url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
}

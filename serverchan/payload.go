/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-19 09:28:17
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-19 09:52:19
* @FilePath: \go-bot\serverchan\payload.go
* @Description: Server酱推送标题兜底：Title → 正文首个非空行 → 固定文案
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package serverchan

import (
	"strings"

	gobot "github.com/kamalyes/go-bot"
)

// titleOf 取推送标题（微信通知栏展示）：
// Title 非空直接使用，否则取正文首个非空行（标题语法符号按原文保留），
// 正文全空时兜底为固定文案
func titleOf(msg *gobot.Message) string {
	if msg.Title != "" {
		return msg.Title
	}
	for _, line := range strings.Split(msg.Text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return "消息通知"
}

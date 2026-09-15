/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-15 08:37:53
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-15 09:52:07
* @FilePath: \go-bot\target.go
* @Description: 平台无关的发送目标定义
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

// TargetType 标识发送目标的语义类型，adapter 据此选择平台的收件人形态
type TargetType string

const (
	// TargetUser 表示目标是一个用户（单聊）
	TargetUser TargetType = "user"
	// TargetChat 表示目标是一个群聊/会话
	TargetChat TargetType = "chat"
)

// Target 是平台无关的发送目标
// Lark：user 映射 open_id、chat 映射 chat_id；Telegram：两类统一为 chat_id
type Target struct {
	// ID 是目标标识：TG chat_id、Lark open_id 或 chat_id
	ID string
	// Type 是目标语义类型，默认按 chat 处理
	Type TargetType
}

// User 构造一个用户（单聊）目标
func User(id string) Target {
	return Target{ID: id, Type: TargetUser}
}

// Chat 构造一个群聊目标
func Chat(id string) Target {
	return Target{ID: id, Type: TargetChat}
}

// validate 在发送前对目标做本地校验
func (t Target) validate() error {
	if t.ID == "" {
		return NewValidationError("", "target id is empty")
	}
	if t.Type == "" {
		return NewValidationError("", "target type is empty")
	}
	return nil
}

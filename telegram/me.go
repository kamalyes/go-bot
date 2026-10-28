/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-10-19 00:00:00
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-10-19 21:36:00
* @FilePath: \go-bot\telegram\me.go
* @Description: Telegram getMe 机器人身份查询，用于凭证验证与用户名回填
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package telegram

import (
	"context"
)

// Me 是 getMe 返回的机器人身份快照，仅携带平台侧信息；
// 凭证有效性等业务状态由调用方在业务层维护
type Me struct {
	// ID 是平台侧的机器人数字标识
	ID int64
	// Username 是机器人的公开用户名，不含 @ 前缀，可为空
	Username string
	// FirstName 是机器人的显示名
	FirstName string
	// CanJoinGroups 指示机器人可否加入群聊
	CanJoinGroups bool
	// CanReadAllGroupMessages 指示群聊中是否可见全部消息（隐私模式关闭）
	CanReadAllGroupMessages bool
}

// meResult 是 getMe 返回的原始结构，仅用于解码
type meResult struct {
	ID       int64  `json:"id"`
	IsBot    bool   `json:"is_bot"`
	Username string `json:"username"`
	//nolint:tagliatelle // 平台返回字段为 snake_case 命名
	FirstName string `json:"first_name"`
	//nolint:tagliatelle // 平台返回字段为 snake_case 命名
	CanJoinGroups           bool `json:"can_join_groups"`
	CanReadAllGroupMessages bool `json:"can_read_all_group_messages"`
}

// GetMe 调用 getMe 查询机器人自身身份，凭证无效时返回平台侧错误
func (a *Adapter) GetMe(ctx context.Context) (*Me, error) {
	const op = "GetMe"
	result, err := callAPI[meResult](a, ctx, op, "getMe", map[string]any{})
	if err != nil {
		return nil, err
	}
	return &Me{
		ID:                      result.ID,
		Username:                result.Username,
		FirstName:               result.FirstName,
		CanJoinGroups:           result.CanJoinGroups,
		CanReadAllGroupMessages: result.CanReadAllGroupMessages,
	}, nil
}

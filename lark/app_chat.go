/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-10-19 00:00:00
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-10-19 21:36:00
* @FilePath: \go-bot\lark\app_chat.go
* @Description: Lark 应用形态群组发现与机器人身份查询
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lark

import (
	"context"
	"strconv"
)

// chatsPageSize 是 im/v1/chats 单页拉取的群组上限
const chatsPageSize = 200

// AppChat 是应用机器人可见的群组快照，只携带平台侧信息；
// 绑定、启用状态等业务状态由调用方在业务层维护
type AppChat struct {
	// ChatID 是群组的平台侧标识，与 gobot.Target.ID 同一取值空间
	ChatID string
	// Name 是群组名称
	Name string
	// OwnerID 是群主的 open_id，可为空
	OwnerID string
}

// chatItem 是 im/v1/chats 返回的群组原始结构，仅用于解码
type chatItem struct {
	//nolint:tagliatelle // 平台返回字段为 snake_case 命名
	ChatID  string `json:"chat_id"`
	Name    string `json:"name"`
	OwnerID string `json:"owner_id"`
}

// chatsPage 是 im/v1/chats 返回的分页数据段，仅用于解码
type chatsPage struct {
	Items     []chatItem `json:"items"`
	PageToken string     `json:"page_token"`
	//nolint:tagliatelle // 平台返回字段为 snake_case 命名
	HasMore bool `json:"has_more"`
}

// ListChats 分页拉取机器人所在的全部群组，按平台顺序返回
func (a *AppAdapter) ListChats(ctx context.Context) ([]AppChat, error) {
	const op = "ListChats"
	chats := make([]AppChat, 0)
	pageToken := ""
	for {
		query := map[string]string{"page_size": strconv.Itoa(chatsPageSize)}
		if pageToken != "" {
			query["page_token"] = pageToken
		}
		var page chatsPage
		if err := a.callApp(ctx, op, "", "/open-apis/im/v1/chats", query, nil, &page); err != nil {
			return nil, err
		}
		for _, item := range page.Items {
			chats = append(chats, AppChat{ChatID: item.ChatID, Name: item.Name, OwnerID: item.OwnerID})
		}
		if !page.HasMore || page.PageToken == "" {
			return chats, nil
		}
		pageToken = page.PageToken
	}
}

// AppBotInfo 是 bot/v3/info 返回的机器人身份快照，用于凭证验证
type AppBotInfo struct {
	// OpenID 是机器人在租户内的 open_id
	OpenID string
	// BotName 是机器人的显示名
	BotName string
}

// botInfoResult 是 bot/v3/info 返回的原始结构，仅用于解码
type botInfoResult struct {
	//nolint:tagliatelle // 平台返回字段为 snake_case 命名
	OpenID  string `json:"open_id"`
	BotName string `json:"bot_name"`
}

// GetBotInfo 查询机器人自身身份，凭证无效时返回平台侧错误
func (a *AppAdapter) GetBotInfo(ctx context.Context) (*AppBotInfo, error) {
	const op = "GetBotInfo"
	var result botInfoResult
	if err := a.callApp(ctx, op, "", "/open-apis/bot/v3/info", nil, nil, &result); err != nil {
		return nil, err
	}
	return &AppBotInfo{OpenID: result.OpenID, BotName: result.BotName}, nil
}

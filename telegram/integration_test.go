/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 17:16:19
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 22:15:19
* @FilePath: \go-bot\telegram\integration_test.go
* @Description: Telegram 真实 API 集成测试：token 连通性与真实投递（环境变量门控）
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package telegram

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	gobot "github.com/kamalyes/go-bot"
)

// skipIntegration 统一门控：未设置凭据或 -short 模式时跳过真实 API 测试
func skipIntegration(t *testing.T) (token string, ok bool) {
	t.Helper()
	if testing.Short() {
		t.Skip("short 模式跳过真实 API 集成测试")
	}
	token = os.Getenv("GOBOT_TELEGRAM_TOKEN")
	if token == "" {
		t.Skip("未设置 GOBOT_TELEGRAM_TOKEN，跳过真实 API 集成测试")
	}
	return token, true
}

// newRealAdapter 用真实 token 构造适配器
func newRealAdapter(t *testing.T, token string) *Adapter {
	t.Helper()
	adapter, err := New(Config{Token: token})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return adapter
}

// TestIntegrationTokenConnectivity 验证真实 token 可通过 getUpdates 连通
// go test ./telegram/ -run TestIntegrationTokenConnectivity -v
//
//	GOBOT_TELEGRAM_TOKEN=<来自 @BotFather 的 token>
func TestIntegrationTokenConnectivity(t *testing.T) {
	token, ok := skipIntegration(t)
	if !ok {
		return
	}
	adapter := newRealAdapter(t, token)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Timeout 为零表示立即返回，不做长轮询
	events, err := adapter.GetEvents(ctx, EventOptions{})
	if err != nil {
		t.Fatalf("GetEvents() error = %v, 期望 token 连通（请检查 token 是否有效）", err)
	}
	t.Logf("连通成功，当前待消费事件 %d 条", len(events))

	// 事件流中出现过的会话应进入已知列表
	for _, chat := range adapter.ListChats() {
		t.Logf("已知会话: id=%s type=%s title=%s", chat.ID, chat.Type, chat.Title)
	}
}

// TestIntegrationSendRealMessage 验证向真实会话投递复杂消息
// go test ./telegram/ -run TestIntegrationSendRealMessage -v
//
//	GOBOT_TELEGRAM_TOKEN=<token> GOBOT_TELEGRAM_CHAT_ID=<先向机器人发一条消息获得>
func TestIntegrationSendRealMessage(t *testing.T) {
	token, ok := skipIntegration(t)
	if !ok {
		return
	}
	chatID := os.Getenv("GOBOT_TELEGRAM_CHAT_ID")
	if chatID == "" {
		t.Skip("未设置 GOBOT_TELEGRAM_CHAT_ID，跳过真实投递")
	}
	adapter := newRealAdapter(t, token)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	msg := gobot.Text(
		"【生产发布通知】order-svc v2.6.0\n" +
			"发布人: kamalyes ｜ 窗口: " + time.Now().Format("2006-01-02 15:04") + "（Asia/Shanghai）\n" +
			"变更内容: 数据库迁移 + 接口限流调整，含特殊字符转义验证 <>&\"'\n" +
			"发布策略: 灰度 10% → 50% → 全量，错误率 > 0.5% 自动回滚").
		AtUsers(chatID)
	result, err := adapter.Send(ctx, gobot.Chat(chatID), msg)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if result == nil || result.MessageID == "" {
		t.Fatalf("Send() result = %+v, 期望携带平台 message_id", result)
	}
	t.Logf("文本消息投递成功 message_id=%s", result.MessageID)

	body := "**巡检时间**: " + time.Now().Format("2006-01-02 15:04:05") + "（Asia/Shanghai）\n" +
		"**巡检范围**: cn-east-1 核心服务，采样窗口 15 分钟\n" +
		"`工具`: go-bot v2.6.0 ｜ `巡检人`: kamalyes\n\n" +
		"## 服务概览\n\n" +
		"| 服务 | 实例数 | QPS | P99 延迟 | 状态 |\n" +
		"| --- | ---: | ---: | ---: | :---: |\n" +
		"| user-api | 12 | 12,400 | 87ms | 健康 |\n" +
		"| order-svc | 3 | 3,200 | 231ms | 健康 |\n" +
		"| legacy-gateway | 1 | 800 | 512ms | 告警 |\n\n" +
		"## 关键事件\n\n" +
		"```go\nretry: 3, backoff: 2s, budget: 30s\ndropRate: 0.02% (order-svc 超时熔断)\n```\n\n" +
		"~~旧告警通道~~ 已于本迭代下线，*新增* Telegram/Lark 双通道，规则详见 [值班手册](https://example.com/runbook?d=oncall&v=3)。"
	md := gobot.Markdown("生产环境巡检报告 📊", body)
	if result, err = adapter.Send(ctx, gobot.Chat(chatID), md); err != nil {
		t.Fatalf("Send(markdown) error = %v", err)
	}
	t.Logf("markdown 消息投递成功 message_id=%s", result.MessageID)
	if !strings.Contains(adapter.cfg.APIBase, "api.telegram.org") {
		t.Logf("注意: 当前 APIBase=%s 非官方地址", adapter.cfg.APIBase)
	}
}

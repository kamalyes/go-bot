/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 17:57:36
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 22:00:28
* @FilePath: \go-bot\dingtalk\integration_test.go
* @Description: dingtalk 真实 webhook 集成测试（环境变量门控，凭据不落仓库）
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package dingtalk

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	gobot "github.com/kamalyes/go-bot"
)

// newWebhookAdapter 基于环境变量里的真实 webhook 构造适配器：
// GOBOT_DINGTALK_WEBHOOK=https://oapi.dingtalk.com/robot/send?access_token=<token>
// 加签机器人另配 GOBOT_DINGTALK_SECRET（安全设置选择加签时必填）
func newWebhookAdapter(t *testing.T) *Adapter {
	t.Helper()
	if testing.Short() {
		t.Skip("short 模式跳过真实 webhook 集成测试")
	}
	webhook := os.Getenv("GOBOT_DINGTALK_WEBHOOK")
	if webhook == "" {
		t.Skip("未设置 GOBOT_DINGTALK_WEBHOOK，跳过真实 webhook 集成测试")
	}
	token := webhookToken(t, webhook)
	adapter, err := New(Config{Token: token, Secret: os.Getenv("GOBOT_DINGTALK_SECRET")})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return adapter
}

// webhookToken 从完整 webhook 地址中提取 access_token 参数
func webhookToken(t *testing.T, webhook string) string {
	t.Helper()
	idx := strings.Index(webhook, "access_token=")
	if idx < 0 {
		t.Fatalf("webhook 地址 %q 不含 access_token 参数", webhook)
	}
	token := webhook[idx+len("access_token="):]
	if amp := strings.Index(token, "&"); amp >= 0 {
		token = token[:amp]
	}
	return token
}

// TestIntegrationWebhookSendComplexText 验证向真实 webhook 投递生产级发布通知文本
// go test ./dingtalk/ -run TestIntegrationWebhookSendComplexText -v
//
//	GOBOT_DINGTALK_WEBHOOK=https://oapi.dingtalk.com/robot/send?access_token=<token>
func TestIntegrationWebhookSendComplexText(t *testing.T) {
	adapter := newWebhookAdapter(t)

	text := strings.Join([]string{
		"【生产发布通知】order-svc v2.6.0",
		"发布人: kamalyes ｜ 窗口: " + time.Now().Format("2006-01-02 15:04") + "（Asia/Shanghai）",
		"变更内容: 数据库迁移 + 接口限流调整，错误率 > 0.5% 自动回滚",
		"发布策略: 灰度 10% → 50% → 全量",
		"监控大盘: https://grafana.example.com/d/order-svc?from=now-1h&refresh=30s",
	}, "\n")
	if _, err := adapter.Send(context.Background(), gobot.Chat("webhook"), gobot.Text(text)); err != nil {
		t.Fatalf("Send() error = %v, 期望真实 webhook 接受该消息", err)
	}
}

// TestIntegrationWebhookSendMarkdownReport 验证向真实 webhook 投递生产级巡检报告：
// 元信息、对齐表格降级、有序列表、代码块、删除线/斜体与链接的完整组合
// go test ./dingtalk/ -run TestIntegrationWebhookSendMarkdownReport -v
//
//	GOBOT_DINGTALK_WEBHOOK=https://oapi.dingtalk.com/robot/send?access_token=<token>
func TestIntegrationWebhookSendMarkdownReport(t *testing.T) {
	adapter := newWebhookAdapter(t)

	content := strings.Join([]string{
		"**巡检时间**: " + time.Now().Format("2006-01-02 15:04:05") + "（Asia/Shanghai）",
		"**巡检范围**: cn-east-1 核心服务，采样窗口 15 分钟",
		"`工具`: go-bot v2.6.0 ｜ `巡检人`: kamalyes",
		"",
		"### 服务概览",
		"",
		"| 服务 | 实例数 | QPS | P99 延迟 | 状态 |",
		"| --- | ---: | ---: | ---: | :---: |",
		"| user-api | 12 | 12,400 | 87ms | ✅ 健康 |",
		"| order-svc | 3 | 3,200 | 231ms | ✅ 健康 |",
		"| legacy-gateway | 1 | 800 | 512ms | 🔥 告警 |",
		"",
		"### 关键事件",
		"",
		"1. order-svc 超时熔断触发，dropRate 0.02%，15 分钟后自愈",
		"2. legacy-gateway P99 突破 500ms，建议下线流量或扩容",
		"",
		"```go",
		"retry: 3, backoff: 2s, budget: 30s",
		"dropRate: 0.02% (order-svc 超时熔断)",
		"```",
		"",
		"~~旧告警通道~~ 已于本迭代下线，*新增* Telegram/Lark/钉钉 三通道，规则详见 [值班手册](https://example.com/runbook?d=oncall&v=3)。",
	}, "\n")
	if _, err := adapter.Send(context.Background(), gobot.Chat("webhook"), gobot.Markdown("生产环境巡检报告", content)); err != nil {
		t.Fatalf("Send() error = %v, 期望真实 webhook 接受该消息", err)
	}
}

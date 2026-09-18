/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-19 17:02:19
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-19 17:28:53
* @FilePath: \go-bot\wecom\integration_test.go
* @Description: wecom 真实 webhook 集成测试（环境变量门控，凭据不落仓库）
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package wecom

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	gobot "github.com/kamalyes/go-bot"
)

// newWebhookAdapter 基于环境变量里的真实 webhook 构造适配器：
// GOBOT_WECOM_WEBHOOK=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=<key>
func newWebhookAdapter(t *testing.T) *Adapter {
	t.Helper()
	if testing.Short() {
		t.Skip("short 模式跳过真实 webhook 集成测试")
	}
	webhook := os.Getenv("GOBOT_WECOM_WEBHOOK")
	if webhook == "" {
		t.Skip("未设置 GOBOT_WECOM_WEBHOOK，跳过真实 webhook 集成测试")
	}
	idx := strings.Index(webhook, "key=")
	if idx < 0 {
		t.Fatalf("webhook 地址 %q 不含 key 参数", webhook)
	}
	key := webhook[idx+len("key="):]
	if amp := strings.Index(key, "&"); amp >= 0 {
		key = key[:amp]
	}
	adapter, err := New(Config{Key: key})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return adapter
}

// TestIntegrationWebhookSendComplexText 验证向真实 webhook 投递生产级发布通知文本
// go test ./wecom/ -run TestIntegrationWebhookSendComplexText -v
//
//	GOBOT_WECOM_WEBHOOK=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=<key>
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
// 元信息、表格对齐降级、代码块去围栏与链接的完整组合
// go test ./wecom/ -run TestIntegrationWebhookSendMarkdownReport -v
//
//	GOBOT_WECOM_WEBHOOK=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=<key>
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
		"```go",
		"retry: 3, backoff: 2s, budget: 30s",
		"dropRate: 0.02% (order-svc 超时熔断)",
		"```",
		"",
		"规则详见 [值班手册](https://example.com/runbook?d=oncall&v=3)。",
	}, "\n")
	if _, err := adapter.Send(context.Background(), gobot.Chat("webhook"), gobot.Markdown("生产环境巡检报告", content)); err != nil {
		t.Fatalf("Send() error = %v, 期望真实 webhook 接受该报告", err)
	}
}

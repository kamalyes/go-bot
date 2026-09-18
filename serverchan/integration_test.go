/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-19 11:02:57
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-19 11:28:19
* @FilePath: \go-bot\serverchan\integration_test.go
* @Description: serverchan 真实推送集成测试（环境变量门控，凭据不落仓库）
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package serverchan

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	gobot "github.com/kamalyes/go-bot"
)

// newPushAdapter 基于环境变量里的真实 SendKey 构造适配器：
// GOBOT_SERVERCHAN_SENDKEY=<sctapi.ftqq.com 地址中的 SendKey>
func newPushAdapter(t *testing.T) *Adapter {
	t.Helper()
	if testing.Short() {
		t.Skip("short 模式跳过真实推送集成测试")
	}
	sendKey := os.Getenv("GOBOT_SERVERCHAN_SENDKEY")
	if sendKey == "" {
		t.Skip("未设置 GOBOT_SERVERCHAN_SENDKEY，跳过真实推送集成测试")
	}
	adapter, err := New(Config{SendKey: sendKey})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return adapter
}

// TestIntegrationPushComplexText 验证向真实 SendKey 投递生产级发布通知
// go test ./serverchan/ -run TestIntegrationPushComplexText -v
//
//	GOBOT_SERVERCHAN_SENDKEY=SCTxxxxxxxx
func TestIntegrationPushComplexText(t *testing.T) {
	adapter := newPushAdapter(t)

	text := strings.Join([]string{
		"【生产发布通知】order-svc v2.6.0",
		"发布人: kamalyes ｜ 窗口: " + time.Now().Format("2006-01-02 15:04") + "（Asia/Shanghai）",
		"变更内容: 数据库迁移 + 接口限流调整，错误率 > 0.5% 自动回滚",
		"发布策略: 灰度 10% → 50% → 全量",
		"监控大盘: https://grafana.example.com/d/order-svc?from=now-1h&refresh=30s",
	}, "\n")
	if _, err := adapter.Send(context.Background(), gobot.Chat("push"), gobot.Text(text)); err != nil {
		t.Fatalf("Send() error = %v, 期望真实推送接受该消息", err)
	}
}

// TestIntegrationPushMarkdownReport 验证向真实 SendKey 投递生产级巡检报告：
// 元信息、表格、有序列表、代码块与链接的完整组合（平台侧 markdown 渲染）
// go test ./serverchan/ -run TestIntegrationPushMarkdownReport -v
//
//	GOBOT_SERVERCHAN_SENDKEY=SCTxxxxxxxx
func TestIntegrationPushMarkdownReport(t *testing.T) {
	adapter := newPushAdapter(t)

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
		"~~旧告警通道~~ 已于本迭代下线，规则详见 [值班手册](https://example.com/runbook?d=oncall&v=3)。",
	}, "\n")
	if _, err := adapter.Send(context.Background(), gobot.Chat("push"), gobot.Markdown("生产环境巡检报告", content)); err != nil {
		t.Fatalf("Send() error = %v, 期望真实推送接受该报告", err)
	}
}

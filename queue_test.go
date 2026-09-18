/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-19 11:16:08
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-19 11:51:55
* @FilePath: \go-bot\queue_test.go
* @Description: 队列 SPI 任务模型测试：标识唯一性与序列化往返
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import (
	"encoding/json"
	"reflect"
	"sync"
	"testing"
	"time"
)

// TestNewTaskIDUnique 验证并发下任务标识不重复且非空
func TestNewTaskIDUnique(t *testing.T) {
	const total = 200
	ids := make(chan string, total)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < total/8; j++ {
				ids <- newTaskID()
			}
		}()
	}
	wg.Wait()
	close(ids)

	seen := make(map[string]struct{}, total)
	for id := range ids {
		if id == "" {
			t.Fatal("newTaskID() 返回空标识")
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("newTaskID() 出现重复标识 %q", id)
		}
		seen[id] = struct{}{}
	}
}

// TestQueueTaskJSONRoundTrip 验证任务快照（消息 + 目标 + 元数据）
// 经过 JSON 序列化往返后字段无损，队列 adapter 的跨进程传输依赖这一点
func TestQueueTaskJSONRoundTrip(t *testing.T) {
	task := &QueueTask{
		ID: "task-001",
		Message: Message{
			Type:       MsgTypeMarkdown,
			Title:      "【巡检报告】cn-east-1",
			Text:       "| 服务 | 实例 | QPS |\n| --- | ---: |\n| user-api | 12 | 8600 |\n",
			MentionAll: false,
			AtUserIDs:  []string{"ou_7dab8ce3", "123456789"},
		},
		Targets:     []Target{Chat("-1001234567890"), User("ou_7dab8ce3")},
		PublishedAt: time.Date(2026, 9, 19, 2, 51, 8, 0, time.Local),
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded QueueTask
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if decoded.ID != task.ID {
		t.Fatalf("ID = %q, 期望 %q", decoded.ID, task.ID)
	}
	if !reflect.DeepEqual(decoded.Message, task.Message) {
		t.Fatalf("Message = %+v, 期望 %+v", decoded.Message, task.Message)
	}
	if len(decoded.Targets) != len(task.Targets) {
		t.Fatalf("Targets 数量 = %d, 期望 %d", len(decoded.Targets), len(task.Targets))
	}
	for i, target := range task.Targets {
		if decoded.Targets[i] != target {
			t.Fatalf("Targets[%d] = %+v, 期望 %+v", i, decoded.Targets[i], target)
		}
	}
	if !decoded.PublishedAt.Equal(task.PublishedAt) {
		t.Fatalf("PublishedAt = %v, 期望 %v", decoded.PublishedAt, task.PublishedAt)
	}
}

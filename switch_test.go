/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 22:09:18
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 22:09:18
* @FilePath: \go-bot\switch_test.go
* @Description: 运行期静音开关测试：零值、nil 接收者与并发翻转
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import (
	"sync"
	"testing"
)

// TestSwitchZeroValueEnabled 验证零值开关即启用
func TestSwitchZeroValueEnabled(t *testing.T) {
	var sw Switch
	if !sw.Enabled() {
		t.Fatal("零值 Switch 应处于启用态")
	}
}

// TestNilSwitchEnabled 验证 nil 开关视为启用
func TestNilSwitchEnabled(t *testing.T) {
	var sw *Switch
	if !sw.Enabled() {
		t.Fatal("nil *Switch 应视为启用，等价于未配置开关")
	}
}

// TestSwitchDisableEnableRoundtrip 验证开关可反复翻转
func TestSwitchDisableEnableRoundtrip(t *testing.T) {
	sw := NewSwitch()
	if !sw.Enabled() {
		t.Fatal("NewSwitch() 应返回启用态")
	}
	sw.Disable()
	if sw.Enabled() {
		t.Fatal("Disable() 后应为静音态")
	}
	sw.Disable() // 重复静音是幂等的
	if sw.Enabled() {
		t.Fatal("重复 Disable() 应保持静音态")
	}
	sw.Enable()
	if !sw.Enabled() {
		t.Fatal("Enable() 后应恢复启用态")
	}
}

// TestSwitchConcurrentFlips 验证多 goroutine 并发读写（配合 -race 检测数据竞争）
func TestSwitchConcurrentFlips(t *testing.T) {
	sw := NewSwitch()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); sw.Disable() }()
		go func() { defer wg.Done(); sw.Enable() }()
	}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = sw.Enabled() }()
	}
	wg.Wait()
}

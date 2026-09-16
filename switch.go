/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-16 08:12:36
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-16 09:07:52
* @FilePath: \go-bot\switch.go
* @Description: 运行期静音开关
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import "sync/atomic"

// Switch 是可在运行期翻转的发送开关零值即启用
// 因此 nil 的 *Switch 也视为启用Disable 之后，共享该实例的 Bot 的 Send 调用直接返回：
// 不发请求、不计入 sent/error，单独计入 muted，保证成功率不被静音污染
// 状态存放在 atomic.Bool 中，可被多个 goroutine 并发读写
type Switch struct {
	off atomic.Bool
}

// NewSwitch 返回一个处于启用状态的开关
func NewSwitch() *Switch { return &Switch{} }

// Disable 静音发送对 nil 接收者调用会 panic——静默忽略一次 Disable
// 会让调用方以为已停发，实际仍在发送
func (s *Switch) Disable() { s.off.Store(true) }

// Enable 恢复发送对 nil 接收者调用会 panic，理由同 Disable
func (s *Switch) Enable() { s.off.Store(false) }

// Enabled 报告当前是否允许发送nil 接收者返回 true，
// 使「未配置开关」与「开关处于启用态」是同一件事
func (s *Switch) Enabled() bool { return s == nil || !s.off.Load() }

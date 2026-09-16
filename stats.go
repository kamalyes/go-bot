/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-16 08:18:53
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-16 09:15:26
* @FilePath: \go-bot\stats.go
* @Description: 进程内发送统计，atomic 计数
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import "sync/atomic"

// Stats 使用 atomic 跟踪 Bot 级指标，无锁并发安全
// 三类计数互斥：成功计 sent，失败计 error，静音计 muted；
// 静音既不算成功也不算失败，因此不会污染成功率
type Stats struct {
	totalSent  atomic.Int64
	totalError atomic.Int64
	totalMuted atomic.Int64
}

// IncSent 递增成功计数
func (s *Stats) IncSent() { s.totalSent.Add(1) }

// IncError 递增失败计数（按发送任务计，重试多次仅计一次）
func (s *Stats) IncError() { s.totalError.Add(1) }

// IncMuted 递增静音丢弃计数
func (s *Stats) IncMuted() { s.totalMuted.Add(1) }

// TotalSent 返回成功发送总数
func (s *Stats) TotalSent() int64 { return s.totalSent.Load() }

// TotalError 返回失败总数
func (s *Stats) TotalError() int64 { return s.totalError.Load() }

// TotalMuted 返回静音丢弃总数
func (s *Stats) TotalMuted() int64 { return s.totalMuted.Load() }

// SuccessRate 返回 [0,1] 的成功率；无样本时返回 0
func (s *Stats) SuccessRate() float64 {
	sent, failed := s.TotalSent(), s.TotalError()
	total := sent + failed
	if total == 0 {
		return 0
	}
	return float64(sent) / float64(total)
}

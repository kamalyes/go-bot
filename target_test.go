/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2026-09-18 22:07:52
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2026-09-18 22:07:52
* @FilePath: \go-bot\target_test.go
* @Description: 发送目标定义测试：构造与本地校验
*
* Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gobot

import (
	"strings"
	"testing"
)

// TestUserConstructor 验证用户（单聊）目标构造
func TestUserConstructor(t *testing.T) {
	target := User("ou_8c1e5a2f")
	if target.ID != "ou_8c1e5a2f" || target.Type != TargetUser {
		t.Fatalf("User() = %+v, 期望 user 类型目标", target)
	}
}

// TestChatConstructor 验证群聊目标构造
func TestChatConstructor(t *testing.T) {
	target := Chat("oc_773abc")
	if target.ID != "oc_773abc" || target.Type != TargetChat {
		t.Fatalf("Chat() = %+v, 期望 chat 类型目标", target)
	}
}

// TestTargetValidate 验证发送前的目标本地校验
func TestTargetValidate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		target  Target
		wantErr string
	}{
		{"用户目标合法", User("42"), ""},
		{"群聊目标合法", Chat("-100233"), ""},
		{"含特殊字符的合法 id", Chat("grp::prod-🚨(上海)"), ""},
		{"ID 为空", Target{Type: TargetChat}, "target id is empty"},
		{"类型为空", Target{ID: "42"}, "target type is empty"},
	} {
		err := tc.target.validate()
		if tc.wantErr == "" {
			if err != nil {
				t.Fatalf("%s: validate() = %v, 期望通过", tc.name, err)
			}
			continue
		}
		if err == nil {
			t.Fatalf("%s: validate() = nil, 期望返回错误", tc.name)
		}
		if !strings.Contains(err.Error(), tc.wantErr) {
			t.Fatalf("%s: validate() = %v, 期望包含 %q", tc.name, err, tc.wantErr)
		}
	}
}

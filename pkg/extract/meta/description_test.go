// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package meta

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanDescription(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  string
	}{
		{
			"bilibili statistics block is cut",
			"视频中出现的工具 已整理至👇下载地址：https://example.com/x 喜欢的点个三连呗，谢谢啦~, " +
				"视频播放量 62、弹幕量 0、点赞数 1、投硬币枚数 0、收藏人数 8、转发人数 0, " +
				"视频作者 创心宝库, 作者简介 资源在简介或评论区自取～，相关视频：别的视频",
			"视频中出现的工具 已整理至👇下载地址：https://example.com/x 喜欢的点个三连呗，谢谢啦~",
		},
		{
			"related videos only",
			"一段足够长的真实简介，讲清楚这个视频到底在做什么，值得留下来。相关视频：第一个，第二个",
			"一段足够长的真实简介，讲清楚这个视频到底在做什么，值得留下来。",
		},
		{
			"plain description is untouched",
			"华为AI相关专利占比13%，是无线通讯之外第三大专利方向",
			"华为AI相关专利占比13%，是无线通讯之外第三大专利方向",
		},
		{
			"whitespace is collapsed",
			"  两行\n\t之间  有多余空格  ",
			"两行 之间 有多余空格",
		},
		{
			"empty stays empty",
			"   ",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.out, cleanDescription(tt.in))
		})
	}
}

func TestCleanDescriptionLimit(t *testing.T) {
	long := strings.Repeat("这是一句话。", 80)
	out := cleanDescription(long)
	assert.LessOrEqual(t, len([]rune(out)), descriptionLimit+1)
	assert.True(t, strings.HasSuffix(out, "…"))
	assert.NotContains(t, out, "\n")
}

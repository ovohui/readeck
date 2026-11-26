// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBrandNameAndClass(t *testing.T) {
	bili := displayItem("标题", "", "www.bilibili.com", "www.bilibili.com", "article")
	assert.Equal(t, "哔哩哔哩", brandName(bili))
	assert.Equal(t, "brand-bilibili", brandClass(bili))

	xhs := displayItem("标题", "", "小红书", "www.xiaohongshu.com", "article")
	assert.Equal(t, "小红书", brandName(xhs))
	assert.Equal(t, "brand-xiaohongshu", brandClass(xhs))

	imported := displayItem("标题", "", "import.local", "import.local", "article")
	assert.Equal(t, "本地导入", brandName(imported))

	unknown := displayItem("标题", "", "example.org", "example.org", "article")
	assert.Equal(t, "example.org", brandName(unknown))
	assert.Contains(t, brandClass(unknown), "brand-c")
}

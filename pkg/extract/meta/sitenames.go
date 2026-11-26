// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package meta

// siteLabels are the site names that need more than the generic "last word
// after a separator" rule: they contain a space, a second brand, or are part
// of a longer suffix such as "- OSCHINA - 开源 × AI · 开发者生态社区".
//
// Short, ambiguous names ("界面", "知乎", "快讯") are deliberately absent: the
// generic rule already copes with "标题 - 知乎", and cutting at a word that can
// also be the subject of a headline would do more harm than good.
var siteLabels = map[string]bool{
	"哔哩哔哩":      true,
	"bilibili":  true,
	"财新网":       true,
	"界面新闻":      true,
	"网易政务":      true,
	"澎湃新闻":      true,
	"中国政库":      true,
	"the paper": true,
	"oschina":   true,
	"ithome":    true,
	"少数派":       true,
	"什么值得买":     true,
	"36氪":       true,
	"虎扑社区":      true,
	"开源中国":      true,
	"腾讯新闻":      true,
	"微信公众平台":    true,
	"微信公众号":     true,
	"小红书":       true,
	"smzdm":     true,
	"sspai":     true,
	"infoq":     true,
	"cnblogs":   true,
	"juejin":    true,
	"douban":    true,
	"hupu":      true,
}

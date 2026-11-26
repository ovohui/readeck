// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"fmt"
	"hash/fnv"
	"strings"

	"codeberg.org/readeck/readeck/internal/bookmarks/dataset"
)

// brandColors are the colours people already associate with a site. They are
// used for the card of a page that has no picture; a solid brand colour with
// the site name reads much better than a gradient with a tiny favicon on it.
var brandColors = map[string]string{
	"bilibili.com":    "#FB7299",
	"xiaohongshu.com": "#FF2442",
	"zhihu.com":       "#0084FF",
	"weibo.com":       "#E6162D",
	"douban.com":      "#2E963D",
	"sspai.com":       "#D71A1B",
	"36kr.com":        "#1E6FFF",
	"ithome.com":      "#D22222",
	"oschina.net":     "#3AA757",
	"juejin.cn":       "#1E80FF",
	"cnblogs.com":     "#2B6698",
	"thepaper.cn":     "#D71518",
	"people.com.cn":   "#C7000B",
	"cctv.com":        "#C7000B",
	"qq.com":          "#1E6FFF",
	"163.com":         "#D22222",
	"sohu.com":        "#FF6A00",
	"caixin.com":      "#B01F24",
	"cls.cn":          "#C7000B",
	"yicai.com":       "#D7000F",
	"jiemian.com":     "#003B7A",
	"hupu.com":        "#D7000F",
	"toutiao.com":     "#F04142",
	"jd.com":          "#E1251B",
	"baidu.com":       "#2932E1",
	"weixin.qq.com":   "#07C160",
	"import.local":    "#4B5563",
}

// brandFallbacks are used for sites that are not in the table. They are dark
// enough for white text and deliberately varied, but stable per domain.
var brandFallbacks = []string{
	"#2F6FED", "#E4572E", "#1B998B", "#6A4C93", "#C1121F",
	"#3A7D44", "#0F766E", "#B45309", "#7C3AED", "#0369A1",
}

var brandNames = map[string]string{
	"bilibili.com":    "哔哩哔哩",
	"xiaohongshu.com": "小红书",
	"zhihu.com":       "知乎",
	"weibo.com":       "微博",
	"douban.com":      "豆瓣",
	"sspai.com":       "少数派",
	"36kr.com":        "36氪",
	"ithome.com":      "IT之家",
	"oschina.net":     "开源中国",
	"juejin.cn":       "掘金",
	"cnblogs.com":     "博客园",
	"thepaper.cn":     "澎湃新闻",
	"people.com.cn":   "人民网",
	"cctv.com":        "央视网",
	"qq.com":          "腾讯新闻",
	"163.com":         "网易",
	"sohu.com":        "搜狐",
	"caixin.com":      "财新网",
	"cls.cn":          "财联社",
	"yicai.com":       "第一财经",
	"jiemian.com":     "界面新闻",
	"hupu.com":        "虎扑",
	"toutiao.com":     "今日头条",
	"jd.com":          "京东",
	"baidu.com":       "百度",
	"weixin.qq.com":   "微信公众号",
	"import.local":    "本地导入",
}

// brandName returns the short name printed on a solid card.
func brandName(item *dataset.Bookmark) string {
	if item.SiteName != "" && item.SiteName != item.Site {
		name := strings.TrimSpace(item.SiteName)
		if i := strings.IndexAny(name, "-|·"); i > 3 {
			name = strings.TrimSpace(name[:i])
		}
		if name != "" {
			return name
		}
	}

	host := strings.TrimSpace(item.Site)
	host = strings.TrimPrefix(host, "www.")
	if name, ok := brandNames[host]; ok {
		return name
	}
	if host != "" {
		return host
	}
	return strings.TrimSpace(item.Domain)
}

// brandColor returns the solid background colour of a card without a picture.
func brandColor(item *dataset.Bookmark) string {
	if host := strings.ToLower(item.Site); host != "" {
		for domain, color := range brandColors {
			if host == domain || strings.HasSuffix(host, "."+domain) {
				return color
			}
		}
	}

	name := strings.ToLower(brandName(item))
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return brandFallbacks[int(h.Sum32())%len(brandFallbacks)]
}

var brandColorNames = map[string]string{
	"#FB7299": "bilibili",
	"#FF2442": "xiaohongshu",
	"#0084FF": "zhihu",
	"#E6162D": "weibo",
	"#2E963D": "douban",
	"#D71A1B": "sspai",
	"#1E6FFF": "blue",
	"#D22222": "red",
	"#3AA757": "green",
	"#1E80FF": "juejin",
	"#2B6698": "cnblogs",
	"#D71518": "thepaper",
	"#C7000B": "red",
	"#FF6A00": "sohu",
	"#B01F24": "caixin",
	"#D7000F": "red2",
	"#003B7A": "jiemian",
	"#F04142": "toutiao",
	"#E1251B": "jd",
	"#2932E1": "baidu",
	"#07C160": "weixin",
}

// brandClass returns the CSS class carrying the card colour. An inline style
// would be simpler, but the reading page ships a CSP with a style nonce, and
// inline style attributes are blocked by it.
func brandClass(item *dataset.Bookmark) string {
	color := brandColor(item)
	if name, ok := brandColorNames[color]; ok {
		return "brand-" + name
	}
	for i, c := range brandFallbacks {
		if c == color {
			return fmt.Sprintf("brand-c%d", i)
		}
	}
	return "brand-c0"
}

// brandTextColor picks white or near-black text for a background colour.
func brandTextColor(item *dataset.Bookmark) string {
	c := brandColor(item)
	var r, g, b int
	if _, err := fmt.Sscanf(c, "#%02x%02x%02x", &r, &g, &b); err != nil {
		return "#ffffff"
	}
	// Relative luminance, sRGB approximation. The threshold keeps the common
	// brand colours (Bilibili pink, Xiaohongshu red, Zhihu blue) on white.
	lum := 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)
	if lum > 186 {
		return "#111827"
	}
	return "#ffffff"
}

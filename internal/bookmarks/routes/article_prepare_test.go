// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/readeck/readeck/internal/bookmarks/dataset"
)

func prepareItem(typ, title string) *dataset.Bookmark {
	item := displayItem(title, "", "站点", "example.com", typ)
	item.Resources["image"] = &dataset.BookmarkFile{Src: "/bm/xx/img/cover.jpg"}
	return item
}

func TestPrepareArticleVideoPoster(t *testing.T) {
	item := prepareItem("video", "标题")
	raw := `<section><article><figure><video controls src="./_resources/x.mp4"></video></figure></article></section>`
	out := prepareArticleHTML(item, raw)
	assert.Contains(t, out, `poster="/bm/xx/img/cover.jpg"`)
	assert.Contains(t, out, `<video`)
}

func TestPrepareArticleAnchoredPosterIsKept(t *testing.T) {
	item := prepareItem("video", "标题")
	raw := `<figure><video controls poster="/poster.jpg" src="./_resources/x.mp4"></video></figure>`
	out := prepareArticleHTML(item, raw)
	assert.Contains(t, out, `poster="/poster.jpg"`)
	assert.NotContains(t, out, "/bm/xx/img/cover.jpg")
}

func TestPrepareArticleDuplicateHeading(t *testing.T) {
	item := prepareItem("article", "上海市委常委会举行扩大会议的决定")
	raw := `<section><article><h2>上海市委常委会举行扩大会议的决定</h2><p>正文第一段。</p></article></section>`
	out := prepareArticleHTML(item, raw)
	assert.NotContains(t, out, "<h2>")
	assert.Contains(t, out, "正文第一段。")
}

func TestPrepareArticleKeepsInnerHeading(t *testing.T) {
	item := prepareItem("article", "标题")
	raw := `<section><article><p>开头。</p><h2>标题</h2><p>正文。</p></article></section>`
	out := prepareArticleHTML(item, raw)
	assert.Contains(t, out, "<h2>标题</h2>")
}

func TestPrepareArticlePhotoGallery(t *testing.T) {
	item := prepareItem("photo", "标题")
	raw := `<section><article><p>说明。</p>` +
		`<figure><img src="a.jpg"></figure><figure><img src="b.jpg"></figure><figure><img src="c.jpg"></figure>` +
		`</article></section>`
	out := prepareArticleHTML(item, raw)
	assert.Contains(t, out, `class="photo-gallery"`)
	assert.Equal(t, 1, strings.Count(out, `class="photo-gallery"`))
	assert.Equal(t, 3, strings.Count(out, "<figure>"))
}

func TestPrepareArticlePhotoPostByContent(t *testing.T) {
	item := displayItem("四条照片的笔记", "这是一条只有照片的笔记。", "www.xiaohongshu.com", "www.xiaohongshu.com", "article")
	raw := `<section><article><p>这是一条只有照片的笔记。</p>` +
		`<figure><img src="a.jpg"></figure><figure><img src="b.jpg"></figure>` +
		`<figure><img src="c.jpg"></figure><figure><img src="d.jpg"></figure>` +
		`</article></section>`
	out := prepareArticleHTML(item, raw)
	assert.Contains(t, out, `class="photo-gallery"`)
}

func TestPrepareArticleSinglePhotoIsNotGrouped(t *testing.T) {
	item := prepareItem("photo", "标题")
	raw := `<section><article><figure><img src="a.jpg"></figure></article></section>`
	out := prepareArticleHTML(item, raw)
	assert.NotContains(t, out, "photo-gallery")
}

func TestPrepareArticleVideoOnlyIsNotEmpty(t *testing.T) {
	item := prepareItem("video", "标题")
	raw := `<figure><video controls src="./_resources/x.mp4"></video></figure>`
	assert.NotEqual(t, "", prepareArticleHTML(item, raw))
}

func TestPrepareArticleEmpty(t *testing.T) {
	item := prepareItem("article", "标题")
	assert.Equal(t, "", prepareArticleHTML(item, "   "))
}

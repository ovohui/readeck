// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package bleach_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/go-shiori/dom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"codeberg.org/readeck/readeck/pkg/bleach"
)

func TestClean(t *testing.T) {
	tests := []struct {
		fragment string
		expected string
	}{
		{
			`<p id="id-p" class="test" data-test="1" onClick="bar" rel="link" sizes="x1">foo</p>`,
			`<body><p id="id-p">foo</p></body>`,
		},
		{
			`<div><iframe src="http://example.net/" /></div>`,
			`<body></body>`,
		},
		{
			`<div><noscript>test</noscript></div>`,
			`<body><div><div>test</div></div></body>`,
		},
		{
			`<div><custom>hi<script>alert("test")</script></custom></div>`,
			`<body><div><div>hi</div></div></body>`,
		},
		{
			`<p><font>test</font></div>`,
			`<body><p><span>test</span></p></body>`,
		},
		{
			`<div><iframe src="http://example.net/" /><link><script></div>`,
			`<body></body>`,
		},
		{
			`<p>test</p><span></span><br /><p>test 2</p>`,
			`<body><p>test</p><br/><p>test 2</p></body>`,
		},
		{
			`<p>test</p><div><span></span></div><br /><p>test 2</p>`,
			`<body><p>test</p><br/><p>test 2</p></body>`,
		},
		{
			`<p>test</p><div><span> </span></div><br /><p>test 😺</p>`,
			`<body><p>test</p><div><span> </span></div><br/><p>test 😺</p></body>`,
		},
		{
			`<p><a name="foo"></a></p><p>test</p>`,
			`<body><p><a name="foo"></a></p><p>test</p></body>`,
		},
		{
			`<pre> test </pre>`,
			`<body><pre> test </pre></body>`,
		},
		{
			`<pre>
				test:
						<code>abc</code>
			</pre>`,
			`<body><pre>				test:
						<code>abc</code>
			</pre></body>`,
		},
		{
			`<table><tr><td></td><td>test</td></tr>
			<tr><td>A</td><td>B</td></tr></table>`,
			`<body><table><tbody><tr><td></td><td>test</td></tr>
			<tr><td>A</td><td>B</td></tr></tbody></table></body>`,
		},
		{
			`<math><mtable><mrow><mtd></mtd><mtd>2</mtd></mrow><mrow></mrow></mtable><mspace></mspace></math>`,
			`<body><math><mtable><mrow><mtd></mtd><mtd>2</mtd></mrow><mrow></mrow></mtable><mspace></mspace></math></body>`,
		},
		{
			`<p><a href="foo">link</a></p>`,
			`<body><p><a href="foo" rel="nofollow noopener noreferrer">link</a></p></body>`,
		},
		{
			`<p><a name="foo"></a></p>`,
			`<body><p><a name="foo"></a></p></body>`,
		},
		{
			`<p id="id-p" class="test" data-test="1" onClick="bar" rel="link" sizes="x1"><a name="foo"></a></p><p><a href="foo">link</a><span></span><hr></p>`,
			`<body><p id="id-p"><a name="foo"></a></p><p><a href="foo" rel="nofollow noopener noreferrer">link</a></p><hr/></body>`,
		},
		{
			`<div>
				<audio>1</audio>
				<applet>2</applet>
				<base/>
				<button>3</button>
				<canvas>4</canvas>
				<dialog>5</dialog>
				<embed type="video/quicktime" src="movie.mov"/>
				<frameset cols="400, 500"><frame src="myframe"/></frameset>
				<iframe src="myframe">6</iframe>
				<input value="hi"/>
				<link href="url"/>
				<meta content="hi"/>
				<param/>
				<portal>7</portal>
				<script>8</script>
				<select>9</select>
				<slot>10</slot>
				<source/>
				<style>11</style>
				<template>12</template>
				<textarea>13</textarea>
				<track default/>
				<video>14</video>
			</div>`,
			`<body></body>`,
		},
		{
			`<img style="max-width:100%" src="url" data-readeck-width="800" data-readeck-height="600"/>`,
			`<body><img src="url" data-readeck-width="800" data-readeck-height="600"/></body>`,
		},
		{
			`<p>test</p><div><span> </span></div><br /><p>😺</p>`,
			`<body><p>test</p><div><span> </span></div><br/><p>😺</p></body>`,
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			r := strings.NewReader(test.fragment)
			node, err := html.Parse(r)
			require.NoError(t, err)

			bleach.DefaultPolicy.Clean(node)
			require.Equal(t, test.expected, dom.OuterHTML(dom.QuerySelector(node, "body")))
		})
	}
}

func TestClean_DataAtom(t *testing.T) {
	addElement := func(parent *html.Node, tagName string) *html.Node {
		el := &html.Node{
			Type: html.ElementNode,
			// This test on purpose populates only Data but no DataAtom.
			Data: tagName,
		}
		parent.AppendChild(el)
		return el
	}

	doc := &html.Node{Type: html.DocumentNode}
	htmlEl := addElement(doc, "html")
	body := addElement(htmlEl, "body")
	p := addElement(body, "p")

	text := &html.Node{
		Type: html.TextNode,
		Data: "Hi",
	}
	p.AppendChild(text)

	expectedHTML := `<html><body><p>Hi</p></body></html>`

	bleach.DefaultPolicy.Clean(doc)
	assert.Equal(t, expectedHTML, dom.OuterHTML(doc))
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{string([]rune{0, 1, 31, 32, 33, 65}), " !A"},
		{string([]rune{147, 65, 12616}), "Aㅈ"},
		{string([]rune{145, 128571, 155}), "😻"},
		{"test\n\tabc", "test\n\tabc"},
		{"test\r\n\tabc", "test\r\n\tabc"},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			require.Equal(t, test.expected, bleach.SanitizeString(test.input))
		})
	}
}

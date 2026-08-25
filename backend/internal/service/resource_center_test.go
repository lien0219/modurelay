package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeBannedWords(t *testing.T) {
	got := normalizeBannedWords([]string{" scam ", "SCAM", "", "广告", "广告"})
	require.Equal(t, []string{"scam", "广告"}, got)
}

func TestResourceURLPattern(t *testing.T) {
	require.True(t, resourceURLPattern.MatchString("请访问 https://example.com 获取资源"))
	require.True(t, resourceURLPattern.MatchString("www.example.com"))
	require.False(t, resourceURLPattern.MatchString("这是一段没有链接的内容"))
}

func TestNormalizeResourceIDs(t *testing.T) {
	got := normalizeResourceIDs([]int64{3, 0, 3, -1, 7})
	require.Equal(t, []int64{3, 7}, got)
}

func TestSanitizeResourceContent(t *testing.T) {
	html, text, err := sanitizeResourceContent(`<p>Hello <strong>world</strong></p><script>alert(1)</script><a href="https://example.com">link</a>`)
	require.NoError(t, err)
	require.NotContains(t, html, "script")
	require.Contains(t, html, "<strong>world</strong>")
	require.Contains(t, html, `rel="noopener noreferrer"`)
	require.Equal(t, "Hello worldlink", text)
}

func TestNormalizeResourcePage(t *testing.T) {
	page, size := normalizeResourcePage(0, 999)
	require.Equal(t, 1, page)
	require.Equal(t, ResourceMaxPageSize, size)
}

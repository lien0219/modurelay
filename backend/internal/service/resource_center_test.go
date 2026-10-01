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

func TestResourceSensitivePublicIdentity(t *testing.T) {
	require.True(t, resourceSensitivePublicIdentity("13455212442"))
	require.True(t, resourceSensitivePublicIdentity("+86 134-5521-2442"))
	require.True(t, resourceSensitivePublicIdentity("user@example.com"))
	require.True(t, resourceSensitivePublicIdentity("123456"))
	require.False(t, resourceSensitivePublicIdentity("cui"))
	require.False(t, resourceSensitivePublicIdentity("星河"))
}

func TestResourcePublicAuthorNameDoesNotFallbackToLoginIdentity(t *testing.T) {
	require.Equal(t, "cui", resourcePublicAuthorName(24, "cui", "private@example.com", "user"))
	require.Equal(t, "用户 24", resourcePublicAuthorName(24, "13455212442", "private@example.com", "user"))
	require.Equal(t, "用户 24", resourcePublicAuthorName(24, "", "private@example.com", "user"))
	require.Equal(t, "官方管理员", resourcePublicAuthorName(1, "13800138000", "admin@example.com", "admin"))
}

func TestResourceSafeSnapshotNameProtectsHistoricalPII(t *testing.T) {
	require.Equal(t, "用户 77", resourceSafeSnapshotName(77, "someone@example.com", "user"))
	require.Equal(t, "用户 77", resourceSafeSnapshotName(77, "+1 (202) 555-0101", "user"))
	require.Equal(t, "forum-user", resourceSafeSnapshotName(77, "forum-user", "user"))
}

package service

import (
	"bytes"
	"mime/multipart"
	"net/textproto"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPrepareGrokVideoGenerationForwardBodyMultipartTextOnly(t *testing.T) {
	body, contentType := buildGrokVideoMultipart(t, func(writer *multipart.Writer) {
		require.NoError(t, writer.WriteField("model", "grok-imagine-video-1.5"))
		require.NoError(t, writer.WriteField("prompt", "cinematic city walk"))
		require.NoError(t, writer.WriteField("seconds", "5"))
		require.NoError(t, writer.WriteField("size", "854x480"))
		require.NoError(t, writer.WriteField("resolution_name", "480p"))
		require.NoError(t, writer.WriteField("generate_audio", "true"))
		require.NoError(t, writer.WriteField("watermark", "false"))
		require.NoError(t, writer.WriteField("mode", "frames"))
	})

	out, outType, err := prepareGrokMediaForwardBody(GrokMediaEndpointVideosGenerations, body, contentType)
	require.NoError(t, err)
	assert.Equal(t, "application/json", outType)
	assert.Equal(t, "grok-imagine-video-1.5", gjson.GetBytes(out, "model").String())
	assert.Equal(t, "cinematic city walk", gjson.GetBytes(out, "prompt").String())
	assert.Equal(t, int64(5), gjson.GetBytes(out, "duration").Int())
	assert.Equal(t, "16:9", gjson.GetBytes(out, "aspect_ratio").String())
	assert.Equal(t, "480p", gjson.GetBytes(out, "resolution").String())
	assert.True(t, gjson.GetBytes(out, "generate_audio").Bool())
	for _, legacy := range []string{"seconds", "size", "resolution_name", "watermark", "mode"} {
		assert.False(t, gjson.GetBytes(out, legacy).Exists(), legacy)
	}
}

func TestPrepareGrokVideoGenerationForwardBodyMultipartFirstFrame(t *testing.T) {
	body, contentType := buildGrokVideoMultipart(t, func(writer *multipart.Writer) {
		require.NoError(t, writer.WriteField("model", "grok-imagine-video-1.5"))
		require.NoError(t, writer.WriteField("prompt", "animate this still"))
		require.NoError(t, writer.WriteField("seconds", "6"))
		require.NoError(t, writer.WriteField("resolution_name", "720p"))
		require.NoError(t, writer.WriteField("mode", "frames"))
		writeGrokVideoImagePart(t, writer, "first_frame", "first.png", []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a})
	})

	out, outType, err := prepareGrokMediaForwardBody(GrokMediaEndpointVideosGenerations, body, contentType)
	require.NoError(t, err)
	assert.Equal(t, "application/json", outType)
	assert.Equal(t, "grok-imagine-video-1.5", gjson.GetBytes(out, "model").String())
	assert.True(t, strings.HasPrefix(gjson.GetBytes(out, "image.url").String(), "data:image/png;base64,"))
	assert.False(t, gjson.GetBytes(out, "first_frame").Exists())
}

func TestPrepareGrokVideoGenerationForwardBodyMultipartReferences(t *testing.T) {
	body, contentType := buildGrokVideoMultipart(t, func(writer *multipart.Writer) {
		require.NoError(t, writer.WriteField("model", "grok-imagine-video-1.5"))
		require.NoError(t, writer.WriteField("prompt", "use these references"))
		require.NoError(t, writer.WriteField("mode", "reference"))
		writeGrokVideoImagePart(t, writer, "image[]", "a.png", []byte{0x89, 'P', 'N', 'G', 1})
		writeGrokVideoImagePart(t, writer, "image[]", "b.png", []byte{0x89, 'P', 'N', 'G', 2})
	})

	out, outType, err := prepareGrokMediaForwardBody(GrokMediaEndpointVideosGenerations, body, contentType)
	require.NoError(t, err)
	assert.Equal(t, "application/json", outType)
	refs := gjson.GetBytes(out, "reference_images").Array()
	require.Len(t, refs, 2)
	for _, ref := range refs {
		assert.True(t, strings.HasPrefix(ref.Get("url").String(), "data:image/png;base64,"))
	}
}

func TestPrepareGrokVideoGenerationReferenceCaps1080PAt720P(t *testing.T) {
	body, contentType := buildGrokVideoMultipart(t, func(writer *multipart.Writer) {
		require.NoError(t, writer.WriteField("model", "grok-imagine-video-1.5"))
		require.NoError(t, writer.WriteField("prompt", "reference clip"))
		require.NoError(t, writer.WriteField("mode", "reference"))
		require.NoError(t, writer.WriteField("resolution_name", "1080p"))
		writeGrokVideoImagePart(t, writer, "image[]", "reference.png", []byte{0x89, 'P', 'N', 'G', 3})
	})

	out, _, err := prepareGrokMediaForwardBody(GrokMediaEndpointVideosGenerations, body, contentType)
	require.NoError(t, err)
	assert.Equal(t, "720p", gjson.GetBytes(out, "resolution").String())
}

func TestNormalizeGrokVideoGenerationJSONCompatibility(t *testing.T) {
	body := []byte(`{
		"model":"grok-imagine-video-1.5",
		"prompt":"test",
		"seconds":"10",
		"size":"720x1280",
		"resolution_name":"720p",
		"watermark":false,
		"mode":"frames",
		"first_frame":{"image_url":"data:image/png;base64,AAAA"}
	}`)

	out, contentType, err := prepareGrokMediaForwardBody(GrokMediaEndpointVideosGenerations, body, "application/json")
	require.NoError(t, err)
	out, contentType, err = normalizeGrokMediaForwardBody(GrokMediaEndpointVideosGenerations, out, contentType)
	require.NoError(t, err)
	assert.Equal(t, "application/json", contentType)
	assert.Equal(t, int64(10), gjson.GetBytes(out, "duration").Int())
	assert.Equal(t, "9:16", gjson.GetBytes(out, "aspect_ratio").String())
	assert.Equal(t, "720p", gjson.GetBytes(out, "resolution").String())
	assert.Equal(t, "data:image/png;base64,AAAA", gjson.GetBytes(out, "image.url").String())
	assert.Equal(t, "image_url", gjson.GetBytes(out, "image.type").String())
	for _, legacy := range []string{"seconds", "size", "resolution_name", "watermark", "mode", "first_frame"} {
		assert.False(t, gjson.GetBytes(out, legacy).Exists(), legacy)
	}
}

func buildGrokVideoMultipart(t *testing.T, fill func(*multipart.Writer)) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fill(writer)
	require.NoError(t, writer.Close())
	return buf.Bytes(), writer.FormDataContentType()
}

func writeGrokVideoImagePart(t *testing.T, writer *multipart.Writer, field, filename string, data []byte) {
	t.Helper()
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="`+field+`"; filename="`+filename+`"`)
	header.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
}

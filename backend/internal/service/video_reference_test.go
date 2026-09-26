package service

import "testing"

func TestParseUnifiedVideoReferencesPreservesCanvasRoles(t *testing.T) {
	body := []byte(`{"media":[{"type":"first_frame","url":"https://example.com/a.png"},{"type":"last_frame","url":"https://example.com/b.png"},{"type":"reference_video","url":"https://example.com/ref.mp4"},{"type":"reference_audio","url":"https://example.com/ref.mp3"}]}`)
	refs := parseUnifiedVideoReferences(body)
	if len(refs) != 4 {
		t.Fatalf("refs=%#v", refs)
	}
	if mode := normalizeUnifiedVideoMode("", refs); mode != "frames2video" {
		t.Fatalf("mode=%q", mode)
	}
	images, videos, audios := splitVideoReferences(refs)
	if len(images) != 2 || len(videos) != 1 || len(audios) != 1 {
		t.Fatalf("images=%#v videos=%#v audios=%#v", images, videos, audios)
	}
}

func TestAIStarsLabOpenAPIReferenceValidation(t *testing.T) {
	if AIStarsLabOpenAPIReferencesSupported([]byte(`{"media":[{"type":"reference_image","url":"data:image/png;base64,AAAA"}]}`)) {
		t.Fatal("data URL should not be treated as an OpenAPI public reference")
	}
	refs := []videoReference{
		{Kind: videoReferenceFirstFrame, URL: "https://example.com/a.png"},
		{Kind: videoReferenceLastFrame, URL: "https://example.com/b.png"},
	}
	if err := validateAIStarsLabOpenAPIReferences("frames2video", refs); err != nil {
		t.Fatal(err)
	}
}

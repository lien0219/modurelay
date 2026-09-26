package service

import (
	"errors"
	"net/url"
	"strings"

	"github.com/tidwall/gjson"
)

type videoReference struct {
	Kind string
	URL  string
}

const (
	videoReferenceFirstFrame = "first_frame"
	videoReferenceLastFrame  = "last_frame"
	videoReferenceImage      = "reference_image"
	videoReferenceVideo      = "reference_video"
	videoReferenceAudio      = "reference_audio"
)

func appendVideoReference(out []videoReference, seen map[string]struct{}, kind, rawURL string) []videoReference {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return out
	}
	switch kind {
	case videoReferenceFirstFrame, videoReferenceLastFrame, videoReferenceImage, videoReferenceVideo, videoReferenceAudio:
	default:
		return out
	}
	key := kind + "\x00" + rawURL
	if _, exists := seen[key]; exists {
		return out
	}
	seen[key] = struct{}{}
	return append(out, videoReference{Kind: kind, URL: rawURL})
}

func parseVideoMediaReferences(body []byte) []videoReference {
	if !gjson.ValidBytes(body) {
		return nil
	}
	media := gjson.GetBytes(body, "media")
	if !media.IsArray() {
		return nil
	}
	refs := make([]videoReference, 0, len(media.Array()))
	seen := make(map[string]struct{}, len(media.Array()))
	for _, item := range media.Array() {
		kind := strings.ToLower(strings.TrimSpace(item.Get("type").String()))
		rawURL := strings.TrimSpace(item.Get("url").String())
		if rawURL == "" {
			rawURL = strings.TrimSpace(item.Get("image_url.url").String())
		}
		if rawURL == "" {
			rawURL = strings.TrimSpace(item.Get("video_url.url").String())
		}
		if rawURL == "" {
			rawURL = strings.TrimSpace(item.Get("audio_url.url").String())
		}
		refs = appendVideoReference(refs, seen, kind, rawURL)
	}
	return refs
}

func parseUnifiedVideoReferences(body []byte) []videoReference {
	if !gjson.ValidBytes(body) {
		return nil
	}
	refs := parseVideoMediaReferences(body)
	seen := make(map[string]struct{}, len(refs)+8)
	for _, ref := range refs {
		seen[ref.Kind+"\x00"+ref.URL] = struct{}{}
	}

	appendValues := func(path, kind string) {
		value := gjson.GetBytes(body, path)
		if !value.Exists() {
			return
		}
		values := []gjson.Result{value}
		if value.IsArray() {
			values = value.Array()
		}
		for _, item := range values {
			rawURL := strings.TrimSpace(item.String())
			if item.IsObject() {
				rawURL = strings.TrimSpace(item.Get("url").String())
			}
			refs = appendVideoReference(refs, seen, kind, rawURL)
		}
	}

	if top := extractGrokMediaImageURL(gjson.GetBytes(body, "image")); top != "" {
		refs = appendVideoReference(refs, seen, videoReferenceFirstFrame, top)
	}
	if last := extractGrokMediaImageURL(gjson.GetBytes(body, "last_frame")); last != "" {
		refs = appendVideoReference(refs, seen, videoReferenceLastFrame, last)
	}
	appendValues("reference_images", videoReferenceImage)
	appendValues("images", videoReferenceImage)
	appendValues("metadata.images", videoReferenceImage)
	appendValues("metadata.videos", videoReferenceVideo)
	appendValues("metadata.audios", videoReferenceAudio)
	return refs
}

func normalizeUnifiedVideoMode(raw string, refs []videoReference) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "text", "text2video", "text-to-video":
		return "text2video"
	case "frames", "frames2video", "first_last", "first-last":
		return "frames2video"
	case "reference", "image2video", "image-to-video":
		return "image2video"
	case "":
	default:
		return strings.TrimSpace(raw)
	}
	hasFirst, hasLast := false, false
	for _, ref := range refs {
		switch ref.Kind {
		case videoReferenceFirstFrame:
			hasFirst = true
		case videoReferenceLastFrame:
			hasLast = true
		}
	}
	if hasFirst && hasLast {
		return "frames2video"
	}
	if len(refs) > 0 {
		return "image2video"
	}
	return "text2video"
}

func splitVideoReferences(refs []videoReference) (images, videos, audios []string) {
	for _, ref := range refs {
		switch ref.Kind {
		case videoReferenceFirstFrame, videoReferenceLastFrame, videoReferenceImage:
			images = append(images, ref.URL)
		case videoReferenceVideo:
			videos = append(videos, ref.URL)
		case videoReferenceAudio:
			audios = append(audios, ref.URL)
		}
	}
	return images, videos, audios
}

func isHTTPMediaReference(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func AIStarsLabOpenAPIReferencesSupported(body []byte) bool {
	for _, ref := range parseUnifiedVideoReferences(body) {
		if !isHTTPMediaReference(ref.URL) {
			return false
		}
	}
	return true
}

func validateAIStarsLabOpenAPIReferences(mode string, refs []videoReference) error {
	images, videos, audios := splitVideoReferences(refs)
	for _, ref := range refs {
		if !isHTTPMediaReference(ref.URL) {
			return errors.New("AIStarsLab OpenAPI reference media must use a public http(s) URL")
		}
	}
	switch mode {
	case "text2video":
		if len(refs) != 0 {
			return errors.New("text2video does not accept reference media")
		}
	case "frames2video":
		if len(images) != 2 || len(videos) != 0 || len(audios) != 0 {
			return errors.New("frames2video requires exactly two reference images")
		}
	case "image2video":
		if len(refs) == 0 {
			return errors.New("image2video requires at least one reference media item")
		}
	}
	return nil
}


func SeedanceCompatibleReferencesSupported(body []byte) bool {
	for _, ref := range parseUnifiedVideoReferences(body) {
		if ref.Kind == videoReferenceVideo || ref.Kind == videoReferenceAudio {
			return false
		}
	}
	return true
}

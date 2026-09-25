package service

import (
	"net/url"
	"strings"
	"unicode"
)

// VideoModelRef separates an externally visible provider-qualified model ID
// from the canonical model family used for routing and account eligibility.
// RawModel is always preserved so provider adapters can forward the exact ID
// required by an aggregator.
type VideoModelRef struct {
	RawModel       string
	ChannelCode    string
	CanonicalModel string
}

func ParseVideoModelRef(model string) VideoModelRef {
	raw := strings.TrimSpace(model)
	ref := VideoModelRef{RawModel: raw, CanonicalModel: raw}
	if raw == "" {
		return ref
	}
	idx := strings.IndexByte(raw, ':')
	if idx <= 0 || idx >= len(raw)-1 {
		return ref
	}
	channel := strings.TrimSpace(raw[:idx])
	canonical := strings.TrimSpace(raw[idx+1:])
	if !validVideoChannelCode(channel) || canonical == "" {
		return ref
	}
	ref.ChannelCode = channel
	ref.CanonicalModel = canonical
	return ref
}

func validVideoChannelCode(channel string) bool {
	if channel == "" || len(channel) > 64 {
		return false
	}
	for _, r := range channel {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case '-', '_', '.':
			continue
		default:
			return false
		}
	}
	return true
}

func CanonicalVideoModel(model string) string {
	return ParseVideoModelRef(model).CanonicalModel
}

func IsSeedanceVideoModel(model string) bool {
	canonical := strings.ToLower(strings.TrimSpace(CanonicalVideoModel(model)))
	return strings.HasPrefix(canonical, "seedance-")
}

func aiStarsLabBaseURLKind(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !strings.EqualFold(parsed.Hostname(), "api.video.aistarslab.com") {
		return ""
	}
	path := strings.TrimRight(strings.ToLower(strings.TrimSpace(parsed.Path)), "/")
	switch {
	case strings.HasSuffix(path, "/openapi"):
		return "openapi"
	case strings.HasSuffix(path, "/openai"):
		return "openai"
	default:
		return ""
	}
}

func IsAIStarsLabOpenAICompatibleAccount(account *Account) bool {
	return account != nil && account.Type == AccountTypeAPIKey &&
		aiStarsLabBaseURLKind(account.GetCredential("base_url")) == "openai"
}

func IsAIStarsLabOpenAPIAccount(account *Account) bool {
	return account != nil && account.Type == AccountTypeAPIKey &&
		aiStarsLabBaseURLKind(account.GetCredential("base_url")) == "openapi"
}

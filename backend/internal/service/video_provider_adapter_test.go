package service

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestPrepareAIStarsLabOpenAPIVideoCreate(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://api.video.aistarslab.com/openapi",
			"model_mapping": map[string]any{"seedance-2.0": "48:seedance-2.0"},
		},
	}
	body := []byte(`{"model":"48:seedance-2.0","prompt":"hello","duration":5,"resolution":"720p","aspect_ratio":"9:16"}`)
	out, upstream, _, err := prepareAIStarsLabOpenAPIVideoCreate(account, body, "application/json", "seedance-2.0")
	if err != nil {
		t.Fatal(err)
	}
	if upstream != "48:seedance-2.0" {
		t.Fatalf("upstream=%q", upstream)
	}
	if gjson.GetBytes(out, "channel").String() != "48" || gjson.GetBytes(out, "model").String() != "seedance-2.0" {
		t.Fatalf("unexpected model routing: %s", out)
	}
	if gjson.GetBytes(out, "quality").String() != "720p" || gjson.GetBytes(out, "aspectRatio").String() != "9:16" {
		t.Fatalf("unexpected options: %s", out)
	}
}

func TestPrepareSeedanceCompatibleCreateBodyStripsSupplierPrefix(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://ark.cn-beijing.volces.com/api/v3",
			"openai_capabilities": []any{"seedance"},
			"model_mapping": map[string]any{"seedance-2.0": "doubao-seedance-2-0-test"},
		},
	}
	body := []byte(`{"model":"48:seedance-2.0","prompt":"hello","duration":5,"resolution":"720p","aspect_ratio":"9:16","generate_audio":true}`)
	out, _, upstream, err := prepareSeedanceCompatibleCreateBody(account, body, "application/json", "seedance-2.0")
	if err != nil {
		t.Fatal(err)
	}
	if upstream != "doubao-seedance-2-0-test" || gjson.GetBytes(out, "model").String() != upstream {
		t.Fatalf("unexpected upstream model: %q body=%s", upstream, out)
	}
	if gjson.GetBytes(out, "ratio").String() != "9:16" || gjson.GetBytes(out, "resolution").String() != "720p" {
		t.Fatalf("unexpected official request: %s", out)
	}
	if gjson.GetBytes(out, "content.0.type").String() != "text" {
		t.Fatalf("missing text content: %s", out)
	}
}

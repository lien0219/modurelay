package service

import "testing"

func TestParseVideoModelRef(t *testing.T) {
	tests := []struct {
		in, channel, canonical string
	}{
		{"48:seedance-2.0", "48", "seedance-2.0"},
		{"test:test-video", "test", "test-video"},
		{"51:grok-imagine-video-1.5", "51", "grok-imagine-video-1.5"},
		{"seedance-2.0", "", "seedance-2.0"},
		{"https://example.com/model", "", "https://example.com/model"},
		{"bad channel:model", "", "bad channel:model"},
	}
	for _, tt := range tests {
		got := ParseVideoModelRef(tt.in)
		if got.ChannelCode != tt.channel || got.CanonicalModel != tt.canonical {
			t.Fatalf("ParseVideoModelRef(%q) = channel %q canonical %q", tt.in, got.ChannelCode, got.CanonicalModel)
		}
	}
}

func TestQualifiedVideoModelFallsBackToCanonicalAccountMapping(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Credentials: map[string]any{
		"model_mapping": map[string]any{"seedance-2.0": "doubao-seedance-2-0-test"},
	}}
	if !account.IsModelSupported("48:seedance-2.0") {
		t.Fatal("qualified model should be admitted by canonical model mapping")
	}
	if got := account.GetMappedModel("48:seedance-2.0"); got != "doubao-seedance-2-0-test" {
		t.Fatalf("mapped model = %q", got)
	}

	supplier := &Account{Platform: PlatformOpenAI, Credentials: map[string]any{
		"model_mapping": map[string]any{"48:seedance-2.0": "48:seedance-2.0"},
	}}
	if got := supplier.GetMappedModel("48:seedance-2.0"); got != "48:seedance-2.0" {
		t.Fatalf("exact supplier mapping must win, got %q", got)
	}
}

func TestDetectModelPlatformQualifiedVideoModels(t *testing.T) {
	cases := []struct{ model, platform string }{
		{"48:seedance-2.0", PlatformOpenAI},
		{"51:grok-imagine-video-1.5", PlatformGrok},
		{"59:minimax-hailuo", PlatformMiniMax},
	}
	for _, tc := range cases {
		platform, ok := DetectModelPlatform(tc.model)
		if !ok || platform != tc.platform {
			t.Fatalf("DetectModelPlatform(%q) = %q, %v", tc.model, platform, ok)
		}
	}
}

func TestAIStarsLabAccountKinds(t *testing.T) {
	openai := &Account{Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://api.video.aistarslab.com/openai"}}
	if !IsAIStarsLabOpenAICompatibleAccount(openai) || IsAIStarsLabOpenAPIAccount(openai) {
		t.Fatal("OpenAI-compatible AIStarsLab account classification failed")
	}
	openapi := &Account{Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://api.video.aistarslab.com/openapi/"}}
	if !IsAIStarsLabOpenAPIAccount(openapi) || IsAIStarsLabOpenAICompatibleAccount(openapi) {
		t.Fatal("OpenAPI AIStarsLab account classification failed")
	}
}

func TestSupportsQualifiedVideoSupplierModelRequiresExplicitConfiguration(t *testing.T) {
	plain := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "x"}}
	if SupportsQualifiedVideoSupplierModel(plain, "48:seedance-2.0") {
		t.Fatal("plain OpenAI account must not receive a provider-qualified model")
	}

	mapped := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"api_key": "x", "base_url": "https://supplier.example/v1",
		"model_mapping": map[string]any{"48:seedance-2.0": "48:seedance-2.0"},
	}}
	if !SupportsQualifiedVideoSupplierModel(mapped, "48:seedance-2.0") {
		t.Fatal("exact qualified model mapping should opt the account in")
	}

	canonicalOnly := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"api_key": "x", "base_url": "https://ark.cn-beijing.volces.com/api/v3",
		"model_mapping": map[string]any{"seedance-2.0": "doubao-seedance-2-0"},
	}}
	if SupportsQualifiedVideoSupplierModel(canonicalOnly, "48:seedance-2.0") {
		t.Fatal("canonical official mapping must not opt an account into supplier routing")
	}
}

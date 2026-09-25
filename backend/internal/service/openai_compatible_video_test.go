package service

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestCompatibleVideoPlatformAndCapability(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax, PlatformOpenCodeGo} {
		if !IsOpenAICompatibleVideoPlatform(platform) {
			t.Fatalf("expected %s to support compatible video", platform)
		}
	}
	for _, platform := range []string{PlatformGrok, PlatformGemini, PlatformAnthropic, PlatformComposite} {
		if IsOpenAICompatibleVideoPlatform(platform) {
			t.Fatalf("did not expect %s to use generic video adapter", platform)
		}
	}

	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}
	if !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideos) {
		t.Fatal("unrestricted API-key account should allow videos")
	}
	// Legacy capability lists predate Videos and must not become a breaking
	// deny-list after deployment.
	account.Credentials[openAIEndpointCapabilitiesCredentialKey] = []any{"chat_completions", "embeddings", "seedance"}
	if !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideos) {
		t.Fatal("legacy capability list should keep videos enabled")
	}
	account.Credentials["openai_video_enabled"] = false
	if account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideos) {
		t.Fatal("explicit video disable must be respected")
	}
	account.Credentials["openai_video_enabled"] = true
	if !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideos) {
		t.Fatal("explicit video enable must be respected")
	}
}

func TestCompatibleVideoEndpointPaths(t *testing.T) {
	got := compatibleVideoEndpointPaths(GrokMediaEndpointVideosGenerations, "")
	if len(got) != 2 || got[0] != "/videos" || got[1] != "/videos/generations" {
		t.Fatalf("unexpected create paths: %#v", got)
	}
	got = compatibleVideoEndpointPaths(GrokMediaEndpointVideoStatus, "task/1")
	if len(got) != 2 || got[0] != "/videos/task%2F1" || got[1] != "/videos/generations/task%2F1" {
		t.Fatalf("unexpected status paths: %#v", got)
	}
}

func TestPrepareCompatibleVideoBodyJSON(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	body := []byte(`{"model":"62:wan-3.0","prompt":"hello","duration":5,"resolution":"480p","audio":true}`)
	rewritten, contentType, model, err := prepareCompatibleVideoBody(account, body, "application/json", "wan-3.0")
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "application/json" || model != "wan-3.0" || gjson.GetBytes(rewritten, "model").String() != "wan-3.0" {
		t.Fatalf("unexpected rewritten body: %s", rewritten)
	}
	if gjson.GetBytes(rewritten, "duration").Int() != 5 || !gjson.GetBytes(rewritten, "audio").Bool() {
		t.Fatalf("provider fields were not preserved: %s", rewritten)
	}
}


func TestPrepareCompatibleVideoBodySeedanceAddsCompatibilityAliases(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	body := []byte(`{"model":"47:seedance-2.0","prompt":"hello","duration":5,"resolution":"720p","aspect_ratio":"9:16","generate_audio":true}`)
	rewritten, contentType, model, err := prepareCompatibleVideoBody(account, body, "application/json", "seedance-2.0")
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "application/json" || model != "seedance-2.0" {
		t.Fatalf("unexpected content type/model: %q %q", contentType, model)
	}
	for path, want := range map[string]string{
		"model":               "seedance-2.0",
		"resolution":          "720p",
		"resolution_name":     "720p",
		"metadata.resolution": "720p",
		"aspect_ratio":        "9:16",
		"ratio":               "9:16",
	} {
		if got := gjson.GetBytes(rewritten, path).String(); got != want {
			t.Fatalf("%s=%q want %q body=%s", path, got, want, rewritten)
		}
	}
	if got := gjson.GetBytes(rewritten, "duration").Int(); got != 5 {
		t.Fatalf("duration=%d body=%s", got, rewritten)
	}
	if got := gjson.GetBytes(rewritten, "seconds").Int(); got != 5 {
		t.Fatalf("seconds=%d body=%s", got, rewritten)
	}
}

func TestPrepareCompatibleVideoBodyNonSeedanceDoesNotAddAliases(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	body := []byte(`{"model":"62:wan-3.0","duration":5,"resolution":"720p","aspect_ratio":"16:9"}`)
	rewritten, _, _, err := prepareCompatibleVideoBody(account, body, "application/json", "wan-3.0")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"resolution_name", "metadata.resolution", "seconds", "ratio"} {
		if gjson.GetBytes(rewritten, path).Exists() {
			t.Fatalf("unexpected alias %s in %s", path, rewritten)
		}
	}
}

func TestPrepareCompatibleVideoBodyMultipart(t *testing.T) {
	var source bytes.Buffer
	writer := multipart.NewWriter(&source)
	_ = writer.WriteField("model", "62:wan-3.0")
	_ = writer.WriteField("prompt", "hello")
	_ = writer.WriteField("resolution_name", "720p")
	contentType := writer.FormDataContentType()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	rewritten, rewrittenType, model, err := prepareCompatibleVideoBody(account, source.Bytes(), contentType, "wan-3.0")
	if err != nil {
		t.Fatal(err)
	}
	if model != "wan-3.0" {
		t.Fatalf("unexpected model: %q", model)
	}
	_, params, err := mime.ParseMediaType(rewrittenType)
	if err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(bytes.NewReader(rewritten), params["boundary"])
	fields := map[string]string{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		var value strings.Builder
		if _, err := io.Copy(&value, part); err != nil {
			t.Fatal(err)
		}
		fields[part.FormName()] = value.String()
		_ = part.Close()
	}
	if fields["model"] != "wan-3.0" || fields["prompt"] != "hello" || fields["resolution_name"] != "720p" {
		t.Fatalf("unexpected multipart fields: %#v", fields)
	}
}

func TestCompatibleVideoForwardResultCompletedStatus(t *testing.T) {
	body := []byte(`{"id":"task-1","status":"completed","model":"wan-3.0","video_url":"https://example.com/out.mp4","duration":5,"resolution":"720p"}`)
	result := compatibleVideoForwardResult(GrokMediaEndpointVideoStatus, "task-1", GrokMediaRequestInfo{}, "", "wan-3.0", body)
	if result.VideoCount != 1 || result.ResponseID != "task-1" || result.VideoDurationSeconds != 5 || result.VideoResolution != "720p" || result.Model != "wan-3.0" {
		t.Fatalf("unexpected completion result: %#v", result)
	}
}


func TestPrepareCompatibleVideoBodyAIStarsLabKeepsQualifiedModelAndNormalizesFields(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "test",
			"base_url": "https://api.video.aistarslab.com/openai",
		},
	}
	body := []byte(`{"model":"48:seedance-2.0","prompt":"hello","duration":5,"resolution":"720p","aspect_ratio":"9:16","audio":true,"generate_audio":true,"watermark":false}`)
	rewritten, contentType, model, err := prepareCompatibleVideoBody(account, body, "application/json", "48:seedance-2.0")
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "application/json" || model != "48:seedance-2.0" {
		t.Fatalf("unexpected content type/model: %q %q", contentType, model)
	}
	if got := gjson.GetBytes(rewritten, "model").String(); got != "48:seedance-2.0" {
		t.Fatalf("model=%q body=%s", got, rewritten)
	}
	if got := gjson.GetBytes(rewritten, "seconds").String(); got != "5" {
		t.Fatalf("seconds=%q body=%s", got, rewritten)
	}
	if got := gjson.GetBytes(rewritten, "size").String(); got != "9:16" {
		t.Fatalf("size=%q body=%s", got, rewritten)
	}
	if got := gjson.GetBytes(rewritten, "metadata.resolution").String(); got != "720p" {
		t.Fatalf("metadata.resolution=%q body=%s", got, rewritten)
	}
	for _, path := range []string{"resolution", "aspect_ratio", "audio", "generate_audio", "watermark"} {
		if gjson.GetBytes(rewritten, path).Exists() {
			t.Fatalf("unsupported supplier alias %s leaked into %s", path, rewritten)
		}
	}
}

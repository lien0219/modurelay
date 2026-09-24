package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestVideoGatewayPlatformSupported(t *testing.T) {
	for _, platform := range []string{
		service.PlatformGrok,
		service.PlatformComposite,
		service.PlatformOpenAI,
		service.PlatformKimi,
		service.PlatformZhipu,
		service.PlatformDeepseek,
		service.PlatformMiniMax,
		service.PlatformOpenCodeGo,
	} {
		if !videoGatewayPlatformSupported(platform) {
			t.Fatalf("expected %s to support /v1/videos", platform)
		}
	}
	for _, platform := range []string{service.PlatformAnthropic, service.PlatformGemini, ""} {
		if videoGatewayPlatformSupported(platform) {
			t.Fatalf("did not expect %s to support /v1/videos", platform)
		}
	}
}

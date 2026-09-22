package service

import "strings"

// publicVerificationChannelName returns a stable, provider-neutral label for
// ordinary user responses. Admin APIs retain the configured channel name,
// while user DTOs never trust a configurable public_name that could expose an
// upstream provider identity.
func publicVerificationChannelName(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "channel_1", "email_channel_1":
		return "Channel 1"
	case "channel_2", "email_channel_2":
		return "Channel 2"
	default:
		return "Channel"
	}
}

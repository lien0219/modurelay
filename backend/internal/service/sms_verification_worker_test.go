package service

import (
	"testing"
	"time"
)

func TestSMSProviderPollDelayKeepsFiveSIMBaseline(t *testing.T) {
	now := time.Now()
	for _, age := range []time.Duration{0, 2 * time.Minute, 20 * time.Minute} {
		if got := smsProviderPollDelay("5sim", now.Add(-age), now); got != smsVerificationPollInterval {
			t.Fatalf("5SIM age %s: got %s, want %s", age, got, smsVerificationPollInterval)
		}
	}
}

func TestSMSProviderPollDelayBacksOffOnlySMSPVA(t *testing.T) {
	now := time.Now()
	cases := []struct {
		age  time.Duration
		want time.Duration
	}{
		{30 * time.Second, 5 * time.Second},
		{2 * time.Minute, 10 * time.Second},
		{10 * time.Minute, 25 * time.Second},
	}
	for _, tc := range cases {
		if got := smsProviderPollDelay("smspva", now.Add(-tc.age), now); got != tc.want {
			t.Fatalf("SMSPVA age %s: got %s, want %s", tc.age, got, tc.want)
		}
	}
}

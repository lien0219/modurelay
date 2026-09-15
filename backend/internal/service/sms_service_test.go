package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSMSProviderCapabilitiesAreSeparated(t *testing.T) {
	for _, code := range []string{"5sim", "smspool", "sms_activate", "onlinesim", "pingme"} {
		provider := providerFor(code, "https://example.invalid", "test-key")
		if provider == nil {
			t.Fatalf("provider %s was not registered", code)
		}
		cap := provider.Capabilities(context.Background())
		if !cap.Temporary {
			t.Fatalf("provider %s must support temporary numbers", code)
		}
		if (code == "5sim" || code == "smspool" || code == "sms_activate") && cap.Rental {
			t.Fatalf("provider %s must not advertise rental", code)
		}
		if (code == "onlinesim" || code == "pingme") && !cap.Rental {
			t.Fatalf("provider %s must advertise its configured rental capability", code)
		}
	}
}

func TestSMSActivateUsesRealActionContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "secret" || r.URL.Query().Get("action") != "getPrices" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"google":{"1":{"cost":0.8,"count":4}}}`))
	}))
	defer server.Close()
	p := providerFor("sms_activate", server.URL, "secret")
	quote, err := p.Quote(context.Background(), SMSQuoteRequest{ServiceCode: "google", CountryCode: "US", ProductType: "temporary"})
	if err != nil {
		t.Fatal(err)
	}
	if quote.Stock != 4 || quote.Cost.String() != "0.8" {
		t.Fatalf("unexpected quote: %#v", quote)
	}
}

func TestSMSPublicDTODoesNotExposeProviderFields(t *testing.T) {
	payload, err := json.Marshal(struct {
		Order   SMSOrder
		Channel SMSPublicChannel
	}{Order: SMSOrder{ID: "public", ChannelCode: "channel_1", Price: 1.2}, Channel: SMSPublicChannel{Code: "channel_1", ProviderCost: 0.8, GradeMultiplier: 1.2, GradeFixedMarkup: 0.1}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	for _, forbidden := range []string{"provider_id", "provider_order_id", "provider_cost", "provider_raw_error", "credential"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("public DTO leaked %q: %s", forbidden, text)
		}
	}
}

func TestSMSUnknownProviderStatesDoNotBecomeSuccess(t *testing.T) {
	if got := normalizeSMSStatus("STATUS_WAIT_CODE"); got != "active" {
		t.Fatalf("waiting status = %q", got)
	}
	if got := normalizeSMSStatus("provider_timeout"); got != "provider_unknown" {
		t.Fatalf("unknown status = %q", got)
	}
}

func TestSMSTimeoutDetection(t *testing.T) {
	if !isSMSProviderTimeout(context.DeadlineExceeded) {
		t.Fatal("deadline should be treated as provider timeout")
	}
	if isSMSProviderTimeout(errors.New("bad request")) {
		t.Fatal("ordinary errors must not be treated as timeout")
	}
	if _, ok := rentalDuration(2, "hour"); !ok {
		t.Fatal("hour duration should be accepted")
	}
	if got, _ := rentalDuration(2, "hour"); got != 2*time.Hour {
		t.Fatalf("unexpected duration: %s", got)
	}
}

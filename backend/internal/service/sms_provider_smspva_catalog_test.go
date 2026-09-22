package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSMSPVACatalogAcceptsResponseLargerThanDefaultLimit(t *testing.T) {
	const row = `{"service":"opt20","serviceDescription":"Telegram","country":"US","price":"0.75"}`
	var body strings.Builder
	body.Grow(int(smsPVADefaultResponseLimit) + len(row) + 1024)
	_, _ = body.WriteString(`{"statusCode":200,"data":[`)
	for body.Len() <= int(smsPVADefaultResponseLimit)+1024 {
		if body.Len() > len(`{"statusCode":200,"data":[`) {
			_ = body.WriteByte(',')
		}
		_, _ = body.WriteString(row)
	}
	_, _ = body.WriteString(`]}`)
	if int64(body.Len()) <= smsPVADefaultResponseLimit || int64(body.Len()) >= smsPVACatalogResponseLimit {
		t.Fatalf("fixture size=%d, want between %d and %d", body.Len(), smsPVADefaultResponseLimit, smsPVACatalogResponseLimit)
	}
	if !json.Valid([]byte(body.String())) {
		t.Fatal("large catalog fixture is not valid JSON")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/activation/servicesprices" || r.URL.Query().Get("voice") != "0" {
			t.Fatalf("unexpected request: %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
		if r.Header.Get("apikey") != "secret" {
			t.Fatal("missing SMSPVA apikey header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body.String()))
	}))
	defer server.Close()

	provider := providerFor("smspva", server.URL, "secret")
	catalog, ok := provider.(SMSCatalogProvider)
	if !ok {
		t.Fatal("SMSPVA provider does not implement SMSCatalogProvider")
	}
	services, countries, err := catalog.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 1 || services[0].Code != "opt20" || services[0].ProviderCode != "opt20" || services[0].Category != "activation" || services[0].ProviderCost != 0.75 || len(countries) != 1 || countries[0].ISO2 != "US" {
		t.Fatalf("unexpected catalog: services=%#v countries=%#v", services, countries)
	}
}

func TestSMSPVARequestJSONRejectsOversizeResponse(t *testing.T) {
	responseBody := []byte(`{"statusCode":200,"data":"` + strings.Repeat("x", 256) + `"}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(responseBody)
	}))
	defer server.Close()

	provider, ok := providerFor("smspva", server.URL, "secret").(*smsPVAProvider)
	if !ok {
		t.Fatal("SMSPVA provider type assertion failed")
	}
	var env smsPVAEnvelope
	if _, err := provider.requestJSONWithLimit(context.Background(), http.MethodGet, "catalog", nil, int64(len(responseBody)), &env); err != nil {
		t.Fatalf("exact response limit should succeed: %v", err)
	}
	_, err := provider.requestJSONWithLimit(context.Background(), http.MethodGet, "catalog", nil, int64(len(responseBody)-1), &env)
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("exceeds %d byte limit", len(responseBody)-1)) {
		t.Fatalf("error=%v, want explicit response limit failure", err)
	}
	if strings.Contains(err.Error(), "unexpected end of JSON input") {
		t.Fatalf("oversize response was misreported as truncated JSON: %v", err)
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), strings.Repeat("x", 32)) {
		t.Fatalf("oversize error leaked request or response data: %v", err)
	}
}

func TestSMSPVARentalCountriesParseCurrentObjectShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/rent.php" || r.URL.Query().Get("apikey") != "secret" {
			t.Fatalf("unexpected rental request: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("method") {
		case "get_country_by_service":
			_, _ = w.Write([]byte(`{"status":1,"data":{"US":{"service_name":"Telegram","price":"1.25","country":"US","phoneAmount":4},"DE":{"service_name":"Telegram","price":"0.80","country":"DE","phoneAmount":0}}}`))
		case "getcountries":
			_, _ = w.Write([]byte(`{"status":1,"data":[{"name":"United States","code":"US"},{"name":"Germany","code":"DE"}]}`))
		default:
			t.Fatalf("unexpected rental method: %s", r.URL.Query().Get("method"))
		}
	}))
	defer server.Close()

	provider, ok := providerFor("smspva", server.URL, "secret").(SMSProductServiceCountryProvider)
	if !ok {
		t.Fatal("SMSPVA provider does not implement SMSProductServiceCountryProvider")
	}
	items, err := provider.CountriesForServiceProduct(context.Background(), "opt20", "rental", 1, "week")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("countries=%#v", items)
	}
	byCode := make(map[string]SMSCountryCatalogItem, len(items))
	for _, item := range items {
		byCode[item.ISO2] = item
	}
	if got := byCode["US"]; got.NameEN != "United States" || got.Stock != 4 || got.ProviderCost != 1.25 || !got.Available {
		t.Fatalf("US=%#v", got)
	}
	if got := byCode["DE"]; got.NameEN != "Germany" || got.Stock != 0 || got.ProviderCost != 0.8 || got.Available {
		t.Fatalf("DE=%#v", got)
	}
}

func TestSMSPVARentalCountriesKeepLegacyArrayCompatibility(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if method := r.URL.Query().Get("method"); method != "get_country_by_service" {
			t.Fatalf("unexpected method: %s", method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":1,"data":[{"name":"United States","code":"US"}]}`)
	}))
	defer server.Close()

	provider, ok := providerFor("smspva", server.URL, "secret").(SMSProductServiceCountryProvider)
	if !ok {
		t.Fatal("SMSPVA provider does not implement SMSProductServiceCountryProvider")
	}
	items, err := provider.CountriesForServiceProduct(context.Background(), "opt20", "rental", 1, "week")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ISO2 != "US" || items[0].NameEN != "United States" || !items[0].Available {
		t.Fatalf("countries=%#v", items)
	}
}

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type sonjjProvider struct {
	code, baseURL, apiKey string
	client                *http.Client
	limiter               *emailProviderLimiter
}

func sonjjCapabilities() EmailProviderCapabilities {
	return EmailProviderCapabilities{
		TemporaryInbox: true, GenerateSingle: true, InboxList: true,
		MessageRead: true, Polling: true, VerificationCode: true,
		VerificationURL: true, HTMLMessage: true, PrivateInbox: true,
		PersistentInbox: false, ProviderRefund: false,
	}
}

func (p *sonjjProvider) Code() string { return p.code }
func (p *sonjjProvider) Capabilities(context.Context) EmailProviderCapabilities {
	return sonjjCapabilities()
}

func (p *sonjjProvider) request(ctx context.Context, method, path string, query url.Values, out any) error {
	if strings.TrimSpace(p.apiKey) == "" {
		return ErrEmailProviderCredentialMissing
	}
	if p.limiter == nil {
		p.limiter = newEmailProviderLimiter(4, 250*time.Millisecond)
	}
	if err := p.limiter.acquire(ctx); err != nil {
		return err
	}
	defer p.limiter.release()

	target := strings.TrimRight(p.baseURL, "/") + "/" + strings.TrimLeft(path, "/")
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Api-Key", p.apiKey)
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("email provider request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		e := &EmailProviderHTTPError{StatusCode: resp.StatusCode}
		if value := strings.TrimSpace(resp.Header.Get("Retry-After")); value != "" {
			e.RetryAfter = parseRetryAfter(value, time.Now())
		}
		return e
	}
	if out != nil && len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return errors.New("email provider returned malformed response")
		}
	}
	return nil
}

func (p *sonjjProvider) Health(ctx context.Context) error {
	// The domains endpoint is authenticated, cheap, and does not allocate a
	// Gmail/Outlook mailbox from the provider pool.
	var out map[string]any
	return p.request(ctx, http.MethodGet, "/v1/temp_email/domains", nil, &out)
}

func parseSonjjAddressType(value string) (provider string, kind string, err error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "gmail", "gmail_real":
		return "gmail", "real", nil
	case "gmail_alias":
		return "gmail", "alias", nil
	case "outlook", "outlook_real":
		return "outlook", "real", nil
	case "outlook_alias":
		return "outlook", "alias", nil
	default:
		return "", "", errors.New("unsupported email address type")
	}
}

func encodeSonjjInboxID(timestamp int64, provider, kind string) string {
	return strconv.FormatInt(timestamp, 10) + "|" + provider + "|" + kind
}

func decodeSonjjInboxID(value, addressType string) (int64, string, string) {
	parts := strings.Split(strings.TrimSpace(value), "|")
	if len(parts) == 3 {
		if ts, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
			return ts, parts[1], parts[2]
		}
	}
	provider, kind, _ := parseSonjjAddressType(addressType)
	return time.Now().Add(-2 * time.Minute).Unix(), provider, kind
}

func (p *sonjjProvider) GenerateInbox(ctx context.Context, req GenerateInboxRequest) (*GeneratedInbox, error) {
	provider, kind, err := parseSonjjAddressType(req.AddressType)
	if err != nil {
		return nil, err
	}
	var out struct {
		Email     string `json:"email"`
		Timestamp int64  `json:"timestamp"`
		Type      string `json:"type"`
	}
	path := "/v1/temp_" + provider + "/random"
	if err := p.request(ctx, http.MethodGet, path, url.Values{"type": {kind}}, &out); err != nil {
		return nil, err
	}
	out.Email = strings.TrimSpace(out.Email)
	if out.Email == "" {
		return nil, errors.New("email provider returned no address")
	}
	if out.Timestamp <= 0 {
		out.Timestamp = time.Now().Unix()
	}
	return &GeneratedInbox{
		ProviderInboxID: encodeSonjjInboxID(out.Timestamp, provider, kind),
		EmailAddress:    out.Email,
		AddressType:     provider + "_" + kind,
		ExpiresAt:       time.Now().Add(15 * time.Minute),
	}, nil
}

func (p *sonjjProvider) ListMessages(ctx context.Context, req ListMessagesRequest) (*MessageListResult, error) {
	timestamp, provider, _ := decodeSonjjInboxID(req.ProviderInboxID, "")
	if provider == "" {
		if strings.Contains(strings.ToLower(req.EmailAddress), "outlook") || strings.Contains(strings.ToLower(req.EmailAddress), "hotmail") {
			provider = "outlook"
		} else {
			provider = "gmail"
		}
	}
	var out struct {
		Messages []struct {
			MID         string `json:"mid"`
			TextDate    string `json:"textDate"`
			TextFrom    string `json:"textFrom"`
			TextSubject string `json:"textSubject"`
			TextTo      string `json:"textTo"`
		} `json:"messages"`
	}
	query := url.Values{
		"email":     {strings.TrimSpace(req.EmailAddress)},
		"timestamp": {strconv.FormatInt(timestamp, 10)},
	}
	if err := p.request(ctx, http.MethodGet, "/v1/temp_"+provider+"/inbox", query, &out); err != nil {
		return nil, err
	}
	result := &MessageListResult{Messages: make([]ProviderEmailSummary, 0, len(out.Messages))}
	for _, item := range out.Messages {
		id := strings.TrimSpace(item.MID)
		if id == "" {
			id = hashString(item.TextFrom + item.TextSubject + item.TextDate + item.TextTo)
		}
		result.Messages = append(result.Messages, ProviderEmailSummary{
			ProviderMessageID: id,
			FromAddress:       strings.TrimSpace(item.TextFrom),
			ToAddress:         strings.TrimSpace(item.TextTo),
			Subject:           strings.TrimSpace(item.TextSubject),
			ReceivedAt:        parseProviderTime(item.TextDate),
		})
	}
	return result, nil
}

func (p *sonjjProvider) GetMessage(ctx context.Context, req GetMessageRequest) (*ProviderEmailMessage, error) {
	_, provider, _ := decodeSonjjInboxID(req.ProviderInboxID, "")
	if provider == "" {
		if strings.Contains(strings.ToLower(req.EmailAddress), "outlook") || strings.Contains(strings.ToLower(req.EmailAddress), "hotmail") {
			provider = "outlook"
		} else {
			provider = "gmail"
		}
	}
	var out struct {
		Body string `json:"body"`
	}
	if err := p.request(ctx, http.MethodGet, "/v1/temp_"+provider+"/message", url.Values{
		"email": {strings.TrimSpace(req.EmailAddress)},
		"mid":   {strings.TrimSpace(req.ProviderMessageID)},
	}, &out); err != nil {
		return nil, err
	}
	body := strings.TrimSpace(out.Body)
	msg := &ProviderEmailMessage{
		ProviderMessageID: strings.TrimSpace(req.ProviderMessageID),
		ToAddress:         strings.TrimSpace(req.EmailAddress),
	}
	if strings.Contains(strings.ToLower(body), "<html") || strings.Contains(strings.ToLower(body), "<body") {
		msg.HTMLBody = body
		msg.TextBody = HTMLToText(body)
	} else {
		msg.TextBody = body
	}
	raw, _ := json.Marshal(out)
	msg.RawPayload = raw
	return msg, nil
}

func (p *sonjjProvider) DeleteMessage(context.Context, DeleteMessageRequest) error {
	// Message removal exists for Gmail but is not required for the receive-only
	// verification lifecycle. Keep the capability disabled until both Gmail
	// and Outlook deletion contracts are symmetric and tested.
	return errors.New("email provider message deletion is disabled")
}

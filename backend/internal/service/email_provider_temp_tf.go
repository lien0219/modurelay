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
	"strings"
	"sync"
	"time"
)

type tempTFProvider struct {
	code, baseURL string
	client        *http.Client
	limiter       *emailProviderLimiter
}

var tempTFMessageCache sync.Map // key=email|message_id -> ProviderEmailMessage

func tempTFCapabilities() EmailProviderCapabilities {
	return EmailProviderCapabilities{
		TemporaryInbox: true, GenerateSingle: true, InboxList: true,
		MessageRead: true, Polling: true, VerificationCode: true,
		VerificationURL: true, HTMLMessage: true, PrivateInbox: false,
		PersistentInbox: false, ProviderRefund: false,
	}
}

func (p *tempTFProvider) Code() string { return p.code }
func (p *tempTFProvider) Capabilities(context.Context) EmailProviderCapabilities {
	return tempTFCapabilities()
}

func (p *tempTFProvider) request(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	if p.limiter == nil {
		p.limiter = newEmailProviderLimiter(2, time.Second)
	}
	if err := p.limiter.acquire(ctx); err != nil {
		return err
	}
	defer p.limiter.release()

	target := strings.TrimRight(p.baseURL, "/") + "/" + strings.TrimLeft(path, "/")
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
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
		if v := strings.TrimSpace(resp.Header.Get("Retry-After")); v != "" {
			e.RetryAfter = parseRetryAfter(v, time.Now())
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

func (p *tempTFProvider) Health(ctx context.Context) error {
	var out map[string]any
	return p.request(ctx, http.MethodGet, "/stats", url.Values{
		"dot": {"1"}, "plus": {"1"}, "providers": {"gmail"},
	}, nil, &out)
}

func (p *tempTFProvider) GenerateInbox(ctx context.Context, req GenerateInboxRequest) (*GeneratedInbox, error) {
	addressType := strings.ToLower(strings.TrimSpace(req.AddressType))
	switch addressType {
	case "gmail", "outlook", "hotmail":
	default:
		return nil, errors.New("unsupported email address type")
	}
	query := url.Values{"plus": {"1"}, "providers": {addressType}}
	if addressType == "gmail" {
		query.Set("dot", "1")
	}
	var out struct {
		Email string `json:"email"`
	}
	if err := p.request(ctx, http.MethodGet, "/account", query, nil, &out); err != nil {
		return nil, err
	}
	out.Email = strings.TrimSpace(out.Email)
	if out.Email == "" {
		return nil, errors.New("email provider returned no address")
	}
	return &GeneratedInbox{
		ProviderInboxID: out.Email,
		EmailAddress:    out.Email,
		AddressType:     addressType,
		ExpiresAt:       time.Now().Add(15 * time.Minute),
	}, nil
}

func tempTFCacheKey(email, id string) string {
	return strings.ToLower(strings.TrimSpace(email)) + "|" + strings.TrimSpace(id)
}

func (p *tempTFProvider) ListMessages(ctx context.Context, req ListMessagesRequest) (*MessageListResult, error) {
	var out struct {
		Data []struct {
			ID              string         `json:"id"`
			Subject         string         `json:"subject"`
			From            string         `json:"from"`
			Date            string         `json:"date"`
			Body            string         `json:"body"`
			BodyContentType string         `json:"bodyContentType"`
			Attachments     []map[string]any `json:"attachments"`
		} `json:"data"`
	}
	if err := p.request(ctx, http.MethodPost, "/check", nil, map[string]string{"email": req.EmailAddress}, &out); err != nil {
		return nil, err
	}
	result := &MessageListResult{Messages: make([]ProviderEmailSummary, 0, len(out.Data))}
	for _, item := range out.Data {
		received := parseProviderTime(item.Date)
		summary := ProviderEmailSummary{
			ProviderMessageID: strings.TrimSpace(item.ID),
			FromAddress:       strings.TrimSpace(item.From),
			ToAddress:         strings.TrimSpace(req.EmailAddress),
			Subject:           strings.TrimSpace(item.Subject),
			ReceivedAt:        received,
		}
		if summary.ProviderMessageID == "" {
			summary.ProviderMessageID = hashString(summary.FromAddress + summary.Subject + item.Date + item.Body)
		}
		raw, _ := json.Marshal(item)
		msg := ProviderEmailMessage{
			ProviderMessageID: summary.ProviderMessageID,
			FromAddress:       summary.FromAddress,
			ToAddress:         summary.ToAddress,
			Subject:           summary.Subject,
			ReceivedAt:        received,
			RawPayload:        raw,
		}
		if strings.EqualFold(item.BodyContentType, "html") {
			msg.HTMLBody = item.Body
			msg.TextBody = HTMLToText(item.Body)
		} else {
			msg.TextBody = item.Body
		}
		tempTFMessageCache.Store(tempTFCacheKey(req.EmailAddress, summary.ProviderMessageID), msg)
		result.Messages = append(result.Messages, summary)
	}
	return result, nil
}

func (p *tempTFProvider) GetMessage(ctx context.Context, req GetMessageRequest) (*ProviderEmailMessage, error) {
	if cached, ok := tempTFMessageCache.Load(tempTFCacheKey(req.EmailAddress, req.ProviderMessageID)); ok {
		if msg, ok := cached.(ProviderEmailMessage); ok {
			copy := msg
			return &copy, nil
		}
	}
	list, err := p.ListMessages(ctx, ListMessagesRequest{ProviderInboxID: req.ProviderInboxID, EmailAddress: req.EmailAddress})
	if err != nil {
		return nil, err
	}
	for _, item := range list.Messages {
		if item.ProviderMessageID == req.ProviderMessageID {
			if cached, ok := tempTFMessageCache.Load(tempTFCacheKey(req.EmailAddress, req.ProviderMessageID)); ok {
				if msg, ok := cached.(ProviderEmailMessage); ok {
					copy := msg
					return &copy, nil
				}
			}
		}
	}
	return nil, errors.New("email message not found")
}

func (p *tempTFProvider) DeleteMessage(context.Context, DeleteMessageRequest) error {
	return errors.New("email provider does not support message deletion")
}

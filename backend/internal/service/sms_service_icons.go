package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
)

const (
	smsProviderIconMaxBytes = 512 << 10
	smsProviderIconTTL      = 12 * time.Hour
)

type smsProviderIconCacheEntry struct {
	ContentType string
	Data        []byte
	ExpiresAt   time.Time
}

var smsProviderIconCache sync.Map

func normalizeSMSPVAIconPath(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", errors.New("provider icon path is empty")
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		u, err := url.Parse(value)
		if err != nil {
			return "", errors.New("invalid provider icon URL")
		}
		if !strings.EqualFold(u.Scheme, "https") || !strings.EqualFold(u.Hostname(), "smspva.com") {
			return "", errors.New("provider icon host is not allowed")
		}
		value = strings.TrimPrefix(u.EscapedPath(), "/")
	}
	value = strings.TrimPrefix(value, "/")
	cleaned := path.Clean("/" + value)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if !strings.HasPrefix(cleaned, "images/ico/") || strings.Contains(cleaned, "..") {
		return "", errors.New("provider icon path is not allowed")
	}
	return cleaned, nil
}

func (s *SMSService) ServiceIcon(ctx context.Context, providerCode, serviceCode string) ([]byte, string, error) {
	providerCode = strings.ToLower(strings.TrimSpace(providerCode))
	serviceCode = strings.ToLower(strings.TrimSpace(serviceCode))
	if providerCode != "smspva" || serviceCode == "" {
		return nil, "", errors.New("service icon is unavailable")
	}

	var iconPath string
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(c.raw_metadata->>'icon_path','')
		FROM sms_provider_catalog_services c
		JOIN sms_providers p ON p.id=c.provider_id
		WHERE p.code=$1 AND lower(c.provider_service_code)=lower($2) AND c.enabled
		LIMIT 1`, providerCode, serviceCode).Scan(&iconPath); err != nil {
		return nil, "", err
	}
	normalized, err := normalizeSMSPVAIconPath(iconPath)
	if err != nil {
		return nil, "", err
	}
	cacheKey := providerCode + ":" + normalized
	if cached, ok := smsProviderIconCache.Load(cacheKey); ok {
		entry := cached.(smsProviderIconCacheEntry)
		if time.Now().Before(entry.ExpiresAt) {
			return append([]byte(nil), entry.Data...), entry.ContentType, nil
		}
		smsProviderIconCache.Delete(cacheKey)
	}

	target := "https://smspva.com/" + normalized
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/png,image/jpeg,image/gif,image/x-icon,image/vnd.microsoft.icon;q=0.9,*/*;q=0.1")

	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("provider service icon request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("provider service icon returned HTTP %d", resp.StatusCode)
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	switch contentType {
	case "image/png", "image/jpeg", "image/webp", "image/gif", "image/x-icon", "image/vnd.microsoft.icon":
	default:
		return nil, "", errors.New("provider service icon returned unsupported content type")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, smsProviderIconMaxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 || len(data) > smsProviderIconMaxBytes {
		return nil, "", errors.New("provider service icon exceeds allowed size")
	}

	entry := smsProviderIconCacheEntry{
		ContentType: contentType,
		Data:        append([]byte(nil), data...),
		ExpiresAt:   time.Now().Add(smsProviderIconTTL),
	}
	smsProviderIconCache.Store(cacheKey, entry)
	return data, contentType, nil
}

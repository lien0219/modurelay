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

func detectSMSServiceIconType(data []byte) string {
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return "image/png"
	}
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "image/jpeg"
	}
	if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		return "image/gif"
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	if len(data) >= 4 && data[0] == 0x00 && data[1] == 0x00 && data[2] == 0x01 && data[3] == 0x00 {
		return "image/x-icon"
	}
	return ""
}

func (s *SMSService) ServiceIcon(ctx context.Context, serviceCode string) ([]byte, string, error) {
	serviceCode = strings.ToLower(strings.TrimSpace(serviceCode))
	if serviceCode == "" {
		return nil, "", errors.New("service icon is unavailable")
	}

	var providerCode, iconPath string
	if err := s.db.QueryRowContext(ctx, `SELECT p.code,COALESCE(c.raw_metadata->>'icon_path','')
		FROM sms_provider_catalog_services c
		JOIN sms_providers p ON p.id=c.provider_id
		WHERE p.enabled AND c.enabled
		  AND lower(c.provider_service_code)=lower($1)
		  AND COALESCE(c.raw_metadata->>'icon_path','')<>''
		  AND p.code='smspva'
		ORDER BY c.observed_at DESC
		LIMIT 1`, serviceCode).Scan(&providerCode, &iconPath); err != nil {
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
	data, err := io.ReadAll(io.LimitReader(resp.Body, smsProviderIconMaxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 || len(data) > smsProviderIconMaxBytes {
		return nil, "", errors.New("provider service icon exceeds allowed size")
	}
	detectedType := detectSMSServiceIconType(data)
	if detectedType == "" {
		return nil, "", errors.New("provider service icon content is not a supported image")
	}
	switch contentType {
	case "image/png", "image/jpeg", "image/webp", "image/gif", "image/x-icon", "image/vnd.microsoft.icon":
		// Keep the provider MIME only when it agrees with a recognized image
		// family; legacy ICO servers commonly report either icon MIME.
		if (contentType == "image/x-icon" || contentType == "image/vnd.microsoft.icon") && detectedType == "image/x-icon" {
			contentType = "image/x-icon"
		} else if contentType != detectedType {
			contentType = detectedType
		}
	case "application/octet-stream", "":
		contentType = detectedType
	default:
		return nil, "", errors.New("provider service icon returned unsupported content type")
	}

	entry := smsProviderIconCacheEntry{
		ContentType: contentType,
		Data:        append([]byte(nil), data...),
		ExpiresAt:   time.Now().Add(smsProviderIconTTL),
	}
	smsProviderIconCache.Store(cacheKey, entry)
	return data, contentType, nil
}

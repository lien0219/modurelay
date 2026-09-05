package urlvalidator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	resolvedIPValidationTimeout = 5 * time.Second
	pinnedDialFallbackDelay     = 250 * time.Millisecond
)

var blockedIPv4Networks = []*net.IPNet{
	mustParseCIDR("0.0.0.0/8"),
	mustParseCIDR("100.64.0.0/10"),
}

type ipResolver interface {
	LookupIP(context.Context, string, string) ([]net.IP, error)
}

type resolvedHostContextKey struct{}

type resolvedHost struct {
	host string
	ips  []net.IP
}

type ValidationOptions struct {
	AllowedHosts     []string
	RequireAllowlist bool
	AllowPrivate     bool
}

var ErrSameHTTPOrigin = errors.New("upstream URL must not target this deployment")

// RejectSameHTTPOrigin prevents a configured upstream from recursively
// forwarding requests back into this deployment. Paths and explicit default
// ports do not change an HTTP origin.
func RejectSameHTTPOrigin(candidateRaw, deploymentRaw string) error {
	if strings.TrimSpace(deploymentRaw) == "" {
		return nil
	}

	candidate, err := parseHTTPOrigin(candidateRaw)
	if err != nil {
		return errors.New("upstream URL origin is invalid")
	}
	deployment, err := parseHTTPOrigin(deploymentRaw)
	if err != nil {
		return errors.New("deployment frontend URL origin is invalid")
	}
	if candidate == deployment {
		return ErrSameHTTPOrigin
	}
	return nil
}

type httpOrigin struct {
	scheme string
	host   string
	port   string
}

func parseHTTPOrigin(raw string) (httpOrigin, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return httpOrigin{}, errors.New("invalid HTTP origin")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return httpOrigin{}, errors.New("invalid HTTP origin scheme")
	}
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(parsed.Hostname())), ".")
	if host == "" {
		return httpOrigin{}, errors.New("invalid HTTP origin host")
	}
	port := parsed.Port()
	if port == "" {
		if scheme == "http" {
			port = "80"
		} else {
			port = "443"
		}
	}
	return httpOrigin{scheme: scheme, host: host, port: port}, nil
}

// ValidateHTTPURL validates an outbound HTTP/HTTPS URL.
//
// It provides a single validation entry point that supports:
// - scheme 校验（https 或可选允许 http）
// - 可选 allowlist（支持 *.example.com 通配）
// - allow_private_hosts 策略（阻断 localhost/私网字面量 IP）
//
// 注意：DNS Rebinding 防护（解析后 IP 校验）应在实际发起请求时执行，避免 TOCTOU。
func ValidateHTTPURL(raw string, allowInsecureHTTP bool, opts ValidationOptions) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("url is required")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid url: %s", trimmed)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "https" && (!allowInsecureHTTP || scheme != "http") {
		return "", fmt.Errorf("invalid url scheme: %s", parsed.Scheme)
	}

	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return "", errors.New("invalid host")
	}
	if !opts.AllowPrivate && isBlockedHost(host) {
		return "", fmt.Errorf("host is not allowed: %s", host)
	}

	if port := parsed.Port(); port != "" {
		num, err := strconv.Atoi(port)
		if err != nil || num <= 0 || num > 65535 {
			return "", fmt.Errorf("invalid port: %s", port)
		}
	}

	allowlist := normalizeAllowlist(opts.AllowedHosts)
	if opts.RequireAllowlist && len(allowlist) == 0 {
		return "", errors.New("allowlist is not configured")
	}
	if len(allowlist) > 0 && !isAllowedHost(host, allowlist) {
		return "", fmt.Errorf("host is not allowed: %s", host)
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func ValidateURLFormat(raw string, allowInsecureHTTP bool) (string, error) {
	// 最小格式校验：仅保证 URL 可解析且 scheme 合规，不做白名单/私网/SSRF 校验
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("url is required")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid url: %s", trimmed)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "https" && (!allowInsecureHTTP || scheme != "http") {
		return "", fmt.Errorf("invalid url scheme: %s", parsed.Scheme)
	}

	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return "", errors.New("invalid host")
	}

	if port := parsed.Port(); port != "" {
		num, err := strconv.Atoi(port)
		if err != nil || num <= 0 || num > 65535 {
			return "", fmt.Errorf("invalid port: %s", port)
		}
	}

	return strings.TrimRight(trimmed, "/"), nil
}

func ValidateHTTPSURL(raw string, opts ValidationOptions) (string, error) {
	return ValidateHTTPURL(raw, false, opts)
}

// ValidateResolvedIP is a policy pre-check. Call ResolveAndPinHost and use the
// resulting context during socket dial when the caller must also prevent a DNS
// validation-to-dial race.
func ValidateResolvedIP(host string) error {
	ctx, cancel := context.WithTimeout(context.Background(), resolvedIPValidationTimeout)
	defer cancel()
	_, err := resolveAllowedIPs(ctx, host, net.DefaultResolver)
	return err
}

// ResolveAndPinHost resolves every address once, rejects the full result when
// any address violates the public-network policy, and stores the validated
// addresses in the returned context for the transport dialer.
func ResolveAndPinHost(ctx context.Context, host string) (context.Context, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	lookupCtx, cancel := context.WithTimeout(ctx, resolvedIPValidationTimeout)
	defer cancel()

	ips, err := resolveAllowedIPs(lookupCtx, host, net.DefaultResolver)
	if err != nil {
		return nil, err
	}
	return WithValidatedResolvedHostIPs(ctx, host, ips)
}

// WithValidatedResolvedHostIPs stores an already resolved host mapping after
// applying the same public-network policy used by ResolveAndPinHost. Callers
// with a controlled resolver can use it without weakening the dial-time guard.
func WithValidatedResolvedHostIPs(ctx context.Context, host string, ips []net.IP) (context.Context, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	host = normalizeHost(host)
	if host == "" {
		return nil, errors.New("host is empty")
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("resolved addresses are empty for %s", host)
	}
	pinned := make([]net.IP, 0, len(ips))
	seen := make(map[string]struct{}, len(ips))
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return nil, fmt.Errorf("resolved ip %s is not allowed", ip.String())
		}
		key := ip.String()
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		pinned = append(pinned, append(net.IP(nil), ip...))
	}
	return context.WithValue(ctx, resolvedHostContextKey{}, resolvedHost{
		host: host,
		ips:  pinned,
	}), nil
}

func resolveAllowedIPs(ctx context.Context, host string, resolver ipResolver) ([]net.IP, error) {
	host = normalizeHost(host)
	if host == "" {
		return nil, errors.New("host is empty")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return nil, fmt.Errorf("resolved ip %s is not allowed", ip.String())
		}
		return []net.IP{ip}, nil
	}
	if resolver == nil {
		return nil, errors.New("dns resolver is unavailable")
	}

	ips, err := resolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("dns resolution failed: %w", err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("dns resolution returned no addresses for %s", host)
	}

	unique := make([]net.IP, 0, len(ips))
	seen := make(map[string]struct{}, len(ips))
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return nil, fmt.Errorf("resolved ip %s is not allowed", ip.String())
		}
		key := ip.String()
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, ip)
	}
	return unique, nil
}

// PinnedDialAddresses returns the validated IP:port targets for address. A
// false result means the context does not contain a pin for this hostname.
func PinnedDialAddresses(ctx context.Context, address string) ([]string, bool, error) {
	if ctx == nil {
		return nil, false, nil
	}
	pinned, ok := ctx.Value(resolvedHostContextKey{}).(resolvedHost)
	if !ok || pinned.host == "" || len(pinned.ips) == 0 {
		return nil, false, nil
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, false, err
	}
	if normalizeHost(host) != pinned.host {
		return nil, false, nil
	}
	addresses := make([]string, 0, len(pinned.ips))
	for _, ip := range pinned.ips {
		addresses = append(addresses, net.JoinHostPort(ip.String(), port))
	}
	return addresses, true, nil
}

// DialContextWithPinnedIPs tries only the already validated addresses when a
// matching host pin is present. It never falls back to another DNS lookup.
func DialContextWithPinnedIPs(
	ctx context.Context,
	network string,
	address string,
	dial func(context.Context, string, string) (net.Conn, error),
) (net.Conn, error) {
	if dial == nil {
		return nil, errors.New("dial function is nil")
	}
	addresses, pinned, err := PinnedDialAddresses(ctx, address)
	if err != nil {
		return nil, err
	}
	if !pinned {
		return dial(ctx, network, address)
	}
	return dialPinnedAddresses(ctx, network, address, addresses, dial)
}

type pinnedDialResult struct {
	conn    net.Conn
	err     error
	address string
}

// dialPinnedAddresses preserves the validated address set while avoiding a
// full request-timeout penalty when the resolver's first address is
// unreachable. Candidates are staggered like Happy Eyeballs; they are never
// resolved again and losing connections are closed after cancellation.
func dialPinnedAddresses(
	ctx context.Context,
	network string,
	originalAddress string,
	addresses []string,
	dial func(context.Context, string, string) (net.Conn, error),
) (net.Conn, error) {
	if len(addresses) == 0 {
		return nil, fmt.Errorf("validated addresses are empty for %s", originalAddress)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	dialCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan pinnedDialResult)
	launch := func(candidate string) {
		go func() {
			conn, err := dial(dialCtx, network, candidate)
			result := pinnedDialResult{conn: conn, err: err, address: candidate}
			select {
			case results <- result:
			case <-dialCtx.Done():
				if conn != nil {
					_ = conn.Close()
				}
			}
		}()
	}

	launched := 1
	completed := 0
	launch(addresses[0])
	timer := time.NewTimer(pinnedDialFallbackDelay)
	defer timer.Stop()
	var errs []error

	for {
		select {
		case result := <-results:
			completed++
			if result.err == nil {
				cancel()
				return result.conn, nil
			}
			errs = append(errs, fmt.Errorf("%s: %w", result.address, result.err))
			if completed == len(addresses) {
				return nil, fmt.Errorf("dial validated addresses for %s: %w", originalAddress, errors.Join(errs...))
			}
			// Do not wait for the fallback timer when every launched attempt has
			// already failed and another validated address is available.
			if completed == launched && launched < len(addresses) {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				launch(addresses[launched])
				launched++
				if launched < len(addresses) {
					timer.Reset(pinnedDialFallbackDelay)
				}
			}
		case <-timer.C:
			if launched < len(addresses) {
				launch(addresses[launched])
				launched++
				if launched < len(addresses) {
					timer.Reset(pinnedDialFallbackDelay)
				}
			}
		case <-ctx.Done():
			cancel()
			return nil, fmt.Errorf("dial validated addresses for %s: %w", originalAddress, ctx.Err())
		}
	}
}

func normalizeAllowlist(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	for _, v := range values {
		entry := strings.ToLower(strings.TrimSpace(v))
		if entry == "" {
			continue
		}
		if host, _, err := net.SplitHostPort(entry); err == nil {
			entry = host
		}
		normalized = append(normalized, entry)
	}
	return normalized
}

func isAllowedHost(host string, allowlist []string) bool {
	for _, entry := range allowlist {
		if entry == "" {
			continue
		}
		if strings.HasPrefix(entry, "*.") {
			suffix := strings.TrimPrefix(entry, "*.")
			if host == suffix || strings.HasSuffix(host, "."+suffix) {
				return true
			}
			continue
		}
		if host == entry {
			return true
		}
	}
	return false
}

// IsBlockedHost 报告 host 是否为 localhost、*.localhost，或回环、私网、链路本地、未指定地址的字面量 IP。
// 只判断字面量，域名的解析结果由 ValidateResolvedIP 校验。
func IsBlockedHost(host string) bool {
	return isBlockedHost(strings.ToLower(strings.TrimSpace(host)))
}

func isBlockedHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return true
		}
	}
	return false
}

func normalizeHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil || (ip.To4() == nil && ip.To16() == nil) {
		return true
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		for _, network := range blockedIPv4Networks {
			if network.Contains(ipv4) {
				return true
			}
		}
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified()
}

func mustParseCIDR(raw string) *net.IPNet {
	_, network, err := net.ParseCIDR(raw)
	if err != nil {
		panic(err)
	}
	return network
}

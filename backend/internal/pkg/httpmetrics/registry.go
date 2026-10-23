// Package httpmetrics records per-instance HTTP counters and histograms for
// Prometheus text exposition. These series describe one process. They are not
// the cluster gauges on /metrics, and scrapers must sum counters across pods.
package httpmetrics

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	SourceApp     = "app"
	SourceIngress = "ingress"
)

// Duration and TTFT buckets cover short API calls and long model streams.
// The +Inf bucket is added during exposition.
var histogramBounds = []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300}

var defaultRegistry = newRegistry()

type seriesKey struct {
	method   string
	status   string
	source   string
	route    string
	protocol string
}

type histogramKey struct {
	method   string
	source   string
	route    string
	protocol string
}

type counterSeries struct {
	value uint64
}

type histogramSeries struct {
	buckets []uint64
	sum     float64
	count   uint64
}

type registry struct {
	mu        sync.Mutex
	requests  map[seriesKey]*counterSeries
	started   map[seriesKey]*counterSeries
	durations map[histogramKey]*histogramSeries
	ttft      map[histogramKey]*histogramSeries
}

func newRegistry() *registry {
	return &registry{
		requests:  map[seriesKey]*counterSeries{},
		started:   map[seriesKey]*counterSeries{},
		durations: map[histogramKey]*histogramSeries{},
		ttft:      map[histogramKey]*histogramSeries{},
	}
}

// Reset clears the process registry. Tests use it to isolate observations.
func Reset() {
	defaultRegistry.mu.Lock()
	defer defaultRegistry.mu.Unlock()
	defaultRegistry.requests = map[seriesKey]*counterSeries{}
	defaultRegistry.started = map[seriesKey]*counterSeries{}
	defaultRegistry.durations = map[histogramKey]*histogramSeries{}
	defaultRegistry.ttft = map[histogramKey]*histogramSeries{}
}

// Observe records one finished HTTP request. Probes are not passed here.
// hasTTFT is false when the response produced no first output byte.
func Observe(method string, statusCode int, source string, duration time.Duration, ttft time.Duration, hasTTFT bool) {
	ObserveDimensions(method, statusCode, source, "other", "http", duration, ttft, hasTTFT)
}

// Render returns Prometheus text for the instance HTTP series.
func Render() string {
	return defaultRegistry.render()
}

func (r *registry) observe(method string, statusCode int, source, route, protocol string, duration time.Duration, ttft time.Duration, hasTTFT bool) {
	if r == nil {
		return
	}
	method = normalizeMethod(method)
	source = normalizeSource(source)
	status := normalizeStatus(statusCode)
	r.mu.Lock()
	defer r.mu.Unlock()
	key := seriesKey{method: method, status: status, source: source, route: route, protocol: protocol}
	counter := r.requests[key]
	if counter == nil {
		counter = &counterSeries{}
		r.requests[key] = counter
	}
	counter.value++
	histKey := histogramKey{method: method, source: source, route: route, protocol: protocol}
	observeHistogram(r.durations, histKey, seconds(duration))
	if hasTTFT && source == SourceApp && statusCode >= 200 && statusCode < 400 && ttft >= 0 {
		observeHistogram(r.ttft, histKey, seconds(ttft))
	}
}

func observeHistogram(store map[histogramKey]*histogramSeries, key histogramKey, value float64) {
	if value < 0 {
		value = 0
	}
	series := store[key]
	if series == nil {
		series = &histogramSeries{buckets: make([]uint64, len(histogramBounds)+1)}
		store[key] = series
	}
	series.count++
	series.sum += value
	for i, bound := range histogramBounds {
		if value <= bound {
			series.buckets[i]++
		}
	}
	series.buckets[len(histogramBounds)]++
}

func (r *registry) render() string {
	if r == nil {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var out strings.Builder
	writeCounter(&out, "modurelay_http_requests_started_total", "HTTP arrivals before handler completion, excluding probes and scrapes.", r.started)
	writeCounter(&out, "modurelay_http_requests_total", "HTTP requests handled by this instance, excluding probes and metrics scrapes.", r.requests)
	writeHistogram(&out, "modurelay_http_request_duration_seconds", "HTTP request duration in seconds for this instance.", r.durations)
	writeHistogram(&out, "modurelay_http_ttft_seconds", "Time to the first output byte in seconds. Requests without a first output are omitted.", r.ttft)
	return out.String()
}

func writeCounter(out *strings.Builder, name, help string, series map[seriesKey]*counterSeries) {
	out.WriteString("# HELP " + name + " " + help + "\n")
	out.WriteString("# TYPE " + name + " counter\n")
	keys := make([]seriesKey, 0, len(series))
	for key := range series {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].source != keys[j].source {
			return keys[i].source < keys[j].source
		}
		if keys[i].method != keys[j].method {
			return keys[i].method < keys[j].method
		}
		if keys[i].route != keys[j].route {
			return keys[i].route < keys[j].route
		}
		if keys[i].protocol != keys[j].protocol {
			return keys[i].protocol < keys[j].protocol
		}
		return keys[i].status < keys[j].status
	})
	for _, key := range keys {
		item := series[key]
		if item == nil {
			continue
		}
		out.WriteString(name)
		out.WriteString(`{method="`)
		out.WriteString(escapeLabel(key.method))
		out.WriteString(`",source="`)
		out.WriteString(escapeLabel(key.source))
		out.WriteString(`",route="`)
		out.WriteString(escapeLabel(key.route))
		if key.protocol != "" {
			out.WriteString(`",protocol="`)
			out.WriteString(escapeLabel(key.protocol))
		}
		if key.status != "" {
			out.WriteString(`",status_code="`)
			out.WriteString(escapeLabel(key.status))
		}
		out.WriteString(`"} `)
		out.WriteString(strconv.FormatUint(item.value, 10))
		out.WriteByte('\n')
	}
}

func writeHistogram(out *strings.Builder, name, help string, series map[histogramKey]*histogramSeries) {
	out.WriteString("# HELP " + name + " " + help + "\n")
	out.WriteString("# TYPE " + name + " histogram\n")
	keys := make([]histogramKey, 0, len(series))
	for key := range series {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].source != keys[j].source {
			return keys[i].source < keys[j].source
		}
		if keys[i].method != keys[j].method {
			return keys[i].method < keys[j].method
		}
		if keys[i].route != keys[j].route {
			return keys[i].route < keys[j].route
		}
		return keys[i].protocol < keys[j].protocol
	})
	for _, key := range keys {
		item := series[key]
		if item == nil {
			continue
		}
		labels := `method="` + escapeLabel(key.method) + `",source="` + escapeLabel(key.source) + `",route="` + escapeLabel(key.route) + `",protocol="` + escapeLabel(key.protocol) + `"`
		for i, bound := range histogramBounds {
			out.WriteString(name + "_bucket{" + labels + `,le="` + formatBound(bound) + `"} `)
			out.WriteString(strconv.FormatUint(item.buckets[i], 10))
			out.WriteByte('\n')
		}
		out.WriteString(name + "_bucket{" + labels + `,le="+Inf"} `)
		out.WriteString(strconv.FormatUint(item.buckets[len(histogramBounds)], 10))
		out.WriteByte('\n')
		out.WriteString(name + "_sum{" + labels + "} " + strconv.FormatFloat(item.sum, 'f', -1, 64) + "\n")
		out.WriteString(name + "_count{" + labels + "} " + strconv.FormatUint(item.count, 10) + "\n")
	}
}

func normalizeMethod(method string) string {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return strings.ToUpper(strings.TrimSpace(method))
	default:
		return "OTHER"
	}
}

func normalizeSource(source string) string {
	if source == SourceIngress {
		return SourceIngress
	}
	return SourceApp
}

func normalizeStatus(code int) string {
	if code < 100 || code > 599 {
		return "0"
	}
	return strconv.Itoa(code)
}

func seconds(d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return d.Seconds()
}

func formatBound(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func escapeLabel(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}

// ExcludedPath reports paths that are probes or metric scrapes.
func ExcludedPath(path string) bool {
	switch path {
	case "/health", "/ready", "/metrics", "/metrics/instance", "/setup/status":
		return true
	default:
		return false
	}
}

// ObserveDimensions bounds every caller-controlled dimension to fixed enums.
func ObserveDimensions(method string, statusCode int, source, route, protocol string, duration, ttft time.Duration, hasTTFT bool) {
	switch route {
	case "responses", "messages", "chat_completions", "gemini", "embeddings", "images", "audio", "models":
	default:
		route = "other"
	}
	switch protocol {
	case "sse", "websocket":
	default:
		protocol = "http"
	}
	defaultRegistry.observe(method, statusCode, source, route, protocol, duration, ttft, hasTTFT)
}

// Route classifies a route without ever exposing request paths or IDs as labels.
func Route(path string) string {
	// Match the registered route template, never label with a model name or ID.
	if strings.HasSuffix(path, "/models/*modelAction") {
		return "gemini"
	}
	for _, item := range []struct{ suffix, name string }{
		{"/responses", "responses"}, {"/messages", "messages"}, {"/chat/completions", "chat_completions"},
		{"/embeddings", "embeddings"}, {"/images/generations", "images"}, {"/images/edits", "images"},
		{"/audio/transcriptions", "audio"}, {"/audio/translations", "audio"}, {"/audio/speech", "audio"}, {"/models", "models"},
	} {
		if strings.HasSuffix(path, item.suffix) {
			return item.name
		}
	}
	return "other"
}

// ObserveStarted counts arrivals independently from finished responses. Ingress
// rejection classification is known only when the request completes.
func ObserveStarted(method, path string) {
	r := defaultRegistry
	r.mu.Lock()
	defer r.mu.Unlock()
	key := seriesKey{method: normalizeMethod(method), source: SourceApp, route: Route(path)}
	if r.started[key] == nil {
		r.started[key] = &counterSeries{}
	}
	r.started[key].value++
}

package admin

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

const (
	detectionProtocolAuto      = "auto"
	detectionProtocolOpenAI    = "openai"
	detectionProtocolAnthropic = "anthropic"
	detectionProtocolGemini    = "gemini"

	detectionModeStandard = "standard"
	detectionModeDeep     = "deep"
)

type DetectionCenterHandler struct{}

func NewDetectionCenterHandler() *DetectionCenterHandler { return &DetectionCenterHandler{} }

type detectionDiscoveryRequest struct {
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Protocol string `json:"protocol"`
}

type detectionRunRequest struct {
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Protocol string `json:"protocol"`
	Model    string `json:"model"`
	Mode     string `json:"mode"`
}

type detectionAttempt struct {
	Protocol   string `json:"protocol"`
	Status     string `json:"status"`
	HTTPStatus int    `json:"http_status,omitempty"`
	LatencyMS  int64  `json:"latency_ms,omitempty"`
	Message    string `json:"message"`
}

type detectedModel struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Provider     string         `json:"provider,omitempty"`
	Protocols    []string       `json:"protocols"`
	Capabilities map[string]any `json:"capabilities,omitempty"`
}

type detectionDiscoveryResponse struct {
	SuggestedProtocol string             `json:"suggested_protocol"`
	Protocols         []string           `json:"protocols"`
	Models            []detectedModel    `json:"models"`
	Attempts          []detectionAttempt `json:"attempts"`
}

type detectionEvidence struct {
	Kind            string `json:"kind"`
	Label           string `json:"label"`
	Expected        string `json:"expected,omitempty"`
	Actual          string `json:"actual,omitempty"`
	HTTPStatus      int    `json:"http_status,omitempty"`
	DurationMS      int64  `json:"duration_ms,omitempty"`
	RequestSummary  string `json:"request_summary,omitempty"`
	ResponseExcerpt string `json:"response_excerpt,omitempty"`
}

type detectionProbeResult struct {
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	Category        string              `json:"category"`
	Status          string              `json:"status"`
	Confidence      float64             `json:"confidence"`
	Summary         string              `json:"summary"`
	FailureReason   string              `json:"failure_reason,omitempty"`
	ReasonCode      string              `json:"reason_code,omitempty"`
	PossibleCauses  []string            `json:"possible_causes,omitempty"`
	Recommendations []string            `json:"recommendations,omitempty"`
	Evidence        []detectionEvidence `json:"evidence,omitempty"`
}

type detectionSummary struct {
	Total         int `json:"total"`
	Success       int `json:"success"`
	Failed        int `json:"failed"`
	Partial       int `json:"partial"`
	Inconclusive  int `json:"inconclusive"`
	NotApplicable int `json:"not_applicable"`
	Unavailable   int `json:"unavailable"`
}

type detectionReport struct {
	ReportID    string                 `json:"report_id"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt time.Time              `json:"completed_at"`
	BaseURL     string                 `json:"base_url"`
	Protocol    string                 `json:"protocol"`
	Model       string                 `json:"model"`
	Mode        string                 `json:"mode"`
	Summary     detectionSummary       `json:"summary"`
	Probes      []detectionProbeResult `json:"probes"`
	Notes       []string               `json:"notes,omitempty"`
}

func (h *DetectionCenterHandler) Discover(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req detectionDiscoveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	req.Protocol = normalizeDetectionProtocol(req.Protocol)
	target, err := newDetectionTarget(c.Request.Context(), req.BaseURL, req.APIKey)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result := discoverDetectionModels(c.Request.Context(), target, req.Protocol)
	response.Success(c, result)
}

func (h *DetectionCenterHandler) Run(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req detectionRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	req.Protocol = normalizeDetectionProtocol(req.Protocol)
	req.Mode = normalizeDetectionMode(req.Mode)
	req.Model = strings.TrimSpace(req.Model)
	if req.Model == "" {
		response.BadRequest(c, "model is required")
		return
	}
	target, err := newDetectionTarget(c.Request.Context(), req.BaseURL, req.APIKey)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	started := time.Now().UTC()
	protocol := req.Protocol
	protocolProbe := detectionProbeResult{}
	if protocol == detectionProtocolAuto {
		protocol, protocolProbe = detectProtocolForModel(c.Request.Context(), target, req.Model)
		if protocol == "" {
			report := detectionReport{
				ReportID:    detectionID("report"),
				StartedAt:   started,
				CompletedAt: time.Now().UTC(),
				BaseURL:     target.baseURL,
				Protocol:    detectionProtocolAuto,
				Model:       req.Model,
				Mode:        req.Mode,
				Probes:      []detectionProbeResult{protocolProbe},
				Notes:       defaultDetectionNotes(),
			}
			report.Summary = summarizeDetectionProbes(report.Probes)
			response.Success(c, report)
			return
		}
	}

	probes := make([]detectionProbeResult, 0, 16)
	if req.Protocol == detectionProtocolAuto {
		probes = append(probes, protocolProbe)
	}
	probes = append(probes, runProtocolDetection(c.Request.Context(), target, protocol, req.Model, req.Mode)...)
	report := detectionReport{
		ReportID:    detectionID("report"),
		StartedAt:   started,
		CompletedAt: time.Now().UTC(),
		BaseURL:     target.baseURL,
		Protocol:    protocol,
		Model:       req.Model,
		Mode:        req.Mode,
		Probes:      probes,
		Notes:       defaultDetectionNotes(),
	}
	report.Summary = summarizeDetectionProbes(probes)
	response.Success(c, report)
}

func normalizeDetectionProtocol(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case detectionProtocolOpenAI, detectionProtocolAnthropic, detectionProtocolGemini:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return detectionProtocolAuto
	}
}

func normalizeDetectionMode(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), detectionModeStandard) {
		return detectionModeStandard
	}
	return detectionModeDeep
}

func summarizeDetectionProbes(probes []detectionProbeResult) detectionSummary {
	out := detectionSummary{Total: len(probes)}
	for _, p := range probes {
		switch p.Status {
		case "success":
			out.Success++
		case "failed":
			out.Failed++
		case "partial":
			out.Partial++
		case "not_applicable":
			out.NotApplicable++
		case "unavailable":
			out.Unavailable++
		default:
			out.Inconclusive++
		}
	}
	return out
}

func defaultDetectionNotes() []string {
	return []string{
		"检测只使用本次请求提交的 API Key；服务端不持久化该 Key，报告也不会返回该 Key。",
		"HTTP 200 只代表请求链路成功；高级能力必须同时满足结构、行为或负向探针证据才会判定为成功。",
		"缓存、计费、动态路由等服务端状态无法只凭单次客户端响应完全归因；证据不足时会返回无法确认，而不是强行判定。",
	}
}

func mergeDetectedModels(models []detectedModel) []detectedModel {
	merged := make(map[string]detectedModel, len(models))
	for _, model := range models {
		id := strings.TrimSpace(model.ID)
		if id == "" {
			continue
		}
		current, ok := merged[id]
		if !ok {
			model.ID = id
			model.Protocols = uniqueSortedStrings(model.Protocols)
			merged[id] = model
			continue
		}
		current.Protocols = uniqueSortedStrings(append(current.Protocols, model.Protocols...))
		if current.Name == "" {
			current.Name = model.Name
		}
		if current.Provider == "" {
			current.Provider = model.Provider
		}
		if len(current.Capabilities) == 0 && len(model.Capabilities) > 0 {
			current.Capabilities = model.Capabilities
		}
		merged[id] = current
	}
	out := make([]detectedModel, 0, len(merged))
	for _, model := range merged {
		out = append(out, model)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].ID) < strings.ToLower(out[j].ID) })
	return out
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func statusFromHTTP(code int) string {
	if code >= http.StatusOK && code < http.StatusMultipleChoices {
		return "success"
	}
	if code == http.StatusUnauthorized || code == http.StatusForbidden {
		return "unavailable"
	}
	return "failed"
}

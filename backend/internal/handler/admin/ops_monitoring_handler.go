package admin

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *OpsHandler) GetMonitoringCapabilities(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	result, err := h.opsService.GetMonitoringCapabilities(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *OpsHandler) QueryMonitoring(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	q := service.OpsMonitoringQuery{Metric: c.Query("metric"), Cluster: c.Query("cluster"), Namespace: c.Query("namespace"), Service: c.Query("service"), Deployment: c.Query("deployment"), Pod: c.Query("pod"), UID: c.Query("uid"), Source: c.Query("source"), Window: c.Query("window"), Protocol: c.Query("protocol"), Route: c.Query("route"), StatusCode: c.Query("status_code")}
	for key, target := range map[string]*time.Time{"start": &q.Start, "end": &q.End} {
		if raw := c.Query(key); raw != "" {
			value, err := time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				response.BadRequest(c, key+" must be RFC3339")
				return
			}
			*target = value
		}
	}
	if raw := c.Query("step"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			response.BadRequest(c, "step must be a positive integer")
			return
		}
		q.StepSeconds = value
	}
	if raw := c.Query("min_requests"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			response.BadRequest(c, "min_requests must be an integer")
			return
		}
		q.MinRequests = value
	}
	if raw := c.Query("exclude_429"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			response.BadRequest(c, "exclude_429 must be a boolean")
			return
		}
		q.Exclude429 = value
	}
	result, err := h.opsService.QueryMonitoring(c.Request.Context(), q)
	if err != nil {
		if errors.Is(err, service.ErrOpsMonitoringQuery) {
			response.BadRequest(c, err.Error())
		} else {
			response.ErrorFrom(c, err)
		}
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, result)
}

func (h *OpsHandler) ListMonitoringEvents(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	if c.Query("service") != "" || c.Query("deployment") != "" {
		response.BadRequest(c, "events support cluster, namespace, pod and uid filters")
		return
	}
	filter := service.OpsMonitoringEventsFilter{Cluster: c.Query("cluster"), Namespace: c.Query("namespace"), Pod: c.Query("pod"), UID: c.Query("uid"), ObjectKind: c.Query("object_kind"), ObjectName: c.Query("object_name"), ObjectUID: c.Query("object_uid")}
	for key, target := range map[string]*time.Time{"start": &filter.Start, "end": &filter.End} {
		if raw := c.Query(key); raw != "" {
			value, err := time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				response.BadRequest(c, key+" must be RFC3339")
				return
			}
			*target = value
		}
	}
	if raw := c.Query("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 500 {
			response.BadRequest(c, "limit must be 1..500")
			return
		}
		filter.Limit = value
	}
	result, err := h.opsService.ListMonitoringEvents(c.Request.Context(), filter)
	if err != nil {
		if errors.Is(err, service.ErrOpsMonitoringQuery) {
			response.BadRequest(c, err.Error())
		} else {
			response.ErrorFrom(c, err)
		}
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, result)
}

package middleware

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const requestIDHeader = "X-Request-ID"

var (
	requestInstanceOnce sync.Once
	requestInstanceName = "unknown"
)

// RequestLogger 在请求入口注入 request-scoped logger。
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request == nil {
			c.Next()
			return
		}

		requestID, validRequestID := normalizeCorrelationID(c.GetHeader(requestIDHeader))
		if !validRequestID {
			requestID = uuid.NewString()
		}
		c.Header(requestIDHeader, requestID)

		ctx := context.WithValue(c.Request.Context(), ctxkey.RequestID, requestID)
		clientRequestID, _ := ctx.Value(ctxkey.ClientRequestID).(string)
		clientRequestID, _ = normalizeCorrelationID(clientRequestID)

		fields := []zap.Field{
			zap.String("component", "http"),
			zap.String("request_id", requestID),
			zap.String("client_request_id", strings.TrimSpace(clientRequestID)),
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method),
			zap.String("instance", requestInstance()),
		}
		// Read on each request. The process environment is in memory, and tests change these values.
		if pod := strings.TrimSpace(os.Getenv("POD_NAME")); pod != "" {
			fields = append(fields, zap.String("pod", pod))
		}
		if namespace := strings.TrimSpace(os.Getenv("POD_NAMESPACE")); namespace != "" {
			fields = append(fields, zap.String("namespace", namespace))
		}
		if uid := strings.TrimSpace(os.Getenv("POD_UID")); uid != "" {
			fields = append(fields, zap.String("pod_uid", uid))
		}
		if cluster := strings.TrimSpace(os.Getenv("OPS_MONITORING_CLUSTER_NAME")); cluster != "" {
			fields = append(fields, zap.String("cluster", cluster))
		}
		requestLogger := logger.With(fields...)

		ctx = logger.IntoContext(ctx, requestLogger)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func requestInstance() string {
	requestInstanceOnce.Do(func() {
		host, err := os.Hostname()
		host = strings.TrimSpace(host)
		if err != nil || host == "" {
			return
		}
		requestInstanceName = host
	})
	return requestInstanceName
}

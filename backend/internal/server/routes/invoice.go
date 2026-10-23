package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func RegisterInvoiceRoutes(v1 *gin.RouterGroup, h *handler.InvoiceHandler, jwt middleware.JWTAuthMiddleware, admin middleware.AdminAuthMiddleware, audit middleware.AuditLogMiddleware, settings *service.SettingService, limiter *middleware.PanelRateLimiter) {
	user := v1.Group("/invoices", gin.HandlerFunc(jwt), middleware.BackendModeUserGuard(settings), limiter.Global(), gin.HandlerFunc(audit))
	user.GET("/eligibility", h.Eligibility)
	user.GET("", h.List)
	user.POST("", h.Create)
	user.GET("/:id", h.Get)
	user.GET("/:id/document", h.Document)
	user.POST("/:id/cancel", h.Cancel)
	management := v1.Group("/admin/invoices", gin.HandlerFunc(admin), gin.HandlerFunc(audit), middleware.AdminComplianceGuard(settings))
	management.GET("/config", h.GetConfig)
	management.PUT("/config", h.UpdateConfig)
	management.GET("/export", h.Export)
	management.POST("/export", h.ExportSelected)
	management.GET("", h.AdminList)
	management.GET("/:id", h.AdminGet)
	management.GET("/:id/account", h.AdminAccount)
	management.POST("/:id/document", h.UploadDocument)
	management.GET("/:id/document", h.AdminDocument)
	management.PATCH("/:id", h.AdminUpdate)
}

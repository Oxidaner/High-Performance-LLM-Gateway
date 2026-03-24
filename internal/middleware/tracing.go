package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// Tracing creates request-level OpenTelemetry spans.
func Tracing() gin.HandlerFunc {
	tracer := otel.Tracer("llm-gateway/http")

	return func(c *gin.Context) {
		spanName := fmt.Sprintf("%s %s", c.Request.Method, c.FullPath())
		if c.FullPath() == "" {
			spanName = fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path)
		}

		ctx, span := tracer.Start(c.Request.Context(), spanName)
		startedAt := time.Now()
		c.Request = c.Request.WithContext(ctx)

		span.SetAttributes(
			attribute.String("http.method", c.Request.Method),
			attribute.String("http.route", c.FullPath()),
			attribute.String("http.target", c.Request.URL.Path),
		)

		c.Next()

		latency := time.Since(startedAt)
		statusCode := c.Writer.Status()
		span.SetAttributes(
			attribute.Int("http.status_code", statusCode),
			attribute.Int64("http.server_duration_ms", latency.Milliseconds()),
		)
		if statusCode >= 500 {
			span.SetStatus(codes.Error, fmt.Sprintf("http %d", statusCode))
		} else {
			span.SetStatus(codes.Ok, "")
		}
		span.End()
	}
}

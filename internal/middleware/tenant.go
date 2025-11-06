package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type contextKey string

const TenantIDKey contextKey = "tenant_id"

// TenantID middleware extracts tenant ID from header
func TenantID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantIDStr := r.Header.Get("X-Tenant-ID")
		if tenantIDStr == "" {
			log.Warn().Msg("Missing X-Tenant-ID header")
			http.Error(w, "X-Tenant-ID header is required", http.StatusBadRequest)
			return
		}

		tenantID, err := uuid.Parse(tenantIDStr)
		if err != nil {
			log.Warn().Err(err).Str("tenant_id", tenantIDStr).Msg("Invalid tenant ID")
			http.Error(w, "Invalid X-Tenant-ID format", http.StatusBadRequest)
			return
		}

		// Add tenant ID to context
		ctx := context.WithValue(r.Context(), TenantIDKey, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetTenantID extracts tenant ID from context
func GetTenantID(ctx context.Context) (uuid.UUID, bool) {
	tenantID, ok := ctx.Value(TenantIDKey).(uuid.UUID)
	return tenantID, ok
}

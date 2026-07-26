package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestProtectedRoutesRequireLogin 确保所有业务数据接口都不能匿名访问。
func TestProtectedRoutesRequireLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterAPIRoutes(router)

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/query-connections"},
		{method: http.MethodPost, path: "/api/query-data", body: `{"connectionName":"test","sql":"SELECT 1"}`},
		{method: http.MethodPost, path: "/api/query-export-excel", body: `{"connectionName":"test","sql":"SELECT 1"}`},
		{method: http.MethodPost, path: "/api/query-metadata", body: `{"connectionName":"test","keyword":""}`},
		{method: http.MethodGet, path: "/api/sql-favorites"},
		{method: http.MethodPost, path: "/api/sql-favorites", body: `{}`},
		{method: http.MethodPut, path: "/api/sql-favorites", body: `{}`},
		{method: http.MethodDelete, path: "/api/sql-favorites", body: `{}`},
		{method: http.MethodGet, path: "/api/admin/users"},
		{method: http.MethodGet, path: "/api/admin/db-connections"},
		{method: http.MethodPost, path: "/api/admin/db-connections/test", body: `{}`},
	}

	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d: %s", response.Code, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), `"ok":false`) {
				t.Fatalf("expected fixed unauthorized JSON, got %s", response.Body.String())
			}
		})
	}
}

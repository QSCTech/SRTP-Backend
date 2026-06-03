package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequireAuthRejectsMissingHeader(t *testing.T) {
	router := testRouter()
	router.GET("/me", RequireAuth(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/me", nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuthStoresAuthUIDInContext(t *testing.T) {
	router := testRouter()
	router.GET("/me", RequireAuth(), func(c *gin.Context) {
		authUID, _ := c.Request.Context().Value(AuthUIDKey).(string)
		if authUID != "auth-1" {
			t.Fatalf("authUID=%q want %q", authUID, "auth-1")
		}
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	request.Header.Set("X-Auth-UID", "auth-1")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestCurrentUserAllowsMissingHeader(t *testing.T) {
	router := testRouter()
	router.GET("/rooms/:roomId", CurrentUser(), func(c *gin.Context) {
		authUID, _ := c.Request.Context().Value(AuthUIDKey).(string)
		if authUID != "" {
			t.Fatalf("authUID=%q want empty", authUID)
		}
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/rooms/1", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status=%d want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestAuthByRouteProtectsOnlyConfiguredRoutes(t *testing.T) {
	router := testRouter()
	router.Use(AuthByRoute())
	router.GET("/me", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.GET("/rooms", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/me", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("/me status=%d want %d", recorder.Code, http.StatusUnauthorized)
	}

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/rooms", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("/rooms status=%d want %d", recorder.Code, http.StatusNoContent)
	}
}

func testRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

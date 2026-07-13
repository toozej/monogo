package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSearchRejectsUnknownProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/search", Search)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/search?q=test&searchSource=unknown", http.NoBody)
	r.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

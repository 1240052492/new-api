package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuthAccessTokenCarriesUserRole(t *testing.T) {
	setupOAuthServerTest(t)
	gin.SetMode(gin.TestMode)

	admin := &model.User{Username: "guide-admin", Password: "test-password", Role: 10, Status: 1, Group: "default", AffCode: "ga1", AuthVersion: 1}
	require.NoError(t, model.DB.Create(admin).Error)

	oauthService := service.NewOAuthService()
	token, err := oauthService.GenerateAccessToken(admin.Id, "infinite-canvas-client", "user:read")
	require.NoError(t, err)

	claims, err := oauthService.GetTokenInfo(token)
	require.NoError(t, err)
	assert.Equal(t, admin.Id, claims.UserID)
	assert.Equal(t, 10, claims.Role)
}

func setupCreationGuideRouter(t *testing.T) *gin.Engine {
	setupOAuthServerTest(t)
	gin.SetMode(gin.TestMode)

	// Tests run without Redis; the cache write in UpdateUserSetting is a
	// no-op in this mode and the database stays the source of truth.
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedisEnabled })

	router := gin.New()
	router.GET("/api/creation-guide/status", GetCreationGuideStatus)
	router.POST("/api/creation-guide/ack", AckCreationGuide)
	return router
}

func doCreationGuideRequest(t *testing.T, router *gin.Engine, method, path, bearer string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestCreationGuideAckSuppressesReminderPerAccount(t *testing.T) {
	router := setupCreationGuideRouter(t)

	user := &model.User{Username: "guide-user", Password: "test-password", Role: 1, Status: 1, Group: "default", AffCode: "gu1", AuthVersion: 1}
	require.NoError(t, model.DB.Create(user).Error)
	oauthService := service.NewOAuthService()
	token, err := oauthService.GenerateAccessToken(user.Id, "infinite-canvas-client", "user:read")
	require.NoError(t, err)

	// Missing token is rejected.
	recorder := doCreationGuideRequest(t, router, http.MethodGet, "/api/creation-guide/status", "", "")
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)

	// Tampered token is rejected.
	recorder = doCreationGuideRequest(t, router, http.MethodGet, "/api/creation-guide/status", token+"x", "")
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)

	// Initial status has no acknowledgements.
	recorder = doCreationGuideRequest(t, router, http.MethodGet, "/api/creation-guide/status", token, "")
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"reminder_days": 7, "acked_at": {}}`, recorder.Body.String())

	// Invalid platform is rejected without touching settings.
	recorder = doCreationGuideRequest(t, router, http.MethodPost, "/api/creation-guide/ack", token, `{"platform": "canvas"}`)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	// A valid acknowledgement is recorded for the account.
	recorder = doCreationGuideRequest(t, router, http.MethodPost, "/api/creation-guide/ack", token, `{"platform": "image"}`)
	require.Equal(t, http.StatusOK, recorder.Code)

	reloaded, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	acks := reloaded.GetSetting().CreationGuideAcks
	require.Len(t, acks, 1)
	assert.WithinDuration(t, time.Now(), time.Unix(acks["image"], 0), time.Minute)

	// The stored acknowledgement shows up in the next status read.
	recorder = doCreationGuideRequest(t, router, http.MethodGet, "/api/creation-guide/status", token, "")
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"reminder_days":7`)
	assert.NotContains(t, recorder.Body.String(), `"image":0`)
	assert.Contains(t, recorder.Body.String(), `"image":`)

	// A second account does not inherit the acknowledgement.
	other := &model.User{Username: "guide-other", Password: "test-password", Role: 1, Status: 1, Group: "default", AffCode: "gu2", AuthVersion: 1}
	require.NoError(t, model.DB.Create(other).Error)
	otherToken, err := oauthService.GenerateAccessToken(other.Id, "infinite-canvas-client", "user:read")
	require.NoError(t, err)
	recorder = doCreationGuideRequest(t, router, http.MethodGet, "/api/creation-guide/status", otherToken, "")
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"reminder_days": 7, "acked_at": {}}`, recorder.Body.String())
}

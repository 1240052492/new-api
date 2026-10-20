package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const sessionProbeRedirectURI = "http://canvas.local/oauth/callback"

func setupSessionProbeTest(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := model.DB
	previousRedis := common.RedisEnabled
	previousSecret := common.SessionSecret
	previousSecure := common.SessionCookieSecure
	previousTrusted := common.SessionCookieTrustedURLs

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}, &model.OAuthApplication{}))

	model.DB = db
	common.RedisEnabled = false
	common.SessionSecret = "session-probe-test-secret"
	common.SessionCookieSecure = false
	common.SessionCookieTrustedURLs = nil

	t.Cleanup(func() {
		model.DB = previousDB
		common.RedisEnabled = previousRedis
		common.SessionSecret = previousSecret
		common.SessionCookieSecure = previousSecure
		common.SessionCookieTrustedURLs = previousTrusted
		require.NoError(t, sqlDB.Close())
	})

	return db
}

func newSessionProbeRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/user/auth/session-status", middleware.CORSForOAuthClients(), ProbeAuthSession)
	return router
}

func createSessionProbeUser(t *testing.T, db *gorm.DB) *model.User {
	t.Helper()
	user := &model.User{
		Username: "session-probe-user", Password: "unused", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "probe1",
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func probeSession(t *testing.T, router *gin.Engine, cookie *http.Cookie, origin string) (int, string, string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/user/auth/session-status", nil)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder.Code, recorder.Body.String(), recorder.Header().Get("Access-Control-Allow-Origin")
}

// The probe is the Canvas silent-SSO contract: it always answers 200 with a
// single boolean, so the client branches without error handling, and it reads
// the refresh cookie itself because the middleware chain cannot see that
// cookie outside the /api/user/auth prefix.
func TestSessionProbeReportsLiveCookieSession(t *testing.T) {
	db := setupSessionProbeTest(t)
	router := newSessionProbeRouter()
	user := createSessionProbeUser(t, db)

	status, body, _ := probeSession(t, router, nil, "")
	require.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"authenticated": false}`, body, "no cookie means no session")

	bundle, err := service.CreateLoginSession(user.Id, "password", "127.0.0.1", "probe-agent")
	require.NoError(t, err)

	status, body, _ = probeSession(t, router, &http.Cookie{Name: service.RefreshCookieName, Value: bundle.RefreshToken}, "")
	require.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"authenticated": true}`, body)
}

func TestSessionProbeRejectsForgedStaleAndRevokedCookies(t *testing.T) {
	db := setupSessionProbeTest(t)
	router := newSessionProbeRouter()
	user := createSessionProbeUser(t, db)
	bundle, err := service.CreateLoginSession(user.Id, "password", "127.0.0.1", "probe-agent")
	require.NoError(t, err)
	cookieName := service.RefreshCookieName

	t.Run("forged secret", func(t *testing.T) {
		status, body, _ := probeSession(t, router, &http.Cookie{
			Name:  cookieName,
			Value: bundle.Session.SID + ".forged-secret",
		}, "")
		require.Equal(t, http.StatusOK, status)
		assert.JSONEq(t, `{"authenticated": false}`, body)
	})

	t.Run("malformed value", func(t *testing.T) {
		status, body, _ := probeSession(t, router, &http.Cookie{Name: cookieName, Value: "not-a-session"}, "")
		require.Equal(t, http.StatusOK, status)
		assert.JSONEq(t, `{"authenticated": false}`, body)
	})

	t.Run("rotated away by a refresh", func(t *testing.T) {
		rotated, _, err := service.RefreshLoginSession(bundle.RefreshToken, bundle.Session.SID, "127.0.0.1", "probe-agent")
		require.NoError(t, err)

		_, body, _ := probeSession(t, router, &http.Cookie{Name: cookieName, Value: bundle.RefreshToken}, "")
		assert.JSONEq(t, `{"authenticated": false}`, body, "the superseded secret must not pass the probe")

		_, body, _ = probeSession(t, router, &http.Cookie{Name: cookieName, Value: rotated.RefreshToken}, "")
		assert.JSONEq(t, `{"authenticated": true}`, body)
	})

	t.Run("revoked session", func(t *testing.T) {
		revocable, err := service.CreateLoginSession(user.Id, "password", "127.0.0.1", "probe-agent")
		require.NoError(t, err)
		_, err = model.RevokeUserSession(user.Id, revocable.Session.SID, "test_revoked")
		require.NoError(t, err)

		status, body, _ := probeSession(t, router, &http.Cookie{Name: cookieName, Value: revocable.RefreshToken}, "")
		require.Equal(t, http.StatusOK, status)
		assert.JSONEq(t, `{"authenticated": false}`, body)
	})
}

// Only origins the console already trusts may read a credentialed answer:
// the request's own origin, the session-cookie allowlist, and the redirect
// URIs of active OAuth applications. Everything else gets no CORS headers,
// so the browser blocks the read.
func TestSessionProbeReflectsOnlyTrustedOrigins(t *testing.T) {
	db := setupSessionProbeTest(t)
	router := newSessionProbeRouter()

	app := &model.OAuthApplication{
		Name: "Infinite Canvas", ClientID: "client_session_probe",
		Scopes: "user:read", OwnerUserID: 1,
	}
	require.NoError(t, app.SetRedirectURIs([]string{sessionProbeRedirectURI}))
	require.NoError(t, db.Create(app).Error)

	cases := []struct {
		name          string
		origin        string
		wantReflected bool
	}{
		{name: "registered oauth client", origin: "http://canvas.local", wantReflected: true},
		{name: "unregistered origin", origin: "http://evil.example", wantReflected: false},
		{name: "lookalike origin", origin: "http://canvas.local.evil.example", wantReflected: false},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			status, _, allowOrigin := probeSession(t, router, nil, test.origin)
			require.Equal(t, http.StatusOK, status)
			if test.wantReflected {
				assert.Equal(t, test.origin, allowOrigin)
			} else {
				assert.Empty(t, allowOrigin)
			}
		})
	}
}

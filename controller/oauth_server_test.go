package controller

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const oauthTestVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
const oauthTestChallenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"

func setupOAuthServerTest(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.OAuthApplication{}, &model.OAuthAuthorizationCode{}, &model.User{}))

	previousDB := model.DB
	previousSessionSecret := common.SessionSecret
	model.DB = db
	common.SessionSecret = "oauth-test-session-secret"
	t.Cleanup(func() {
		model.DB = previousDB
		common.SessionSecret = previousSessionSecret
		require.NoError(t, sqlDB.Close())
	})

	return db
}

func TestOAuthDeleteHidesApplicationAndPKCEAuthorizationCompletesOnce(t *testing.T) {
	db := setupOAuthServerTest(t)
	gin.SetMode(gin.TestMode)

	user := &model.User{Username: "oauth-test-user", Password: "test-password", Role: 1, Status: 1, Group: "default", AuthVersion: 1}
	require.NoError(t, db.Create(user).Error)
	app := &model.OAuthApplication{
		Name:         "Infinite Canvas",
		ClientID:     "infinite-canvas-client",
		ClientSecret: "unused-by-pkce-client",
		RedirectURIs: `["http://localhost:5173/oauth/callback"]`,
		Scopes:       `["user:read"]`,
		OwnerUserID:  user.Id,
		Status:       model.OAuthApplicationStatusActive,
	}
	require.NoError(t, db.Create(app).Error)

	state := "canvas-state"
	query := url.Values{
		"client_id":             {app.ClientID},
		"redirect_uri":          {"http://localhost:5173/oauth/callback"},
		"response_type":         {"code"},
		"response_mode":         {"json"},
		"scope":                 {"user:read"},
		"state":                 {state},
		"code_challenge":        {oauthTestChallenge},
		"code_challenge_method": {"S256"},
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/oauth/authorize?"+query.Encode(), nil)
	ctx.Set("id", user.Id)
	ctx.Set("session_id", "oauth-test-session")
	ctx.Set("auth_version", int64(1))
	ctx.Set("session_version", int64(1))
	OAuthAuthorize(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var authorization struct {
		Code        string `json:"code"`
		State       string `json:"state"`
		RedirectURI string `json:"redirect_uri"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &authorization))
	assert.NotEmpty(t, authorization.Code)
	assert.Equal(t, state, authorization.State)
	assert.Equal(t, "http://localhost:5173/oauth/callback", authorization.RedirectURI)

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {app.ClientID},
		"code":          {authorization.Code},
		"redirect_uri":  {authorization.RedirectURI},
		"code_verifier": {"wrong-verifier"},
	}
	request := httptest.NewRequest(http.MethodPost, "/api/oauth/token", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = request
	OAuthToken(ctx)
	assert.Equal(t, http.StatusBadRequest, recorder.Code, "a wrong verifier must not consume the code")

	form.Set("code_verifier", oauthTestVerifier)
	request = httptest.NewRequest(http.MethodPost, "/api/oauth/token", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = request
	OAuthToken(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var token OAuthTokenResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &token))
	assert.NotEmpty(t, token.AccessToken)
	assert.Equal(t, 3600, token.ExpiresIn)

	request = httptest.NewRequest(http.MethodPost, "/api/oauth/token", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = request
	OAuthToken(ctx)
	assert.Equal(t, http.StatusBadRequest, recorder.Code, "an authorization code must be single-use")

	require.NoError(t, model.DeleteOAuthApplication(app.ID))
	apps, total, err := model.ListOAuthApplications(user.Id, 0, 10)
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.Empty(t, apps)
	_, err = model.GetOAuthApplicationByID(app.ID)
	assert.ErrorIs(t, err, model.ErrOAuthApplicationNotFound)
	_, err = service.NewOAuthService().ValidateClientCredentials(app.ClientID, "any-secret")
	assert.ErrorIs(t, err, model.ErrOAuthApplicationNotFound)
	assert.ErrorIs(t, model.DeleteOAuthApplication(app.ID), model.ErrOAuthApplicationNotFound)
}

func TestOAuthClientConfigResolvesNewestActiveApplicationByRedirectURI(t *testing.T) {
	db := setupOAuthServerTest(t)
	gin.SetMode(gin.TestMode)

	callback := "http://localhost:5173/oauth/callback"
	replaced := &model.OAuthApplication{
		Name:         "Infinite Canvas",
		ClientID:     "client_replaced",
		ClientSecret: "unused-by-pkce-client",
		RedirectURIs: `["` + callback + `"]`,
		Scopes:       `["user:read"]`,
		OwnerUserID:  1,
		Status:       model.OAuthApplicationStatusActive,
	}
	require.NoError(t, db.Create(replaced).Error)
	require.NoError(t, model.DeleteOAuthApplication(replaced.ID))

	current := &model.OAuthApplication{
		Name:         "infinite-canvas",
		ClientID:     "client_current",
		ClientSecret: "unused-by-pkce-client",
		RedirectURIs: `["` + callback + `","http://127.0.0.1:5173/oauth/callback"]`,
		Scopes:       `["user:read"]`,
		OwnerUserID:  1,
		Status:       model.OAuthApplicationStatusActive,
	}
	require.NoError(t, db.Create(current).Error)

	inactive := &model.OAuthApplication{
		Name:         "infinite-canvas-inactive",
		ClientID:     "client_inactive",
		ClientSecret: "unused-by-pkce-client",
		RedirectURIs: `["` + callback + `"]`,
		Scopes:       `["user:read"]`,
		OwnerUserID:  1,
		Status:       model.OAuthApplicationStatusInactive,
	}
	require.NoError(t, db.Create(inactive).Error)
	// GORM skips the zero-value Status on insert because the column has a
	// default, so the inactive state must be written explicitly.
	require.NoError(t, db.Model(inactive).Update("status", model.OAuthApplicationStatusInactive).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/oauth/client-config?redirect_uri="+url.QueryEscape(callback), nil)
	GetOAuthClientConfig(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var payload struct {
		ClientID              string `json:"client_id"`
		Scope                 string `json:"scope"`
		AuthorizationEndpoint string `json:"authorization_endpoint"`
		TokenEndpoint         string `json:"token_endpoint"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	assert.Equal(t, "client_current", payload.ClientID, "soft-deleted and inactive applications must be excluded")
	assert.Equal(t, "user:read", payload.Scope)
	assert.Equal(t, "/oauth/authorize", payload.AuthorizationEndpoint)
	assert.Equal(t, "/api/oauth/token", payload.TokenEndpoint)

	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/oauth/client-config?redirect_uri="+url.QueryEscape("http://localhost:5173/other"), nil)
	GetOAuthClientConfig(ctx)
	assert.Equal(t, http.StatusNotFound, recorder.Code)

	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/oauth/client-config", nil)
	GetOAuthClientConfig(ctx)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

package controller

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// OAuthAuthorizeRequest represents the OAuth authorization request
type OAuthAuthorizeRequest struct {
	ClientID            string `form:"client_id" binding:"required"`
	RedirectURI         string `form:"redirect_uri" binding:"required"`
	ResponseType        string `form:"response_type" binding:"required"`
	ResponseMode        string `form:"response_mode"`
	Scope               string `form:"scope"`
	State               string `form:"state" binding:"required"`
	CodeChallenge       string `form:"code_challenge"`
	CodeChallengeMethod string `form:"code_challenge_method"`
}

// OAuthAuthorize handles GET /oauth/authorize
// Implements the authorization endpoint of OAuth 2.0 Authorization Code Flow
func OAuthAuthorize(c *gin.Context) {
	var req OAuthAuthorizeRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "invalid_request",
			"error_description": "Missing or invalid parameters",
		})
		return
	}

	if req.ResponseMode != "" && req.ResponseMode != "json" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "error_description": "Unsupported response_mode"})
		return
	}

	// Get OAuth service
	oauthService := service.NewOAuthService()

	// Validate client_id
	app, err := model.GetOAuthApplicationByClientID(req.ClientID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client", "error_description": "Client not found or inactive"})
		return
	}

	// Validate redirect_uri
	if err := oauthService.ValidateRedirectURI(app, req.RedirectURI); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "invalid_request",
			"error_description": err.Error(),
		})
		return
	}
	if req.ResponseType != "code" {
		writeOAuthAuthorizeError(c, req, "unsupported_response_type", "Only 'code' response type is supported")
		return
	}
	if (req.CodeChallenge == "") != (req.CodeChallengeMethod == "") || (req.CodeChallenge != "" && req.CodeChallengeMethod != "S256") {
		writeOAuthAuthorizeError(c, req, "invalid_request", "A valid S256 code_challenge is required")
		return
	}

	// Get authenticated user
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		// User not authenticated, redirect to login
		// The client should handle this redirect and show login UI
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":             "access_denied",
			"error_description": "User authentication required",
		})
		return
	}

	// Validate scope if present
	if req.Scope != "" {
		if valid, err := oauthService.ValidateScope(app, req.Scope); !valid {
			redirectWithError(c, req.RedirectURI, req.State, "invalid_scope", err.Error())
			return
		}
	}

	// Generate authorization code
	code, err := oauthService.GenerateAuthorizationCode(req.ClientID, identity.UserID, req.RedirectURI, req.Scope, req.CodeChallenge, req.CodeChallengeMethod)
	if err != nil {
		redirectWithError(c, req.RedirectURI, req.State, "server_error", "Failed to generate authorization code")
		return
	}

	if req.ResponseMode == "json" {
		c.JSON(http.StatusOK, gin.H{
			"code":         code,
			"state":        req.State,
			"redirect_uri": req.RedirectURI,
		})
		return
	}

	// Redirect back to the validated client URI with the authorization code.
	redirectURL, err := url.Parse(req.RedirectURI)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "error_description": "Invalid redirect_uri"})
		return
	}
	redirectParams := redirectURL.Query()
	redirectParams.Set("code", code)
	redirectParams.Set("state", req.State)
	redirectURL.RawQuery = redirectParams.Encode()
	c.Redirect(http.StatusFound, redirectURL.String())
}

func writeOAuthAuthorizeError(c *gin.Context, req OAuthAuthorizeRequest, errorCode, errorDescription string) {
	if req.ResponseMode == "json" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errorCode, "error_description": errorDescription})
		return
	}
	redirectWithError(c, req.RedirectURI, req.State, errorCode, errorDescription)
}

// GetOAuthClientConfig handles GET /api/oauth/client-config
// Public discovery endpoint for PKCE public clients (Infinite Canvas): given a
// redirect_uri, it returns the client_id of the most recently created active
// application registered for it. client_id is not secret for PKCE clients, and
// no credentials are returned, so applications can be rotated from the console
// without rebuilding or reconfiguring the client.
func GetOAuthClientConfig(c *gin.Context) {
	redirectURI := c.Query("redirect_uri")
	if redirectURI == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "invalid_request",
			"error_description": "redirect_uri is required",
		})
		return
	}

	app, err := model.FindActiveOAuthApplicationByRedirectURI(redirectURI)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":             "invalid_client",
			"error_description": "No active application is registered for this redirect_uri",
		})
		return
	}

	var scopes []string
	if app.Scopes != "" {
		_ = common.UnmarshalJsonStr(app.Scopes, &scopes)
	}

	c.JSON(http.StatusOK, gin.H{
		"client_id":              app.ClientID,
		"scope":                  strings.Join(scopes, " "),
		"authorization_endpoint": "/oauth/authorize",
		"token_endpoint":         "/api/oauth/token",
	})
}

// OAuthTokenRequest represents the OAuth token exchange request
type OAuthTokenRequest struct {
	GrantType    string `form:"grant_type" binding:"required"`
	Code         string `form:"code" binding:"required"`
	ClientID     string `form:"client_id" binding:"required"`
	ClientSecret string `form:"client_secret"`
	RedirectURI  string `form:"redirect_uri" binding:"required"`
	CodeVerifier string `form:"code_verifier"`
}

// OAuthTokenResponse represents the OAuth token exchange response
type OAuthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope,omitempty"`
}

// OAuthToken handles POST /oauth/token
// Implements the token endpoint of OAuth 2.0 Authorization Code Flow
func OAuthToken(c *gin.Context) {
	var req OAuthTokenRequest

	// Parse form data
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "invalid_request",
			"error_description": "Missing or invalid parameters",
		})
		return
	}

	// Validate grant_type
	if req.GrantType != "authorization_code" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "unsupported_grant_type",
			"error_description": "Only 'authorization_code' grant type is supported",
		})
		return
	}

	// Get OAuth service
	oauthService := service.NewOAuthService()

	// Exchange authorization code for access token
	accessToken, err := oauthService.ExchangeAuthorizationCodeForToken(req.Code, req.ClientID, req.RedirectURI, req.ClientSecret, req.CodeVerifier)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "invalid_grant",
			"error_description": err.Error(),
		})
		return
	}

	// Return token response
	c.JSON(http.StatusOK, OAuthTokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   3600, // 1 hour
	})
}

// Helper function to redirect with error
func redirectWithError(c *gin.Context, redirectURI, state, errorCode, errorDescription string) {
	params := url.Values{
		"error": {errorCode},
	}
	if errorDescription != "" {
		params.Add("error_description", errorDescription)
	}
	if state != "" {
		params.Add("state", state)
	}

	redirectURL := redirectURI + "?" + params.Encode()
	c.Redirect(http.StatusFound, redirectURL)
}

// OAuthApplicationRequest represents a request to create/update an OAuth application
type OAuthApplicationRequest struct {
	Name         string   `json:"name" binding:"required,min=1,max=100"`
	RedirectURIs []string `json:"redirect_uris" binding:"required,min=1"`
	Scopes       string   `json:"scopes"`
	AutoApprove  bool     `json:"auto_approve"`
	Description  string   `json:"description"`
}

// CreateOAuthApplication handles POST /api/admin/oauth/applications
// Creates a new OAuth application
func CreateOAuthApplication(c *gin.Context) {
	var req OAuthApplicationRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorT(c, "Invalid request body")
		return
	}

	// Get authenticated user
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		common.ApiErrorT(c, "User authentication required")
		return
	}

	// Get user
	user, err := model.GetUserById(identity.UserID, false)
	if err != nil {
		common.ApiErrorT(c, "User not found")
		return
	}

	// Generate client ID and secret
	clientID := "client_" + common.GetRandomString(32)
	clientSecret := common.GetRandomString(64)

	// Create application
	app := &model.OAuthApplication{
		Name:        req.Name,
		ClientID:    clientID,
		Scopes:      req.Scopes,
		AutoApprove: req.AutoApprove,
		OwnerUserID: user.Id,
		Description: req.Description,
	}

	if err := app.SetRedirectURIs(req.RedirectURIs); err != nil {
		common.ApiErrorT(c, "Invalid redirect URIs")
		return
	}

	if err := model.CreateOAuthApplication(app, clientSecret); err != nil {
		common.ApiErrorT(c, "Failed to create OAuth application")
		return
	}

	// Return the created application with client secret
	c.JSON(http.StatusCreated, gin.H{
		"id":            app.ID,
		"name":          app.Name,
		"client_id":     app.ClientID,
		"client_secret": clientSecret, // Only shown once on creation
		"redirect_uris": req.RedirectURIs,
		"scopes":        app.Scopes,
		"auto_approve":  app.AutoApprove,
		"created_at":    app.CreatedAt,
	})
}

// GetOAuthApplications handles GET /api/admin/oauth/applications
// Lists OAuth applications for the authenticated user
func GetOAuthApplications(c *gin.Context) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		common.ApiErrorT(c, "User authentication required")
		return
	}

	offset := 0
	limit := 10
	if p := c.Query("offset"); p != "" {
		if o, err := strconv.Atoi(p); err == nil {
			offset = o
		}
	}
	if p := c.Query("limit"); p != "" {
		if l, err := strconv.Atoi(p); err == nil {
			limit = l
		}
	}

	// Validate limit
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 10
	}

	apps, total, err := model.ListOAuthApplications(identity.UserID, offset, limit)
	if err != nil {
		common.ApiErrorT(c, "Failed to list applications")
		return
	}

	// Build response without client secrets
	var response []gin.H
	for _, app := range apps {
		uris, _ := app.GetRedirectURIs()
		response = append(response, gin.H{
			"id":            app.ID,
			"name":          app.Name,
			"client_id":     app.ClientID,
			"redirect_uris": uris,
			"scopes":        app.Scopes,
			"auto_approve":  app.AutoApprove,
			"description":   app.Description,
			"created_at":    app.CreatedAt,
			"updated_at":    app.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"data":  response,
	})
}

// DeleteOAuthApplication handles DELETE /api/admin/oauth/applications/:id
// Deletes an OAuth application
func DeleteOAuthApplication(c *gin.Context) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		common.ApiErrorT(c, "User authentication required")
		return
	}

	id := c.Param("id")
	appID, err := strconv.Atoi(id)
	if err != nil {
		common.ApiErrorT(c, "Invalid application ID")
		return
	}

	// Get application
	app, err := model.GetOAuthApplicationByID(appID)
	if err != nil {
		common.ApiErrorT(c, "Application not found")
		return
	}

	// Check ownership
	if app.OwnerUserID != identity.UserID {
		common.ApiErrorT(c, "You don't have permission to delete this application")
		return
	}

	// Delete application
	if err := model.DeleteOAuthApplication(appID); err != nil {
		common.ApiErrorT(c, "Failed to delete application")
		return
	}

	common.ApiSuccess(c, gin.H{"message": "Application deleted successfully"})
}

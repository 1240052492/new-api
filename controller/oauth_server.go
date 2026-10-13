package controller

import (
	"net/http"
	"net/url"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// OAuthAuthorizeRequest represents the OAuth authorization request
type OAuthAuthorizeRequest struct {
	ClientID     string `form:"client_id" binding:"required"`
	RedirectURI  string `form:"redirect_uri" binding:"required"`
	ResponseType string `form:"response_type" binding:"required"`
	Scope        string `form:"scope"`
	State        string `form:"state" binding:"required"`
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

	// Validate response_type
	if req.ResponseType != "code" {
		redirectWithError(c, req.RedirectURI, req.State, "unsupported_response_type", "Only 'code' response type is supported")
		return
	}

	// Get OAuth service
	oauthService := service.NewOAuthService()

	// Validate client_id
	app, err := model.GetOAuthApplicationByClientID(req.ClientID)
	if err != nil {
		redirectWithError(c, req.RedirectURI, req.State, "invalid_client", "Client not found or inactive")
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
	code, err := oauthService.GenerateAuthorizationCode(req.ClientID, identity.UserID, req.RedirectURI, req.Scope)
	if err != nil {
		redirectWithError(c, req.RedirectURI, req.State, "server_error", "Failed to generate authorization code")
		return
	}

	// Redirect back to client with authorization code
	redirectParams := url.Values{
		"code":  {code},
		"state": {req.State},
	}

	redirectURL := req.RedirectURI + "?" + redirectParams.Encode()
	c.Redirect(http.StatusFound, redirectURL)
}

// OAuthTokenRequest represents the OAuth token exchange request
type OAuthTokenRequest struct {
	GrantType    string `form:"grant_type" binding:"required"`
	Code         string `form:"code" binding:"required_if=GrantType authorization_code"`
	ClientID     string `form:"client_id" binding:"required"`
	ClientSecret string `form:"client_secret" binding:"required"`
	RedirectURI  string `form:"redirect_uri" binding:"required"`
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

	// Validate client credentials
	app, err := oauthService.ValidateClientCredentials(req.ClientID, req.ClientSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":             "invalid_client",
			"error_description": err.Error(),
		})
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

	// Exchange authorization code for access token
	accessToken, err := oauthService.ExchangeAuthorizationCodeForToken(req.Code, req.ClientID, req.RedirectURI)
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
	clientID := "client_" + common.GenerateRandomString(32)
	clientSecret := common.GenerateRandomString(64)

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
		"id":              app.ID,
		"name":            app.Name,
		"client_id":       app.ClientID,
		"client_secret":   clientSecret, // Only shown once on creation
		"redirect_uris":   req.RedirectURIs,
		"scopes":          app.Scopes,
		"auto_approve":    app.AutoApprove,
		"created_at":      app.CreatedAt,
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
		_, err := common.ParseInt(p)
		if err == nil {
			offset, _ = common.ParseInt(p)
		}
	}
	if p := c.Query("limit"); p != "" {
		_, err := common.ParseInt(p)
		if err == nil {
			limit, _ = common.ParseInt(p)
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
			"id":           app.ID,
			"name":         app.Name,
			"client_id":    app.ClientID,
			"redirect_uris": uris,
			"scopes":       app.Scopes,
			"auto_approve": app.AutoApprove,
			"description":  app.Description,
			"created_at":   app.CreatedAt,
			"updated_at":   app.UpdatedAt,
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
	appID, err := common.ParseInt(id)
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

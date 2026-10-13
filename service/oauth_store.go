package service

import (
	"time"

	"github.com/QuantumNous/new-api/model"
)

// OAuthStore provides a simplified OAuth token storage abstraction
// This is NOT using go-oauth2 directly, but implements our own OAuth logic

// ClientStore holds OAuth client information
type ClientStore struct {
	app *model.OAuthApplication
}

// NewClientStore creates a new client store
func NewClientStore(app *model.OAuthApplication) *ClientStore {
	return &ClientStore{app: app}
}

// GetClient returns the OAuth client information
func (cs *ClientStore) GetClient() *model.OAuthApplication {
	return cs.app
}

// TokenStore holds token information (in-memory, not persisted)
type TokenStore struct {
	accessToken string
	expiresIn   int64
	tokenType   string
}

// NewTokenStore creates a new token store
func NewTokenStore(accessToken string, expiresIn int64) *TokenStore {
	return &TokenStore{
		accessToken: accessToken,
		expiresIn:   expiresIn,
		tokenType:   "Bearer",
	}
}

// GetAccessToken returns the access token
func (ts *TokenStore) GetAccessToken() string {
	return ts.accessToken
}

// GetExpiresIn returns the token expiration time in seconds
func (ts *TokenStore) GetExpiresIn() int64 {
	return ts.expiresIn
}

// GetTokenType returns the token type
func (ts *TokenStore) GetTokenType() string {
	return ts.tokenType
}

// AuthorizationCodeStore manages authorization codes
type AuthorizationCodeStore struct {
	code      string
	expiresAt time.Time
}

// NewAuthorizationCodeStore creates a new authorization code store
func NewAuthorizationCodeStore(code string, ttl time.Duration) *AuthorizationCodeStore {
	return &AuthorizationCodeStore{
		code:      code,
		expiresAt: time.Now().Add(ttl),
	}
}

// GetCode returns the authorization code
func (acs *AuthorizationCodeStore) GetCode() string {
	return acs.code
}

// IsExpired checks if the code is expired
func (acs *AuthorizationCodeStore) IsExpired() bool {
	return time.Now().After(acs.expiresAt)
}

// TokenResponse represents the token endpoint response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope,omitempty"`
}

// ErrorResponse represents an OAuth error response
type ErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorURI         string `json:"error_uri,omitempty"`
}

var (
	// OAuth error constants
	ErrorInvalidRequest      = "invalid_request"
	ErrorInvalidClient       = "invalid_client"
	ErrorInvalidGrant        = "invalid_grant"
	ErrorUnauthorizedClient  = "unauthorized_client"
	ErrorUnsupportedGrantType = "unsupported_grant_type"
	ErrorInvalidScope        = "invalid_scope"
	ErrorServerError         = "server_error"
)


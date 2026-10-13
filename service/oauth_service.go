package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/golang-jwt/jwt/v5"
)

// OAuthService handles OAuth 2.0 server business logic
type OAuthService struct{}

const (
	oauthTokenPurpose = "oauth_access"
)

// NewOAuthService creates a new OAuth service
func NewOAuthService() *OAuthService {
	return &OAuthService{}
}

// oauthSigningKey generates a signing key for OAuth tokens
func oauthSigningKey() []byte {
	mac := hmac.New(sha256.New, []byte(common.SessionSecret))
	_, _ = mac.Write([]byte("new-api/oauth/" + oauthTokenPurpose + "/v1"))
	return mac.Sum(nil)
}

// ValidateClientCredentials validates client_id and client_secret
func (os *OAuthService) ValidateClientCredentials(clientID, clientSecret string) (*model.OAuthApplication, error) {
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("client_id and client_secret are required")
	}

	app, err := model.GetOAuthApplicationByClientID(clientID)
	if err != nil {
		return nil, fmt.Errorf("invalid client_id: %w", err)
	}

	if !app.ValidateClientSecret(clientSecret) {
		return nil, fmt.Errorf("invalid client_secret")
	}

	return app, nil
}

// ValidateRedirectURI validates that the redirect URI matches the registered URI
func (os *OAuthService) ValidateRedirectURI(app *model.OAuthApplication, redirectURI string) error {
	if !app.IsRedirectURIAllowed(redirectURI) {
		return fmt.Errorf("invalid redirect_uri")
	}
	return nil
}

// GenerateAuthorizationCode generates and stores an authorization code
func (os *OAuthService) GenerateAuthorizationCode(clientID string, userID int, redirectURI, scope string) (string, error) {
	code, err := model.GenerateAuthorizationCode()
	if err != nil {
		return "", fmt.Errorf("failed to generate authorization code: %w", err)
	}

	authCode := &model.OAuthAuthorizationCode{
		Code:        code,
		ClientID:    clientID,
		UserID:      userID,
		RedirectURI: redirectURI,
		Scope:       scope,
	}

	if err := model.CreateAuthorizationCode(authCode); err != nil {
		return "", fmt.Errorf("failed to store authorization code: %w", err)
	}

	return code, nil
}

// ExchangeAuthorizationCodeForToken exchanges an authorization code for an access token
func (os *OAuthService) ExchangeAuthorizationCodeForToken(code, clientID, redirectURI string) (string, error) {
	// Validate and consume the authorization code
	authCode, err := model.GetAuthorizationCode(code)
	if err != nil {
		return "", fmt.Errorf("invalid authorization code: %w", err)
	}

	// Verify client_id and redirect_uri match
	if authCode.ClientID != clientID {
		return "", fmt.Errorf("client_id mismatch")
	}

	if authCode.RedirectURI != redirectURI {
		return "", fmt.Errorf("redirect_uri mismatch")
	}

	// Mark the authorization code as consumed
	if err := model.ValidateAndConsumeAuthorizationCode(code, clientID, redirectURI, authCode.UserID); err != nil {
		return "", fmt.Errorf("failed to consume authorization code: %w", err)
	}

	// Generate JWT access token
	accessToken, err := os.GenerateAccessToken(authCode.UserID, clientID, authCode.Scope)
	if err != nil {
		return "", fmt.Errorf("failed to generate access token: %w", err)
	}

	return accessToken, nil
}

// OAuthTokenClaims represents OAuth token claims
type OAuthTokenClaims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	ClientID string `json:"client_id"`
	Scope    string `json:"scope"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

// GenerateAccessToken generates a JWT access token
func (os *OAuthService) GenerateAccessToken(userID int, clientID, scope string) (string, error) {
	// Get user info
	user, err := model.GetUserById(userID, false)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}

	// Generate JWT token with 1 hour expiry for access tokens
	now := time.Now()
	expiry := now.Add(1 * time.Hour)

	claims := OAuthTokenClaims{
		UserID:    user.Id,
		Username:  user.Username,
		Email:     user.Email,
		ClientID:  clientID,
		Scope:     scope,
		TokenType: "oauth_access_token",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:   "new-api",
			Subject:  fmt.Sprintf("%d", user.Id),
			Audience: jwt.ClaimStrings{clientID},
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(oauthSigningKey())
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, nil
}

// GetTokenInfo extracts and validates token information
func (os *OAuthService) GetTokenInfo(tokenString string) (*OAuthTokenClaims, error) {
	// Parse JWT token
	token, err := jwt.ParseWithClaims(tokenString, &OAuthTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return oauthSigningKey(), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*OAuthTokenClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Verify token type
	if claims.TokenType != "oauth_access_token" {
		return nil, fmt.Errorf("invalid token type")
	}

	return claims, nil
}

// ValidateScope checks if the requested scope is valid
func (os *OAuthService) ValidateScope(app *model.OAuthApplication, requestedScope string) (bool, error) {
	if requestedScope == "" {
		return true, nil // scope is optional
	}

	var allowedScopes []string
	if err := common.UnmarshalJsonStr(app.Scopes, &allowedScopes); err != nil {
		// If no scopes configured, treat as allowing any scope
		return true, nil
	}

	// Check if requested scope is in allowed list
	for _, scope := range allowedScopes {
		if scope == requestedScope {
			return true, nil
		}
	}

	return false, fmt.Errorf("requested scope not allowed")
}

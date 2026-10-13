package model

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"gorm.io/gorm"
)

const (
	// AuthorizationCodeTTL defines how long an authorization code is valid (10 minutes)
	AuthorizationCodeTTL = 10 * time.Minute
)

// OAuthAuthorizationCode represents an authorization code used in OAuth 2.0 Authorization Code Flow
type OAuthAuthorizationCode struct {
	Code         string    `json:"code" gorm:"type:varchar(128);primaryKey"`
	ClientID     string    `json:"client_id" gorm:"type:varchar(64);not null;index:idx_client_expires"`
	UserID       int       `json:"user_id" gorm:"not null;index"`
	RedirectURI  string    `json:"redirect_uri" gorm:"type:varchar(512);not null"`
	Scope        string    `json:"scope" gorm:"type:varchar(255)"`
	ExpiresAt    time.Time `json:"expires_at" gorm:"not null;index:idx_client_expires"`
	CreatedAt    time.Time `json:"created_at"`
	ConsumedAt   *time.Time `json:"-"` // Tracks when the code was consumed
}

func (OAuthAuthorizationCode) TableName() string {
	return "oauth_authorization_codes"
}

var (
	ErrAuthorizationCodeNotFound = errors.New("authorization code not found")
	ErrAuthorizationCodeExpired  = errors.New("authorization code has expired")
	ErrAuthorizationCodeUsed     = errors.New("authorization code has already been used")
)

// GenerateAuthorizationCode generates a random 64-byte authorization code
func GenerateAuthorizationCode() (string, error) {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Use URL-safe base64 encoding for the code
	code := base64.URLEncoding.EncodeToString(b)
	return code, nil
}

// CreateAuthorizationCode stores a new authorization code in the database
func CreateAuthorizationCode(code *OAuthAuthorizationCode) error {
	if code.Code == "" {
		return errors.New("code is required")
	}
	if code.ClientID == "" {
		return errors.New("client_id is required")
	}
	if code.UserID == 0 {
		return errors.New("user_id is required")
	}

	code.CreatedAt = time.Now()
	code.ExpiresAt = time.Now().Add(AuthorizationCodeTTL)

	return DB.Create(code).Error
}

// GetAuthorizationCode retrieves an authorization code by its value
func GetAuthorizationCode(codeValue string) (*OAuthAuthorizationCode, error) {
	var code OAuthAuthorizationCode

	err := DB.Where("code = ?", codeValue).First(&code).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAuthorizationCodeNotFound
		}
		return nil, err
	}

	// Check if code has expired
	if time.Now().After(code.ExpiresAt) {
		return nil, ErrAuthorizationCodeExpired
	}

	// Check if code has already been consumed
	if code.ConsumedAt != nil {
		return nil, ErrAuthorizationCodeUsed
	}

	return &code, nil
}

// ValidateAndConsumeAuthorizationCode validates and marks a code as consumed
// This should be done in a transaction to ensure atomicity
func ValidateAndConsumeAuthorizationCode(codeValue, clientID, redirectURI string, userID int) error {
	now := time.Now()

	// Update the code to mark it as consumed in a single atomic operation
	result := DB.Model(&OAuthAuthorizationCode{}).
		Where("code = ? AND client_id = ? AND redirect_uri = ? AND user_id = ? AND consumed_at IS NULL", codeValue, clientID, redirectURI, userID).
		Where("expires_at > ?", now).
		Update("consumed_at", now)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		// Try to get the code to determine which validation failed
		var code OAuthAuthorizationCode
		if err := DB.Where("code = ?", codeValue).First(&code).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAuthorizationCodeNotFound
			}
			return err
		}

		if code.ExpiresAt.Before(now) {
			return ErrAuthorizationCodeExpired
		}
		if code.ConsumedAt != nil {
			return ErrAuthorizationCodeUsed
		}

		// Mismatch in client_id, redirect_uri, or user_id
		return errors.New("authorization code validation failed")
	}

	return nil
}

// CleanupExpiredAuthorizationCodes removes all expired authorization codes
func CleanupExpiredAuthorizationCodes() error {
	return DB.Where("expires_at < ?", time.Now()).Delete(&OAuthAuthorizationCode{}).Error
}

// CleanupConsumedAuthorizationCodes removes consumed codes (can be done async)
func CleanupConsumedAuthorizationCodes() error {
	// Remove consumed codes older than 1 hour
	expiryTime := time.Now().Add(-1 * time.Hour)
	return DB.Where("consumed_at IS NOT NULL AND consumed_at < ?", expiryTime).
		Delete(&OAuthAuthorizationCode{}).Error
}

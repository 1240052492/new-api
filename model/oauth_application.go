package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	// OAuthApplicationStatus
	OAuthApplicationStatusActive   = 1
	OAuthApplicationStatusInactive = 0
)

// OAuthApplication represents an OAuth 2.0 application registered in the system
type OAuthApplication struct {
	ID             int       `json:"id" gorm:"primaryKey"`
	Name           string    `json:"name" gorm:"type:varchar(100);not null"`
	ClientID       string    `json:"client_id" gorm:"type:varchar(64);uniqueIndex;not null"`
	ClientSecret   string    `json:"-" gorm:"type:varchar(128);not null"` // bcrypt hash
	RedirectURIs   string    `json:"redirect_uris" gorm:"type:text;not null"` // JSON array
	Scopes         string    `json:"scopes" gorm:"type:varchar(255)"`
	AutoApprove    bool      `json:"auto_approve" gorm:"default:false"`
	OwnerUserID    int       `json:"owner_user_id" gorm:"index;not null"`
	Status         int       `json:"status" gorm:"default:1"` // 1: active, 0: inactive
	Description    string    `json:"description" gorm:"type:text"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	DeletedAt      *time.Time `json:"-" gorm:"index"`
}

func (OAuthApplication) TableName() string {
	return "oauth_applications"
}

var (
	ErrOAuthApplicationNotFound   = errors.New("oauth application not found")
	ErrOAuthApplicationDuplicate  = errors.New("oauth application with this client_id already exists")
	ErrInvalidRedirectURI         = errors.New("redirect uri not allowed")
	ErrInvalidClientSecret        = errors.New("invalid client secret")
)

// CreateOAuthApplication creates a new OAuth application
func CreateOAuthApplication(app *OAuthApplication, plainSecret string) error {
	if app.ClientID == "" {
		return errors.New("client_id is required")
	}
	if plainSecret == "" {
		return errors.New("client_secret is required")
	}

	// Hash the client secret using bcrypt
	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(plainSecret), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash client secret: %w", err)
	}

	app.ClientSecret = string(hashedSecret)
	app.Status = OAuthApplicationStatusActive

	if err := DB.Create(app).Error; err != nil {
		// Check if it's a unique constraint violation
		if err.Error() == "UNIQUE constraint failed: oauth_applications.client_id" ||
		   err.Error() == "Error 1062: Duplicate entry" ||
		   err.Error() == "duplicate key value violates unique constraint" {
			return ErrOAuthApplicationDuplicate
		}
		return err
	}

	return nil
}

// GetOAuthApplicationByClientID retrieves an application by its client_id
func GetOAuthApplicationByClientID(clientID string) (*OAuthApplication, error) {
	var app OAuthApplication
	err := DB.Where("client_id = ? AND status = ?", clientID, OAuthApplicationStatusActive).
		First(&app).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOAuthApplicationNotFound
		}
		return nil, err
	}

	return &app, nil
}

// GetOAuthApplicationByID retrieves an application by its ID
func GetOAuthApplicationByID(id int) (*OAuthApplication, error) {
	var app OAuthApplication
	err := DB.Where("id = ? AND status = ?", id, OAuthApplicationStatusActive).
		First(&app).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOAuthApplicationNotFound
		}
		return nil, err
	}

	return &app, nil
}

// ValidateClientSecret verifies that the provided secret matches the stored hash
func (app *OAuthApplication) ValidateClientSecret(plainSecret string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(app.ClientSecret), []byte(plainSecret))
	return err == nil
}

// IsRedirectURIAllowed checks if the redirect URI is in the allowed list
func (app *OAuthApplication) IsRedirectURIAllowed(redirectURI string) bool {
	var uris []string
	if err := common.UnmarshalJsonStr(app.RedirectURIs, &uris); err != nil {
		return false
	}

	for _, uri := range uris {
		if uri == redirectURI {
			return true
		}
	}

	return false
}

// GetRedirectURIs parses and returns the redirect URIs
func (app *OAuthApplication) GetRedirectURIs() ([]string, error) {
	var uris []string
	if err := common.UnmarshalJsonStr(app.RedirectURIs, &uris); err != nil {
		return nil, err
	}
	return uris, nil
}

// SetRedirectURIs sets the redirect URIs from a slice
func (app *OAuthApplication) SetRedirectURIs(uris []string) error {
	data, err := common.Marshal(uris)
	if err != nil {
		return err
	}
	app.RedirectURIs = string(data)
	return nil
}

// GetRedirectURI returns the first redirect URI (for compatibility)
func (app *OAuthApplication) GetRedirectURI() string {
	uris, err := app.GetRedirectURIs()
	if err != nil || len(uris) == 0 {
		return ""
	}
	return uris[0]
}

// UpdateOAuthApplication updates an application (except client secret)
func UpdateOAuthApplication(app *OAuthApplication) error {
	return DB.Model(app).
		Select("name", "redirect_uris", "scopes", "auto_approve", "status", "description", "updated_at").
		Updates(app).Error
}

// DeleteOAuthApplication soft-deletes an application
func DeleteOAuthApplication(id int) error {
	return DB.Model(&OAuthApplication{}).Where("id = ?", id).Update("deleted_at", time.Now()).Error
}

// ListOAuthApplications lists applications for a user
func ListOAuthApplications(userID int, offset, limit int) ([]OAuthApplication, int64, error) {
	var apps []OAuthApplication
	var total int64

	query := DB.Where("owner_user_id = ?", userID)

	if err := query.Model(&OAuthApplication{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Offset(offset).Limit(limit).Find(&apps).Error; err != nil {
		return nil, 0, err
	}

	return apps, total, nil
}

package controller

import (
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// Workbench creation guide dialog frequency control. The Canvas frontend asks
// for the user's acknowledgement timestamps and records new ones here, so the
// reminder interval follows the account instead of the browser. Guests never
// call these endpoints; their cadence stays frontend-only.

const (
	// creationGuideReminderDays is how long an acknowledgement suppresses the
	// guide dialog. It is returned to the client so a future operational
	// change takes effect without a frontend release.
	creationGuideReminderDays = 7

	creationGuidePlatformImage = "image"
	creationGuidePlatformVideo = "video"
)

var creationGuidePlatforms = map[string]bool{
	creationGuidePlatformImage: true,
	creationGuidePlatformVideo: true,
}

type creationGuideStatusResponse struct {
	ReminderDays int              `json:"reminder_days"`
	AckedAt      map[string]int64 `json:"acked_at"`
}

type creationGuideAckRequest struct {
	Platform string `json:"platform" binding:"required"`
}

// authenticateOAuthAccessToken resolves the Bearer OAuth access token issued
// by POST /api/oauth/token. It returns nil after writing the error response.
func authenticateOAuthAccessToken(c *gin.Context) *service.OAuthTokenClaims {
	token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
	if !ok || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":             "invalid_token",
			"error_description": "Missing OAuth access token",
		})
		return nil
	}
	claims, err := service.NewOAuthService().GetTokenInfo(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":             "invalid_token",
			"error_description": "Invalid or expired OAuth access token",
		})
		return nil
	}
	return claims
}

func buildCreationGuideStatus(setting dto.UserSetting) creationGuideStatusResponse {
	ackedAt := make(map[string]int64, len(setting.CreationGuideAcks))
	for platform, at := range setting.CreationGuideAcks {
		if creationGuidePlatforms[platform] {
			ackedAt[platform] = at
		}
	}
	return creationGuideStatusResponse{ReminderDays: creationGuideReminderDays, AckedAt: ackedAt}
}

// GetCreationGuideStatus handles GET /api/creation-guide/status
func GetCreationGuideStatus(c *gin.Context) {
	claims := authenticateOAuthAccessToken(c)
	if claims == nil {
		return
	}

	user, err := model.GetUserById(claims.UserID, false)
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":             "invalid_token",
			"error_description": "User no longer exists",
		})
		return
	}

	c.JSON(http.StatusOK, buildCreationGuideStatus(user.GetSetting()))
}

// AckCreationGuide handles POST /api/creation-guide/ack
func AckCreationGuide(c *gin.Context) {
	claims := authenticateOAuthAccessToken(c)
	if claims == nil {
		return
	}

	var req creationGuideAckRequest
	if err := c.ShouldBindJSON(&req); err != nil || !creationGuidePlatforms[req.Platform] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "invalid_request",
			"error_description": "platform must be one of: image, video",
		})
		return
	}

	user, err := model.GetUserById(claims.UserID, false)
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":             "invalid_token",
			"error_description": "User no longer exists",
		})
		return
	}

	setting := user.GetSetting()
	if setting.CreationGuideAcks == nil {
		setting.CreationGuideAcks = make(map[string]int64, 1)
	}
	setting.CreationGuideAcks[req.Platform] = time.Now().Unix()
	if err := model.UpdateUserSetting(claims.UserID, setting); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":             "ack_failed",
			"error_description": "Failed to record the acknowledgement",
		})
		return
	}

	c.JSON(http.StatusOK, buildCreationGuideStatus(setting))
}

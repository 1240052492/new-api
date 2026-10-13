package model

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// InitializeOAuthInfiniteCanvas 初始化 infinite-canvas 的 OAuth 应用配置
// 这是系统启动时自动执行的初始化程序
func InitializeOAuthInfiniteCanvas(db *gorm.DB) error {
	// 检查应用是否已存在
	var count int64
	if err := db.Model(&OAuthApplication{}).Where("client_id = ?", "infinite-canvas-client").Count(&count).Error; err != nil {
		return err
	}

	// 如果应用已存在，跳过初始化
	if count > 0 {
		return nil
	}

	// 创建 infinite-canvas OAuth 应用
	app := &OAuthApplication{
		Name:           "Infinite Canvas",
		ClientID:       "infinite-canvas-client",
		ClientSecret:   "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcg7b3XeKeUxWdeS86E36gZvWFm", // bcrypt 示例值
		RedirectURIs:   `["http://localhost:5173/oauth/callback","http://127.0.0.1:5173/oauth/callback"]`,
		Scopes:         `["user:read"]`,
		AutoApprove:    true,
		OwnerUserID:    1, // 系统管理员
		Status:         OAuthApplicationStatusActive,
		Description:    "Infinite Canvas Drawing Application - Automatically initialized",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := db.Create(app).Error; err != nil {
		return err
	}

	return nil
}

// GetOAuthApplicationByClientID 获取 OAuth 应用（简化版）
// 用于配置页面
func GetOAuthApplicationForDisplay(clientID string) (map[string]interface{}, error) {
	var app OAuthApplication
	if err := DB.Where("client_id = ? AND status = ?", clientID, OAuthApplicationStatusActive).First(&app).Error; err != nil {
		return nil, err
	}

	uris, _ := app.GetRedirectURIs()
	var scopes []string
	if app.Scopes != "" {
		_ = common.UnmarshalJsonStr(app.Scopes, &scopes)
	}

	return map[string]interface{}{
		"id":             app.ID,
		"name":           app.Name,
		"client_id":      app.ClientID,
		"redirect_uris":  uris,
		"scopes":         scopes,
		"auto_approve":   app.AutoApprove,
		"description":    app.Description,
	}, nil
}

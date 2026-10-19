package model

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// ThirdPartyApp 描述一个外部第三方系统入口，展示在顶部导航或侧边栏。
type ThirdPartyApp struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	URL           string `json:"url"`
	ShowInTopNav  bool   `json:"show_in_top_nav"`
	ShowInSidebar bool   `json:"show_in_sidebar"`
	Enabled       bool   `json:"enabled"`
	OAuthClientID string `json:"oauth_client_id,omitempty"`
}

const thirdPartyAppsOptionKey = "ThirdPartyApps"

var thirdPartyAppsMigrateOnce sync.Once

// GetThirdPartyApps 返回第三方应用列表。
// 首次访问时自动把旧的无限画布配置（InfiniteCanvasURL +
// HeaderNavModules.infiniteCanvas + SidebarModulesAdmin.chat.infiniteCanvas）
// 迁移为第一条记录，之后统一由 ThirdPartyApps option 维护。
func GetThirdPartyApps() []ThirdPartyApp {
	thirdPartyAppsMigrateOnce.Do(migrateLegacyInfiniteCanvasApp)
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	apps := parseThirdPartyApps(common.OptionMap[thirdPartyAppsOptionKey])
	if apps == nil {
		return []ThirdPartyApp{}
	}
	return apps
}

func parseThirdPartyApps(raw string) []ThirdPartyApp {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var apps []ThirdPartyApp
	if err := json.Unmarshal([]byte(raw), &apps); err != nil {
		return nil
	}
	return apps
}

func migrateLegacyInfiniteCanvasApp() {
	common.OptionMapRWMutex.RLock()
	rawApps := common.OptionMap[thirdPartyAppsOptionKey]
	rawURL := common.OptionMap["InfiniteCanvasURL"]
	rawHeaderNav := common.OptionMap["HeaderNavModules"]
	rawSidebar := common.OptionMap["SidebarModulesAdmin"]
	common.OptionMapRWMutex.RUnlock()

	if len(parseThirdPartyApps(rawApps)) > 0 {
		return
	}

	url := strings.TrimSpace(rawURL)
	if url == "" {
		url = "http://localhost:5173/"
	}

	showInTopNav := true
	var headerNav map[string]any
	if raw := strings.TrimSpace(rawHeaderNav); raw != "" {
		if err := json.Unmarshal([]byte(raw), &headerNav); err == nil {
			if v, ok := headerNav["infiniteCanvas"].(bool); ok {
				showInTopNav = v
			}
		}
	}

	showInSidebar := true
	if raw := strings.TrimSpace(rawSidebar); raw != "" {
		var sidebar struct {
			Chat struct {
				InfiniteCanvas *bool `json:"infiniteCanvas"`
			} `json:"chat"`
		}
		if err := json.Unmarshal([]byte(raw), &sidebar); err == nil && sidebar.Chat.InfiniteCanvas != nil {
			showInSidebar = *sidebar.Chat.InfiniteCanvas
		}
	}

	entry := ThirdPartyApp{
		ID:            "infinite-canvas",
		Name:          "Infinite Canvas",
		URL:           url,
		ShowInTopNav:  showInTopNav,
		ShowInSidebar: showInSidebar,
		Enabled:       true,
	}
	payload, err := json.Marshal([]ThirdPartyApp{entry})
	if err != nil {
		return
	}
	if err := UpdateOption(thirdPartyAppsOptionKey, string(payload)); err != nil {
		common.SysError(common.LogText("failed to migrate legacy Infinite Canvas config into ThirdPartyApps: %s", err.Error()))
	}
}

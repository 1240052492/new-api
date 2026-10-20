package middleware

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowCredentials = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"*"}
	return cors.New(config)
}

// CORSForOAuthClients answers credentialed CORS requests for the companion
// apps registered as OAuth applications.
//
// Browsers reject a wildcard Allow-Origin on credentialed requests, so the
// request Origin has to be reflected. Reflecting any origin would let any site
// on the internet call a cookie-authenticated endpoint on the visitor's
// behalf, so the reflection is limited to the origins the console has
// explicitly trusted: the request's own origin, the SESSION_COOKIE_TRUSTED_URL
// list, and the redirect URIs of active OAuth applications.
func CORSForOAuthClients() gin.HandlerFunc {
	return func(c *gin.Context) {
		if origin := c.GetHeader("Origin"); origin != "" && isTrustedClientOrigin(c.Request, origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func isTrustedClientOrigin(request *http.Request, origin string) bool {
	normalized, err := common.NormalizeOrigin(origin)
	if err != nil {
		return false
	}
	if isAllowedSessionOrigin(request, normalized) {
		return true
	}
	redirectURIs, err := model.ListActiveOAuthApplicationRedirectURIs()
	if err != nil {
		return false
	}
	for _, redirectURI := range redirectURIs {
		parsed, err := url.Parse(strings.TrimSpace(redirectURI))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
			continue
		}
		redirectOrigin, err := common.NormalizeOrigin(parsed.Scheme + "://" + parsed.Host)
		if err != nil {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(redirectOrigin), []byte(normalized)) == 1 {
			return true
		}
	}
	return false
}

func Version() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-New-Api-Version", common.Version)
		c.Next()
	}
}

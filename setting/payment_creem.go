package setting

// CreemEnabled is the explicit operator switch for the user-facing Creem
// channel. Defaults to true so existing deployments keep their current
// behavior (credentials + compliance gate visibility).
var CreemEnabled = false

var CreemApiKey = ""
var CreemProducts = "[]"
var CreemTestMode = false
var CreemWebhookSecret = ""

package setting

// StripeEnabled is the explicit operator switch for the user-facing Stripe
// channel. Defaults to true so existing deployments keep their current
// behavior (credentials + compliance gate visibility); operators can turn it
// off to hide Stripe without wiping credentials.
var StripeEnabled = true

var StripeApiSecret = ""
var StripeWebhookSecret = ""
var StripePriceId = ""
var StripeUnitPrice = 8.0
var StripeMinTopUp = 1
var StripePromotionCodesEnabled = false

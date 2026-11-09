package setting

// Waffo Pancake hosted checkout configuration. StoreID + ProductID are
// operator-bound via SaveWaffoPancakeConfig. WaffoPancakeEnabled is the
// explicit operator switch for the user-facing channel; it defaults to true
// so existing deployments keep their current behavior
// (credentials + compliance gate visibility).
var (
	WaffoPancakeEnabled    = false
	WaffoPancakeMerchantID string
	WaffoPancakePrivateKey string
	WaffoPancakeReturnURL  string
	WaffoPancakeUnitPrice  float64 = 1.0
	WaffoPancakeMinTopUp   float64 = 1
	WaffoPancakeStoreID    string
	WaffoPancakeProductID  string
)

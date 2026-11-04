package setting

// Alipay direct gateway configuration, integrated against the official
// Alipay OpenAPI ("电脑网站支付 alipay.trade.page.pay" and
// "手机网站支付 alipay.trade.wap.pay" products, RSA2 signing).
//
// Enabled is an explicit operator switch: configuring credentials alone must
// never expose the channel to end users — a typo in a key would otherwise
// break live top-up. Channel is user-visible only when Enabled is true and
// every required credential is present.
//
// Reference (official docs):
//   - 签名与验签: https://opendocs.alipay.com/common/02kipl
//   - 电脑网站支付: https://opendocs.alipay.com/open/270/105899
//   - 手机网站支付: https://opendocs.alipay.com/open/02ivbs
var (
	AlipayEnabled       bool
	AlipayAppID         string
	AlipayAppPrivateKey string // 应用私钥 PKCS8，仅用于签名请求
	AlipayPublicKey     string // 支付宝公钥，用于验签异步通知与同步响应
	AlipayGateway       string // 官方网关，默认 https://openapi.alipay.com/gateway.do
	AlipayNotifyURL     string // 异步通知地址，留空则基于系统回调地址自动拼接
	AlipayReturnURL     string // 同步返回地址，留空则基于系统地址自动拼接
)

const DefaultAlipayGateway = "https://openapi.alipay.com/gateway.do"

func GetAlipayGateway() string {
	if AlipayGateway == "" {
		return DefaultAlipayGateway
	}
	return AlipayGateway
}

package setting

// WeChat Pay APIv3 direct gateway configuration, integrated against the
// official WeChat Pay merchant docs ("Native支付" and "H5支付" products).
//
// Enabled is an explicit operator switch: configuring credentials alone must
// never expose the channel to end users — a wrong APIv3 key or serial number
// would otherwise break live top-up.
//
// Signature verification follows the official 公钥/平台证书模式: callbacks are
// verified with the WeChat Pay platform public key identified by
// WechatPayPlatformPublicKeyID (format PUB_KEY_ID_xxx). Callback payloads are
// decrypted with the 32-byte APIv3 key using AEAD_AES_256_GCM.
//
// Reference (official docs):
//   - 签名生成/验证: https://pay.weixin.qq.com/doc/v3/merchant/4012365342
//   - Native下单: https://pay.weixin.qq.com/doc/v3/merchant/4012791873
//   - 支付通知: https://pay.weixin.qq.com/doc/v3/merchant/4012791861
var (
	WechatPayEnabled             bool
	WechatPayMchID               string
	WechatPayAppID               string
	WechatPaySerialNo            string // 商户 API 证书序列号
	WechatPayPrivateKey          string // 商户私钥 apiclient_key.pem 内容
	WechatPayAPIv3Key            string // 32 字节 APIv3 密钥，用于回调解密
	WechatPayPlatformPublicKey   string // 微信支付公钥，用于回调验签
	WechatPayPlatformPublicKeyID string // 微信支付公钥 ID，形如 PUB_KEY_ID_xxx
	WechatPayNotifyURL           string // 支付通知地址，留空则基于系统回调地址自动拼接
	WechatPayAPIBase             string // APIv3 网关地址，留空使用官方默认；本地联调可指向模拟网关
)

const DefaultWechatPayAPIBase = "https://api.mch.weixin.qq.com"

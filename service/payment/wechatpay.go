package payment

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WechatPayClient speaks the official WeChat Pay APIv3 protocol: request
// authorization via WECHATPAY2-SHA256-RSA2048, response and callback
// verification against the WeChat Pay platform public key, and callback
// payload decryption via AEAD_AES_256_GCM with the 32-byte APIv3 key.
type WechatPayClient struct {
	MchID               string
	AppID               string
	SerialNo            string
	PrivateKey          *rsa.PrivateKey
	APIv3Key            []byte
	PlatformPublicKey   *rsa.PublicKey
	PlatformPublicKeyID string
	NotifyURL           string
	APIBase             string
	HTTPClient          *http.Client
}

const wechatPayAuthorizationScheme = "WECHATPAY2-SHA256-RSA2048"

const defaultWechatPayAPIBase = "https://api.mch.weixin.qq.com"

// WechatPayTransaction mirrors the payment-order fields returned by the
// query-order API that the billing flow relies on.
type WechatPayTransaction struct {
	OutTradeNo    string `json:"out_trade_no"`
	TransactionID string `json:"transaction_id"`
	TradeState    string `json:"trade_state"`
	TradeType     string `json:"trade_type"`
	Amount        struct {
		Total    int64  `json:"total"`
		Currency string `json:"currency"`
	} `json:"amount"`
}

const WechatPayStateSuccess = "SUCCESS"

func NewWechatPayClient(mchID string, appID string, serialNo string, privateKeyPEM string, apiv3Key string, platformPublicKey string, platformPublicKeyID string, notifyURL string) (*WechatPayClient, error) {
	privateKey, err := loadRSAPrivateKey(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("invalid WeChat Pay merchant private key: %w", err)
	}
	publicKey, err := loadRSAPublicKey(platformPublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid WeChat Pay platform public key: %w", err)
	}
	key := []byte(strings.TrimSpace(apiv3Key))
	if len(key) != 32 {
		return nil, errors.New("WeChat Pay APIv3 key must be exactly 32 bytes")
	}
	return &WechatPayClient{
		MchID:               mchID,
		AppID:               appID,
		SerialNo:            serialNo,
		PrivateKey:          privateKey,
		APIv3Key:            key,
		PlatformPublicKey:   publicKey,
		PlatformPublicKeyID: platformPublicKeyID,
		NotifyURL:           notifyURL,
		APIBase:             defaultWechatPayAPIBase,
		HTTPClient:          &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func randomNonce() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.StdEncoding.EncodeToString(buf)
}

// signMessage implements the official signature construction: the message is
// `METHOD\nURL\nTIMESTAMP\nNONCE\nBODY\n` (request) or
// `TIMESTAMP\nNONCE\nBODY\n` (response/callback verification), signed with
// SHA256 with RSA.
func (c *WechatPayClient) signMessage(message []byte) (string, error) {
	digest := sha256.Sum256(message)
	signature, err := rsa.SignPKCS1v15(rand.Reader, c.PrivateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign WeChat Pay request: %w", err)
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func (c *WechatPayClient) verifyMessage(message []byte, signature string) error {
	raw, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return errors.New("WeChat Pay signature is not valid base64")
	}
	digest := sha256.Sum256(message)
	return rsa.VerifyPKCS1v15(c.PlatformPublicKey, crypto.SHA256, digest[:], raw)
}

type wechatPayAPIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e wechatPayAPIError) Error() string {
	return fmt.Sprintf("WeChat Pay API error code=%s message=%s", e.Code, e.Message)
}

// do performs a signed APIv3 request and verifies the response signature over
// the raw response body, per the official 签名验证 documentation.
func (c *WechatPayClient) do(method string, path string, body []byte, result any) error {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := randomNonce()
	url := c.APIBase + path

	message := method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + string(body) + "\n"
	signature, err := c.signMessage([]byte(message))
	if err != nil {
		return err
	}
	authorization := fmt.Sprintf(
		`%s mchid="%s",nonce_str="%s",signature="%s",timestamp="%s",serial_no="%s"`,
		wechatPayAuthorizationScheme, c.MchID, nonce, signature, timestamp, c.SerialNo,
	)

	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "new-api")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("WeChat Pay API request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("failed to read WeChat Pay response: %w", err)
	}

	respTimestamp := resp.Header.Get("Wechatpay-Timestamp")
	respNonce := resp.Header.Get("Wechatpay-Nonce")
	respSignature := resp.Header.Get("Wechatpay-Signature")
	respSerial := resp.Header.Get("Wechatpay-Serial")
	if respSignature == "" || respSerial == "" {
		return errors.New("WeChat Pay response is missing signature headers")
	}
	// 官方要求校验序列号：公钥模式下序列号为 PUB_KEY_ID_xxx，必须与配置一致，
	// 否则视为不可信响应。
	if respSerial != c.PlatformPublicKeyID {
		return fmt.Errorf("WeChat Pay response serial %q does not match configured platform public key id", respSerial)
	}
	respMessage := respTimestamp + "\n" + respNonce + "\n" + string(respBody) + "\n"
	if err := c.verifyMessage([]byte(respMessage), respSignature); err != nil {
		return fmt.Errorf("WeChat Pay response signature verification failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr wechatPayAPIError
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Code != "" {
			return apiErr
		}
		return fmt.Errorf("WeChat Pay API returned http %d", resp.StatusCode)
	}
	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to parse WeChat Pay response: %w", err)
		}
	}
	return nil
}

type wechatPayAmount struct {
	Total    int64  `json:"total"`
	Currency string `json:"currency"`
}

type nativeOrderResult struct {
	CodeURL string `json:"code_url"`
}

type h5OrderResult struct {
	H5URL string `json:"h5_url"`
}

// NativeOrder places a Native order (PC scan-to-pay) and returns code_url,
// which the frontend renders as a QR code. amount.total is in cents (分).
func (c *WechatPayClient) NativeOrder(outTradeNo string, description string, totalCents int64, timeExpire string) (string, error) {
	body := map[string]any{
		"appid":        c.AppID,
		"mchid":        c.MchID,
		"description":  description,
		"out_trade_no": outTradeNo,
		"notify_url":   c.NotifyURL,
		"amount":       wechatPayAmount{Total: totalCents, Currency: "CNY"},
	}
	if timeExpire != "" {
		body["time_expire"] = timeExpire
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	var result nativeOrderResult
	if err := c.do(http.MethodPost, "/v3/pay/transactions/native", encoded, &result); err != nil {
		return "", err
	}
	if result.CodeURL == "" {
		return "", errors.New("WeChat Pay Native order returned empty code_url")
	}
	return result.CodeURL, nil
}

// H5Order places an H5 order (mobile browser outside WeChat) and returns
// h5_url. scene_info.payer_client_ip is required by the official H5 order
// API.
func (c *WechatPayClient) H5Order(outTradeNo string, description string, totalCents int64, payerClientIP string) (string, error) {
	body := map[string]any{
		"appid":        c.AppID,
		"mchid":        c.MchID,
		"description":  description,
		"out_trade_no": outTradeNo,
		"notify_url":   c.NotifyURL,
		"amount":       wechatPayAmount{Total: totalCents, Currency: "CNY"},
		"scene_info": map[string]any{
			"payer_client_ip": payerClientIP,
		},
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	var result h5OrderResult
	if err := c.do(http.MethodPost, "/v3/pay/transactions/h5", encoded, &result); err != nil {
		return "", err
	}
	if result.H5URL == "" {
		return "", errors.New("WeChat Pay H5 order returned empty h5_url")
	}
	return result.H5URL, nil
}

// QueryOrder fetches the order state by merchant order number.
func (c *WechatPayClient) QueryOrder(outTradeNo string) (*WechatPayTransaction, error) {
	path := "/v3/pay/transactions/out-trade-no/" + outTradeNo + "?mchid=" + c.MchID
	var tx WechatPayTransaction
	if err := c.do(http.MethodGet, path, nil, &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

// CloseOrder closes a timed-out order so it can no longer be paid.
func (c *WechatPayClient) CloseOrder(outTradeNo string) error {
	path := "/v3/pay/transactions/out-trade-no/" + outTradeNo + "/close"
	body, err := json.Marshal(map[string]string{"mchid": c.MchID})
	if err != nil {
		return err
	}
	return c.do(http.MethodPost, path, body, nil)
}

// WechatPayNotifyResource is the decrypted callback payload.
type WechatPayNotifyResource struct {
	OriginalType   string `json:"original_type"`
	Algorithm      string `json:"algorithm"`
	Ciphertext     string `json:"ciphertext"`
	AssociatedData string `json:"associated_data"`
	Nonce          string `json:"nonce"`
}

type wechatPayNotifyEnvelope struct {
	ResourceType string                  `json:"resource_type"`
	EventType    string                  `json:"event_type"`
	Summary      string                  `json:"summary"`
	Resource     WechatPayNotifyResource `json:"resource"`
}

// DecryptNotify verifies the callback signature over `timestamp\nnonce\nbody\n`
// using the WeChat Pay platform public key, enforces the serial number, and
// decrypts resource.ciphertext with AEAD_AES_256_GCM per the official
// 支付通知 documentation. The returned envelope carries the decrypted
// transaction JSON in Plain.
func (c *WechatPayClient) DecryptNotify(timestamp string, nonce string, serial string, signature string, rawBody []byte) (*wechatPayNotifyEnvelope, json.RawMessage, error) {
	if serial != c.PlatformPublicKeyID {
		return nil, nil, fmt.Errorf("WeChat Pay notify serial %q does not match configured platform public key id", serial)
	}
	message := timestamp + "\n" + nonce + "\n" + string(rawBody) + "\n"
	if err := c.verifyMessage([]byte(message), signature); err != nil {
		return nil, nil, fmt.Errorf("WeChat Pay notify signature verification failed: %w", err)
	}

	var envelope wechatPayNotifyEnvelope
	if err := json.Unmarshal(rawBody, &envelope); err != nil {
		return nil, nil, fmt.Errorf("WeChat Pay notify body is not JSON: %w", err)
	}
	if envelope.Resource.Algorithm != "AEAD_AES_256_GCM" {
		return nil, nil, fmt.Errorf("unsupported WeChat Pay notify algorithm %q", envelope.Resource.Algorithm)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(envelope.Resource.Ciphertext)
	if err != nil {
		return nil, nil, fmt.Errorf("WeChat Pay notify ciphertext is not valid base64: %w", err)
	}
	block, err := aes.NewCipher(c.APIv3Key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonceBytes := []byte(envelope.Resource.Nonce)
	if len(nonceBytes) != 12 {
		return nil, nil, errors.New("WeChat Pay notify nonce must be 12 bytes")
	}
	plain, err := gcm.Open(nil, nonceBytes, ciphertext, []byte(envelope.Resource.AssociatedData))
	if err != nil {
		return nil, nil, errors.New("failed to decrypt WeChat Pay notify resource, check the APIv3 key")
	}
	return &envelope, json.RawMessage(plain), nil
}

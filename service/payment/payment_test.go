package payment

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func generateTestKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate rsa key: %v", err)
	}
	pub, ok := key.Public().(*rsa.PublicKey)
	if !ok {
		t.Fatalf("public key type assertion failed")
	}
	return key, pub
}

func encodeTestKeyPair(t *testing.T, key *rsa.PrivateKey) (privatePEM string, publicBase64 string) {
	t.Helper()
	der := x509MarshalPKCS8(t, key)
	privatePEM = "-----BEGIN PRIVATE KEY-----\n" + base64.StdEncoding.EncodeToString(der) + "\n-----END PRIVATE KEY-----\n"
	pubDer := x509MarshalPKIX(t, &key.PublicKey)
	publicBase64 = base64.StdEncoding.EncodeToString(pubDer)
	return privatePEM, publicBase64
}

func TestAlipaySignAndNotifyVerifyRoundTrip(t *testing.T) {
	key, _ := generateTestKeyPair(t)
	privatePEM, publicBase64 := encodeTestKeyPair(t, key)

	client, err := NewAlipayClient("2021000000000000", privatePEM, publicBase64, "https://openapi.alipay.com/gateway.do", "https://example.com/notify", "https://example.com/return")
	if err != nil {
		t.Fatalf("failed to build client: %v", err)
	}

	// Build a signed notify payload the way Alipay does: all params signed
	// except sign and sign_type.
	params := map[string]string{
		"app_id":        "2021000000000000",
		"trade_status":  "TRADE_SUCCESS",
		"out_trade_no":  "ALIUSR1NOabc123456",
		"total_amount":  "10.00",
		"sign_type":     "RSA2",
		"charset":       "utf-8",
		"notify_id":     "202610310022",
		"empty_ignored": "",
	}
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || k == "sign_type" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	// sort keys
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+params[k])
	}
	message := []byte(strings.Join(pairs, "&"))
	digest := sha256.Sum256(message)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}
	params["sign"] = base64.StdEncoding.EncodeToString(sig)

	if err := client.VerifyNotifyParams(params); err != nil {
		t.Fatalf("expected notify verification to succeed, got %v", err)
	}

	// Tampering with any signed field must fail verification.
	params["total_amount"] = "0.01"
	if err := client.VerifyNotifyParams(params); err == nil {
		t.Fatalf("expected tampered notify to fail verification")
	}
	params["total_amount"] = "10.00"

	// A non-RSA2 sign_type must be rejected outright.
	params["sign_type"] = "RSA"
	if err := client.VerifyNotifyParams(params); err == nil {
		t.Fatalf("expected non-RSA2 sign_type to be rejected")
	}
}

func TestLoadKeysFromBareBase64(t *testing.T) {
	key, _ := generateTestKeyPair(t)
	_, publicBase64 := encodeTestKeyPair(t, key)

	// 支付宝公钥是纯 base64（无 PEM 头），必须可解析。
	if _, err := loadRSAPublicKey(publicBase64); err != nil {
		t.Fatalf("expected bare base64 public key to load, got %v", err)
	}

	// 应用私钥常为纯 base64 PKCS8（无 PEM 头），必须可解析。
	der := x509MarshalPKCS8(t, key)
	if _, err := loadRSAPrivateKey(base64.StdEncoding.EncodeToString(der)); err != nil {
		t.Fatalf("expected bare base64 private key to load, got %v", err)
	}
}

func TestWechatPayNotifyVerifyAndDecrypt(t *testing.T) {
	merchantKey, _ := generateTestKeyPair(t)
	platformKey, _ := generateTestKeyPair(t)

	merchantPEM, _ := encodeTestKeyPair(t, merchantKey)
	_, platformPublicBase64 := encodeTestKeyPair(t, platformKey)

	apiv3Key := "0123456789abcdef0123456789abcdef"
	client, err := NewWechatPayClient("1900000001", "wx1234567890", "SERIAL123", merchantPEM, apiv3Key, platformPublicBase64, "PUB_KEY_ID_3000000001", "https://example.com/notify")
	if err != nil {
		t.Fatalf("failed to build client: %v", err)
	}

	// Encrypt a transaction payload the way WeChat Pay does.
	plain := []byte(`{"out_trade_no":"WXUSR1NOabc123456","transaction_id":"1000","trade_state":"SUCCESS","amount":{"total":1000,"currency":"CNY"}}`)
	// Real WeChat Pay nonces are 12 ASCII bytes; binary bytes would not
	// survive the JSON round-trip in a string field.
	nonce := []byte("aBcDeFgHiJkL")
	block, err := aes.NewCipher(client.APIv3Key)
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("failed to create gcm: %v", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plain, []byte("transaction"))

	resource := map[string]any{
		"original_type":   "transaction",
		"algorithm":       "AEAD_AES_256_GCM",
		"ciphertext":      base64.StdEncoding.EncodeToString(ciphertext),
		"associated_data": "transaction",
		"nonce":           string(nonce),
	}
	envelope := map[string]any{
		"id":            "EV-1",
		"create_time":   "2026-10-31T12:00:00+08:00",
		"resource_type": "encrypt-resource",
		"event_type":    "TRANSACTION.SUCCESS",
		"summary":       "支付成功",
		"resource":      resource,
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("failed to marshal envelope: %v", err)
	}

	// Sign the callback body the way WeChat Pay does: timestamp\nnonce\nbody\n
	ts := "1761879999"
	nonceStr := "noncenoncen"
	message := ts + "\n" + nonceStr + "\n" + string(body) + "\n"
	digest := sha256.Sum256([]byte(message))
	sig, err := rsa.SignPKCS1v15(rand.Reader, platformKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}
	signature := base64.StdEncoding.EncodeToString(sig)

	resultEnvelope, plainResource, err := client.DecryptNotify(ts, nonceStr, "PUB_KEY_ID_3000000001", signature, body)
	if err != nil {
		t.Fatalf("expected notify to verify and decrypt, got %v", err)
	}
	if resultEnvelope.EventType != "TRANSACTION.SUCCESS" {
		t.Fatalf("unexpected event type %q", resultEnvelope.EventType)
	}
	var parsed struct {
		OutTradeNo string `json:"out_trade_no"`
		TradeState string `json:"trade_state"`
	}
	if err := json.Unmarshal(plainResource, &parsed); err != nil {
		t.Fatalf("failed to parse decrypted resource: %v", err)
	}
	if parsed.OutTradeNo != "WXUSR1NOabc123456" || parsed.TradeState != "SUCCESS" {
		t.Fatalf("unexpected decrypted payload: %+v", parsed)
	}

	// Wrong serial must be rejected before any parsing.
	if _, _, err := client.DecryptNotify(ts, nonceStr, "PUB_KEY_ID_9999999999", signature, body); err == nil {
		t.Fatalf("expected serial mismatch to be rejected")
	}

	// Tampered body must fail signature verification.
	if _, _, err := client.DecryptNotify(ts, nonceStr, "PUB_KEY_ID_3000000001", signature, append(body, byte(' '))); err == nil {
		t.Fatalf("expected tampered body to fail verification")
	}

	// Wrong APIv3 key must fail decryption.
	wrongKeyClient, err := NewWechatPayClient("1900000001", "wx1234567890", "SERIAL123", merchantPEM, "ffffffffffffffffffffffffffffffff", platformPublicBase64, "PUB_KEY_ID_3000000001", "https://example.com/notify")
	if err != nil {
		t.Fatalf("failed to build wrong-key client: %v", err)
	}
	if _, _, err := wrongKeyClient.DecryptNotify(ts, nonceStr, "PUB_KEY_ID_3000000001", signature, body); err == nil {
		t.Fatalf("expected wrong APIv3 key to fail decryption")
	}
}

func TestAlipayPagePayURLIsSigned(t *testing.T) {
	key, _ := generateTestKeyPair(t)
	privatePEM, publicBase64 := encodeTestKeyPair(t, key)

	client, err := NewAlipayClient("2021000000000000", privatePEM, publicBase64, "https://openapi.alipay.com/gateway.do", "https://example.com/notify", "https://example.com/return")
	if err != nil {
		t.Fatalf("failed to build client: %v", err)
	}
	payURL, err := client.PagePayURL("ALIUSR1NOabc123456", "10.00", "Top-up 10")
	if err != nil {
		t.Fatalf("failed to build pay url: %v", err)
	}
	if !strings.HasPrefix(payURL, "https://openapi.alipay.com/gateway.do?") {
		t.Fatalf("unexpected pay url prefix: %s", payURL)
	}
	parsed, err := url.Parse(payURL)
	if err != nil {
		t.Fatalf("failed to parse pay url: %v", err)
	}
	query := parsed.Query()
	for _, required := range []string{"app_id", "method", "sign", "sign_type", "timestamp", "biz_content", "notify_url", "return_url"} {
		if query.Get(required) == "" {
			t.Fatalf("pay url missing required param %q", required)
		}
	}
	if query.Get("method") != "alipay.trade.page.pay" {
		t.Fatalf("unexpected method %q", query.Get("method"))
	}
	if query.Get("sign_type") != "RSA2" {
		t.Fatalf("unexpected sign_type %q", query.Get("sign_type"))
	}
}

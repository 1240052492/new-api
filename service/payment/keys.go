// Package payment implements direct-gateway clients for Alipay OpenAPI and
// WeChat Pay APIv3, following the official integration documents:
//   - Alipay: https://opendocs.alipay.com/common/02kipl (签名与验签),
//     https://opendocs.alipay.com/open/270/105899 (电脑网站支付),
//     https://opendocs.alipay.com/open/02ivbs (手机网站支付)
//   - WeChat Pay: https://pay.weixin.qq.com/doc/v3/merchant/4012365342 (签名生成/验证),
//     https://pay.weixin.qq.com/doc/v3/merchant/4012791873 (Native下单),
//     https://pay.weixin.qq.com/doc/v3/merchant/4012791861 (支付通知)
package payment

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

// loadRSAPrivateKey parses an RSA private key for request signing. It accepts
// PEM blocks (PKCS#8 or PKCS#1) as well as the bare base64 body produced by
// the Alipay key generation tool, which ships without PEM headers.
func loadRSAPrivateKey(raw string) (*rsa.PrivateKey, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("private key is empty")
	}

	var der []byte
	if block, _ := pem.Decode([]byte(trimmed)); block != nil {
		der = block.Bytes
	} else {
		decoded, err := base64.StdEncoding.DecodeString(stripKeyWhitespace(trimmed))
		if err != nil {
			return nil, fmt.Errorf("private key is neither PEM nor valid base64: %w", err)
		}
		der = decoded
	}

	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key is not an RSA key")
		}
		return rsaKey, nil
	}
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	return nil, errors.New("unsupported private key format, expected PKCS#8 or PKCS#1")
}

// loadRSAPublicKey parses an RSA public key used to verify gateway
// signatures. Alipay ships the platform public key as a bare base64 X.509
// PKIX blob; WeChat Pay's platform public key arrives as PEM or base64.
func loadRSAPublicKey(raw string) (*rsa.PublicKey, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("public key is empty")
	}

	var der []byte
	if block, _ := pem.Decode([]byte(trimmed)); block != nil {
		der = block.Bytes
	} else {
		decoded, err := base64.StdEncoding.DecodeString(stripKeyWhitespace(trimmed))
		if err != nil {
			return nil, fmt.Errorf("public key is neither PEM nor valid base64: %w", err)
		}
		der = decoded
	}

	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("unsupported public key format: %w", err)
	}
	rsaKey, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not an RSA key")
	}
	return rsaKey, nil
}

// stripKeyWhitespace removes newlines and spaces from bare base64 key bodies,
// matching how operators paste keys from the platforms' consoles.
func stripKeyWhitespace(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		switch r {
		case '\n', '\r', ' ', '\t':
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

package payment

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// AlipayClient calls the official Alipay OpenAPI gateway with RSA2 (SHA256
// with RSA) request signing, per the official 签名与验签 documentation.
type AlipayClient struct {
	AppID           string
	AppPrivateKey   *rsa.PrivateKey
	AlipayPublicKey *rsa.PublicKey
	Gateway         string
	NotifyURL       string
	ReturnURL       string
	HTTPClient      *http.Client
}

// AlipayTradeQueryResult carries the fields the gateway returns from
// alipay.trade.query that the billing flow relies on.
type AlipayTradeQueryResult struct {
	TradeNo     string
	OutTradeNo  string
	TradeStatus string
	TotalAmount string
}

const (
	AlipayStatusTradeSuccess  = "TRADE_SUCCESS"
	AlipayStatusTradeFinished = "TRADE_FINISHED"
)

func NewAlipayClient(appID string, appPrivateKey string, alipayPublicKey string, gateway string, notifyURL string, returnURL string) (*AlipayClient, error) {
	privateKey, err := loadRSAPrivateKey(appPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid Alipay app private key: %w", err)
	}
	publicKey, err := loadRSAPublicKey(alipayPublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid Alipay platform public key: %w", err)
	}
	return &AlipayClient{
		AppID:           appID,
		AppPrivateKey:   privateKey,
		AlipayPublicKey: publicKey,
		Gateway:         gateway,
		NotifyURL:       notifyURL,
		ReturnURL:       returnURL,
		HTTPClient:      &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// signParams implements the official RSA2 signing rule: exclude `sign` and
// `sign_type`, drop empty values, sort keys by ASCII order, join as k=v&…,
// then SHA256-with-RSA sign and base64-encode.
func (c *AlipayClient) signParams(params map[string]string) (string, error) {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || k == "sign_type" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+params[k])
	}
	message := []byte(strings.Join(pairs, "&"))

	digest := sha256.Sum256(message)
	signature, err := rsa.SignPKCS1v15(rand.Reader, c.AppPrivateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign Alipay request: %w", err)
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

// verifySignature verifies a signed message against the Alipay platform
// public key. The signature is base64 encoded, SHA256 with RSA.
func (c *AlipayClient) verifySignature(message []byte, signature string) error {
	raw, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return errors.New("alipay signature is not valid base64")
	}
	digest := sha256.Sum256(message)
	return rsa.VerifyPKCS1v15(c.AlipayPublicKey, crypto.SHA256, digest[:], raw)
}

func alipayTimestamp() string {
	return time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05")
}

// buildPagePayURL assembles the gateway redirect URL for the page-style
// products (alipay.trade.page.pay / alipay.trade.wap.pay). The official docs
// allow submitting these products via an HTTP GET to the gateway with all
// signed parameters in the query string.
func (c *AlipayClient) buildPagePayURL(method string, productCode string, outTradeNo string, totalAmount string, subject string) (string, error) {
	bizContent, err := json.Marshal(map[string]string{
		"out_trade_no": outTradeNo,
		"total_amount": totalAmount,
		"subject":      subject,
		"product_code": productCode,
	})
	if err != nil {
		return "", err
	}

	params := map[string]string{
		"app_id":      c.AppID,
		"method":      method,
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   alipayTimestamp(),
		"version":     "1.0",
		"notify_url":  c.NotifyURL,
		"return_url":  c.ReturnURL,
		"biz_content": string(bizContent),
	}
	signature, err := c.signParams(params)
	if err != nil {
		return "", err
	}

	query := url.Values{}
	for k, v := range params {
		if v == "" {
			continue
		}
		query.Set(k, v)
	}
	query.Set("sign", signature)
	return c.Gateway + "?" + query.Encode(), nil
}

// PagePayURL builds the 电脑网站支付 (alipay.trade.page.pay) redirect URL.
func (c *AlipayClient) PagePayURL(outTradeNo string, totalAmount string, subject string) (string, error) {
	return c.buildPagePayURL("alipay.trade.page.pay", "FAST_INSTANT_TRADE_PAY", outTradeNo, totalAmount, subject)
}

// WapPayURL builds the 手机网站支付 (alipay.trade.wap.pay) redirect URL.
func (c *AlipayClient) WapPayURL(outTradeNo string, totalAmount string, subject string) (string, error) {
	return c.buildPagePayURL("alipay.trade.wap.pay", "QUICK_WAP_WAY", outTradeNo, totalAmount, subject)
}

// VerifyNotifyParams verifies the signature of an asynchronous notification
// following the official rsaCheckV1 procedure: exclude sign and sign_type,
// drop empty values, sort and join, then verify with the Alipay platform
// public key.
func (c *AlipayClient) VerifyNotifyParams(params map[string]string) error {
	signature := params["sign"]
	if signature == "" {
		return errors.New("alipay notify is missing sign")
	}
	if signType := params["sign_type"]; signType != "RSA2" {
		return fmt.Errorf("alipay notify uses unsupported sign_type %q, only RSA2 is accepted", signType)
	}

	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || k == "sign_type" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+params[k])
	}
	message := []byte(strings.Join(pairs, "&"))
	return c.verifySignature(message, signature)
}

// callGateway submits a signed form request to the gateway and verifies the
// response signature over the raw response node bytes, as required by the
// official 签名与验签 guide for synchronous responses.
func (c *AlipayClient) callGateway(method string, bizContent string, responseNode string, result any) error {
	params := map[string]string{
		"app_id":      c.AppID,
		"method":      method,
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   alipayTimestamp(),
		"version":     "1.0",
		"biz_content": bizContent,
	}
	signature, err := c.signParams(params)
	if err != nil {
		return err
	}
	params["sign"] = signature

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	resp, err := c.HTTPClient.PostForm(c.Gateway, form)
	if err != nil {
		return fmt.Errorf("alipay gateway request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("failed to read alipay gateway response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("alipay gateway returned http %d", resp.StatusCode)
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("alipay gateway response is not JSON: %w", err)
	}
	node, ok := envelope[responseNode]
	if !ok {
		return errors.New("alipay gateway response is missing " + responseNode)
	}
	respSign, ok := envelope["sign"]
	if !ok {
		return errors.New("alipay gateway response is missing sign")
	}
	var signString string
	if err := json.Unmarshal(respSign, &signString); err != nil {
		return fmt.Errorf("alipay gateway response sign is not a string: %w", err)
	}
	if err := c.verifySignature(node, signString); err != nil {
		return fmt.Errorf("alipay gateway response signature verification failed: %w", err)
	}
	if err := json.Unmarshal(node, result); err != nil {
		return fmt.Errorf("failed to parse %s: %w", responseNode, err)
	}
	return nil
}

type alipayGatewayResponse struct {
	Code    string `json:"code"`
	Msg     string `json:"msg"`
	SubCode string `json:"sub_code"`
	SubMsg  string `json:"sub_msg"`
}

func (r alipayGatewayResponse) error() error {
	if r.Code == "10000" {
		return nil
	}
	return fmt.Errorf("alipay gateway error code=%s msg=%s sub_code=%s sub_msg=%s", r.Code, r.Msg, r.SubCode, r.SubMsg)
}

// TradeQuery calls alipay.trade.query and returns the trade status.
func (c *AlipayClient) TradeQuery(outTradeNo string) (*AlipayTradeQueryResult, error) {
	bizContent, err := json.Marshal(map[string]string{"out_trade_no": outTradeNo})
	if err != nil {
		return nil, err
	}
	var payload struct {
		alipayGatewayResponse
		TradeNo     string `json:"trade_no"`
		OutTradeNo  string `json:"out_trade_no"`
		TradeStatus string `json:"trade_status"`
		TotalAmount string `json:"total_amount"`
	}
	if err := c.callGateway("alipay.trade.query", string(bizContent), "alipay_trade_query_response", &payload); err != nil {
		return nil, err
	}
	return &AlipayTradeQueryResult{
		TradeNo:     payload.TradeNo,
		OutTradeNo:  payload.OutTradeNo,
		TradeStatus: payload.TradeStatus,
		TotalAmount: payload.TotalAmount,
	}, nil
}

// TradeClose calls alipay.trade.close so a timed-out pending order can no
// longer be paid.
func (c *AlipayClient) TradeClose(outTradeNo string) error {
	bizContent, err := json.Marshal(map[string]string{"out_trade_no": outTradeNo})
	if err != nil {
		return err
	}
	var payload alipayGatewayResponse
	return c.callGateway("alipay.trade.close", string(bizContent), "alipay_trade_close_response", &payload)
}

// SanitizeNotifyLog redacts the notify parameter dump so key material never
// reaches logs.
func SanitizeNotifyLog(params map[string]string) string {
	allowed := make(map[string]string, len(params))
	for k, v := range params {
		if strings.Contains(strings.ToLower(k), "key") || k == "sign" {
			allowed[k] = "***"
			continue
		}
		allowed[k] = v
	}
	data, _ := json.Marshal(allowed)
	return string(bytes.TrimSpace(data))
}

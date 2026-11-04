// paytest 是支付宝/微信直连支付的本地联调工具，用于在无真实商户凭据时
// 验证协议实现的正确性。所有签名/加密均严格按官方文档规则构造：
//
//   - 支付宝异步通知：RSA2（SHA256 with RSA），剔除 sign/sign_type、
//     空值不参与、按 key ASCII 升序 k=v& 拼接后签名，应答须为纯文本 success。
//   - 微信支付通知：SHA256-RSA2048 验签串 timestamp\nnonce\nbody\n，
//     resource 用 APIv3 密钥 AEAD_AES_256_GCM 解密，成功应答 200 空体。
//
// 用法示例见 scripts/paytest/README.md。
package main

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "paytest: "+format+"\n", args...)
	os.Exit(1)
}

// ---------- 密钥加载与生成 ----------

func loadPrivateKey(path string) *rsa.PrivateKey {
	data, err := os.ReadFile(path)
	if err != nil {
		fatalf("read private key %s: %v", path, err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		fatalf("private key %s is not PEM", path)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// 兼容 PKCS1
		if k1, err1 := x509.ParsePKCS1PrivateKey(block.Bytes); err1 == nil {
			return k1
		}
		fatalf("parse private key %s: %v", path, err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		fatalf("private key %s is not RSA", path)
	}
	return rsaKey
}

func loadPublicKey(path string) *rsa.PublicKey {
	data, err := os.ReadFile(path)
	if err != nil {
		fatalf("read public key %s: %v", path, err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		fatalf("public key %s is not PEM", path)
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		if k, err1 := x509.ParsePKCS1PublicKey(block.Bytes); err1 == nil {
			return k
		}
		fatalf("parse public key %s: %v", path, err)
	}
	rsaKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		fatalf("public key %s is not RSA", path)
	}
	return rsaKey
}

func cmdGen(args []string) {
	fs := flag.NewFlagSet("gen", flag.ExitOnError)
	out := fs.String("out", ".", "输出目录")
	bits := fs.Int("bits", 2048, "RSA 位数")
	_ = fs.Parse(args)

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fatalf("mkdir: %v", err)
	}
	key, err := rsa.GenerateKey(rand.Reader, *bits)
	if err != nil {
		fatalf("generate key: %v", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		fatalf("marshal public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	privPath := filepath.Join(*out, "paytest_private.pem")
	pubPath := filepath.Join(*out, "paytest_public.pem")
	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		fatalf("write private key: %v", err)
	}
	if err := os.WriteFile(pubPath, pubPEM, 0o644); err != nil {
		fatalf("write public key: %v", err)
	}
	fmt.Printf("已生成测试密钥对：\n  私钥 %s（勿外泄）\n  公钥 %s（填入管理端作为支付宝公钥/微信支付公钥）\n", privPath, pubPath)
}

func signSHA256(key *rsa.PrivateKey, message []byte) string {
	digest := sha256.Sum256(message)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		fatalf("sign: %v", err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

// ---------- 支付宝异步通知模拟 ----------

func cmdAlipayNotify(args []string) {
	fs := flag.NewFlagSet("alipay-notify", flag.ExitOnError)
	notifyURL := fs.String("url", "http://localhost:3000/api/user/alipay/notify", "异步通知地址")
	keyPath := fs.String("key", "paytest_private.pem", "与后台“支付宝公钥”配对的测试私钥")
	appID := fs.String("app-id", "2026test0000", "后台配置的 AlipayAppID")
	outTradeNo := fs.String("out-trade-no", "", "本地订单号（后台下单返回的 trade_no）")
	total := fs.String("total", "", "通知金额（元，两位小数，须与订单一致）")
	status := fs.String("status", "TRADE_SUCCESS", "trade_status")
	tradeNo := fs.String("trade-no", "", "支付宝流水号（可自定义）")
	badSign := fs.Bool("bad-sign", false, "故意破坏签名（验证验签拒绝）")
	_ = fs.Parse(args)

	if *outTradeNo == "" || *total == "" {
		fatalf("必须提供 -out-trade-no 与 -total")
	}
	if *tradeNo == "" {
		*tradeNo = fmt.Sprintf("2026%d%04d", time.Now().Unix(), os.Getpid()%10000)
	}
	key := loadPrivateKey(*keyPath)

	now := time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05")
	params := map[string]string{
		"gmt_create":   now,
		"gmt_payment":  now,
		"seller_id":    "2088test",
		"app_id":       *appID,
		"out_trade_no": *outTradeNo,
		"trade_no":     *tradeNo,
		"trade_status": *status,
		"total_amount": *total,
		"sign_type":    "RSA2",
	}
	// 官方 rsaCheckV1 拼接规则：剔除 sign/sign_type、空值不参与、key 升序。
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
	sig := signSHA256(key, []byte(strings.Join(pairs, "&")))
	if *badSign {
		b := []byte(sig)
		if b[0] == 'A' {
			b[0] = 'B'
		} else {
			b[0] = 'A'
		}
		sig = string(b)
	}

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	form.Set("sign", sig)

	resp, err := http.Post(*notifyURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		fatalf("POST %s: %v", *notifyURL, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("HTTP %d\n响应: %s\n", resp.StatusCode, string(body))
	if *status == "TRADE_SUCCESS" || *status == "TRADE_FINISHED" {
		if resp.StatusCode == 200 && strings.TrimSpace(string(body)) == "success" {
			fmt.Println("结果: 通知受理成功（符合官方 success 应答要求）")
		} else {
			fmt.Println("结果: 通知被拒绝（验签失败/金额不符/订单不符时为预期行为）")
		}
	}
}

// ---------- 微信支付通知模拟 ----------

func cmdWechatNotify(args []string) {
	fs := flag.NewFlagSet("wechat-notify", flag.ExitOnError)
	notifyURL := fs.String("url", "http://localhost:3000/api/user/wechatpay/notify", "支付通知地址")
	keyPath := fs.String("key", "paytest_private.pem", "与后台“微信支付公钥”配对的测试私钥")
	serial := fs.String("serial", "PUB_KEY_ID_TEST", "后台配置的公钥 ID")
	apiv3Key := fs.String("apiv3key", "", "后台配置的 32 字节 APIv3 密钥")
	outTradeNo := fs.String("out-trade-no", "", "本地订单号")
	totalCents := fs.Int64("total-cents", 0, "通知金额（分，须与订单一致）")
	txID := fs.String("transaction-id", "", "微信支付订单号（可自定义）")
	event := fs.String("event", "TRANSACTION.SUCCESS", "event_type")
	badSign := fs.Bool("bad-sign", false, "故意破坏签名（验证验签拒绝）")
	_ = fs.Parse(args)

	if *outTradeNo == "" || *totalCents <= 0 {
		fatalf("必须提供 -out-trade-no 与 -total-cents")
	}
	if len([]byte(*apiv3Key)) != 32 {
		fatalf("-apiv3key 必须为 32 字节")
	}
	if *txID == "" {
		*txID = fmt.Sprintf("4200001test%d", time.Now().UnixNano()%1e10)
	}
	key := loadPrivateKey(*keyPath)

	plain := map[string]any{
		"out_trade_no":   *outTradeNo,
		"transaction_id": *txID,
		"trade_state":    "SUCCESS",
		"trade_type":     "NATIVE",
		"amount": map[string]any{
			"total":          *totalCents,
			"currency":       "CNY",
			"payer_total":    *totalCents,
			"payer_currency": "CNY",
		},
	}
	plainBytes, _ := json.Marshal(plain)

	// AEAD_AES_256_GCM 加密 resource，associated_data 与官方一致为 "transaction"。
	// nonce 须与 resource.nonce 字段完全一致（官方为 12 字符字符串）。
	nonceRaw := make([]byte, 6)
	if _, err := rand.Read(nonceRaw); err != nil {
		fatalf("read nonce: %v", err)
	}
	nonceStr := fmt.Sprintf("%x", nonceRaw) // 12 个 hex 字符
	block, err := aes.NewCipher([]byte(*apiv3Key))
	if err != nil {
		fatalf("aes: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		fatalf("gcm: %v", err)
	}
	ciphertext := gcm.Seal(nil, []byte(nonceStr), plainBytes, []byte("transaction"))

	envelope := map[string]any{
		"resource_type": "encrypt-resource",
		"event_type":    *event,
		"summary":       "支付成功",
		"resource": map[string]any{
			"original_type":   "transaction",
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      base64.StdEncoding.EncodeToString(ciphertext),
			"associated_data": "transaction",
			"nonce":           nonceStr,
		},
	}
	body, _ := json.Marshal(envelope)

	ts := fmt.Sprintf("%d", time.Now().Unix())
	reqNonce := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))[:16]
	message := ts + "\n" + reqNonce + "\n" + string(body) + "\n"
	sig := signSHA256(key, []byte(message))
	if *badSign {
		b := []byte(sig)
		if b[0] == 'A' {
			b[0] = 'B'
		} else {
			b[0] = 'A'
		}
		sig = string(b)
	}

	req, err := http.NewRequest(http.MethodPost, *notifyURL, bytes.NewReader(body))
	if err != nil {
		fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Wechatpay-Timestamp", ts)
	req.Header.Set("Wechatpay-Nonce", reqNonce)
	req.Header.Set("Wechatpay-Serial", *serial)
	req.Header.Set("Wechatpay-Signature", sig)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fatalf("POST %s: %v", *notifyURL, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("HTTP %d\n响应: %s\n", resp.StatusCode, strings.TrimSpace(string(respBody)))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Println("结果: 通知受理成功（官方要求成功应答 200/204 空体）")
	} else {
		fmt.Println("结果: 通知被拒绝（验签失败/金额不符/订单不符时为预期行为）")
	}
}

// ---------- 微信支付模拟网关 ----------

type mockOrder struct {
	outTradeNo string
	totalCents int64
	notifyURL  string
	paid       bool
	txID       string
}

type mockServer struct {
	mu      sync.Mutex
	orders  map[string]*mockOrder
	key     *rsa.PrivateKey
	serial  string
	onClaim func(o *mockOrder) // 订单“支付成功”回调（例如触发通知）
}

func (s *mockServer) signHeaders(body []byte) http.Header {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonce := fmt.Sprintf("mock%d%d", time.Now().UnixNano(), os.Getpid())
	message := ts + "\n" + nonce + "\n" + string(body) + "\n"
	h := http.Header{}
	h.Set("Request-ID", "mock-"+nonce)
	h.Set("Wechatpay-Timestamp", ts)
	h.Set("Wechatpay-Nonce", nonce)
	h.Set("Wechatpay-Serial", s.serial)
	h.Set("Wechatpay-Signature", signSHA256(s.key, []byte(message)))
	return h
}

func (s *mockServer) handleNativeH5(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AppID       string `json:"appid"`
		MchID       string `json:"mchid"`
		Description string `json:"description"`
		OutTradeNo  string `json:"out_trade_no"`
		NotifyURL   string `json:"notify_url"`
		Amount      struct {
			Total    int64  `json:"total"`
			Currency string `json:"currency"`
		} `json:"amount"`
	}
	raw, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(raw, &body); err != nil || body.OutTradeNo == "" {
		writeJSONError(w, http.StatusBadRequest, "PARAM_ERROR", "bad body")
		return
	}
	s.mu.Lock()
	s.orders[body.OutTradeNo] = &mockOrder{
		outTradeNo: body.OutTradeNo,
		totalCents: body.Amount.Total,
		notifyURL:  body.NotifyURL,
	}
	s.mu.Unlock()
	fmt.Printf("[mock] %s 下单 out_trade_no=%s total=%d分 notify_url=%s\n", r.URL.Path, body.OutTradeNo, body.Amount.Total, body.NotifyURL)

	var respBody []byte
	if strings.HasSuffix(r.URL.Path, "/native") {
		respBody, _ = json.Marshal(map[string]string{
			"code_url": "weixin://wxpay/mock/" + body.OutTradeNo,
		})
	} else {
		respBody, _ = json.Marshal(map[string]string{
			"h5_url": "https://wx.tenpay.com/mock_h5/" + body.OutTradeNo,
		})
	}
	for k, vs := range s.signHeaders(respBody) {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBody)
}

func (s *mockServer) handleQuery(w http.ResponseWriter, r *http.Request, outTradeNo string) {
	s.mu.Lock()
	order := s.orders[outTradeNo]
	s.mu.Unlock()
	if order == nil {
		writeJSONError(w, http.StatusNotFound, "ORDER_NOT_EXIST", "order not found")
		return
	}
	state := "NOTPAY"
	txID := ""
	if order.paid {
		state = "SUCCESS"
		txID = order.txID
	}
	respBody, _ := json.Marshal(map[string]any{
		"out_trade_no":   order.outTradeNo,
		"transaction_id": txID,
		"trade_state":    state,
		"trade_type":     "NATIVE",
		"amount": map[string]any{
			"total":          order.totalCents,
			"currency":       "CNY",
			"payer_total":    order.totalCents,
			"payer_currency": "CNY",
		},
	})
	for k, vs := range s.signHeaders(respBody) {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBody)
}

func writeJSONError(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": code, "message": message})
}

func cmdWechatMock(args []string) {
	fs := flag.NewFlagSet("wechat-mock", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:9600", "监听地址")
	keyPath := fs.String("key", "paytest_private.pem", "平台测试私钥（响应签名用）")
	serial := fs.String("serial", "PUB_KEY_ID_TEST", "公钥 ID（后台须配置一致）")
	_ = fs.Parse(args)

	s := &mockServer{
		orders: map[string]*mockOrder{},
		key:    loadPrivateKey(*keyPath),
		serial: *serial,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v3/pay/transactions/native", s.handleNativeH5)
	mux.HandleFunc("/v3/pay/transactions/h5", s.handleNativeH5)
	// 模拟“用户扫码支付成功”的运维入口：
	//   curl -X POST 'http://127.0.0.1:9600/__pay?out_trade_no=XXX'
	// 之后查单接口返回 SUCCESS；真实通知请再用 wechat-notify 子命令发送。
	mux.HandleFunc("/__pay", func(w http.ResponseWriter, r *http.Request) {
		no := r.URL.Query().Get("out_trade_no")
		s.mu.Lock()
		order := s.orders[no]
		if order != nil && !order.paid {
			order.paid = true
			order.txID = fmt.Sprintf("4200001mock%d", time.Now().UnixNano()%1e10)
		}
		s.mu.Unlock()
		if order == nil {
			writeJSONError(w, http.StatusNotFound, "ORDER_NOT_EXIST", "order not found")
			return
		}
		fmt.Printf("[mock] 订单标记已支付 out_trade_no=%s txid=%s\n", no, order.txID)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// /v3/pay/transactions/out-trade-no/{no} 与 .../{no}/close
		prefix := "/v3/pay/transactions/out-trade-no/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "unknown path "+r.URL.Path)
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, prefix)
		rest = strings.TrimSuffix(rest, "/close")
		no := strings.Split(rest, "?")[0]
		if strings.HasSuffix(r.URL.Path, "/close") {
			fmt.Printf("[mock] 关单 out_trade_no=%s\n", no)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		s.handleQuery(w, r, no)
	})

	fmt.Printf("微信支付模拟网关已启动 http://%s （serial=%s）\n", *addr, *serial)
	fmt.Println("提示：后台 WechatPayAPIBase 指向本地址后，可用 /__pay 标记订单支付，再用 wechat-notify 发真实通知。")
	if err := http.ListenAndServe(*addr, mux); err != nil {
		fatalf("listen: %v", err)
	}
}

// ---------- 入口 ----------

func main() {
	usage := `paytest — 支付宝/微信直连支付本地联调工具

子命令:
  gen             生成测试 RSA 密钥对（paytest_private.pem / paytest_public.pem）
  alipay-notify   模拟支付宝异步通知（RSA2 签名表单）
  wechat-notify   模拟微信支付通知（SHA256-RSA2048 签名 + AES-256-GCM 加密）
  wechat-mock     启动微信支付 APIv3 模拟网关（Native/H5 下单、查单、关单）

示例:
  go run ./scripts/paytest gen -out paytest-keys
  go run ./scripts/paytest alipay-notify -key paytest-keys/paytest_private.pem \
      -app-id 2026test0000 -out-trade-no ALIUSR1NOxxx -total 7.30
  go run ./scripts/paytest wechat-mock -addr 127.0.0.1:9600 -key paytest-keys/paytest_private.pem
  go run ./scripts/paytest wechat-notify -key paytest-keys/paytest_private.pem \
      -apiv3key 0123456789abcdef0123456789abcdef -out-trade-no WXUSR1NOxxx -total-cents 730
`
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "gen":
		cmdGen(os.Args[2:])
	case "alipay-notify":
		cmdAlipayNotify(os.Args[2:])
	case "wechat-notify":
		cmdWechatNotify(os.Args[2:])
	case "wechat-mock":
		cmdWechatMock(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "未知子命令 %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
}

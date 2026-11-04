package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/payment"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// getWechatPayClient assembles the direct WeChat Pay APIv3 client.
func getWechatPayClient() (*payment.WechatPayClient, error) {
	if !isWechatPayWebhookConfigured() {
		return nil, errors.New("wechatpay gateway is not configured")
	}
	notifyURL := setting.WechatPayNotifyURL
	if strings.TrimSpace(notifyURL) == "" {
		notifyURL = service.GetCallbackAddress() + "/api/user/wechatpay/notify"
	}
	client, err := payment.NewWechatPayClient(
		setting.WechatPayMchID,
		setting.WechatPayAppID,
		setting.WechatPaySerialNo,
		setting.WechatPayPrivateKey,
		setting.WechatPayAPIv3Key,
		setting.WechatPayPlatformPublicKey,
		setting.WechatPayPlatformPublicKeyID,
		notifyURL,
	)
	if err != nil {
		return nil, err
	}
	// 留空使用官方网关；本地联调可指向模拟网关验证协议实现。
	if base := strings.TrimSpace(setting.WechatPayAPIBase); base != "" {
		client.APIBase = strings.TrimRight(base, "/")
	}
	return client, nil
}

type WechatPayRequest struct {
	Amount  int64  `json:"amount"`
	Channel string `json:"channel"`
}

func RequestWechatPay(c *gin.Context) {
	if !isWechatPayTopUpEnabled() {
		paymentError(c, common.NewMessage("WeChat Pay gateway is not enabled"))
		return
	}
	var req WechatPayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Amount <= 0 {
		paymentError(c, common.NewMessage("Invalid parameters"))
		return
	}
	if req.Amount < getMinTopup() {
		paymentError(c, common.NewMessage("Top-up amount cannot be less than {{min}}", map[string]any{"min": getMinTopup()}))
		return
	}
	id := c.GetInt("id")
	if rejectInvalidTopUpQuota(c, id, req.Amount) {
		return
	}
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		paymentError(c, common.NewMessage("Failed to get the user group"))
		return
	}
	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		paymentError(c, common.NewMessage("Payment amount is too low"))
		return
	}

	client, err := getWechatPayClient()
	if err != nil {
		logger.LogError(c.Request.Context(), common.LogText("WechatPay client init failed user_id=%d error=%q", id, err.Error()))
		paymentError(c, common.NewMessage("Payment information has not been configured by administrator"))
		return
	}

	// 官方金额单位为分（整数）。
	totalCents := decimal.NewFromFloat(payMoney).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
	if totalCents <= 0 {
		paymentError(c, common.NewMessage("Payment amount is too low"))
		return
	}

	channel := req.Channel
	if channel != "native" && channel != "h5" {
		if isMobileRequest(c) {
			channel = "h5"
		} else {
			channel = "native"
		}
	}

	tradeNo := fmt.Sprintf("WXUSR%dNO%s%d", id, common.GetRandomString(6), time.Now().Unix())
	description := fmt.Sprintf("Top-up %d", req.Amount)

	var codeURL string
	var h5URL string
	switch channel {
	case "h5":
		h5URL, err = client.H5Order(tradeNo, description, totalCents, c.ClientIP())
	default:
		channel = "native"
		codeURL, err = client.NativeOrder(tradeNo, description, totalCents, "")
	}
	if err != nil {
		logger.LogError(c.Request.Context(), common.LogText("WechatPay failed to start payment user_id=%d trade_no=%s channel=%s amount=%d error=%q", id, tradeNo, channel, req.Amount, err.Error()))
		paymentError(c, common.NewMessage("Failed to start payment"))
		return
	}

	amount := req.Amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dAmount := decimal.NewFromInt(amount)
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		amount = dAmount.Div(dQuotaPerUnit).IntPart()
	}
	topUp := &model.TopUp{
		UserId:          id,
		Amount:          amount,
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodWxpayDirect,
		PaymentProvider: model.PaymentProviderWechatPay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), common.LogText("WechatPay failed to create top-up order user_id=%d trade_no=%s amount=%d error=%q", id, tradeNo, req.Amount, err.Error()))
		paymentError(c, common.NewMessage("Failed to create order"))
		return
	}
	logger.LogInfo(c.Request.Context(), common.LogText("WechatPay top-up order created user_id=%d trade_no=%s channel=%s amount=%d money=%.2f", id, tradeNo, channel, req.Amount, payMoney))
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"channel":  channel,
			"trade_no": tradeNo,
			"code_url": codeURL,
			"h5_url":   h5URL,
		},
	})
}

type wechatPayNotifyPlain struct {
	OutTradeNo    string `json:"out_trade_no"`
	TransactionID string `json:"transaction_id"`
	TradeState    string `json:"trade_state"`
	Amount        struct {
		Total    int64  `json:"total"`
		Currency string `json:"currency"`
	} `json:"amount"`
}

// WechatPayNotify handles the official payment notification: verify the
// SHA256-RSA2048 signature against the WeChat Pay platform public key, check
// the Wechatpay-Serial, decrypt resource.ciphertext with AEAD_AES_256_GCM,
// validate the order and amount, then credit idempotently. Success answers
// HTTP 200 with an empty body; any failure answers 4xx/5xx with
// {"code":"FAIL"} so WeChat Pay retries per its official schedule.
func WechatPayNotify(c *gin.Context) {
	if !isWechatPayWebhookEnabled() {
		logger.LogWarn(c.Request.Context(), common.LogText("WechatPay webhook rejected reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		c.JSON(http.StatusInternalServerError, gin.H{"code": "FAIL", "message": "webhook disabled"})
		return
	}

	rawBody, err := c.GetRawData()
	if err != nil || len(rawBody) == 0 {
		logger.LogWarn(c.Request.Context(), common.LogText("WechatPay webhook failed to read body path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "empty body"})
		return
	}

	client, err := getWechatPayClient()
	if err != nil {
		logger.LogError(c.Request.Context(), common.LogText("WechatPay client init failed path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"code": "FAIL", "message": "client unavailable"})
		return
	}

	envelope, plain, err := client.DecryptNotify(
		c.GetHeader("Wechatpay-Timestamp"),
		c.GetHeader("Wechatpay-Nonce"),
		c.GetHeader("Wechatpay-Serial"),
		c.GetHeader("Wechatpay-Signature"),
		rawBody,
	)
	if err != nil {
		logger.LogWarn(c.Request.Context(), common.LogText("WechatPay webhook verification failed path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "verification failed"})
		return
	}
	if envelope.EventType != "TRANSACTION.SUCCESS" {
		logger.LogInfo(c.Request.Context(), common.LogText("WechatPay webhook ignored event type=%s client_ip=%s", envelope.EventType, c.ClientIP()))
		c.Status(http.StatusOK)
		return
	}

	var payload wechatPayNotifyPlain
	if err := json.Unmarshal(plain, &payload); err != nil {
		logger.LogError(c.Request.Context(), common.LogText("WechatPay webhook failed to parse decrypted resource client_ip=%s error=%q", c.ClientIP(), err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "invalid resource"})
		return
	}

	tradeNo := payload.OutTradeNo
	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil || topUp.PaymentProvider != model.PaymentProviderWechatPay {
		logger.LogWarn(c.Request.Context(), common.LogText("WechatPay webhook order not found or provider mismatch trade_no=%s client_ip=%s", tradeNo, c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "order not found"})
		return
	}
	// 官方要求核对订单金额（分）与本地订单一致，防止部分支付或篡改。
	expectedCents := decimal.NewFromFloat(topUp.Money).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
	if payload.Amount.Total != expectedCents {
		logger.LogWarn(c.Request.Context(), common.LogText("WechatPay webhook amount mismatch trade_no=%s notify_total=%d expected=%d client_ip=%s", tradeNo, payload.Amount.Total, expectedCents, c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "amount mismatch"})
		return
	}

	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)
	alreadyDone, err := model.RechargeDirectTopUp(tradeNo, model.PaymentProviderWechatPay, "WechatPay", c.ClientIP())
	if err != nil {
		logger.LogError(c.Request.Context(), common.LogText("WechatPay top-up processing failed trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"code": "FAIL", "message": "processing failed"})
		return
	}
	if alreadyDone {
		logger.LogInfo(c.Request.Context(), common.LogText("WechatPay duplicate callback ignored trade_no=%s client_ip=%s", tradeNo, c.ClientIP()))
	} else {
		logger.LogInfo(c.Request.Context(), common.LogText("WechatPay top-up succeeded trade_no=%s transaction_id=%s client_ip=%s", tradeNo, payload.TransactionID, c.ClientIP()))
	}
	c.Status(http.StatusOK)
}

type WechatPayQueryRequest struct {
	TradeNo string `json:"trade_no"`
}

// WechatPayQuery lets the wallet page poll the official query-order API as
// the "查单补偿" path while the notification is in flight.
func WechatPayQuery(c *gin.Context) {
	if !isWechatPayTopUpEnabled() {
		common.ApiErrorT(c, "WeChat Pay gateway is not enabled")
		return
	}
	var req WechatPayQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.TradeNo == "" {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	id := c.GetInt("id")
	topUp := model.GetTopUpByTradeNo(req.TradeNo)
	if topUp == nil || topUp.UserId != id || topUp.PaymentProvider != model.PaymentProviderWechatPay {
		common.ApiErrorT(c, "Top-up order does not exist")
		return
	}
	if topUp.Status == common.TopUpStatusSuccess {
		common.ApiSuccess(c, gin.H{"paid": true, "trade_state": "SUCCESS"})
		return
	}

	client, err := getWechatPayClient()
	if err != nil {
		common.ApiErrorT(c, "Payment information has not been configured by administrator")
		return
	}
	tx, err := client.QueryOrder(req.TradeNo)
	if err != nil {
		logger.LogWarn(c.Request.Context(), common.LogText("WechatPay query failed trade_no=%s error=%q", req.TradeNo, err.Error()))
		common.ApiSuccess(c, gin.H{"paid": false, "trade_state": ""})
		return
	}
	paid := tx.TradeState == payment.WechatPayStateSuccess
	if paid {
		// 回调可能延迟，主动补单与入账在同一幂等事务内完成。
		LockOrder(req.TradeNo)
		defer UnlockOrder(req.TradeNo)
		if _, err := model.RechargeDirectTopUp(req.TradeNo, model.PaymentProviderWechatPay, "WechatPay", c.ClientIP()); err != nil {
			logger.LogError(c.Request.Context(), common.LogText("WechatPay query reconciliation failed trade_no=%s error=%q", req.TradeNo, err.Error()))
		}
	}
	common.ApiSuccess(c, gin.H{"paid": paid, "trade_state": tx.TradeState})
}

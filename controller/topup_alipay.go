package controller

import (
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

// getAlipayClient assembles the direct Alipay OpenAPI client. Notify and
// return URLs default to the system callback address so operators only need
// overrides when routing callbacks to a dedicated domain.
func getAlipayClient() (*payment.AlipayClient, error) {
	if !isAlipayWebhookConfigured() {
		return nil, errors.New("alipay gateway is not configured")
	}
	callbackAddress := service.GetCallbackAddress()
	notifyURL := setting.AlipayNotifyURL
	if strings.TrimSpace(notifyURL) == "" {
		notifyURL = callbackAddress + "/api/user/alipay/notify"
	}
	returnURL := setting.AlipayReturnURL
	if strings.TrimSpace(returnURL) == "" {
		returnURL = paymentReturnPath("/usage-logs")
	}
	return payment.NewAlipayClient(
		setting.AlipayAppID,
		setting.AlipayAppPrivateKey,
		setting.AlipayPublicKey,
		setting.GetAlipayGateway(),
		notifyURL,
		returnURL,
	)
}

func isMobileRequest(c *gin.Context) bool {
	ua := strings.ToLower(c.GetHeader("User-Agent"))
	return strings.Contains(ua, "mobile") || strings.Contains(ua, "android") || strings.Contains(ua, "iphone")
}

func RequestAlipayPay(c *gin.Context) {
	if !isAlipayTopUpEnabled() {
		paymentError(c, common.NewMessage("Alipay gateway is not enabled"))
		return
	}
	var req EpayRequest
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

	client, err := getAlipayClient()
	if err != nil {
		logger.LogError(c.Request.Context(), common.LogText("Alipay client init failed user_id=%d error=%q", id, err.Error()))
		paymentError(c, common.NewMessage("Payment information has not been configured by administrator"))
		return
	}

	// 官方要求金额为元、保留两位小数的字符串。
	totalAmount := decimal.NewFromFloat(payMoney).StringFixed(2)
	tradeNo := fmt.Sprintf("ALIUSR%dNO%s%d", id, common.GetRandomString(6), time.Now().Unix())
	subject := fmt.Sprintf("Top-up %d", req.Amount)

	var payURL string
	channel := "pc"
	if isMobileRequest(c) {
		channel = "wap"
		payURL, err = client.WapPayURL(tradeNo, totalAmount, subject)
	} else {
		payURL, err = client.PagePayURL(tradeNo, totalAmount, subject)
	}
	if err != nil {
		logger.LogError(c.Request.Context(), common.LogText("Alipay failed to start payment user_id=%d trade_no=%s amount=%d error=%q", id, tradeNo, req.Amount, err.Error()))
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
		PaymentMethod:   model.PaymentMethodAlipayDirect,
		PaymentProvider: model.PaymentProviderAlipay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), common.LogText("Alipay failed to create top-up order user_id=%d trade_no=%s amount=%d error=%q", id, tradeNo, req.Amount, err.Error()))
		paymentError(c, common.NewMessage("Failed to create order"))
		return
	}
	logger.LogInfo(c.Request.Context(), common.LogText("Alipay top-up order created user_id=%d trade_no=%s channel=%s amount=%d money=%.2f", id, tradeNo, channel, req.Amount, payMoney))
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"url":     payURL,
		"data":    gin.H{"channel": channel, "trade_no": tradeNo},
	})
}

// AlipayNotify handles the official asynchronous payment notification: form
// params, RSA2 verification against the Alipay platform public key, local
// order/amount validation, then idempotent crediting. The response must be
// the plain text "success" exactly — anything else makes Alipay retry for
// up to 25 hours.
func AlipayNotify(c *gin.Context) {
	if !isAlipayWebhookEnabled() {
		logger.LogWarn(c.Request.Context(), common.LogText("Alipay webhook rejected reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	var params map[string]string
	if c.Request.Method == http.MethodPost {
		if err := c.Request.ParseForm(); err != nil {
			logger.LogError(c.Request.Context(), common.LogText("Alipay webhook failed to parse form path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
			_, _ = c.Writer.Write([]byte("fail"))
			return
		}
		params = make(map[string]string, len(c.Request.PostForm))
		for key := range c.Request.PostForm {
			params[key] = c.Request.PostForm.Get(key)
		}
	} else {
		params = make(map[string]string, len(c.Request.URL.Query()))
		for key := range c.Request.URL.Query() {
			params[key] = c.Request.URL.Query().Get(key)
		}
	}
	logger.LogInfo(c.Request.Context(), common.LogText("Alipay webhook received path=%q client_ip=%s method=%s params=%s", c.Request.RequestURI, c.ClientIP(), c.Request.Method, payment.SanitizeNotifyLog(params)))

	client, err := getAlipayClient()
	if err != nil {
		logger.LogError(c.Request.Context(), common.LogText("Alipay client init failed path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	// 验签：官方 rsaCheckV1 流程，剔除 sign/sign_type 后排序拼接验签。
	if err := client.VerifyNotifyParams(params); err != nil {
		logger.LogWarn(c.Request.Context(), common.LogText("Alipay webhook signature verification failed path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	// 官方要求校验：app_id 匹配、out_trade_no 存在、total_amount 与订单一致。
	if params["app_id"] != setting.AlipayAppID {
		logger.LogWarn(c.Request.Context(), common.LogText("Alipay webhook app_id mismatch path=%q client_ip=%s notify_app_id=%s", c.Request.RequestURI, c.ClientIP(), params["app_id"]))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	tradeNo := params["out_trade_no"]
	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil || topUp.PaymentProvider != model.PaymentProviderAlipay {
		logger.LogWarn(c.Request.Context(), common.LogText("Alipay webhook order not found or provider mismatch trade_no=%s client_ip=%s", tradeNo, c.ClientIP()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	notifyAmount, amountErr := decimal.NewFromString(params["total_amount"])
	if amountErr != nil || !notifyAmount.Equal(decimal.NewFromFloat(topUp.Money).Round(2)) {
		logger.LogWarn(c.Request.Context(), common.LogText("Alipay webhook amount mismatch trade_no=%s notify_amount=%s order_money=%.2f client_ip=%s", tradeNo, params["total_amount"], topUp.Money, c.ClientIP()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	tradeStatus := params["trade_status"]
	if tradeStatus == payment.AlipayStatusTradeSuccess || tradeStatus == payment.AlipayStatusTradeFinished {
		LockOrder(tradeNo)
		defer UnlockOrder(tradeNo)
		alreadyDone, err := model.RechargeDirectTopUp(tradeNo, model.PaymentProviderAlipay, "Alipay", c.ClientIP())
		if err != nil {
			logger.LogError(c.Request.Context(), common.LogText("Alipay top-up processing failed trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), err.Error()))
			_, _ = c.Writer.Write([]byte("fail"))
			return
		}
		if alreadyDone {
			logger.LogInfo(c.Request.Context(), common.LogText("Alipay duplicate callback ignored trade_no=%s client_ip=%s", tradeNo, c.ClientIP()))
		} else {
			logger.LogInfo(c.Request.Context(), common.LogText("Alipay top-up succeeded trade_no=%s client_ip=%s", tradeNo, c.ClientIP()))
		}
	} else {
		logger.LogInfo(c.Request.Context(), common.LogText("Alipay webhook ignored event trade_no=%s trade_status=%s client_ip=%s", tradeNo, tradeStatus, c.ClientIP()))
	}
	_, _ = c.Writer.Write([]byte("success"))
}

type AlipayQueryRequest struct {
	TradeNo string `json:"trade_no"`
}

// AlipayQuery lets the wallet page poll alipay.trade.query as the official
// "查单补偿" path while the asynchronous notification is in flight.
func AlipayQuery(c *gin.Context) {
	if !isAlipayTopUpEnabled() {
		common.ApiErrorT(c, "Alipay gateway is not enabled")
		return
	}
	var req AlipayQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.TradeNo == "" {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	id := c.GetInt("id")
	topUp := model.GetTopUpByTradeNo(req.TradeNo)
	if topUp == nil || topUp.UserId != id || topUp.PaymentProvider != model.PaymentProviderAlipay {
		common.ApiErrorT(c, "Top-up order does not exist")
		return
	}
	if topUp.Status == common.TopUpStatusSuccess {
		common.ApiSuccess(c, gin.H{"paid": true, "trade_status": "TRADE_SUCCESS"})
		return
	}

	client, err := getAlipayClient()
	if err != nil {
		common.ApiErrorT(c, "Payment information has not been configured by administrator")
		return
	}
	result, err := client.TradeQuery(req.TradeNo)
	if err != nil {
		// 未支付订单查询会返回 ACQ.TRADE_NOT_EXIST，对前端而言就是未支付。
		logger.LogWarn(c.Request.Context(), common.LogText("Alipay query failed trade_no=%s error=%q", req.TradeNo, err.Error()))
		common.ApiSuccess(c, gin.H{"paid": false, "trade_status": ""})
		return
	}
	paid := result.TradeStatus == payment.AlipayStatusTradeSuccess || result.TradeStatus == payment.AlipayStatusTradeFinished
	common.ApiSuccess(c, gin.H{"paid": paid, "trade_status": result.TradeStatus})
}

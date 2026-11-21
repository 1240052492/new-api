package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// GetAffiliateSummary returns the caller's referral statistics together with the
// configured reward rules, so the referral card needs a single request.
func GetAffiliateSummary(c *gin.Context) {
	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	commissionTotal, err := model.SumAffiliateCommission(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	remainingInvitees := -1
	if common.AffiliateMaxInviteeCount > 0 {
		remainingInvitees = max(0, common.AffiliateMaxInviteeCount-user.AffCount)
	}
	common.ApiSuccess(c, gin.H{
		"aff_code":                user.AffCode,
		"aff_count":               user.AffCount,
		"aff_quota":               user.AffQuota,
		"aff_history_quota":       user.AffHistoryQuota,
		"commission_total":        commissionTotal,
		"commission_rate":         common.AffiliateTopUpCommissionRate,
		"reward_to_balance":       common.AffiliateRewardToBalance,
		"max_invitee_count":       common.AffiliateMaxInviteeCount,
		"remaining_invitee_count": remainingInvitees,
	})
}

// GetAffiliateCommissions lists the caller's commission records.
func GetAffiliateCommissions(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.GetAffiliateCommissions(userId, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(records)
	common.ApiSuccess(c, pageInfo)
}

// AdminGetAffiliateCommissions lists the commission records of any inviter.
func AdminGetAffiliateCommissions(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		common.ApiErrorMsg(c, "Invalid user ID")
		return
	}
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.GetAffiliateCommissions(userId, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(records)
	common.ApiSuccess(c, pageInfo)
}

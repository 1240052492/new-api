package model

import (
	"errors"
	"math"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// AffiliateSourceTopUp marks a commission earned from a paid top-up order.
	AffiliateSourceTopUp = "topup"
	// AffiliateSourceRedemption marks a commission earned from a redemption code.
	AffiliateSourceRedemption = "redemption"
)

// AffiliateCommission records one commission payout to an inviter. Reference is
// unique so a replayed payment callback or a repeated redemption cannot pay the
// same source twice.
type AffiliateCommission struct {
	Id         int     `json:"id" gorm:"primaryKey"`
	InviterId  int     `json:"inviter_id" gorm:"index"`
	InviteeId  int     `json:"invitee_id" gorm:"index"`
	SourceType string  `json:"source_type" gorm:"type:varchar(20);index"`
	Reference  string  `json:"reference" gorm:"type:varchar(255);uniqueIndex"`
	BaseAmount float64 `json:"base_amount"`
	Rate       float64 `json:"rate"`
	Quota      int     `json:"quota"`
	CreateTime int64   `json:"create_time" gorm:"index"`
}

// CommissionSource describes a credit event that may earn the inviter a
// commission. A nil source means the credit is not commissionable.
type CommissionSource struct {
	Type       string
	Reference  string
	BaseAmount float64
}

// commissionQuota converts the paid amount into a quota payout. Rounding down
// keeps the payout from exceeding the configured share.
func commissionQuota(baseAmount float64) (int, float64) {
	rate := common.AffiliateTopUpCommissionRate
	if rate <= 0 || baseAmount <= 0 || common.QuotaPerUnit <= 0 {
		return 0, 0
	}
	quota := int(math.Floor(baseAmount * rate / 100 * common.QuotaPerUnit))
	if quota <= 0 {
		return 0, 0
	}
	return quota, rate
}

// settleCommission pays the inviter their configured share of a credit event.
// It runs inside the crediting transaction so a commission cannot be paid for a
// top-up that was rolled back, and it never blocks the credit itself when the
// invitee has no valid inviter.
func settleCommission(tx *gorm.DB, inviteeId int, source *CommissionSource) error {
	if source == nil || common.AffiliateTopUpCommissionRate <= 0 {
		return nil
	}
	quota, rate := commissionQuota(source.BaseAmount)
	if quota <= 0 {
		return nil
	}

	var invitee User
	if err := tx.Select("id", "inviter_id").Where("id = ?", inviteeId).First(&invitee).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	inviterId := invitee.InviterId
	if inviterId == 0 || inviterId == inviteeId {
		return nil
	}

	record := &AffiliateCommission{
		InviterId:  inviterId,
		InviteeId:  inviteeId,
		SourceType: source.Type,
		Reference:  source.Reference,
		BaseAmount: source.BaseAmount,
		Rate:       rate,
		Quota:      quota,
		CreateTime: common.GetTimestamp(),
	}
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "reference"}},
		DoNothing: true,
	}).Create(record)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}

	updates := map[string]any{}
	if common.AffiliateRewardToBalance {
		updates["quota"] = gorm.Expr("quota + ?", quota)
	} else {
		updates["aff_quota"] = gorm.Expr("aff_quota + ?", quota)
	}
	if err := tx.Model(&User{}).Where("id = ?", inviterId).Updates(updates).Error; err != nil {
		return err
	}
	// Invalidation is safe before commit: a rollback only costs a cache refill
	// from the committed balance, while a stale snapshot would hide the payout.
	if err := invalidateUserCache(inviterId); err != nil {
		common.SysLog(common.LogText("failed to invalidate inviter %d cache: %s", inviterId, err.Error()))
	}
	common.SysLog(common.LogText("affiliate commission paid inviter=%d invitee=%d source=%s rate=%.2f%% quota=%d", inviterId, inviteeId, source.Type, rate, quota))
	return nil
}

// GetAffiliateCommissions lists commission records for an inviter.
func GetAffiliateCommissions(inviterId int, pageInfo *common.PageInfo) ([]*AffiliateCommission, int64, error) {
	var total int64
	query := DB.Model(&AffiliateCommission{}).Where("inviter_id = ?", inviterId)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	records := make([]*AffiliateCommission, 0, pageInfo.GetPageSize())
	err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&records).Error
	return records, total, err
}

// SumAffiliateCommission returns the total commission paid to an inviter.
func SumAffiliateCommission(inviterId int) (int64, error) {
	var total int64
	err := DB.Model(&AffiliateCommission{}).Where("inviter_id = ?", inviterId).Select("COALESCE(SUM(quota),0)").Scan(&total).Error
	return total, err
}

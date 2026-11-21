package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func resetAffiliateOptions(t *testing.T) {
	t.Helper()
	oldRate := common.AffiliateTopUpCommissionRate
	oldMode := common.AffiliateRewardToBalance
	oldCap := common.AffiliateMaxInviteeCount
	oldInviterReward := common.QuotaForInviter
	oldInviteeReward := common.QuotaForInvitee
	oldQuotaPerUnit := common.QuotaPerUnit

	common.AffiliateTopUpCommissionRate = 0
	common.AffiliateRewardToBalance = true
	common.AffiliateMaxInviteeCount = 0
	common.QuotaForInviter = 0
	common.QuotaForInvitee = 0
	common.QuotaPerUnit = 500 * 1000.0

	paymentSetting := operation_setting.GetPaymentSetting()
	oldConfirmed := paymentSetting.ComplianceConfirmed
	oldVersion := paymentSetting.ComplianceTermsVersion
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion

	t.Cleanup(func() {
		common.AffiliateTopUpCommissionRate = oldRate
		common.AffiliateRewardToBalance = oldMode
		common.AffiliateMaxInviteeCount = oldCap
		common.QuotaForInviter = oldInviterReward
		common.QuotaForInvitee = oldInviteeReward
		common.QuotaPerUnit = oldQuotaPerUnit
		paymentSetting.ComplianceConfirmed = oldConfirmed
		paymentSetting.ComplianceTermsVersion = oldVersion
	})
}

func createAffiliateUser(t *testing.T, name string, inviterId int) User {
	t.Helper()
	user := User{
		Username:    name,
		Password:    "unused-password-hash",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
		AffCode:     name + "-aff",
		InviterId:   inviterId,
	}
	require.NoError(t, DB.Create(&user).Error)
	return user
}

func reloadUser(t *testing.T, userId int) User {
	t.Helper()
	user, err := GetUserById(userId, true)
	require.NoError(t, err)
	return *user
}

func TestCommissionQuotaRounding(t *testing.T) {
	resetAffiliateOptions(t)
	truncateTables(t)

	common.AffiliateTopUpCommissionRate = 20

	quota, rate := commissionQuota(100)
	assert.Equal(t, 20.0, rate)
	assert.Equal(t, 20*int(common.QuotaPerUnit), quota, "20% of 100 is 20 wallet units")

	// 向下取整：0.000015 * 20% * 500000 = 1.5 个额度，向下取整为 1
	quota, _ = commissionQuota(0.000015)
	assert.Equal(t, 1, quota)
}

func TestCommissionQuotaDisabledAtZeroRate(t *testing.T) {
	resetAffiliateOptions(t)
	truncateTables(t)

	common.AffiliateTopUpCommissionRate = 0
	quota, rate := commissionQuota(100)
	assert.Equal(t, 0, quota)
	assert.Equal(t, 0.0, rate)
}

func TestSettleCommissionCreditsInviterBalance(t *testing.T) {
	resetAffiliateOptions(t)
	truncateTables(t)

	common.AffiliateTopUpCommissionRate = 20
	inviter := createAffiliateUser(t, "inviter", 0)
	invitee := createAffiliateUser(t, "invitee", inviter.Id)

	err := DB.Transaction(func(tx *gorm.DB) error {
		return settleCommission(tx, invitee.Id, &CommissionSource{
			Type:       AffiliateSourceTopUp,
			Reference:  "trade-1",
			BaseAmount: 100,
		})
	})
	require.NoError(t, err)

	assert.Equal(t, 20*int(common.QuotaPerUnit), reloadUser(t, inviter.Id).Quota)
	_, total, err := GetAffiliateCommissions(inviter.Id, &common.PageInfo{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	sum, err := SumAffiliateCommission(inviter.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(20*int(common.QuotaPerUnit)), sum)
}

func TestSettleCommissionIsIdempotent(t *testing.T) {
	resetAffiliateOptions(t)
	truncateTables(t)

	common.AffiliateTopUpCommissionRate = 20
	inviter := createAffiliateUser(t, "inviter", 0)
	invitee := createAffiliateUser(t, "invitee", inviter.Id)

	source := &CommissionSource{Type: AffiliateSourceTopUp, Reference: "trade-dup", BaseAmount: 50}
	for range 2 {
		require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
			return settleCommission(tx, invitee.Id, source)
		}))
	}

	assert.Equal(t, 10*int(common.QuotaPerUnit), reloadUser(t, inviter.Id).Quota, "replayed callbacks must not pay twice")
}

func TestSettleCommissionUsesAffiliateQuotaWhenBalanceDisabled(t *testing.T) {
	resetAffiliateOptions(t)
	truncateTables(t)

	common.AffiliateTopUpCommissionRate = 20
	common.AffiliateRewardToBalance = false
	inviter := createAffiliateUser(t, "inviter", 0)
	invitee := createAffiliateUser(t, "invitee", inviter.Id)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return settleCommission(tx, invitee.Id, &CommissionSource{
			Type:       AffiliateSourceRedemption,
			Reference:  "redemption:1",
			BaseAmount: 100,
		})
	}))

	updated := reloadUser(t, inviter.Id)
	assert.Equal(t, 0, updated.Quota)
	assert.Equal(t, 20*int(common.QuotaPerUnit), updated.AffQuota)
}

func TestSettleCommissionSkipsUsersWithoutInviter(t *testing.T) {
	resetAffiliateOptions(t)
	truncateTables(t)

	common.AffiliateTopUpCommissionRate = 20
	standalone := createAffiliateUser(t, "standalone", 0)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return settleCommission(tx, standalone.Id, &CommissionSource{
			Type:       AffiliateSourceTopUp,
			Reference:  "trade-solo",
			BaseAmount: 100,
		})
	}))

	_, total, err := GetAffiliateCommissions(standalone.Id, &common.PageInfo{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
}

func TestCountInviteeHonoursCap(t *testing.T) {
	resetAffiliateOptions(t)
	truncateTables(t)

	common.AffiliateMaxInviteeCount = 2
	inviter := createAffiliateUser(t, "inviter", 0)

	for i := range 2 {
		accepted, err := countInvitee(inviter.Id, 0)
		require.NoError(t, err)
		assert.True(t, accepted, "invitation %d should be accepted", i+1)
	}

	accepted, err := countInvitee(inviter.Id, 0)
	require.NoError(t, err)
	assert.False(t, accepted, "the third invitation must exceed the cap")
	assert.Equal(t, 2, reloadUser(t, inviter.Id).AffCount)
}

func TestSettleInvitationDropsRelationAtCap(t *testing.T) {
	resetAffiliateOptions(t)
	truncateTables(t)

	common.AffiliateMaxInviteeCount = 1
	common.QuotaForInviter = 5000
	common.QuotaForInvitee = 3000

	inviter := createAffiliateUser(t, "inviter", 0)
	first := createAffiliateUser(t, "first", inviter.Id)
	second := createAffiliateUser(t, "second", inviter.Id)

	settleInvitation(first.Id, inviter.Id)
	settleInvitation(second.Id, inviter.Id)

	assert.Equal(t, 5000, reloadUser(t, inviter.Id).Quota, "inviter is paid once")
	assert.Equal(t, 3000, reloadUser(t, first.Id).Quota)
	assert.Equal(t, 0, reloadUser(t, second.Id).Quota, "over-cap invitee gets no reward")
	assert.Equal(t, 0, reloadUser(t, second.Id).InviterId, "over-cap invitee keeps no relation")
}

func TestSettleInvitationRespectsRewardMode(t *testing.T) {
	resetAffiliateOptions(t)
	truncateTables(t)

	common.AffiliateMaxInviteeCount = 0
	common.QuotaForInviter = 5000
	common.QuotaForInvitee = 3000
	// Referral mode: rewards are parked in aff_quota until the user transfers them.
	common.AffiliateRewardToBalance = false

	inviter := createAffiliateUser(t, "inviter", 0)
	invitee := createAffiliateUser(t, "invitee", inviter.Id)

	settleInvitation(invitee.Id, inviter.Id)

	reloadedInviter := reloadUser(t, inviter.Id)
	assert.Zero(t, reloadedInviter.Quota, "referral mode must not credit the balance")
	assert.Equal(t, 5000, reloadedInviter.AffQuota, "inviter reward goes to the referral wallet")

	reloadedInvitee := reloadUser(t, invitee.Id)
	assert.Zero(t, reloadedInvitee.Quota, "the invitee bonus follows the same mode")
	assert.Equal(t, 3000, reloadedInvitee.AffQuota, "invitee bonus lands in the referral wallet")
}

func TestCreditTopUpQuotaSettlesCommission(t *testing.T) {
	resetAffiliateOptions(t)
	truncateTables(t)

	common.AffiliateTopUpCommissionRate = 20
	inviter := createAffiliateUser(t, "inviter", 0)
	invitee := createAffiliateUser(t, "invitee", inviter.Id)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return creditTopUpQuota(tx, invitee.Id, int(common.QuotaPerUnit), nil, &CommissionSource{
			Type:       AffiliateSourceTopUp,
			Reference:  "trade-credit",
			BaseAmount: 10,
		})
	}))

	assert.Equal(t, int(common.QuotaPerUnit), reloadUser(t, invitee.Id).Quota, "top-up is credited")
	assert.Equal(t, 2*int(common.QuotaPerUnit), reloadUser(t, inviter.Id).Quota, "20% of 10 is paid to the inviter")
}

func TestRedeemPaysAffiliateCommission(t *testing.T) {
	resetAffiliateOptions(t)
	truncateTables(t)

	common.AffiliateTopUpCommissionRate = 20
	inviter := createAffiliateUser(t, "inviter", 0)
	invitee := createAffiliateUser(t, "invitee", inviter.Id)

	code := Redemption{
		Key:         "affiliate-redeem-key",
		Status:      common.RedemptionCodeStatusEnabled,
		Name:        "affiliate test",
		Quota:       50 * int(common.QuotaPerUnit),
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(&code).Error)

	quota, err := Redeem(code.Key, invitee.Id)
	require.NoError(t, err)
	assert.Equal(t, 50*int(common.QuotaPerUnit), quota)
	assert.Equal(t, 50*int(common.QuotaPerUnit), reloadUser(t, invitee.Id).Quota)
	assert.Equal(t, 10*int(common.QuotaPerUnit), reloadUser(t, inviter.Id).Quota, "20% of the 50 face value is paid to the inviter")
}

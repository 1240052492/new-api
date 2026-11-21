package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPhoneTestUser(username string, phone string) *User {
	return &User{
		Username: username,
		Password: "Test123456",
		Phone:    phone,
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
}

func TestPhoneAvailability(t *testing.T) {
	truncateTables(t)
	common.QuotaForNewUser = 0

	user := newPhoneTestUser("phone-owner", "+86 138 0013 8000")
	require.NoError(t, user.Insert(0))
	assert.Equal(t, "13800138000", user.Phone, "phone should be stored in normalized form")

	available, err := IsPhoneAvailable("13800138000", 0)
	require.NoError(t, err)
	assert.False(t, available, "same number in a different format must collide")

	available, err = IsPhoneAvailable("13800138000", user.Id)
	require.NoError(t, err)
	assert.True(t, available, "the owning user may keep their own number")

	available, err = IsPhoneAvailable("13900139000", 0)
	require.NoError(t, err)
	assert.True(t, available)

	// An empty phone never collides, otherwise accounts created through OAuth
	// would block each other.
	require.NoError(t, newPhoneTestUser("phone-empty-a", "").Insert(0))
	require.NoError(t, newPhoneTestUser("phone-empty-b", "").Insert(0))
}

func TestInsertRejectsDuplicatePhone(t *testing.T) {
	truncateTables(t)
	common.QuotaForNewUser = 0

	require.NoError(t, newPhoneTestUser("phone-first", "13900139000").Insert(0))

	err := newPhoneTestUser("phone-second", "139-0013-9000").Insert(0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPhoneAlreadyTaken))

	var count int64
	require.NoError(t, DB.Model(&User{}).Where("username = ?", "phone-second").Count(&count).Error)
	assert.Zero(t, count, "the rejected registration must not leave a row behind")
}

func TestUpdateRejectsDuplicatePhone(t *testing.T) {
	truncateTables(t)
	common.QuotaForNewUser = 0

	first := newPhoneTestUser("phone-update-a", "13700137000")
	require.NoError(t, first.Insert(0))
	second := newPhoneTestUser("phone-update-b", "13600136000")
	require.NoError(t, second.Insert(0))

	conflict := *second
	conflict.Phone = first.Phone
	err := conflict.Update(false)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPhoneAlreadyTaken))

	kept, err := GetUserById(second.Id, false)
	require.NoError(t, err)
	assert.Equal(t, "13600136000", kept.Phone, "the failed update must not change the stored number")

	// Updating an unrelated field without touching the phone keeps the number.
	noPhoneChange := *second
	noPhoneChange.DisplayName = "second renamed"
	require.NoError(t, noPhoneChange.Update(false))
	kept, err = GetUserById(second.Id, false)
	require.NoError(t, err)
	assert.Equal(t, "13600136000", kept.Phone)
	assert.Equal(t, "second renamed", kept.DisplayName)
}

func TestAdminEditPhone(t *testing.T) {
	truncateTables(t)
	common.QuotaForNewUser = 0

	user := newPhoneTestUser("phone-admin-edit", "13500135000")
	require.NoError(t, user.Insert(0))

	// An admin edit that omits the phone must not wipe it.
	withoutPhone := *user
	withoutPhone.Phone = ""
	withoutPhone.DisplayName = "edited without phone"
	require.NoError(t, withoutPhone.Edit(false))
	kept, err := GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, "13500135000", kept.Phone)
	assert.Equal(t, "edited without phone", kept.DisplayName)

	// Supplying a phone updates it in normalized form.
	withPhone := *user
	withPhone.Phone = "+86 133 0013 3000"
	withPhone.DisplayName = "edited with phone"
	require.NoError(t, withPhone.Edit(false))
	kept, err = GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, "13300133000", kept.Phone)
}

func TestSearchUsersByPhone(t *testing.T) {
	truncateTables(t)
	common.QuotaForNewUser = 0

	require.NoError(t, newPhoneTestUser("search-phone-a", "13200132000").Insert(0))
	require.NoError(t, newPhoneTestUser("search-phone-b", "13100131000").Insert(0))

	users, total, err := SearchUsers("13200132000", "", nil, nil, 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, users, 1)
	assert.Equal(t, "search-phone-a", users[0].Username)
}

// An administrator submitting an empty phone clears the field, while an edit that
// does not mention the field leaves it alone (covered by TestAdminEditPhone).
func TestAdminEditClearsPhone(t *testing.T) {
	truncateTables(t)
	common.QuotaForNewUser = 0

	user := newPhoneTestUser("phone-clear", "13400134000")
	require.NoError(t, user.Insert(0))

	cleared := *user
	cleared.Phone = ""
	cleared.PhoneSpecified = true
	require.NoError(t, cleared.Edit(false))

	kept, err := GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Empty(t, kept.Phone, "an explicit empty phone must clear the stored number")

	// Once cleared, the same number can be claimed by another account.
	require.NoError(t, newPhoneTestUser("phone-clear-claimer", "13400134000").Insert(0))
}

func TestLoginByPhone(t *testing.T) {
	truncateTables(t)
	common.QuotaForNewUser = 0

	owner := newPhoneTestUser("login-by-phone", "13800138000")
	require.NoError(t, owner.Insert(0))

	for _, identifier := range []string{"13800138000", "+86 138 0013 8000", "138-0013-8000"} {
		candidate := &User{Username: identifier, Password: "Test123456"}
		require.NoError(t, candidate.ValidateAndFill(), "phone identifier %q should sign in", identifier)
		assert.Equal(t, owner.Id, candidate.Id)
	}

	// Username and email login keep working after the phone fallback.
	byName := &User{Username: "login-by-phone", Password: "Test123456"}
	require.NoError(t, byName.ValidateAndFill())
	assert.Equal(t, owner.Id, byName.Id)

	// Unknown number and wrong password both fail with the same error.
	unknown := &User{Username: "13700137000", Password: "Test123456"}
	assert.True(t, errors.Is(unknown.ValidateAndFill(), ErrInvalidCredentials))

	wrongPassword := &User{Username: "13800138000", Password: "WrongPassword"}
	assert.True(t, errors.Is(wrongPassword.ValidateAndFill(), ErrInvalidCredentials))
}

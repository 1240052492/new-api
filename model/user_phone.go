package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// ErrPhoneAlreadyTaken is returned when a phone number is already bound to
// another account. Numbers are unique across the platform so one mobile line
// cannot be used to farm sign-up rewards.
var ErrPhoneAlreadyTaken = errors.New("phone number is already in use")

// NormalizePhone canonicalizes a phone number for storage and lookup.
func NormalizePhone(phone string) string {
	return common.NormalizePhone(phone)
}

func phoneQuery(tx *gorm.DB, phone string) *gorm.DB {
	if tx == nil {
		tx = DB
	}
	return tx.Unscoped().Model(&User{}).Where("phone = ?", NormalizePhone(phone))
}

// IsPhoneAvailable reports whether the phone number can be used by the given
// user. An empty phone is always available.
func IsPhoneAvailable(phone string, excludeUserID int) (bool, error) {
	phone = NormalizePhone(phone)
	if phone == "" {
		return true, nil
	}
	query := phoneQuery(DB, phone)
	if excludeUserID > 0 {
		query = query.Where("id <> ?", excludeUserID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

// EnsurePhoneAvailable returns ErrPhoneAlreadyTaken when the number is in use.
func EnsurePhoneAvailable(phone string, excludeUserID int) error {
	available, err := IsPhoneAvailable(phone, excludeUserID)
	if err != nil {
		return err
	}
	if !available {
		return ErrPhoneAlreadyTaken
	}
	return nil
}

func ensurePhoneAvailableWithTx(tx *gorm.DB, phone string, excludeUserID int) error {
	phone = NormalizePhone(phone)
	if phone == "" {
		return nil
	}
	query := phoneQuery(tx, phone)
	if excludeUserID > 0 {
		query = query.Where("id <> ?", excludeUserID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrPhoneAlreadyTaken
	}
	return nil
}

// withNormalizedPhoneLock serializes writers that target the same phone number
// inside tx, mirroring the email lock so a "check then write" pair cannot be
// raced into two accounts sharing one number.
func withNormalizedPhoneLock(tx *gorm.DB, phone string, fn func(tx *gorm.DB) error) error {
	phone = NormalizePhone(phone)
	if phone == "" {
		return fn(tx)
	}
	switch {
	case common.UsingMainDatabase(common.DatabaseTypePostgreSQL):
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "phone:"+phone).Error; err != nil {
			return err
		}
	case common.UsingMainDatabase(common.DatabaseTypeMySQL):
		var ids []int
		if err := tx.Raw("SELECT id FROM users WHERE phone = ? FOR UPDATE", phone).Scan(&ids).Error; err != nil {
			return err
		}
	}
	return fn(tx)
}

// BindPhoneToUser atomically checks availability and stores the number on the
// user. Used by self-service profile updates.
func BindPhoneToUser(user *User, phone string) error {
	phone = NormalizePhone(phone)
	if err := DB.Transaction(func(tx *gorm.DB) error {
		return withNormalizedPhoneLock(tx, phone, func(tx *gorm.DB) error {
			if err := ensurePhoneAvailableWithTx(tx, phone, user.Id); err != nil {
				return err
			}
			user.Phone = phone
			return user.UpdateWithTx(tx, false)
		})
	}); err != nil {
		return err
	}
	return updateUserCache(*user)
}

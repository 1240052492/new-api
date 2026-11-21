package common

import (
	"regexp"
	"strings"
)

// Phone numbers are stored as plain digits. The dashboard accepts the four
// mobile formats used across China (mainland, Hong Kong, Macao and Taiwan) and
// strips the separators people habitually type (spaces, dashes, parentheses, a
// leading "+" or the 00 international prefix).

var (
	phoneSeparatorReplacer = strings.NewReplacer(
		" ", "", "\t", "", "-", "", "－", "",
		"(", "", ")", "", "（", "", "）", "",
	)

	// Mainland: 11 digits starting with 1 and a valid second digit.
	mainlandPhonePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)
	// Hong Kong: 8 digits starting with 4, 5, 6, 7 or 9.
	hongKongPhonePattern = regexp.MustCompile(`^[45679]\d{7}$`)
	// Macao: 8 digits starting with 6.
	macaoPhonePattern = regexp.MustCompile(`^6\d{7}$`)
	// Taiwan: 10 digits starting with 09.
	taiwanPhonePattern = regexp.MustCompile(`^09\d{8}$`)
)

// NormalizePhone returns the canonical digit-only form of a phone number.
// Country codes for the supported regions are dropped so "+86 138 0013 8000"
// and "13800138000" are stored identically. Unknown input is only trimmed and
// left for IsValidPhone to reject.
func NormalizePhone(phone string) string {
	phone = phoneSeparatorReplacer.Replace(strings.TrimSpace(phone))
	if phone == "" {
		return ""
	}
	// 0086 138... / 00852 ... → +86 ... / +852 ...
	if strings.HasPrefix(phone, "00") {
		phone = "+" + strings.TrimPrefix(phone, "00")
	}
	switch {
	case strings.HasPrefix(phone, "+86"):
		phone = strings.TrimPrefix(phone, "+86")
	case strings.HasPrefix(phone, "+852"):
		phone = strings.TrimPrefix(phone, "+852")
	case strings.HasPrefix(phone, "+853"):
		phone = strings.TrimPrefix(phone, "+853")
	case strings.HasPrefix(phone, "+886"):
		phone = strings.TrimPrefix(phone, "+886")
		// International Taiwanese numbers drop the trunk "0" (9xx xxx xxx);
		// restore it so the stored form matches the local 09xxxxxxxx format.
		if len(phone) == 9 && strings.HasPrefix(phone, "9") {
			phone = "0" + phone
		}
	}
	return phone
}

// IsSupportedPhoneRegion reports whether the (already normalized) phone number
// matches one of the supported mobile formats.
func IsSupportedPhoneRegion(phone string) bool {
	switch {
	case mainlandPhonePattern.MatchString(phone):
		return true
	case hongKongPhonePattern.MatchString(phone):
		return true
	case macaoPhonePattern.MatchString(phone):
		return true
	case taiwanPhonePattern.MatchString(phone):
		return true
	}
	return false
}

// IsValidPhone normalizes the input and reports whether it is an acceptable
// mobile number.
func IsValidPhone(phone string) bool {
	normalized := NormalizePhone(phone)
	if normalized == "" {
		return false
	}
	return IsSupportedPhoneRegion(normalized)
}

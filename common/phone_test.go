package common

import "testing"

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"13800138000":       "13800138000",
		" 138 0013 8000 ":   "13800138000",
		"+86 138-0013-8000": "13800138000",
		"008613800138000":   "13800138000",
		"+852 6123 4567":    "61234567",
		"+853 66123456":     "66123456",
		"+886 912 345 678":  "0912345678",
		"(021) 1234-5678":   "02112345678",
		"":                  "",
		"+86":               "",
		"13800138000（备用）":   "13800138000备用",
		"13600000000\r\n":   "13600000000",
	}
	for input, want := range cases {
		if got := NormalizePhone(input); got != want {
			t.Errorf("NormalizePhone(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestIsValidPhone(t *testing.T) {
	valid := []string{
		"13800138000",
		"+86 138 0013 8000",
		"19912345678",
		"61234567",   // Hong Kong
		"51234567",   // Hong Kong
		"66123456",   // Macao / Hong Kong overlap
		"0912345678", // Taiwan
		"+886 912 345 678",
	}
	for _, phone := range valid {
		if !IsValidPhone(phone) {
			t.Errorf("IsValidPhone(%q) = false, want true", phone)
		}
	}

	invalid := []string{
		"",
		"12345",
		"12800138000",  // second digit 2 is not a mobile prefix
		"1380013800",   // 10 digits
		"138001380000", // 12 digits
		"1380013800a",
		"01234567890",
		"21234567",
		"091234567",   // Taiwan with 9 digits
		"09123456789", // Taiwan with 11 digits
	}
	for _, phone := range invalid {
		if IsValidPhone(phone) {
			t.Errorf("IsValidPhone(%q) = true, want false", phone)
		}
	}
}

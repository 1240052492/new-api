/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

/**
 * Phone number helpers shared by the sign-up form, the profile card and the
 * admin user drawer. The rules mirror the backend (common/phone.go): mainland
 * China, Hong Kong, Macao and Taiwan mobile numbers, stored as plain digits.
 */

const SEPARATORS = /[\s\-－()（）]/g

// Mainland China: 11 digits starting with 1.
const MAINLAND_PATTERN = /^1[3-9]\d{9}$/
// Hong Kong: 8 digits starting with 4, 5, 6, 7 or 9.
const HONG_KONG_PATTERN = /^[45679]\d{7}$/
// Macao: 8 digits starting with 6.
const MACAO_PATTERN = /^6\d{7}$/
// Taiwan: 10 digits starting with 09.
const TAIWAN_PATTERN = /^09\d{8}$/

/** Strips separators and supported country codes; returns plain digits. */
export function normalizePhone(raw: string): string {
  let phone = (raw ?? '').trim().replace(SEPARATORS, '')
  if (!phone) return ''
  if (phone.startsWith('00')) phone = `+${phone.slice(2)}`
  if (phone.startsWith('+86')) {
    phone = phone.slice(3)
  } else if (phone.startsWith('+852')) {
    phone = phone.slice(4)
  } else if (phone.startsWith('+853')) {
    phone = phone.slice(4)
  } else if (phone.startsWith('+886')) {
    phone = phone.slice(4)
    // International Taiwanese numbers drop the trunk "0".
    if (phone.length === 9 && phone.startsWith('9')) phone = `0${phone}`
  }
  return phone
}

/** Reports whether the value is a supported mobile number. */
export function isValidPhone(raw: string): boolean {
  const phone = normalizePhone(raw)
  if (!phone) return false
  return (
    MAINLAND_PATTERN.test(phone) ||
    HONG_KONG_PATTERN.test(phone) ||
    MACAO_PATTERN.test(phone) ||
    TAIWAN_PATTERN.test(phone)
  )
}

/**
 * Masks the middle digits for end-user surfaces (profile header, referral
 * invites). Administrator screens keep the full number, so this helper is never
 * used there.
 *
 * - 11 digits → `138****8000`
 * - 10 digits → `091****678`
 * - 8 digits  → `66***456`
 */
export function maskPhone(raw: string): string {
  const phone = normalizePhone(raw)
  if (!phone) return ''
  const visibleHead = 3
  const visibleTail = 4
  if (phone.length <= visibleHead + visibleTail) {
    // Too short to keep a meaningful tail: show the first digit only.
    return `${phone.slice(0, 1)}****`
  }
  const masked = '*'.repeat(phone.length - visibleHead - visibleTail)
  return `${phone.slice(0, visibleHead)}${masked}${phone.slice(-visibleTail)}`
}

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
import { describe, expect, it } from 'vitest'

import { isValidPhone, maskPhone, normalizePhone } from './phone'

describe('normalizePhone', () => {
  it('keeps digits and strips separators', () => {
    expect(normalizePhone(' 138 0013 8000 ')).toBe('13800138000')
    expect(normalizePhone('138-0013-8000')).toBe('13800138000')
  })

  it('drops supported country codes', () => {
    expect(normalizePhone('+86 138 0013 8000')).toBe('13800138000')
    expect(normalizePhone('008613800138000')).toBe('13800138000')
    expect(normalizePhone('+852 6123 4567')).toBe('61234567')
    expect(normalizePhone('+886 912 345 678')).toBe('0912345678')
  })

  it('returns an empty string for empty input', () => {
    expect(normalizePhone('')).toBe('')
    expect(normalizePhone('   ')).toBe('')
  })
})

describe('isValidPhone', () => {
  it('accepts mainland, Hong Kong, Macao and Taiwan mobiles', () => {
    expect(isValidPhone('13800138000')).toBe(true)
    expect(isValidPhone('+86 138 0013 8000')).toBe(true)
    expect(isValidPhone('61234567')).toBe(true)
    expect(isValidPhone('66123456')).toBe(true)
    expect(isValidPhone('0912345678')).toBe(true)
  })

  it('rejects malformed numbers', () => {
    expect(isValidPhone('')).toBe(false)
    expect(isValidPhone('12800138000')).toBe(false)
    expect(isValidPhone('1380013800')).toBe(false)
    expect(isValidPhone('138001380000')).toBe(false)
    expect(isValidPhone('1380013800a')).toBe(false)
    expect(isValidPhone('091234567')).toBe(false)
  })
})

describe('maskPhone', () => {
  it('keeps three leading and four trailing digits', () => {
    expect(maskPhone('13800138000')).toBe('138****8000')
    expect(maskPhone('0912345678')).toBe('091***5678')
    expect(maskPhone('61234567')).toBe('612*4567')
  })

  it('normalizes before masking', () => {
    expect(maskPhone('+86 138 0013 8000')).toBe('138****8000')
  })

  it('returns an empty string when there is nothing to mask', () => {
    expect(maskPhone('')).toBe('')
  })

  it('falls back to a short mask for very short input', () => {
    expect(maskPhone('1234567')).toBe('1****')
  })
})

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
import { Phone, Save } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import { isValidPhone, normalizePhone } from '@/lib/phone'

import type { UserProfile } from '../types'

interface AccountPhoneCardProps {
  profile: UserProfile | null
  loading: boolean
  /** Persists the new number; resolves to true when the update succeeded. */
  onSave: (phone: string) => Promise<boolean>
}

export function AccountPhoneCard({
  profile,
  loading,
  onSave,
}: AccountPhoneCardProps) {
  const { t } = useTranslation()
  const [phone, setPhone] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const savedPhone = profile?.phone ?? ''

  useEffect(() => {
    setPhone(savedPhone)
    setError('')
  }, [savedPhone])

  if (loading) {
    return (
      <TitledCard
        title={t('Account Information')}
        description={t('Contact details bound to your account')}
        icon={<Phone className='h-4 w-4' />}
        iconTone='info'
        disableHoverEffect
      >
        <Skeleton className='h-9 w-full' />
      </TitledCard>
    )
  }

  const normalized = normalizePhone(phone)
  const dirty = normalized !== savedPhone

  const handleSave = async () => {
    if (!normalized) {
      setError(t('Please enter your phone number'))
      return
    }
    if (!isValidPhone(normalized)) {
      setError(t('Please enter a valid phone number'))
      return
    }
    setError('')
    setSaving(true)
    const ok = await onSave(normalized)
    setSaving(false)
    if (!ok) {
      setError(t('Phone number is already in use'))
    }
  }

  return (
    <TitledCard
      title={t('Account Information')}
      description={t('Contact details bound to your account')}
      icon={<Phone className='h-4 w-4' />}
      iconTone='info'
      disableHoverEffect
    >
      <div className='space-y-2'>
        <div className='flex flex-col gap-2 sm:flex-row sm:items-center'>
          <Input
            type='tel'
            inputMode='tel'
            autoComplete='tel'
            className='sm:max-w-xs'
            placeholder={t('Enter your phone number')}
            value={phone}
            onChange={(event) => {
              setPhone(event.target.value)
              if (error) setError('')
            }}
            aria-invalid={Boolean(error)}
            aria-label={t('Phone Number')}
          />
          <Button
            type='button'
            onClick={handleSave}
            disabled={!dirty || saving}
            className='sm:w-auto'
          >
            <Save className='mr-1.5 size-4' />
            {t('Save')}
          </Button>
        </div>
        {error ? (
          <p className='text-destructive text-xs'>{error}</p>
        ) : (
          <p className='text-muted-foreground text-xs'>
            {t(
              'Supports mainland China, Hong Kong, Macao and Taiwan mobile numbers.'
            )}
          </p>
        )}
      </div>
    </TitledCard>
  )
}

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

For commercial licensing, please contact support@quantumnous.com
*/
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { formatQuota } from '@/lib/format'

import { formatTimestamp } from '../../lib/billing'
import type { AffiliateCommissionItem } from '../../types'

const PAGE_SIZE = 10
const SKELETON_ROWS = ['row-1', 'row-2', 'row-3', 'row-4']

interface AffiliateCommissionsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** When set, the dialog loads the commissions of that inviter (admin view). */
  inviterId?: number
  loadPage: (
    page: number,
    pageSize: number
  ) => Promise<{ items: AffiliateCommissionItem[]; total: number }>
}

export function AffiliateCommissionsDialog({
  open,
  onOpenChange,
  inviterId,
  loadPage,
}: AffiliateCommissionsDialogProps) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [items, setItems] = useState<AffiliateCommissionItem[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)

  // Keep the loader in a ref so an inline loader prop does not retrigger the
  // fetch effect on every render.
  const loadPageRef = useRef(loadPage)
  useEffect(() => {
    loadPageRef.current = loadPage
  })

  const fetchPage = useCallback(async (target: number) => {
    setLoading(true)
    try {
      const result = await loadPageRef.current(target, PAGE_SIZE)
      setItems(result.items)
      setTotal(result.total)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (!open) {
      return
    }
    setPage(1)
    void fetchPage(1)
  }, [open, fetchPage, inviterId])

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  const handlePage = (target: number) => {
    if (target < 1 || target > totalPages || target === page) {
      return
    }
    setPage(target)
    void fetchPage(target)
  }

  const sourceLabel = (source: string) => {
    if (source === 'redemption') {
      return t('Redemption code')
    }
    return t('Top-up')
  }

  const renderBody = () => {
    if (loading) {
      return (
        <div className='space-y-2'>
          {SKELETON_ROWS.map((row) => (
            <Skeleton key={row} className='h-10 w-full' />
          ))}
        </div>
      )
    }
    if (items.length === 0) {
      return (
        <p className='text-muted-foreground py-8 text-center text-sm'>
          {t('No commission records yet')}
        </p>
      )
    }
    return (
      <div className='overflow-x-auto'>
        <table className='w-full text-sm'>
          <thead>
            <tr className='text-muted-foreground border-border border-b text-left text-xs uppercase'>
              <th className='py-2 pr-3 font-medium'>{t('Time')}</th>
              <th className='py-2 pr-3 font-medium'>{t('Source')}</th>
              <th className='py-2 pr-3 font-medium'>{t('Invited user')}</th>
              <th className='py-2 pr-3 text-right font-medium'>
                {t('Paid amount')}
              </th>
              <th className='py-2 pr-3 text-right font-medium'>{t('Rate')}</th>
              <th className='py-2 text-right font-medium'>{t('Commission')}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.id} className='border-border/60 border-b'>
                <td className='py-2 pr-3 whitespace-nowrap tabular-nums'>
                  {formatTimestamp(item.create_time)}
                </td>
                <td className='py-2 pr-3 whitespace-nowrap'>
                  {sourceLabel(item.source_type)}
                </td>
                <td className='py-2 pr-3 whitespace-nowrap tabular-nums'>
                  #{item.invitee_id}
                </td>
                <td className='py-2 pr-3 text-right whitespace-nowrap tabular-nums'>
                  {formatCurrencyFromUSD(item.base_amount)}
                </td>
                <td className='py-2 pr-3 text-right whitespace-nowrap tabular-nums'>
                  {item.rate}%
                </td>
                <td className='py-2 text-right whitespace-nowrap tabular-nums'>
                  {formatQuota(item.quota)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    )
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Commission Details')}
      description={
        inviterId
          ? t(
              'Every commission earned from the payments of users invited by this account.'
            )
          : t(
              'Every commission earned from the payments of users you invited.'
            )
      }
      contentClassName='max-w-4xl'
    >
      <div className='space-y-3'>
        {renderBody()}


        <div className='flex items-center justify-between'>
          <span className='text-muted-foreground text-xs'>
            {t('{{count}} records', { count: total })}
          </span>
          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              size='sm'
              onClick={() => handlePage(page - 1)}
              disabled={page <= 1 || loading}
            >
              <ChevronLeft className='size-4' />
            </Button>
            <span className='text-xs tabular-nums'>
              {page} / {totalPages}
            </span>
            <Button
              variant='outline'
              size='sm'
              onClick={() => handlePage(page + 1)}
              disabled={page >= totalPages || loading}
            >
              <ChevronRight className='size-4' />
            </Button>
          </div>
        </div>
      </div>
    </Dialog>
  )
}

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
import i18next from 'i18next'
import { useCallback, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'

import { handleServerError } from '@/lib/handle-server-error'

import {
  isApiSuccess,
  queryAlipayPayment,
  queryWechatPayment,
  requestAlipayPayment,
  requestWechatPayment,
} from '../api'
import { isAlipayDirectPayment, isWechatDirectPayment } from '../lib'

// ============================================================================
// Direct Gateway Payment Hook (Alipay OpenAPI / WeChat Pay APIv3)
// ============================================================================

const POLL_INTERVAL_MS = 3000
const POLL_MAX_DURATION_MS = 5 * 60 * 1000

export interface WechatQrSession {
  codeUrl: string
  tradeNo: string
}

type DirectProvider = 'alipay' | 'wechatpay'

/**
 * Drives the two direct Chinese gateways:
 * - Alipay: opens the signed gateway redirect URL (page.pay / wap.pay), then
 *   polls the official query API as the 查单补偿 path.
 * - WeChat Pay: Native orders surface a QR dialog (code_url) while polling;
 *   H5 orders redirect the browser to h5_url per the official H5 flow.
 */
export function useDirectGatewayPayment(options?: {
  onPaid?: () => void
}) {
  const [processing, setProcessing] = useState(false)
  const [wechatQrSession, setWechatQrSession] =
    useState<WechatQrSession | null>(null)
  const pollTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const pollDeadlineRef = useRef(0)
  const onPaidRef = useRef(options?.onPaid)
  onPaidRef.current = options?.onPaid

  const stopPolling = useCallback(() => {
    if (pollTimerRef.current) {
      clearInterval(pollTimerRef.current)
      pollTimerRef.current = null
    }
  }, [])

  useEffect(() => stopPolling, [stopPolling])

  const startPolling = useCallback(
    (provider: DirectProvider, tradeNo: string) => {
      stopPolling()
      pollDeadlineRef.current = Date.now() + POLL_MAX_DURATION_MS
      pollTimerRef.current = setInterval(async () => {
        if (Date.now() > pollDeadlineRef.current) {
          stopPolling()
          return
        }
        try {
          const response =
            provider === 'alipay'
              ? await queryAlipayPayment(tradeNo)
              : await queryWechatPayment(tradeNo)
          if (isApiSuccess(response) && response.data?.paid) {
            stopPolling()
            setWechatQrSession(null)
            toast.success(i18next.t('Payment successful'))
            onPaidRef.current?.()
          }
        } catch {
          // Transient poll errors are ignored; the async notification is the
          // authoritative credit path and reconciliation stays server-side.
        }
      }, POLL_INTERVAL_MS)
    },
    [stopPolling]
  )

  const processDirectGatewayPayment = useCallback(
    async (topupAmount: number, paymentType: string) => {
      try {
        setProcessing(true)
        const amount = Math.floor(topupAmount)

        if (isAlipayDirectPayment(paymentType)) {
          const response = await requestAlipayPayment({
            amount,
            payment_method: paymentType,
          })
          if (!isApiSuccess(response) || !response.url) {
            handleServerError(response, i18next.t('Payment request failed'))
            return false
          }
          window.open(response.url, '_blank')
          toast.success(i18next.t('Redirecting to payment page...'))
          if (response.data?.trade_no) {
            startPolling('alipay', response.data.trade_no)
          }
          return true
        }

        if (isWechatDirectPayment(paymentType)) {
          const response = await requestWechatPayment({
            amount,
            payment_method: paymentType,
          })
          if (!isApiSuccess(response) || !response.data) {
            handleServerError(response, i18next.t('Payment request failed'))
            return false
          }
          const data = response.data
          if (data.channel === 'h5' && data.h5_url) {
            // 官方 H5 流程：微信外浏览器跳转中间页拉起微信支付收银台。
            window.location.assign(data.h5_url)
            if (data.trade_no) {
              startPolling('wechatpay', data.trade_no)
            }
            return true
          }
          if (data.code_url) {
            // 官方 Native 流程：前端将 code_url 渲染为二维码供扫码支付。
            setWechatQrSession({
              codeUrl: data.code_url,
              tradeNo: data.trade_no ?? '',
            })
            if (data.trade_no) {
              startPolling('wechatpay', data.trade_no)
            }
            return true
          }
          handleServerError(response, i18next.t('Payment request failed'))
          return false
        }

        return false
      } catch (error) {
        handleServerError(error, i18next.t('Payment request failed'))
        return false
      } finally {
        setProcessing(false)
      }
    },
    [startPolling]
  )

  const closeWechatQrSession = useCallback(() => {
    stopPolling()
    setWechatQrSession(null)
  }, [stopPolling])

  return {
    processing,
    processDirectGatewayPayment,
    wechatQrSession,
    closeWechatQrSession,
  }
}

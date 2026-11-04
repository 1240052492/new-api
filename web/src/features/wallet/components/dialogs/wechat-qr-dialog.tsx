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
import { QRCodeSVG } from 'qrcode.react'
import { useTranslation } from 'react-i18next'

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

import type { WechatQrSession } from '../../hooks/use-direct-gateway-payment'

interface WechatQrDialogProps {
  session: WechatQrSession | null
  onClose: () => void
}

/**
 * Renders the Native-order code_url as a QR code per the official WeChat
 * Pay Native flow: the user scans it with WeChat to complete payment, and
 * the order is credited by the async notification (or the polling
 * reconciliation path).
 */
export function WechatQrDialog({ session, onClose }: WechatQrDialogProps) {
  const { t } = useTranslation()

  return (
    <Dialog open={session !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className='sm:max-w-sm'>
        <DialogHeader>
          <DialogTitle>{t('WeChat Pay')}</DialogTitle>
          <DialogDescription>
            {t('Scan the QR code with WeChat to complete the payment.')}
          </DialogDescription>
        </DialogHeader>
        <div className='flex flex-col items-center gap-4 py-2'>
          {session ? (
            <>
              <div className='rounded-lg border bg-white p-3'>
                <QRCodeSVG
                  value={session.codeUrl}
                  size={200}
                  marginSize={0}
                  role='img'
                  aria-label={t('WeChat Pay QR code')}
                />
              </div>
              <p className='text-muted-foreground text-center text-sm'>
                {t('This page updates automatically once the payment is done.')}
              </p>
            </>
          ) : null}
        </div>
      </DialogContent>
    </Dialog>
  )
}

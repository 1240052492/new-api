/*
OAuth section registry for system settings navigation
*/

import type { TFunction } from 'i18next'
import { KeySquare } from 'lucide-react'
import type { NavItem } from '@/components/layout/types'

export function getOAuthSectionNavItems(t: TFunction): NavItem[] {
  return [
    {
      id: 'oauth-applications',
      title: t('OAuth Applications'),
      label: t('OAuth Applications'),
      icon: KeySquare,
      href: '/system-settings/oauth/applications',
      description: t('Manage OAuth 2.0 applications'),
    },
  ]
}

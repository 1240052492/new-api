import { zodResolver } from '@hookform/resolvers/zod'
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'
import type {
  HeaderNavModulesConfig,
  SidebarModulesAdminConfig,
} from '../maintenance/config'
import {
  serializeHeaderNavModules,
  serializeSidebarModulesAdmin,
} from '../maintenance/config'

const DEFAULT_INFINITE_CANVAS_URL = 'http://localhost:5173/'

type InfiniteCanvasSectionProps = {
  defaultValue: string
  headerNavConfig: HeaderNavModulesConfig
  headerNavSerialized: string
  sidebarConfig: SidebarModulesAdminConfig
  sidebarSerialized: string
}

type FormValues = {
  InfiniteCanvasURL: string
  showInTopNav: boolean
  showInSidebar: boolean
}

export function InfiniteCanvasSection({
  defaultValue,
  headerNavConfig,
  headerNavSerialized,
  sidebarConfig,
  sidebarSerialized,
}: InfiniteCanvasSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const schema = z.object({
    InfiniteCanvasURL: z.string().trim().url({
      error: () => t('Enter a valid canvas URL'),
    }),
    showInTopNav: z.boolean(),
    showInSidebar: z.boolean(),
  })

  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<FormValues>({
      resolver: zodResolver(schema) as Resolver<FormValues>,
      defaultValues: {
        InfiniteCanvasURL: defaultValue.trim() || DEFAULT_INFINITE_CANVAS_URL,
        showInTopNav: Boolean(headerNavConfig.infiniteCanvas ?? true),
        showInSidebar: Boolean(sidebarConfig.chat?.infiniteCanvas ?? true),
      },
      onSubmit: async (_data, changedFields) => {
        if ('InfiniteCanvasURL' in changedFields) {
          await updateOption.mutateAsync({
            key: 'InfiniteCanvasURL',
            value: String(changedFields.InfiniteCanvasURL).trim(),
          })
        }
        if ('showInTopNav' in changedFields) {
          const payload = serializeHeaderNavModules({
            ...headerNavConfig,
            infiniteCanvas: changedFields.showInTopNav,
          })
          if (payload !== headerNavSerialized) {
            await updateOption.mutateAsync({
              key: 'HeaderNavModules',
              value: payload,
            })
          }
        }
        if ('showInSidebar' in changedFields) {
          const nextSidebar = {
            ...sidebarConfig,
            chat: {
              ...sidebarConfig.chat,
              infiniteCanvas: changedFields.showInSidebar,
            },
          }
          const payload = serializeSidebarModulesAdmin(nextSidebar)
          if (payload !== sidebarSerialized) {
            await updateOption.mutateAsync({
              key: 'SidebarModulesAdmin',
              value: payload,
            })
          }
        }
      },
    })

  return (
    <>
      <FormNavigationGuard when={isDirty} />
      <SettingsSection title={t('Infinite Canvas')}>
        <Form {...form}>
          <SettingsForm onSubmit={handleSubmit}>
            <SettingsPageFormActions
              onSave={handleSubmit}
              onReset={handleReset}
              isSaving={isSubmitting || updateOption.isPending}
              isResetDisabled={!isDirty}
            />
            <FormDirtyIndicator isDirty={isDirty} />
            <SettingsFormGrid>
              <FormField
                control={form.control}
                name='InfiniteCanvasURL'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Infinite Canvas URL')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={DEFAULT_INFINITE_CANVAS_URL}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'URL used by the Infinite Canvas navigation entry. Use the local development URL during debugging and the production URL after deployment.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='showInTopNav'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Show in top navigation')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Show the Infinite Canvas entry in the top navigation.'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                    <FormMessage />
                  </SettingsSwitchItem>
                )}
              />
              <FormField
                control={form.control}
                name='showInSidebar'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Show in sidebar')}</FormLabel>
                      <FormDescription>
                        {t('Show the Infinite Canvas entry in the sidebar.')}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                    <FormMessage />
                  </SettingsSwitchItem>
                )}
              />
            </SettingsFormGrid>
          </SettingsForm>
        </Form>
      </SettingsSection>
    </>
  )
}

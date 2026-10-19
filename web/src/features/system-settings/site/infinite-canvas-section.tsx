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

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import { SettingsForm, SettingsFormGrid } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const DEFAULT_INFINITE_CANVAS_URL = 'http://localhost:5173/'

type InfiniteCanvasSectionProps = {
  defaultValue: string
}

type FormValues = {
  InfiniteCanvasURL: string
}

export function InfiniteCanvasSection({
  defaultValue,
}: InfiniteCanvasSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const schema = z.object({
    InfiniteCanvasURL: z.string().trim().url({
      error: () => t('Enter a valid canvas URL'),
    }),
  })

  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<FormValues>({
      resolver: zodResolver(schema) as Resolver<FormValues>,
      defaultValues: {
        InfiniteCanvasURL: defaultValue.trim() || DEFAULT_INFINITE_CANVAS_URL,
      },
      onSubmit: async (_data, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          await updateOption.mutateAsync({
            key,
            value: String(value).trim(),
          })
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
            </SettingsFormGrid>
          </SettingsForm>
        </Form>
      </SettingsSection>
    </>
  )
}

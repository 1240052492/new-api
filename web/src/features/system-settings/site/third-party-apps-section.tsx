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
import { zodResolver } from '@hookform/resolvers/zod'
import {
  ArrowDown,
  ArrowUp,
  KeyRound,
  Plus,
  Trash2,
} from 'lucide-react'
import { useFieldArray } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { CopyButton } from '@/components/copy-button'
import { Button } from '@/components/ui/button'
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
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

import {
  useCreateOAuthApplication,
} from '../auth/oauth-config/hooks'
import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  parseThirdPartyApps,
  serializeThirdPartyApps,
} from '@/lib/third-party-apps'

const appSchema = z.object({
  id: z.string().min(1),
  name: z
    .string()
    .trim()
    .min(1, { message: 'App name is required' })
    .max(100),
  url: z.string().trim().url({ message: 'Enter a valid URL' }),
  show_in_top_nav: z.boolean(),
  show_in_sidebar: z.boolean(),
  enabled: z.boolean(),
  oauth_client_id: z.string().optional(),
})

const formSchema = z.object({
  apps: z.array(appSchema),
})

type AppFormValues = z.infer<typeof appSchema>
type FormValues = z.infer<typeof formSchema>

function toCallbackUrl(appUrl: string): string | null {
  try {
    return `${new URL(appUrl.trim()).origin}/oauth/callback`
  } catch {
    return null
  }
}

function createEmptyApp(): AppFormValues {
  return {
    id:
      typeof crypto !== 'undefined' && 'randomUUID' in crypto
        ? crypto.randomUUID()
        : `app-${Date.now()}-${Math.round(Math.random() * 1e6)}`,
    name: '',
    url: '',
    show_in_top_nav: true,
    show_in_sidebar: true,
    enabled: true,
    oauth_client_id: undefined,
  }
}

export function ThirdPartyAppsSection({
  defaultValue,
}: {
  defaultValue: string
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const createOAuthApp = useCreateOAuthApplication()

  const initialApps = parseThirdPartyApps(defaultValue)

  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<FormValues>({
      resolver: zodResolver(formSchema),
      defaultValues: {
        apps: initialApps.map((app) => ({
          ...app,
          oauth_client_id: app.oauth_client_id ?? undefined,
        })),
      },
      onSubmit: async (_data) => {
        const apps = form.getValues('apps') ?? []
        await updateOption.mutateAsync({
          key: 'ThirdPartyApps',
          value: serializeThirdPartyApps(apps),
        })
      },
    })

  const { fields, append, remove, move, update } = useFieldArray({
    control: form.control,
    name: 'apps',
  })

  const handleQuickCreateOAuth = async (index: number) => {
    const app = form.getValues(`apps.${index}`)
    if (!app) return
    const callbackUrl = toCallbackUrl(app.url)
    if (!callbackUrl) {
      toast.error(t('Enter a valid URL'))
      return
    }
    try {
      const created = await createOAuthApp.mutateAsync({
        name: app.name.trim() || 'Third-party app',
        redirect_uris: [callbackUrl],
        scopes: 'user:read',
        description: `Created for third-party app entry: ${app.name || app.url}`,
      })
      form.setValue(`apps.${index}.oauth_client_id`, created.client_id, {
        shouldDirty: true,
      })
      update(index, {
        ...app,
        oauth_client_id: created.client_id,
      })
      toast.success(
        t('OAuth login created. Save the app list to persist the binding.')
      )
    } catch (error) {
      const message =
        error instanceof Error ? error.message : 'Failed to create application'
      toast.error(message)
    }
  }

  return (
    <>
      <FormNavigationGuard when={isDirty} />
      <SettingsSection title={t('Third-Party Apps')}>
        <FormDirtyIndicator isDirty={isDirty} />
        <SettingsPageFormActions
          onSave={handleSubmit}
          onReset={handleReset}
          isSaving={isSubmitting || updateOption.isPending}
          isResetDisabled={!isDirty}
        />
        <SettingsForm onSubmit={handleSubmit}>
          <Form {...form}>
          <div className='space-y-4'>
            <FormDescription>
              {t(
                'External systems shown in the top navigation and/or sidebar. Each entry carries its own display switches; use OAuth login to let users sign in through this platform.'
              )}
            </FormDescription>

            {fields.length === 0 && (
              <div className='rounded border border-dashed p-4 text-center text-muted-foreground'>
                {t('No third-party apps configured yet.')}
              </div>
            )}

            {fields.map((field, index) => {
              const clientId = form.watch(`apps.${index}.oauth_client_id`)
              const callbackUrl = toCallbackUrl(
                form.watch(`apps.${index}.url`) ?? ''
              )
              return (
                <div
                  key={field.id}
                  className='space-y-4 rounded-lg border p-4'
                >
                  <div className='flex items-center justify-between'>
                    <div className='flex items-center gap-2'>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon'
                        disabled={index === 0}
                        onClick={() => move(index, index - 1)}
                      >
                        <ArrowUp className='h-4 w-4' />
                      </Button>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon'
                        disabled={index === fields.length - 1}
                        onClick={() => move(index, index + 1)}
                      >
                        <ArrowDown className='h-4 w-4' />
                      </Button>
                      <span className='text-sm font-medium text-muted-foreground'>
                        #{index + 1}
                      </span>
                    </div>
                    <FormField
                      control={form.control}
                      name={`apps.${index}.enabled`}
                      render={({ field: enabledField }) => (
                        <div className='flex items-center gap-2'>
                          <Label htmlFor={`apps.${index}.enabled`}>
                            {t('Enabled')}
                          </Label>
                          <Switch
                            id={`apps.${index}.enabled`}
                            checked={enabledField.value}
                            onCheckedChange={enabledField.onChange}
                          />
                        </div>
                      )}
                    />
                  </div>

                  <SettingsFormGrid>
                    <FormField
                      control={form.control}
                      name={`apps.${index}.name`}
                      render={({ field: nameField }) => (
                        <FormItem>
                          <FormLabel>{t('App name')}</FormLabel>
                          <FormControl>
                            <Input
                              placeholder={t('e.g. Infinite Canvas')}
                              {...nameField}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'Shown as the navigation title for this entry.'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name={`apps.${index}.url`}
                      render={({ field: urlField }) => (
                        <FormItem>
                          <FormLabel>{t('Application URL')}</FormLabel>
                          <FormControl>
                            <Input
                              placeholder='https://canvas.example.com/'
                              {...urlField}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'Target address opened when users click the entry.'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </SettingsFormGrid>

                  <div className='grid gap-4 sm:grid-cols-2'>
                    <FormField
                      control={form.control}
                      name={`apps.${index}.show_in_top_nav`}
                      render={({ field: topNavField }) => (
                        <div className='flex items-center justify-between rounded border p-3'>
                          <div>
                            <Label htmlFor={`apps.${index}.show_in_top_nav`}>
                              {t('Show in top navigation')}
                            </Label>
                            <p className='text-xs text-muted-foreground'>
                              {t('Show this entry in the top navigation.')}
                            </p>
                          </div>
                          <Switch
                            id={`apps.${index}.show_in_top_nav`}
                            checked={topNavField.value}
                            onCheckedChange={topNavField.onChange}
                          />
                        </div>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name={`apps.${index}.show_in_sidebar`}
                      render={({ field: sidebarField }) => (
                        <div className='flex items-center justify-between rounded border p-3'>
                          <div>
                            <Label htmlFor={`apps.${index}.show_in_sidebar`}>
                              {t('Show in sidebar')}
                            </Label>
                            <p className='text-xs text-muted-foreground'>
                              {t('Show this entry in the sidebar.')}
                            </p>
                          </div>
                          <Switch
                            id={`apps.${index}.show_in_sidebar`}
                            checked={sidebarField.value}
                            onCheckedChange={sidebarField.onChange}
                          />
                        </div>
                      )}
                    />
                  </div>

                  <div className='rounded border bg-muted/40 p-3'>
                    <div className='flex items-center justify-between gap-2'>
                      <div className='flex items-center gap-2'>
                        <KeyRound className='h-4 w-4 text-muted-foreground' />
                        <span className='text-sm font-medium'>
                          {t('OAuth login')}
                        </span>
                      </div>
                      {clientId ? (
                        <div className='flex items-center gap-2'>
                          <code className='rounded bg-muted px-2 py-1 text-xs'>
                            {clientId}
                          </code>
                          <CopyButton
                            value={clientId}
                            variant='ghost'
                            size='sm'
                          />
                        </div>
                      ) : (
                        <Button
                          type='button'
                          size='sm'
                          variant='outline'
                          disabled={createOAuthApp.isPending}
                          onClick={() => handleQuickCreateOAuth(index)}
                        >
                          {createOAuthApp.isPending
                            ? t('Creating...')
                            : t('Create OAuth login')}
                        </Button>
                      )}
                    </div>
                    {callbackUrl && (
                      <p className='mt-2 text-xs text-muted-foreground'>
                        {t('Callback URL')}:{' '}
                        <code className='rounded bg-muted px-1 py-0.5'>
                          {callbackUrl}
                        </code>
                        <CopyButton
                          value={callbackUrl}
                          variant='ghost'
                          size='sm'
                        />
                      </p>
                    )}
                  </div>

                  <div className='flex justify-end'>
                    <Button
                      type='button'
                      variant='destructive'
                      size='sm'
                      onClick={() => remove(index)}
                    >
                      <Trash2 className='mr-2 h-4 w-4' />
                      {t('Delete')}
                    </Button>
                  </div>
                </div>
              )
            })}

            <div>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() => append(createEmptyApp())}
              >
                <Plus className='mr-2 h-4 w-4' />
                {t('Add app')}
              </Button>
            </div>
          </div>
          </Form>
        </SettingsForm>
      </SettingsSection>
    </>
  )
}

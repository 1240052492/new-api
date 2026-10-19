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
import { Plus, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { CopyButton } from '@/components/copy-button'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'

import { SettingsSection } from '../components/settings-section'
import {
  useCreateOAuthApplication,
  useDeleteOAuthApplication,
  useOAuthApplications,
} from './oauth-config/hooks'

const createApplicationSchema = z.object({
  name: z.string().min(1, 'Application name is required').max(100),
  redirectUris: z
    .string()
    .min(1, 'At least one redirect URI is required')
    .transform((val) => val.split('\n').filter((uri) => uri.trim()))
    .refine((uris) => uris.length > 0, 'At least one valid redirect URI is required'),
  scopes: z.string().optional(),
  autoApprove: z.boolean().optional().default(false),
  description: z.string().optional(),
})

type CreateApplicationForm = z.infer<typeof createApplicationSchema>

export function OAuthConfigSection() {
  const { t } = useTranslation()
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [newApplicationSecret, setNewApplicationSecret] = useState<string | null>(null)
  const [secretCopyTimeout, setSecretCopyTimeout] = useState(false)

  const { data: applicationsData, isLoading, error } = useOAuthApplications()
  const createMutation = useCreateOAuthApplication()
  const deleteMutation = useDeleteOAuthApplication()

  const form = useForm<CreateApplicationForm>({
    resolver: zodResolver(createApplicationSchema),
    defaultValues: {
      name: '',
      redirectUris: '',
      scopes: 'user:read',
      autoApprove: false,
      description: '',
    },
  })

  const handleCreateApplication = async (data: CreateApplicationForm) => {
    try {
      const result = await createMutation.mutateAsync({
        name: data.name,
        redirect_uris: data.redirectUris,
        scopes: data.scopes,
        auto_approve: data.autoApprove,
        description: data.description,
      })

      // Show the client secret (only shown once)
      if (result.client_secret) {
        setNewApplicationSecret(result.client_secret)
      }

      toast.success('OAuth application created successfully')
      setIsCreateDialogOpen(false)
      form.reset()
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to create application'
      toast.error(message)
    }
  }

  const handleDeleteApplication = async (id: number, name: string) => {
    if (!confirm(`Are you sure you want to delete the OAuth application "${name}"?`)) {
      return
    }

    try {
      await deleteMutation.mutateAsync(id)
      toast.success('OAuth application deleted successfully')
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to delete application'
      toast.error(message)
    }
  }

  const handleCopySecret = async () => {
    if (newApplicationSecret) {
      await navigator.clipboard.writeText(newApplicationSecret)
      setSecretCopyTimeout(true)
      setTimeout(() => setSecretCopyTimeout(false), 2000)
    }
  }

  const getServerUrl = () => {
    if (typeof window !== 'undefined') {
      return `${window.location.protocol}//${window.location.host}`
    }
    return 'http://localhost:3002'
  }

  const callbackUrlFormat = `${getServerUrl()}/oauth/callback`

  return (
    <SettingsSection title="OAuth Applications">
      <div className="space-y-6">
        {/* Redirect URI Format Information */}
        <Alert>
          <AlertTitle>OAuth Callback URL Format</AlertTitle>
          <AlertDescription>
            <div className="mt-2 space-y-2">
              <p>When registering your OAuth application, use this callback URL format:</p>
              <div className="flex items-center gap-2 rounded bg-muted p-2">
                <code className="flex-1 text-sm">{callbackUrlFormat}</code>
                <CopyButton value={callbackUrlFormat} variant="ghost" size="sm" />
              </div>
              <p className="text-xs text-muted-foreground">
                This is the URL where users will be redirected after authorization.
              </p>
            </div>
          </AlertDescription>
        </Alert>

        {/* New Application Secret Display */}
        {newApplicationSecret && (
          <Alert className="border-green-200 bg-green-50">
            <AlertTitle className="text-green-900">Client Secret Generated</AlertTitle>
            <AlertDescription className="text-green-800">
              <div className="mt-2 space-y-2">
                <p className="text-sm">
                  Save this client secret securely. It will not be shown again.
                </p>
                <div className="flex items-center gap-2 rounded bg-white p-2">
                  <code className="flex-1 break-all text-xs font-mono">{newApplicationSecret}</code>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={handleCopySecret}
                    disabled={secretCopyTimeout}
                  >
                    {secretCopyTimeout ? 'Copied!' : 'Copy'}
                  </Button>
                </div>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => setNewApplicationSecret(null)}
                  className="mt-2"
                >
                  Dismiss
                </Button>
              </div>
            </AlertDescription>
          </Alert>
        )}

        {/* Applications List */}
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="font-medium">Your OAuth Applications</h3>
            <Button
              size="sm"
              onClick={() => setIsCreateDialogOpen(true)}
              disabled={createMutation.isPending}
            >
              <Plus className="mr-2 h-4 w-4" />
              Add Application
            </Button>
          </div>

          {isLoading ? (
            <div className="rounded border border-dashed p-4 text-center text-muted-foreground">
              Loading applications...
            </div>
          ) : error ? (
            <div className="rounded border border-red-200 bg-red-50 p-4 text-red-800">
              Failed to load applications: {error instanceof Error ? error.message : 'Unknown error'}
            </div>
          ) : applicationsData?.data && applicationsData.data.length > 0 ? (
            <div className="border rounded-lg overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow className="bg-muted/50">
                    <TableHead>Name</TableHead>
                    <TableHead>Client ID</TableHead>
                    <TableHead>Scopes</TableHead>
                    <TableHead>Created</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {applicationsData.data.map((app) => (
                    <TableRow key={app.id}>
                      <TableCell className="font-medium">{app.name}</TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <code className="text-xs bg-muted px-2 py-1 rounded">
                            {app.client_id}
                          </code>
                          <CopyButton value={app.client_id} variant="ghost" size="sm" />
                        </div>
                      </TableCell>
                      <TableCell className="text-sm">
                        {app.scopes || 'user:read'}
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground">
                        {new Date(app.created_at).toLocaleDateString()}
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          size="sm"
                          variant="destructive"
                          onClick={() => handleDeleteApplication(app.id, app.name)}
                          disabled={deleteMutation.isPending}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          ) : (
            <div className="rounded border border-dashed p-4 text-center text-muted-foreground">
              <p>No OAuth applications yet.</p>
              <p className="text-sm">Create one to get started.</p>
            </div>
          )}
        </div>
      </div>

      {/* Create Application Dialog */}
      <Dialog open={isCreateDialogOpen} onOpenChange={setIsCreateDialogOpen}>
        <DialogContent className="sm:max-w-[500px]">
          <DialogHeader>
            <DialogTitle>Create OAuth Application</DialogTitle>
            <DialogDescription>
              Create a new OAuth application to allow users to authorize through your service.
            </DialogDescription>
          </DialogHeader>

          <Form {...form}>
            <form
              onSubmit={form.handleSubmit(handleCreateApplication)}
              className="space-y-4"
            >
              <FormField
                control={form.control}
                name="name"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Application Name</FormLabel>
                    <FormControl>
                      <Input placeholder="e.g., My App" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="redirectUris"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Redirect URIs</FormLabel>
                    <FormControl>
                      <Textarea
                        placeholder="Enter one URI per line&#10;e.g.&#10;http://localhost:5173/oauth/callback&#10;https://example.com/oauth/callback"
                        rows={4}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      The URLs where users will be redirected after authorization
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="scopes"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Scopes (Optional)</FormLabel>
                    <FormControl>
                      <Input
                        placeholder="e.g., user:read"
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      Comma-separated list of scopes (default: user:read)
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="description"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Description (Optional)</FormLabel>
                    <FormControl>
                      <Textarea
                        placeholder="Describe the purpose of this application"
                        rows={3}
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <DialogFooter>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => setIsCreateDialogOpen(false)}
                >
                  Cancel
                </Button>
                <Button type="submit" disabled={createMutation.isPending}>
                  {createMutation.isPending ? 'Creating...' : 'Create Application'}
                </Button>
              </DialogFooter>
            </form>
          </Form>
        </DialogContent>
      </Dialog>
    </SettingsSection>
  )
}

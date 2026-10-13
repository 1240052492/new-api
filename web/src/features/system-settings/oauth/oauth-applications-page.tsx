/*
OAuth Applications Management Page for new-api
*/

import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { AlertCircle, Copy, Trash2, Plus } from 'lucide-react'

export default function OAuthApplicationsPage() {
  const [applications, setApplications] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    fetchApplications()
  }, [])

  const fetchApplications = async () => {
    try {
      setLoading(true)
      const response = await fetch('/api/admin/oauth/applications', {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('access_token')}`
        }
      })
      if (response.ok) {
        const data = await response.json()
        setApplications(data.data || [])
      } else {
        setError('Failed to load OAuth applications')
      }
    } catch (err) {
      setError('Error fetching applications')
    } finally {
      setLoading(false)
    }
  }

  const deleteApplication = async (id: number) => {
    if (!confirm('Are you sure you want to delete this application?')) return

    try {
      const response = await fetch(`/api/admin/oauth/applications/${id}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('access_token')}`
        }
      })
      if (response.ok) {
        setApplications(applications.filter(app => app.id !== id))
      } else {
        setError('Failed to delete application')
      }
    } catch (err) {
      setError('Error deleting application')
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold">OAuth Applications</h1>
          <p className="text-muted-foreground mt-1">
            Manage OAuth 2.0 applications for third-party integrations
          </p>
        </div>
        <Button size="lg">
          <Plus className="w-4 h-4 mr-2" />
          New Application
        </Button>
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {loading ? (
        <div className="text-center py-12">Loading...</div>
      ) : applications.length === 0 ? (
        <Card>
          <CardContent className="pt-6">
            <div className="text-center py-12">
              <p className="text-muted-foreground mb-4">No OAuth applications yet</p>
              <Button>Create your first application</Button>
            </div>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4">
          {applications.map((app) => (
            <Card key={app.id}>
              <CardHeader>
                <CardTitle>{app.name}</CardTitle>
                <CardDescription>{app.description}</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="text-sm font-medium">Client ID</label>
                    <div className="flex items-center gap-2 mt-1">
                      <code className="bg-muted px-2 py-1 rounded text-sm flex-1 truncate">
                        {app.client_id}
                      </code>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => copyToClipboard(app.client_id)}
                      >
                        <Copy className="w-4 h-4" />
                      </Button>
                    </div>
                  </div>

                  <div>
                    <label className="text-sm font-medium">Redirect URIs</label>
                    <div className="mt-1">
                      {app.redirect_uris?.map((uri: string, idx: number) => (
                        <div key={idx} className="text-sm text-muted-foreground">
                          {uri}
                        </div>
                      ))}
                    </div>
                  </div>
                </div>

                <div className="flex justify-between items-center">
                  <div>
                    <span className="text-sm text-muted-foreground">Created: </span>
                    <span className="text-sm">
                      {new Date(app.created_at).toLocaleDateString()}
                    </span>
                  </div>
                  <Button
                    size="sm"
                    variant="destructive"
                    onClick={() => deleteApplication(app.id)}
                  >
                    <Trash2 className="w-4 h-4 mr-2" />
                    Delete
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* infinite-canvas Information Card */}
      <Card className="border-blue-200 bg-blue-50">
        <CardHeader>
          <CardTitle className="text-blue-900">Infinite Canvas Integration</CardTitle>
          <CardDescription className="text-blue-700">
            Pre-configured OAuth application for Infinite Canvas
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <Alert>
            <AlertCircle className="h-4 w-4" />
            <AlertDescription>
              The default "infinite-canvas-client" application is automatically configured during system initialization.
            </AlertDescription>
          </Alert>

          <div className="grid gap-2 text-sm">
            <div>
              <span className="font-medium">Client ID:</span>
              <code className="bg-white px-2 py-1 rounded ml-2">infinite-canvas-client</code>
            </div>
            <div>
              <span className="font-medium">Authorization URL:</span>
              <code className="bg-white px-2 py-1 rounded ml-2">http://localhost:3002/api/oauth/authorize</code>
            </div>
            <div>
              <span className="font-medium">Token URL:</span>
              <code className="bg-white px-2 py-1 rounded ml-2">http://localhost:3002/api/oauth/token</code>
            </div>
            <div>
              <span className="font-medium">Redirect URI:</span>
              <code className="bg-white px-2 py-1 rounded ml-2">http://localhost:5173/oauth/callback</code>
            </div>
            <div>
              <span className="font-medium">Scope:</span>
              <code className="bg-white px-2 py-1 rounded ml-2">user:read</code>
            </div>
          </div>

          <Button variant="outline" className="w-full">
            Open Infinite Canvas
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}

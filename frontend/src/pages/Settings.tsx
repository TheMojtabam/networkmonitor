import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { LogOut } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { alerts as alertsAPI, setToken } from '@/lib/api'
import { useApp } from '@/store/app'
import { PageHeader } from '@/components/layout/PageHeader'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Skeleton } from '@/components/ui/skeleton'
import type { AlertChannel } from '@/types'

export function Settings() {
  const navigate = useNavigate()
  const qc = useQueryClient()
  const username = useApp((s) => s.username)
  const authDisabled = useApp((s) => s.authDisabled)

  const { data: channelsData, isLoading } = useQuery({
    queryKey: ['alerts', 'channels'],
    queryFn: () => alertsAPI.channels(),
  })

  const channels = channelsData?.channels ?? []

  const setChannels = useMutation({
    mutationFn: (next: AlertChannel[]) => alertsAPI.setChannels(next),
    onSuccess: () => {
      toast.success('Channels updated')
      qc.invalidateQueries({ queryKey: ['alerts', 'channels'] })
    },
    onError: () => toast.error('Failed to save channels'),
  })

  const [webhookURL, setWebhookURL] = useState('')
  const [tgChatId, setTgChatId] = useState('')
  const [tgBotToken, setTgBotToken] = useState('')

  const handleSaveChannels = () => {
    const next: AlertChannel[] = []
    if (webhookURL) {
      next.push({ name: 'webhook', type: 'webhook', webhook: webhookURL })
    }
    if (tgChatId && tgBotToken) {
      next.push({
        name: 'telegram',
        type: 'telegram',
        telegram: { chatId: tgChatId },
      })
    }
    setChannels.mutate(next)
  }

  const handleLogout = () => {
    setToken(null)
    navigate('/login', { replace: true })
  }

  return (
    <div className="p-6 lg:p-8">
      <PageHeader
        title="Settings"
        subtitle="Notification channels, retention, and account"
      />

      <div className="flex flex-col gap-3.5">
        <Card>
          <CardHeader>
            <CardTitle>Notification channels</CardTitle>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className="h-32" />
            ) : (
              <div className="flex flex-col gap-3">
                <SettingsRow
                  title="Webhook URL"
                  description="POST JSON payload on alert trigger"
                >
                  <Input
                    placeholder="https://hooks.example.com/portsleuth"
                    value={webhookURL}
                    onChange={(e) => setWebhookURL(e.target.value)}
                    className="w-72"
                  />
                </SettingsRow>
                <SettingsRow
                  title="Telegram bot token"
                  description="Sends to chat ID configured below"
                >
                  <Input
                    type="password"
                    placeholder="••••••••"
                    value={tgBotToken}
                    onChange={(e) => setTgBotToken(e.target.value)}
                    className="w-72"
                  />
                </SettingsRow>
                <SettingsRow title="Telegram chat ID" description="User or group ID">
                  <Input
                    placeholder="-1001234567890"
                    value={tgChatId}
                    onChange={(e) => setTgChatId(e.target.value)}
                    className="w-72"
                  />
                </SettingsRow>
                <div className="flex justify-end pt-2">
                  <Button
                    variant="primary"
                    onClick={handleSaveChannels}
                    disabled={setChannels.isPending}
                  >
                    {setChannels.isPending ? 'Saving…' : 'Save channels'}
                  </Button>
                </div>
                {channels.length > 0 && (
                  <div className="mt-3 rounded-md border border-border-subtle bg-bg-elevated p-3 text-xs">
                    <div className="mb-1 font-semibold text-fg-secondary">
                      Currently configured:
                    </div>
                    {channels.map((c) => (
                      <div key={c.name} className="font-mono text-fg-tertiary">
                        {c.name} ({c.type}){' '}
                        {c.webhook && <span>→ {c.webhook}</span>}
                        {c.telegram?.chatId && (
                          <span>→ chat {c.telegram.chatId}</span>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Display</CardTitle>
          </CardHeader>
          <CardContent>
            <SettingsRow title="Theme" description="Dark only for now">
              <Button variant="default" disabled>
                Dark
              </Button>
            </SettingsRow>
            <SettingsRow title="Language" description="English">
              <Button variant="default" disabled>
                English
              </Button>
            </SettingsRow>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Account</CardTitle>
          </CardHeader>
          <CardContent>
            <SettingsRow
              title="Authentication"
              description={authDisabled ? 'Disabled in config' : 'JWT-based'}
            >
              <Switch checked={!authDisabled} disabled />
            </SettingsRow>
            {!authDisabled && (
              <SettingsRow
                title="Signed in as"
                description={username || 'unknown'}
              >
                <Button variant="default" onClick={handleLogout}>
                  <LogOut size={14} />
                  Sign out
                </Button>
              </SettingsRow>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>About</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 gap-y-2 text-xs">
              <span className="text-fg-secondary">Version</span>
              <span className="font-mono">1.0.0</span>
              <span className="text-fg-secondary">License</span>
              <span className="font-mono">MIT</span>
              <span className="text-fg-secondary">Source</span>
              <a
                className="font-mono text-accent hover:underline"
                href="https://github.com/TheMojtabam/networkmonitor"
                target="_blank"
                rel="noopener noreferrer"
              >
                github.com/TheMojtabam/networkmonitor
              </a>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

function SettingsRow({
  title,
  description,
  children,
}: {
  title: string
  description?: string
  children: React.ReactNode
}) {
  return (
    <div className="flex items-center justify-between gap-4 border-b border-border-subtle py-3 last:border-b-0">
      <div className="min-w-0">
        <div className="text-sm font-medium">{title}</div>
        {description && (
          <div className="mt-0.5 text-xs text-fg-secondary">{description}</div>
        )}
      </div>
      <div>{children}</div>
    </div>
  )
}

import { ChangePasswordForm } from '@/components/settings/access/change-password-form'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useAuth } from '@/contexts/AuthContext'

export default function AccountPage() {
  const { user } = useAuth()

  return (
    <div className="container space-y-6 p-4">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Account</h1>
        <p className="text-sm text-muted-foreground">Manage your account settings and preferences</p>
      </div>

      <div className="grid gap-6">
        <Card>
          <CardHeader>
            <CardTitle>Profile</CardTitle>
            <CardDescription>Your account information</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div>
                <h3 className="text-sm font-medium">Username</h3>
                <p className="text-sm text-muted-foreground">{user?.username}</p>
              </div>
              <div>
                <h3 className="text-sm font-medium">Role</h3>
                <p className="text-sm text-muted-foreground capitalize">{user?.role}</p>
              </div>
            </div>
          </CardContent>
        </Card>

        <ChangePasswordForm />
      </div>
    </div>
  )
} 
import { ChangePasswordForm } from '@/components/settings/access/change-password-form'

export default function AccessControlPage() {
  return (
    <div className="container space-y-6 p-4">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Access Control</h1>
        <p className="text-sm text-muted-foreground">Manage your account access and security settings</p>
      </div>

      <div className="grid gap-6">
        <ChangePasswordForm />
      </div>
    </div>
  )
} 
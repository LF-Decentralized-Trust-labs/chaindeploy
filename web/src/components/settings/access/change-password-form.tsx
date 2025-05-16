import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import * as z from 'zod'
import { Button } from '@/components/ui/button'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useMutation } from '@tanstack/react-query'
import { toast } from 'sonner'
import { postAuthChangePasswordMutation } from '@/api/client/@tanstack/react-query.gen'

const formSchema = z.object({
  current_password: z.string().min(1, 'Current password is required'),
  new_password: z.string()
    .min(8, 'Password must be at least 8 characters')
    .regex(/[A-Z]/, 'Password must contain at least one uppercase letter')
    .regex(/[a-z]/, 'Password must contain at least one lowercase letter')
    .regex(/[0-9]/, 'Password must contain at least one number')
    .regex(/[^A-Za-z0-9]/, 'Password must contain at least one special character'),
})

type FormValues = z.infer<typeof formSchema>

export function ChangePasswordForm() {
  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      current_password: '',
      new_password: '',
    },
  })

  const changePassword = useMutation({
    ...postAuthChangePasswordMutation(),
    onSuccess: () => {
      toast.success('Password changed successfully')
      form.reset()
    },
    onError: (error: any) => {
      if (error.status === 403) {
        toast.error('Current password is incorrect')
      } else if (error.error.message) {
        toast.error(`Failed to change password: ${error.error.message}`)
      } else if (error.message) {
        toast.error(`Failed to change password: ${error.message}`)
      } else {
        toast.error('An unknown error occurred')
      }
    },
  })

  function onSubmit(data: FormValues) {
    changePassword.mutate({
      body: {
        current_password: data.current_password,
        new_password: data.new_password,
      },
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Change Password</CardTitle>
        <CardDescription>Update your account password</CardDescription>
      </CardHeader>
      <CardContent>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
            <FormField
              control={form.control}
              name="current_password"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Current Password</FormLabel>
                  <FormControl>
                    <Input type="password" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="new_password"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>New Password</FormLabel>
                  <FormControl>
                    <Input type="password" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <Button type="submit" disabled={changePassword.isPending}>
              {changePassword.isPending ? 'Changing Password...' : 'Change Password'}
            </Button>
          </form>
        </Form>
      </CardContent>
    </Card>
  )
} 
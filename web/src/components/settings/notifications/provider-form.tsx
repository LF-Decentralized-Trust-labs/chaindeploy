import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Switch } from '@/components/ui/switch'

const providerFormSchema = z.object({
	name: z.string().min(1, 'Name is required'),
	type: z.literal('SMTP'),
	isDefault: z.boolean(),
	config: z.object({
		host: z.string().min(1, 'SMTP host is required'),
		port: z.coerce.number().min(1, 'SMTP port is required'),
		username: z.string().min(1, 'SMTP username is required'),
		password: z.string().min(1, 'SMTP password is required'),
		from: z.string().email('Must be a valid email address'),
		tls: z.boolean().optional(),
	}),
	notifyNodeDowntime: z.boolean().optional(),
	notifyBackupSuccess: z.boolean().optional(),
	notifyBackupFailure: z.boolean().optional(),
	notifyS3ConnIssue: z.boolean().optional(),
	notifyDiskSpaceWarning: z.boolean().optional(),
})

export type ProviderFormValues = z.infer<typeof providerFormSchema>

interface ProviderFormProps {
	defaultValues: ProviderFormValues
	onSubmit: (values: ProviderFormValues) => Promise<void>
	submitText: string
	onCancel: () => void
	isLoading?: boolean
}

export function ProviderForm({ defaultValues, onSubmit, submitText, onCancel, isLoading }: ProviderFormProps) {
	const form = useForm<ProviderFormValues>({
		resolver: zodResolver(providerFormSchema),
		defaultValues,
	})

	return (
		<Form {...form}>
			<form onSubmit={form.handleSubmit(async (values) => await onSubmit(values))} className="space-y-6">
				<Card>
					<CardHeader>
						<CardTitle>Provider Details</CardTitle>
						<CardDescription>Configure the SMTP provider settings</CardDescription>
					</CardHeader>
					<CardContent className="space-y-4">
						<FormField
							control={form.control}
							name="name"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Provider Name</FormLabel>
									<FormControl>
										<Input {...field} placeholder="My SMTP Provider" />
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>

						<FormField
							control={form.control}
							name="isDefault"
							render={({ field }) => (
								<FormItem className="flex flex-row items-start space-x-3 space-y-0">
									<FormControl>
										<Checkbox checked={field.value} onCheckedChange={field.onChange} />
									</FormControl>
									<div className="space-y-1 leading-none">
										<FormLabel>Set as Default Provider</FormLabel>
										<FormDescription>Use this provider for all notifications by default</FormDescription>
									</div>
								</FormItem>
							)}
						/>

						<div className="grid gap-4 md:grid-cols-2">
							<FormField
								control={form.control}
								name="config.host"
								render={({ field }) => (
									<FormItem>
										<FormLabel>SMTP Host</FormLabel>
										<FormControl>
											<Input {...field} placeholder="smtp.example.com" />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>

							<FormField
								control={form.control}
								name="config.port"
								render={({ field }) => (
									<FormItem>
										<FormLabel>SMTP Port</FormLabel>
										<FormControl>
											<Input {...field} type="number" placeholder="587" />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>

							<FormField
								control={form.control}
								name="config.username"
								render={({ field }) => (
									<FormItem>
										<FormLabel>SMTP Username</FormLabel>
										<FormControl>
											<Input {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>

							<FormField
								control={form.control}
								name="config.password"
								render={({ field }) => (
									<FormItem>
										<FormLabel>SMTP Password</FormLabel>
										<FormControl>
											<Input {...field} type="password" />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>

							<FormField
								control={form.control}
								name="config.from"
								render={({ field }) => (
									<FormItem>
										<FormLabel>From Address</FormLabel>
										<FormControl>
											<Input {...field} placeholder="notifications@example.com" />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>

							<FormField
								control={form.control}
								name="config.tls"
								render={({ field }) => (
									<FormItem className="flex flex-row items-center justify-between rounded-lg border p-4">
										<div className="space-y-0.5">
											<FormLabel>Use TLS</FormLabel>
											<FormDescription>
												Enable TLS encryption for secure email transmission
											</FormDescription>
										</div>
										<FormControl>
											<Switch
												checked={field.value}
												onCheckedChange={field.onChange}
											/>
										</FormControl>
									</FormItem>
								)}
							/>
						</div>
					</CardContent>
				</Card>

				<Card>
					<CardHeader>
						<CardTitle>Notification Settings</CardTitle>
						<CardDescription>Configure which events this provider handles</CardDescription>
					</CardHeader>
					<CardContent className="space-y-4">
						<FormField
							control={form.control}
							name="notifyNodeDowntime"
							render={({ field }) => (
								<FormItem className="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4">
									<FormControl>
										<Checkbox checked={field.value} onCheckedChange={field.onChange} />
									</FormControl>
									<div className="space-y-1 leading-none">
										<FormLabel>Node Downtime</FormLabel>
										<FormDescription>
											Notify when nodes become unavailable
										</FormDescription>
									</div>
								</FormItem>
							)}
						/>

						<FormField
							control={form.control}
							name="notifyBackupSuccess"
							render={({ field }) => (
								<FormItem className="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4">
									<FormControl>
										<Checkbox checked={field.value} onCheckedChange={field.onChange} />
									</FormControl>
									<div className="space-y-1 leading-none">
										<FormLabel>Backup Success</FormLabel>
										<FormDescription>
											Notify when backups complete successfully
										</FormDescription>
									</div>
								</FormItem>
							)}
						/>

						<FormField
							control={form.control}
							name="notifyBackupFailure"
							render={({ field }) => (
								<FormItem className="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4">
									<FormControl>
										<Checkbox checked={field.value} onCheckedChange={field.onChange} />
									</FormControl>
									<div className="space-y-1 leading-none">
										<FormLabel>Backup Failures</FormLabel>
										<FormDescription>
											Notify when backups fail
										</FormDescription>
									</div>
								</FormItem>
							)}
						/>

						<FormField
							control={form.control}
							name="notifyS3ConnIssue"
							render={({ field }) => (
								<FormItem className="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4">
									<FormControl>
										<Checkbox checked={field.value} onCheckedChange={field.onChange} />
									</FormControl>
									<div className="space-y-1 leading-none">
										<FormLabel>S3 Connection Issues</FormLabel>
										<FormDescription>
											Notify when there are problems connecting to S3 storage
										</FormDescription>
									</div>
								</FormItem>
							)}
						/>

						<FormField
							control={form.control}
							name="notifyDiskSpaceWarning"
							render={({ field }) => (
								<FormItem className="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4">
									<FormControl>
										<Checkbox checked={field.value} onCheckedChange={field.onChange} />
									</FormControl>
									<div className="space-y-1 leading-none">
										<FormLabel>Disk Space Warnings</FormLabel>
										<FormDescription>
											Notify when disk space usage exceeds threshold (80%)
										</FormDescription>
									</div>
								</FormItem>
							)}
						/>
					</CardContent>
				</Card>

				<div className="flex gap-4">
					<Button type="submit" disabled={isLoading}>
						{submitText}
					</Button>
					<Button type="button" variant="outline" onClick={onCancel}>
						Cancel
					</Button>
				</div>
			</form>
		</Form>
	)
} 
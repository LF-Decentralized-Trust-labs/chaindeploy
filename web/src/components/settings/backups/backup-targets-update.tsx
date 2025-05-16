import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { putBackupsTargetsById, type HttpBackupTargetResponse } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage, FormDescription } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Pencil } from 'lucide-react'
import { toast } from 'sonner'
import { Checkbox } from '@/components/ui/checkbox'
import { targetFormSchema, type TargetFormValues } from './backup-targets-schema'
import { DropdownMenuItem } from '@/components/ui/dropdown-menu'

interface BackupTargetsUpdateProps {
	target: HttpBackupTargetResponse
	onSuccess: () => void
}

export function BackupTargetsUpdate({ target, onSuccess }: BackupTargetsUpdateProps) {
	const [open, setOpen] = useState(false)

	const form = useForm<TargetFormValues>({
		resolver: zodResolver(targetFormSchema),
		defaultValues: {
			name: target.name,
			endpoint: target.endpoint,
			type: 'S3',
			accessKeyId: target.accessKeyId,
			secretKey: '',
			bucketName: target.bucketName,
			bucketPath: target.bucketPath,
			region: target.region,
			forcePathStyle: target.forcePathStyle,
		},
	})

	const updateMutation = useMutation({
		mutationFn: async (values: TargetFormValues) => {
			try {
				await putBackupsTargetsById({
					path: { id: target.id! },
					body: {
						name: values.name || 'S3',
						endpoint: values.endpoint,
						accessKeyId: values.accessKeyId,
						secretKey: values.secretKey,
						bucketName: values.bucketName,
						bucketPath: values.bucketPath,
						region: values.region,
						forcePathStyle: values.forcePathStyle,
						type: 'S3',
					},
				})
			} catch (error: any) {
				if (error.status === 500) {
					throw new Error('Internal server error. Please try again later.')
				}
				throw error
			}
		},
		onSuccess: () => {
			toast.success('Backup target updated successfully')
			setOpen(false)
			onSuccess()
		},
		onError: (error) => {
			toast.error('Failed to update backup target', {
				description: error.message,
			})
		},
	})

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger asChild>
				<DropdownMenuItem onSelect={(e) => e.preventDefault()}>
					<Pencil className="mr-2 h-4 w-4" />
					Edit
				</DropdownMenuItem>
			</DialogTrigger>
			<DialogContent className="max-h-[calc(100vh-8rem)] overflow-y-auto">
				<DialogHeader>
					<DialogTitle>Update Backup Target</DialogTitle>
				</DialogHeader>
				<Form {...form}>
					<form onSubmit={form.handleSubmit((data) => updateMutation.mutate(data))} className="space-y-4">
						<FormField
							control={form.control}
							name="name"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Name</FormLabel>
									<FormControl>
										<Input {...field} />
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>
						<FormField
							control={form.control}
							name="endpoint"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Endpoint</FormLabel>
									<FormControl>
										<Input {...field} placeholder="https://s3.amazonaws.com" />
									</FormControl>
									<FormDescription>S3 endpoint URL (e.g., https://s3.amazonaws.com for AWS S3)</FormDescription>
									<FormMessage />
								</FormItem>
							)}
						/>
						<FormField
							control={form.control}
							name="accessKeyId"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Access Key ID</FormLabel>
									<FormControl>
										<Input {...field} />
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>
						<FormField
							control={form.control}
							name="secretKey"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Secret Key</FormLabel>
									<FormControl>
										<Input type="password" {...field} />
									</FormControl>

									<FormMessage />
								</FormItem>
							)}
						/>
						<FormField
							control={form.control}
							name="bucketName"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Bucket Name</FormLabel>
									<FormControl>
										<Input {...field} />
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>
						<FormField
							control={form.control}
							name="bucketPath"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Bucket Path</FormLabel>
									<FormControl>
										<Input {...field} placeholder="backups/" />
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>
						<FormField
							control={form.control}
							name="region"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Region</FormLabel>
									<FormControl>
										<Input {...field} placeholder="us-east-1" />
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>
						<FormField
							control={form.control}
							name="forcePathStyle"
							render={({ field }) => (
								<FormItem className="flex flex-row items-start space-x-3 space-y-0 rounded-md border p-4">
									<FormControl>
										<Checkbox checked={field.value} onCheckedChange={field.onChange} />
									</FormControl>
									<div className="space-y-1 leading-none">
										<FormLabel>Force Path Style</FormLabel>
										<FormDescription>Use path-style addressing instead of virtual hosted-style</FormDescription>
									</div>
								</FormItem>
							)}
						/>
						<Button type="submit" disabled={updateMutation.isPending}>
							Update Target
						</Button>
					</form>
				</Form>
			</DialogContent>
		</Dialog>
	)
}

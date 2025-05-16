import { postBackupsTargets } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Form } from '@/components/ui/form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { targetFormSchema, type TargetFormValues } from './backup-targets-schema'
import { FormControl, FormField, FormItem, FormLabel, FormMessage, FormDescription } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'

export function BackupTargetsCreate({ onSuccess }: { onSuccess: () => void }) {
	const [open, setOpen] = useState(false)

	const form = useForm<TargetFormValues>({
		resolver: zodResolver(targetFormSchema),
		defaultValues: {
			type: 'S3',
			forcePathStyle: false,
		},
	})

	const createMutation = useMutation({
		mutationFn: async (values: TargetFormValues) => {
			try {
				await postBackupsTargets({
					body: {
						name: values.name || 'S3',
						type: values.type,
						endpoint: values.endpoint,
						accessKeyId: values.accessKeyId,
						secretKey: values.secretKey,
						bucketName: values.bucketName,
						bucketPath: values.bucketPath,
						region: values.region,
						forcePathStyle: values.forcePathStyle,
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
			toast.success('Backup target created successfully')
			setOpen(false)
			form.reset()
			onSuccess()
		},
		onError: (error) => {
			toast.error('Failed to create backup target', {
				description: error.message,
			})
		},
	})

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger asChild>
				<Button>
					<Plus className="mr-2 h-4 w-4" />
					Add Target
				</Button>
			</DialogTrigger>
			<DialogContent className="max-h-[calc(100vh-8rem)] overflow-y-auto">
				<DialogHeader>
					<DialogTitle>Create Backup Target</DialogTitle>
				</DialogHeader>
				<Form {...form}>
					<form onSubmit={form.handleSubmit((data) => createMutation.mutate(data))} className="space-y-4">
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
						<Button type="submit" disabled={createMutation.isPending}>
							Create Target
						</Button>
					</form>
				</Form>
			</DialogContent>
		</Dialog>
	)
}

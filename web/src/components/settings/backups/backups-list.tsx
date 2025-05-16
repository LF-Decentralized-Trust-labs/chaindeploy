import { postBackups } from '@/api/client'
import { getBackupsOptions, getBackupsSchedulesOptions, getBackupsTargetsOptions } from '@/api/client/@tanstack/react-query.gen'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

const backupFormSchema = z.object({
	targetId: z.coerce.number(),
	scheduleId: z.coerce.number().optional(),
	metadata: z.record(z.string(), z.any()).optional(),
})

type BackupFormValues = z.infer<typeof backupFormSchema>

export function BackupsList() {
	const {
		data: backups,
		isLoading: isLoadingBackups,
		refetch: refetchBackups,
	} = useQuery({
		...getBackupsOptions(),
		// queryKey: ['backups'],
		// queryFn: async () => {
		// 	const { data } = await getBackups({})
		// 	return data
		// },
	})

	const { data: targets, refetch: refetchTargets } = useQuery({
		...getBackupsTargetsOptions(),
	})

	const { data: schedules, refetch: refetchSchedules } = useQuery({
		...getBackupsSchedulesOptions(),
	})

	const [open, setOpen] = useState(false)

	const form = useForm<BackupFormValues>({
		resolver: zodResolver(backupFormSchema),
	})

	const createMutation = useMutation({
		mutationFn: async (values: BackupFormValues) => {
			try {
				await postBackups({
					body: {
						targetId: values.targetId,
						scheduleId: values.scheduleId,
						metadata: values.metadata,
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
			toast.success('Backup created successfully')
			setOpen(false)
			form.reset()
			refetchBackups()
			refetchTargets()
			refetchSchedules()
		},
		onError: (error) => {
			toast.error('Failed to create backup', {
				description: error.message,
			})
		},
	})

	return (
		<Card>
			<CardHeader className="flex flex-row items-center justify-between">
				<div>
					<CardTitle>Backups</CardTitle>
					<CardDescription>View and manage your backups</CardDescription>
				</div>
				<Dialog open={open} onOpenChange={setOpen}>
					<DialogTrigger asChild>
						<Button>
							<Plus className="mr-2 h-4 w-4" />
							Create Backup
						</Button>
					</DialogTrigger>
					<DialogContent className="max-h-[calc(100vh-8rem)] overflow-y-auto">
						<DialogHeader>
							<DialogTitle>Create Backup</DialogTitle>
						</DialogHeader>
						<Form {...form}>
							<form onSubmit={form.handleSubmit((data) => createMutation.mutate(data))} className="space-y-4">
								<FormField
									control={form.control}
									name="targetId"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Backup Target</FormLabel>
											<Select onValueChange={field.onChange} defaultValue={field.value?.toString()}>
												<FormControl>
													<SelectTrigger>
														<SelectValue placeholder="Select a backup target" />
													</SelectTrigger>
												</FormControl>
												<SelectContent>
													{targets?.map((target) => (
														<SelectItem key={target.id} value={target.id?.toString() || ''}>
															{target.name}
														</SelectItem>
													))}
												</SelectContent>
											</Select>
											<FormMessage />
										</FormItem>
									)}
								/>
								<FormField
									control={form.control}
									name="scheduleId"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Backup Schedule (Optional)</FormLabel>
											<Select onValueChange={field.onChange} defaultValue={field.value?.toString()}>
												<FormControl>
													<SelectTrigger>
														<SelectValue placeholder="Select a backup schedule" />
													</SelectTrigger>
												</FormControl>
												<SelectContent>
													{schedules?.map((schedule) => (
														<SelectItem key={schedule.id} value={schedule.id?.toString() || ''}>
															{schedule.name}
														</SelectItem>
													))}
												</SelectContent>
											</Select>
											<FormMessage />
										</FormItem>
									)}
								/>
								<Button type="submit" disabled={createMutation.isPending}>
									Create Backup
								</Button>
							</form>
						</Form>
					</DialogContent>
				</Dialog>
			</CardHeader>
			<CardContent>
				{isLoadingBackups ? (
					<div className="space-y-2">
						<Skeleton className="h-12 w-full" />
						<Skeleton className="h-12 w-full" />
						<Skeleton className="h-12 w-full" />
					</div>
				) : backups?.length === 0 ? (
					<div className="flex min-h-[200px] flex-col items-center justify-center rounded-lg border border-dashed p-8 text-center animate-in fade-in-50">
						<div className="mx-auto flex max-w-[420px] flex-col items-center justify-center text-center">
							<h3 className="mt-4 text-lg font-semibold">No backups</h3>
							<p className="mb-4 mt-2 text-sm text-muted-foreground">You haven't created any backups yet. Create one to get started.</p>
							<Dialog open={open} onOpenChange={setOpen}>
								<DialogTrigger asChild>
									<Button>
										<Plus className="mr-2 h-4 w-4" />
										Create Backup
									</Button>
								</DialogTrigger>
								<DialogContent className="max-h-[calc(100vh-8rem)] overflow-y-auto">
									<DialogHeader>
										<DialogTitle>Create Backup</DialogTitle>
									</DialogHeader>
									<Form {...form}>
										<form onSubmit={form.handleSubmit((data) => createMutation.mutate(data))} className="space-y-4">
											<FormField
												control={form.control}
												name="targetId"
												render={({ field }) => (
													<FormItem>
														<FormLabel>Backup Target</FormLabel>
														<Select onValueChange={field.onChange} defaultValue={field.value?.toString()}>
															<FormControl>
																<SelectTrigger>
																	<SelectValue placeholder="Select a backup target" />
																</SelectTrigger>
															</FormControl>
															<SelectContent>
																{targets?.map((target) => (
																	<SelectItem key={target.id} value={target.id?.toString() || ''}>
																		{target.name}
																	</SelectItem>
																))}
															</SelectContent>
														</Select>
														<FormMessage />
													</FormItem>
												)}
											/>
											<FormField
												control={form.control}
												name="scheduleId"
												render={({ field }) => (
													<FormItem>
														<FormLabel>Backup Schedule (Optional)</FormLabel>
														<Select onValueChange={field.onChange} defaultValue={field.value?.toString()}>
															<FormControl>
																<SelectTrigger>
																	<SelectValue placeholder="Select a backup schedule" />
																</SelectTrigger>
															</FormControl>
															<SelectContent>
																{schedules?.map((schedule) => (
																	<SelectItem key={schedule.id} value={schedule.id?.toString() || ''}>
																		{schedule.name}
																	</SelectItem>
																))}
															</SelectContent>
														</Select>
														<FormMessage />
													</FormItem>
												)}
											/>
											<Button type="submit" disabled={createMutation.isPending}>
												Create Backup
											</Button>
										</form>
									</Form>
								</DialogContent>
							</Dialog>
						</div>
					</div>
				) : (
					<div className="max-h-[calc(100vh-20rem)] overflow-y-auto">
						<Table>
							<TableHeader>
								<TableRow>
									<TableHead>Status</TableHead>
									<TableHead>Size</TableHead>
									<TableHead>Created</TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{backups?.map((backup) => (
									<TableRow key={backup.id}>
										<TableCell>
											<Badge variant={backup.status === 'COMPLETED' ? 'success' : backup.status === 'FAILED' ? 'destructive' : 'secondary'}>{backup.status}</Badge>
											<span className="ml-2">{backup.status === 'FAILED' ? backup.errorMessage : ''}</span>
										</TableCell>
										<TableCell>
											{backup.sizeBytes
												? (() => {
														const sizes = ['B', 'KB', 'MB', 'GB']
														const i = Math.floor(Math.log(backup.sizeBytes) / Math.log(1024))
														return `${(backup.sizeBytes / Math.pow(1024, i)).toFixed(2)} ${sizes[i]}`
												  })()
												: '-'}
										</TableCell>
										<TableCell>{backup.createdAt ? new Date(backup.createdAt).toLocaleString() : '-'}</TableCell>
									</TableRow>
								))}
							</TableBody>
						</Table>
					</div>
				)}
			</CardContent>
		</Card>
	)
}

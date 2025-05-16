import { postBackupsSchedules, putBackupsSchedulesByIdDisable, putBackupsSchedulesByIdEnable } from '@/api/client'
import { deleteBackupsSchedulesByIdMutation, getBackupsSchedulesOptions, getBackupsTargetsOptions } from '@/api/client/@tanstack/react-query.gen'
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
	AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { EllipsisVertical, Plus, Power } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

const scheduleFormSchema = z.object({
	name: z.string().min(1),
	description: z.string().optional(),
	cronExpression: z.string().min(1),
	retentionDays: z.coerce.number().min(1),
	targetId: z.coerce.number(),
	enabled: z.boolean().default(true),
})

type ScheduleFormValues = z.infer<typeof scheduleFormSchema>

export function BackupSchedules() {
	const {
		data: schedules,
		isLoading: isLoadingSchedules,
		refetch,
	} = useQuery({
		...getBackupsSchedulesOptions(),
	})

	const {
		data: targets,
		isLoading: isLoadingTargets,
		refetch: refetchTargets,
	} = useQuery({
		...getBackupsTargetsOptions(),
	})

	const [open, setOpen] = useState(false)

	const form = useForm<ScheduleFormValues>({
		resolver: zodResolver(scheduleFormSchema),
		defaultValues: {
			enabled: true,
			retentionDays: 30,
		},
	})

	const createMutation = useMutation({
		mutationFn: async (values: ScheduleFormValues) => {
			try {
				await postBackupsSchedules({
					body: {
						targetId: values.targetId,
						cronExpression: values.cronExpression,
						name: values.name,
						description: values.description,
						enabled: values.enabled,
						retentionDays: values.retentionDays,
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
			toast.success('Backup schedule created successfully')
			setOpen(false)
			form.reset()
			refetch()
			refetchTargets()
		},
		onError: (error) => {
			toast.error('Failed to create backup schedule', {
				description: error.message,
			})
		},
	})

	const deleteMutation = useMutation({
		...deleteBackupsSchedulesByIdMutation(),
		onSuccess: () => {
			toast.success('Backup schedule deleted successfully')
			refetch()
		},
		onError: (error) => {
			toast.error('Failed to delete backup schedule', {
				description: error.message,
			})
		},
	})

	const toggleMutation = useMutation({
		mutationFn: async ({ id, enabled }: { id: number; enabled: boolean }) => {
			try {
				if (enabled) {
					await putBackupsSchedulesByIdDisable({ path: { id } })
				} else {
					await putBackupsSchedulesByIdEnable({ path: { id } })
				}
			} catch (error: any) {
				if (error.status === 500) {
					throw new Error('Internal server error. Please try again later.')
				}
				throw error
			}
		},
		onSuccess: () => {
			refetch()
		},
		onError: (error) => {
			toast.error('Failed to toggle backup schedule', {
				description: error.message,
			})
		},
	})

	return (
		<Card>
			<CardHeader className="flex flex-row items-center justify-between">
				<div>
					<CardTitle>Backup Schedules</CardTitle>
					<CardDescription>Configure automated backup schedules</CardDescription>
				</div>
				<Dialog open={open} onOpenChange={setOpen}>
					<DialogTrigger asChild>
						<Button>
							<Plus className="mr-2 h-4 w-4" />
							Add Schedule
						</Button>
					</DialogTrigger>
					<DialogContent className="max-h-[calc(100vh-8rem)] overflow-y-auto">
						<DialogHeader>
							<DialogTitle>Create Backup Schedule</DialogTitle>
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
									name="description"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Description</FormLabel>
											<FormControl>
												<Input {...field} />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
								<FormField
									control={form.control}
									name="cronExpression"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Cron Expression</FormLabel>
											<FormControl>
												<Input {...field} placeholder="0 0 * * *" />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
								<FormField
									control={form.control}
									name="retentionDays"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Retention Days</FormLabel>
											<FormControl>
												<Input type="number" {...field} />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
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
								<Button type="submit" disabled={createMutation.isPending}>
									Create Schedule
								</Button>
							</form>
						</Form>
					</DialogContent>
				</Dialog>
			</CardHeader>
			<CardContent>
				{isLoadingSchedules || isLoadingTargets ? (
					<div className="space-y-2">
						<Skeleton className="h-12 w-full" />
						<Skeleton className="h-12 w-full" />
						<Skeleton className="h-12 w-full" />
					</div>
				) : schedules?.length === 0 ? (
					<div className="flex min-h-[200px] flex-col items-center justify-center rounded-lg border border-dashed p-8 text-center animate-in fade-in-50">
						<div className="mx-auto flex max-w-[420px] flex-col items-center justify-center text-center">
							<h3 className="mt-4 text-lg font-semibold">No backup schedules</h3>
							<p className="mb-4 mt-2 text-sm text-muted-foreground">You haven't created any backup schedules yet. Add one to get started.</p>
							<Dialog open={open} onOpenChange={setOpen}>
								<DialogTrigger asChild>
									<Button>
										<Plus className="mr-2 h-4 w-4" />
										Add Schedule
									</Button>
								</DialogTrigger>
								<DialogContent className="max-h-[calc(100vh-8rem)] overflow-y-auto">
									<DialogHeader>
										<DialogTitle>Create Backup Schedule</DialogTitle>
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
												name="description"
												render={({ field }) => (
													<FormItem>
														<FormLabel>Description</FormLabel>
														<FormControl>
															<Input {...field} />
														</FormControl>
														<FormMessage />
													</FormItem>
												)}
											/>
											<FormField
												control={form.control}
												name="cronExpression"
												render={({ field }) => (
													<FormItem>
														<FormLabel>Cron Expression</FormLabel>
														<FormControl>
															<Input {...field} placeholder="0 0 * * *" />
														</FormControl>
														<FormMessage />
													</FormItem>
												)}
											/>
											<FormField
												control={form.control}
												name="retentionDays"
												render={({ field }) => (
													<FormItem>
														<FormLabel>Retention Days</FormLabel>
														<FormControl>
															<Input type="number" {...field} />
														</FormControl>
														<FormMessage />
													</FormItem>
												)}
											/>
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
											<Button type="submit" disabled={createMutation.isPending}>
												Create Schedule
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
									<TableHead>Name</TableHead>
									<TableHead>Schedule</TableHead>
									<TableHead>Status</TableHead>
									<TableHead>Retention</TableHead>
									<TableHead>Last Run</TableHead>
									<TableHead className="w-[50px]"></TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{schedules?.map((schedule) => (
									<TableRow key={schedule.id}>
										<TableCell>{schedule.name}</TableCell>
										<TableCell>{schedule.cronExpression}</TableCell>
										<TableCell>
											<Badge variant={schedule.enabled ? 'default' : 'secondary'}>{schedule.enabled ? 'Enabled' : 'Disabled'}</Badge>
										</TableCell>
										<TableCell>{schedule.retentionDays} days</TableCell>
										<TableCell>{schedule.lastRunAt ? new Date(schedule.lastRunAt).toLocaleString() : '-'}</TableCell>
										<TableCell>
											<DropdownMenu>
												<DropdownMenuTrigger asChild>
													<Button variant="ghost" size="icon">
														<EllipsisVertical className="h-4 w-4" />
													</Button>
												</DropdownMenuTrigger>
												<DropdownMenuContent align="end">
													<DropdownMenuItem
														onClick={() => {
															if (schedule.id) {
																toggleMutation.mutate({
																	id: schedule.id,
																	enabled: schedule.enabled || false,
																})
															}
														}}
													>
														<Power className="mr-2 h-4 w-4" />
														{schedule.enabled ? 'Disable' : 'Enable'}
													</DropdownMenuItem>
													<AlertDialog>
														<AlertDialogTrigger asChild>
															<DropdownMenuItem className="text-destructive" onSelect={(e) => e.preventDefault()}>
																Delete
															</DropdownMenuItem>
														</AlertDialogTrigger>
														<AlertDialogContent>
															<AlertDialogHeader>
																<AlertDialogTitle>Are you absolutely sure?</AlertDialogTitle>
																<AlertDialogDescription>This will permanently delete the backup schedule. This action cannot be undone.</AlertDialogDescription>
															</AlertDialogHeader>
															<AlertDialogFooter>
																<AlertDialogCancel>Cancel</AlertDialogCancel>
																<AlertDialogAction
																	destructive
																	onClick={() => {
																		if (schedule.id) {
																			deleteMutation.mutate({ path: { id: schedule.id } })
																		}
																	}}
																>
																	Delete
																</AlertDialogAction>
															</AlertDialogFooter>
														</AlertDialogContent>
													</AlertDialog>
												</DropdownMenuContent>
											</DropdownMenu>
										</TableCell>
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

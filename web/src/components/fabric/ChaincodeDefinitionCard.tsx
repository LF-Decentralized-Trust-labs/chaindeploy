import { ChainlaunchdeployChaincodeDefinition, ServiceNetworkNode } from '@/api/client'
import {
	deleteScFabricDefinitionsByDefinitionIdMutation,
	postScFabricDefinitionsByDefinitionIdDeployMutation,
	postScFabricDefinitionsByDefinitionIdUndeployMutation,
	putScFabricDefinitionsByDefinitionIdMutation,
} from '@/api/client/@tanstack/react-query.gen'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { MoreVertical } from 'lucide-react'
import { useCallback, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import * as z from 'zod'
import { ApproveChaincodeDialog } from './ApproveChaincodeDialog'
import { CommitChaincodeDialog } from './CommitChaincodeDialog'
import { DefinitionTimeline } from './DefinitionTimeline'
import { InstallChaincodeDialog } from './InstallChaincodeDialog'

const versionFormSchema = z.object({
	endorsementPolicy: z.string().min(1, 'Endorsement policy is required'),
	dockerImage: z.string().min(1, 'Docker image is required'),
	version: z.string().min(1, 'Version is required'),
	sequence: z.number().min(1, 'Sequence must be at least 1'),
	chaincodeAddress: z
		.string()
		.min(1, 'Chaincode address is required')
		.regex(/^([\d]{1,3}\.){3}\d{1,3}:(\d{1,5})$/, 'Chaincode address must be in the format host:port, e.g., 127.0.0.1:8080'),
})

type VersionFormValues = z.infer<typeof versionFormSchema>

interface ChaincodeDefinitionCardProps {
	definition: ChainlaunchdeployChaincodeDefinition
	DefinitionTimelineComponent?: React.ComponentType<{ definitionId: number }>
	availablePeers: ServiceNetworkNode[]
	onSuccess?: () => void
	refetch?: () => void
}

const LIFECYCLE_ACTIONS = ['install', 'approve', 'deploy', 'commit', 'stop'] as const
type LifecycleAction = (typeof LIFECYCLE_ACTIONS)[number]
const actionLabels: Record<LifecycleAction, string> = {
	install: 'Install',
	approve: 'Approve',
	deploy: 'Deploy',
	commit: 'Commit',
	stop: 'Stop Chaincode',
}

export function ChaincodeDefinitionCard({ definition: v, DefinitionTimelineComponent = DefinitionTimeline, availablePeers = [], onSuccess, refetch }: ChaincodeDefinitionCardProps) {
	// Edit dialog state
	const [editOpen, setEditOpen] = useState(false)
	const [timelineKey, setTimelineKey] = useState(0)
	const editForm = useForm<VersionFormValues>({
		resolver: zodResolver(versionFormSchema),
		defaultValues: {
			endorsementPolicy: v.endorsement_policy,
			dockerImage: v.docker_image,
			version: v.version,
			sequence: v.sequence,
			chaincodeAddress: v.chaincode_address || '',
		},
	})

	// Delete dialog state
	const [deleteOpen, setDeleteOpen] = useState(false)

	// Dialog states
	const [installDialogOpen, setInstallDialogOpen] = useState(false)
	const [approveDialogOpen, setApproveDialogOpen] = useState(false)
	const [commitDialogOpen, setCommitDialogOpen] = useState(false)

	// Undeploy dialog state
	const [stopOpen, setStopOpen] = useState(false)

	// Docker status helpers
	const dockerState = useMemo(() => v.docker_info?.state || v.docker_info?.status || v.docker_info?.docker_status || '', [v.docker_info])
	const isDockerRunning = useMemo(() => dockerState.toLowerCase() === 'running', [dockerState])
	const dockerStatusLabel = useMemo(() => (dockerState ? dockerState.charAt(0).toUpperCase() + dockerState.slice(1) : 'Not running'), [dockerState])
	const dockerBadgeVariant = isDockerRunning ? 'success' : dockerState ? 'secondary' : 'outline'

	// Edit mutation
	const editMutation = useMutation({
		...putScFabricDefinitionsByDefinitionIdMutation(),
		onSuccess: () => {
			setEditOpen(false)
			setTimelineKey((prev) => prev + 1)
			onSuccess?.()
			refetch?.()
		},
		onError: (error: any) => {
			let message = 'Failed to update chaincode definition.'
			if (error?.response?.data?.message) {
				message = error.response.data.message
			} else if (error?.message) {
				message = error.message
			}
			toast.error(message)
		},
	})

	// Deploy mutation
	const deployMutation = useMutation({
		...postScFabricDefinitionsByDefinitionIdDeployMutation(),
		onSuccess: () => {
			setTimelineKey((prev) => prev + 1)
			onSuccess?.()
			refetch?.()
		},
	})

	// Undeploy mutation
	const stopMutation = useMutation({
		...postScFabricDefinitionsByDefinitionIdUndeployMutation(),
		onSuccess: () => {
			setTimelineKey((prev) => prev + 1)
			setStopOpen(false)
			onSuccess?.()
			refetch?.()
		},
	})

	// Handlers
	const handleEdit = useCallback(() => setEditOpen(true), [])
	const handleEditDialogOpenChange = useCallback((open: boolean) => setEditOpen(open), [])
	const handleEditSubmit = async (data: VersionFormValues) => {
		try {
			await toast.promise(
				editMutation.mutateAsync({
					path: { definitionId: v.id },
					body: {
						docker_image: data.dockerImage,
						endorsement_policy: data.endorsementPolicy,
						version: data.version,
						sequence: data.sequence,
						chaincode_address: data.chaincodeAddress,
					},
				}),
				{
					loading: 'Updating chaincode definition...',
					success: 'Chaincode definition updated successfully',
					error: (err) => `Failed to update chaincode definition: ${err.message || 'Unknown error'}`,
				}
			)
		} catch (error) {
			// Error is already handled by toast.promise
		}
	}
	const deleteMutation = useMutation({
		...deleteScFabricDefinitionsByDefinitionIdMutation(),
		onSuccess: () => {
			setDeleteOpen(false)
			onSuccess?.()
			refetch?.()
		},
	})
	const handleDelete = useCallback(() => {
		toast.promise(deleteMutation.mutateAsync({ path: { definitionId: v.id } }), {
			loading: 'Deleting chaincode definition...',
			success: 'Chaincode definition deleted successfully',
			error: (err) => `Failed to delete chaincode definition: ${err.message || 'Unknown error'}`,
		})
	}, [deleteMutation, v.id])
	const handleDeleteDialogOpenChange = useCallback((open: boolean) => setDeleteOpen(open), [])

	const handleStop = useCallback(async () => {
		await toast.promise(stopMutation.mutateAsync({ path: { definitionId: v.id } }), {
			loading: 'Stopping chaincode...',
			success: 'Chaincode stopped successfully',
			error: (err) => `Failed to stop chaincode: ${err.message || 'Unknown error'}`,
		})
	}, [stopMutation, v.id])

	// Lifecycle action handler
	const handleLifecycleAction = useCallback(async (action: string) => {
		if (action === 'install') {
			setInstallDialogOpen(true)
		} else if (action === 'approve') {
			setApproveDialogOpen(true)
		} else if (action === 'commit') {
			setCommitDialogOpen(true)
		} else if (action === 'deploy') {
			try {
				await toast.promise(
					deployMutation.mutateAsync({
						path: { definitionId: v.id },
						body: {},
					}),
					{
						loading: 'Deploying chaincode...',
						success: 'Chaincode deployed successfully',
						error: (err) => `Failed to deploy chaincode: ${err.message || 'Unknown error'}`,
					}
				)
			} catch (error) {
				// Error is already handled by toast.promise
			}
		} else if (action === 'stop') {
			setStopOpen(true)
		}
	}, [deployMutation, v.id])

	return (
		<Card key={v.id} className="p-4 mb-4">
			<div className="flex items-center gap-4 mb-2">
				<Badge variant="outline">Version {v.version}</Badge>
				<Badge variant="outline">Sequence {v.sequence}</Badge>
				<Badge variant={dockerBadgeVariant} className="capitalize">
					Docker: {dockerStatusLabel || 'Not running'}
				</Badge>
				<DropdownMenu>
					<DropdownMenuTrigger asChild>
						<Button variant="ghost" size="icon">
							<MoreVertical className="w-5 h-5" />
						</Button>
					</DropdownMenuTrigger>
					<DropdownMenuContent>
						<DropdownMenuItem onClick={handleEdit}>Edit</DropdownMenuItem>
						<DropdownMenuItem onClick={() => setDeleteOpen(true)} disabled={deleteMutation.isPending}>
							{deleteMutation.isPending ? 'Deleting...' : 'Delete'}
						</DropdownMenuItem>
						{isDockerRunning && (
							<DropdownMenuItem onClick={() => setStopOpen(true)} disabled={stopMutation.isPending}>
								{stopMutation.isPending ? 'Stopping...' : 'Stop Chaincode'}
							</DropdownMenuItem>
						)}
						<DropdownMenuItem onClick={() => handleLifecycleAction('deploy')} disabled={deployMutation.isPending}>
							{deployMutation.isPending ? 'Deploying...' : 'Deploy'}
						</DropdownMenuItem>
					</DropdownMenuContent>
				</DropdownMenu>
				<AlertDialog open={deleteOpen} onOpenChange={handleDeleteDialogOpenChange}>
					<AlertDialogContent>
						<AlertDialogHeader>
							<AlertDialogTitle>Delete Chaincode Definition</AlertDialogTitle>
							<AlertDialogDescription>Are you sure you want to delete this chaincode definition? This action cannot be undone.</AlertDialogDescription>
						</AlertDialogHeader>
						{deleteMutation.error && <div className="text-red-500 text-sm mb-2">{deleteMutation.error.message}</div>}
						<AlertDialogFooter>
							<AlertDialogCancel onClick={() => setDeleteOpen(false)}>Cancel</AlertDialogCancel>
							<AlertDialogAction disabled={deleteMutation.isPending} onClick={handleDelete}>
								{deleteMutation.isPending ? 'Deleting...' : 'Delete'}
							</AlertDialogAction>
						</AlertDialogFooter>
					</AlertDialogContent>
				</AlertDialog>
				<AlertDialog open={stopOpen} onOpenChange={setStopOpen}>
					<AlertDialogContent>
						<AlertDialogHeader>
							<AlertDialogTitle>Stop Chaincode</AlertDialogTitle>
							<AlertDialogDescription>
								Are you sure you want to stop this chaincode? This will stop the running chaincode container but keep the definition.
							</AlertDialogDescription>
						</AlertDialogHeader>
						{stopMutation.error && <div className="text-red-500 text-sm mb-2">{stopMutation.error.message}</div>}
						<AlertDialogFooter>
							<AlertDialogCancel onClick={() => setStopOpen(false)}>Cancel</AlertDialogCancel>
							<AlertDialogAction disabled={stopMutation.isPending} onClick={handleStop}>
								{stopMutation.isPending ? 'Stopping...' : 'Stop Chaincode'}
							</AlertDialogAction>
						</AlertDialogFooter>
					</AlertDialogContent>
				</AlertDialog>
			</div>

			{/* Main info rows with Docker details inline */}
			<div className="mb-1 text-sm">
				<span className="font-medium">Endorsement Policy:</span> {v.endorsement_policy}
			</div>
			<div className="mb-1 text-sm">
				<span className="font-medium">Docker Image:</span> {v.docker_image}
				{v.docker_info?.image && v.docker_info?.image !== v.docker_image && <span className="ml-2 text-muted-foreground">({v.docker_info.image})</span>}
			</div>
			{v.docker_info?.name && (
				<div className="mb-1 text-sm">
					<span className="font-medium">Container Name:</span> {v.docker_info.name}
				</div>
			)}
			{v.docker_info?.id && (
				<div className="mb-1 text-sm">
					<span className="font-medium">Container ID:</span> {v.docker_info.id}
				</div>
			)}
			{v.docker_info?.ports && v.docker_info.ports.length > 0 && (
				<div className="mb-1 text-sm">
					<span className="font-medium">Ports:</span> {v.docker_info.ports.join(', ')}
				</div>
			)}
			{v.docker_info?.created && (
				<div className="mb-1 text-sm">
					<span className="font-medium">Created:</span> {new Date(v.docker_info.created * 1000).toLocaleString()}
				</div>
			)}
			<div className="mb-1 text-sm">
				<span className="font-medium">Chaincode Address:</span> {v.chaincode_address}
			</div>
			<div className="mt-2 flex gap-2">
				<Button key="deploy" size="sm" variant="default" onClick={() => handleLifecycleAction('deploy')} disabled={deployMutation.isPending}>
					{deployMutation.isPending ? 'Deploying...' : 'Deploy'}
				</Button>
				{['install', 'approve', 'commit'].map((action) => (
					<Button key={action} size="sm" variant="default" onClick={() => handleLifecycleAction(action)}>
						{actionLabels[action as LifecycleAction]}
					</Button>
				))}
			</div>
			<Dialog open={editOpen} onOpenChange={handleEditDialogOpenChange}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>Edit Chaincode Definition</DialogTitle>
					</DialogHeader>
					<Form {...editForm}>
						<form onSubmit={editForm.handleSubmit(handleEditSubmit)} className="space-y-4">
							<div className="grid grid-cols-2 gap-4">
								<FormField
									control={editForm.control}
									name="version"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Version</FormLabel>
											<FormControl>
												<Input {...field} />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
								<FormField
									control={editForm.control}
									name="sequence"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Sequence</FormLabel>
											<FormControl>
												<Input type="number" {...field} onChange={(e) => field.onChange(parseInt(e.target.value))} />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
							</div>
							<FormField
								control={editForm.control}
								name="endorsementPolicy"
								render={({ field }) => (
									<FormItem>
										<FormLabel>Endorsement Policy</FormLabel>
										<FormControl>
											<Textarea {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={editForm.control}
								name="dockerImage"
								render={({ field }) => (
									<FormItem>
										<FormLabel>Docker Image</FormLabel>
										<FormControl>
											<Input {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={editForm.control}
								name="chaincodeAddress"
								render={({ field }) => (
									<FormItem>
										<FormLabel>Chaincode Address</FormLabel>
										<FormControl>
											<Input {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<DialogFooter>
								<Button type="submit" disabled={editMutation.isPending}>
									{editMutation.isPending ? 'Saving...' : 'Save'}
								</Button>
							</DialogFooter>
						</form>
					</Form>
				</DialogContent>
			</Dialog>
			{/* Install Dialog */}
			<InstallChaincodeDialog installDialogOpen={installDialogOpen} setInstallDialogOpen={setInstallDialogOpen} availablePeers={availablePeers} definitionId={v.id} onSuccess={() => refetch?.()} onError={(error) => refetch?.()} />
			{/* Approve Dialog */}
			<ApproveChaincodeDialog approveDialogOpen={approveDialogOpen} setApproveDialogOpen={setApproveDialogOpen} availablePeers={availablePeers} definitionId={v.id} onSuccess={() => refetch?.()} onError={(error) => refetch?.()} />
			{/* Commit Dialog */}
			<CommitChaincodeDialog commitDialogOpen={commitDialogOpen} setCommitDialogOpen={setCommitDialogOpen} availablePeers={availablePeers} definitionId={v.id} onSuccess={() => refetch?.()} onError={(error) => refetch?.()} />
			<div className="mt-4">
				<div className="text-sm font-medium mb-2">Timeline</div>
				<DefinitionTimelineComponent key={timelineKey} definitionId={v.id} />
			</div>
		</Card>
	)
}

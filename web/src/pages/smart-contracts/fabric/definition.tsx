import {
	getNetworksFabricByIdNodesOptions,
	getScFabricChaincodesByIdOptions,
	getScFabricDefinitionsByDefinitionIdTimelineOptions,
	postScFabricDefinitionsByDefinitionIdApproveMutation,
	postScFabricDefinitionsByDefinitionIdCommitMutation,
	postScFabricDefinitionsByDefinitionIdDeployMutation,
	postScFabricDefinitionsByDefinitionIdInstallMutation,
	putScFabricDefinitionsByDefinitionIdMutation,
} from '@/api/client/@tanstack/react-query.gen'
import { deleteScFabricDefinitionsByDefinitionId, postScFabricChaincodesByChaincodeIdDefinitions } from '@/api/client/sdk.gen'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { MoreVertical, Plus, Check, X as XIcon } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import * as z from 'zod'
import * as timeago from 'timeago.js'

const versionFormSchema = z.object({
	endorsementPolicy: z.string().min(1, 'Endorsement policy is required'),
	dockerImage: z.string().min(1, 'Docker image is required'),
	version: z.string().min(1, 'Version is required'),
	sequence: z.number().min(1, 'Sequence must be at least 1'),
	chaincodeAddress: z
		.string()
		.min(1, 'Chaincode address is required')
		.regex(/^(\d{1,3}\.){3}\d{1,3}:(\d{1,5})$/, 'Chaincode address must be in the format host:port, e.g., 127.0.0.1:8080'),
})

type VersionFormValues = z.infer<typeof versionFormSchema>

const LIFECYCLE_ACTIONS = ['install', 'approve', 'deploy', 'commit'] as const
type LifecycleAction = (typeof LIFECYCLE_ACTIONS)[number]
const actionLabels: Record<LifecycleAction, string> = {
	install: 'Install',
	approve: 'Approve',
	deploy: 'Deploy',
	commit: 'Commit',
}
const actionColors: Record<LifecycleAction, string> = {
	install: 'bg-blue-100 text-blue-800',
	approve: 'bg-yellow-100 text-yellow-800',
	deploy: 'bg-purple-100 text-purple-800',
	commit: 'bg-green-100 text-green-800',
}

function DefinitionTimeline({ definitionId }: { definitionId: number }) {
	const [expanded, setExpanded] = useState(false)
	const { data, isLoading, error } = useQuery(getScFabricDefinitionsByDefinitionIdTimelineOptions({ path: { definitionId } }))

	const sortedData = data?.slice()?.sort((a, b) => new Date(b.created_at!).getTime() - new Date(a.created_at!).getTime()) || []
	const visibleData = expanded ? sortedData : sortedData.slice(0, 5)

	return (
		<div className="space-y-2">
			{visibleData.map((event) => {
				let result: string | undefined = undefined
				if (event.event_data) {
					try {
						const parsed = typeof event.event_data === 'string' ? JSON.parse(event.event_data) : event.event_data
						result = parsed?.result
					} catch {
						// ignore
					}
				}
				return (
					<div key={event.id} className="flex items-start gap-2 text-sm py-1">
						{/* Result icon */}
						<div className="flex items-center justify-center min-w-[24px]">
							{result === 'success' ? <Check className="text-green-600 w-4 h-4" /> : result === 'failure' ? <XIcon className="text-red-600 w-4 h-4" /> : null}
						</div>
						<div className={`px-2 py-1 rounded ${actionColors[event.event_type as LifecycleAction] || 'bg-gray-100 text-gray-800'} min-w-[70px] text-center`} style={{ flexShrink: 0 }}>
							{actionLabels[event.event_type as LifecycleAction] || event.event_type}
						</div>
						<div className="min-w-[90px] text-muted-foreground text-right pr-2" style={{ flexShrink: 0 }}>
							{timeago.format(event.created_at!)}
						</div>
						<div className="flex-1 break-all text-muted-foreground">
							{event.event_data && (typeof event.event_data === 'object' ? JSON.stringify(event.event_data) : String(event.event_data))}
						</div>
					</div>
				)
			})}
			{isLoading && <div className="text-sm text-muted-foreground">Loading timeline...</div>}
			{error && <div className="text-sm text-red-500">Failed to load timeline</div>}
			{sortedData.length === 0 && !isLoading && <div className="text-sm text-muted-foreground">No events yet</div>}
			{sortedData.length > 5 && (
				<Button variant="ghost" size="sm" className="w-full mt-2" onClick={() => setExpanded((v) => !v)}>
					{expanded ? 'Show Less' : 'Show More'}
				</Button>
			)}
		</div>
	)
}

export default function FabricChaincodeDefinitionDetail() {
	const navigate = useNavigate()
	const { id } = useParams<{ id: string }>()
	const queryClient = useQueryClient()

	const [isAddDialogOpen, setIsAddDialogOpen] = useState(false)
	const [editIdx, setEditIdx] = useState<number | null>(null)
	const [installDialogOpen, setInstallDialogOpen] = useState(false)
	const [selectedVersionIdx, setSelectedVersionIdx] = useState<number | null>(null)
	const [selectedPeers, setSelectedPeers] = useState<Set<string>>(new Set())
	const [formError, setFormError] = useState<string | null>(null)
	const [editFormError, setEditFormError] = useState<string | null>(null)
	const [deleteError, setDeleteError] = useState<string | null>(null)
	const [deletingId, setDeletingId] = useState<number | null>(null)
	const [confirmDeleteIdx, setConfirmDeleteIdx] = useState<number | null>(null)
	const [installError, setInstallError] = useState<string | null>(null)
	const [installLoading, setInstallLoading] = useState(false)
	const [approveDialogOpen, setApproveDialogOpen] = useState(false)
	const [commitDialogOpen, setCommitDialogOpen] = useState(false)
	const [selectedPeerId, setSelectedPeerId] = useState<string | null>(null)
	const [approveError, setApproveError] = useState<string | null>(null)
	const [commitError, setCommitError] = useState<string | null>(null)
	const [deployError, setDeployError] = useState<string | null>(null)
	const [deployLoading, setDeployLoading] = useState(false)
	const [expandedTimelines, setExpandedTimelines] = useState<Set<number>>(new Set())

	// Fetch chaincode details
	const {
		data: chaincodeDetail,
		isLoading,
		error,
		refetch,
	} = useQuery({
		...getScFabricChaincodesByIdOptions({ path: { id: Number(id) } }),
		enabled: !!id,
	})

	const def = useMemo(() => chaincodeDetail?.chaincode, [chaincodeDetail])
	const versions = useMemo(() => chaincodeDetail?.definitions || [], [chaincodeDetail?.definitions])

	// Fetch network peers
	const networkId = useMemo(() => def?.network_id, [def])
	const { data: networkNodesResponse } = useQuery({
		...getNetworksFabricByIdNodesOptions(networkId ? { path: { id: networkId } } : { path: { id: 0 } }),
		enabled: !!networkId,
	})

	const availablePeers = useMemo(() => networkNodesResponse?.nodes?.filter((node) => node.node?.nodeType === 'FABRIC_PEER' && node.status === 'joined') || [], [networkNodesResponse?.nodes])

	const form = useForm<VersionFormValues>({
		resolver: zodResolver(versionFormSchema),
		defaultValues: {
			endorsementPolicy: '',
			dockerImage: '',
			version: '1.0',
			sequence: 1,
			chaincodeAddress: '',
		},
	})
	const editForm = useForm<VersionFormValues>({
		resolver: zodResolver(versionFormSchema),
		defaultValues: {
			endorsementPolicy: '',
			dockerImage: '',
			version: '1.0',
			sequence: 1,
			chaincodeAddress: '',
		},
	})

	const createDefinitionMutation = useMutation({
		mutationFn: async (data: VersionFormValues) => {
			if (!id) throw new Error('No chaincode id')
			const response = await postScFabricChaincodesByChaincodeIdDefinitions({
				body: {
					chaincode_id: Number(id),
					docker_image: data.dockerImage,
					endorsement_policy: data.endorsementPolicy,
					version: data.version,
					sequence: data.sequence,
					chaincode_address: data.chaincodeAddress,
				},
			})
			return response.data
		},
		onSuccess: () => {
			refetch()
			setIsAddDialogOpen(false)
			form.reset()
			setFormError(null)
		},
		onError: (error: any) => {
			let message = 'Failed to create chaincode definition.'
			if (error?.response?.data?.message) {
				message = error.response.data.message
			} else if (error?.message) {
				message = error.message
			}
			setFormError(message)
		},
	})

	const editDefinitionMutation = useMutation({
		...putScFabricDefinitionsByDefinitionIdMutation(),
		onSuccess: () => {
			refetch()
			setEditIdx(null)
			setEditFormError(null)
		},
		onError: (error: any) => {
			let message = 'Failed to update chaincode definition.'
			if (error?.response?.data?.message) {
				message = error.response.data.message
			} else if (error?.message) {
				message = error.message
			}
			setEditFormError(message)
		},
	})

	const deleteDefinitionMutation = useMutation({
		mutationFn: async (definitionId: number) => {
			setDeletingId(definitionId)
			setDeleteError(null)
			await deleteScFabricDefinitionsByDefinitionId({ path: { definitionId } })
		},
		onSuccess: () => {
			refetch()
			setDeletingId(null)
		},
		onError: (error: any) => {
			let message = 'Failed to delete chaincode definition.'
			if (error?.response?.data?.message) {
				message = error.response.data.message
			} else if (error?.message) {
				message = error.message
			}
			setDeleteError(message)
			setDeletingId(null)
		},
	})

	const onSubmit = async (data: VersionFormValues) => {
		setFormError(null)
		await createDefinitionMutation.mutateAsync(data)
	}

	const handleAction = (idx: number, action: LifecycleAction) => {
		if (action === 'install') {
			setSelectedVersionIdx(idx)
			setInstallDialogOpen(true)
			return
		}
		// No need to update versions in local state, use only API data for rendering versions
	}
	const installMutation = useMutation({
		...postScFabricDefinitionsByDefinitionIdInstallMutation(),
		onSuccess: (_, variables) => {
			toast.success('Chaincode installed successfully')
			refetch()
			setInstallDialogOpen(false)
			setSelectedPeers(new Set())
			setSelectedVersionIdx(null)
			refreshTimeline(variables.path.definitionId)
		},
		onError: (error: any) => {
			let message = 'Failed to install chaincode.'
			if (error?.response?.data?.message) {
				message = error.response.data.message
			} else if (error?.message) {
				message = error.message
			}
			setInstallError(message)
		},
	})

	const approveMutation = useMutation({
		...postScFabricDefinitionsByDefinitionIdApproveMutation(),
		onSuccess: (_, variables) => {
			toast.success('Chaincode approved successfully')
			setApproveDialogOpen(false)
			setSelectedPeerId(null)
			setApproveError(null)
			refetch()
			refreshTimeline(variables.path.definitionId)
		},
		onError: (error: any) => {
			let message = 'Failed to approve chaincode.'
			if (error?.response?.data?.message) {
				message = error.response.data.message
			} else if (error?.message) {
				message = error.message
			}
			setApproveError(message)
		},
	})

	const commitMutation = useMutation({
		...postScFabricDefinitionsByDefinitionIdCommitMutation(),
		onSuccess: (_, variables) => {
			toast.success('Chaincode committed successfully')
			setCommitDialogOpen(false)
			setSelectedPeerId(null)
			setCommitError(null)
			refetch()
			refreshTimeline(variables.path.definitionId)
		},
		onError: (error: any) => {
			let message = 'Failed to commit chaincode.'
			if (error?.response?.data?.message) {
				message = error.response.data.message
			} else if (error?.message) {
				message = error.message
			}
			setCommitError(message)
		},
	})

	const deployMutation = useMutation({
		...postScFabricDefinitionsByDefinitionIdDeployMutation(),
		onSuccess: (_, variables) => {
			toast.success('Chaincode deployed successfully')
			setDeployLoading(false)
			setDeployError(null)
			refetch()
			refreshTimeline(variables.path.definitionId)
		},
		onError: (error: any, variables) => {
			let message = 'Failed to deploy chaincode.'
			if (error?.response?.data?.message) {
				message = error.response.data.message
			} else if (error?.message) {
				message = error.message
			}
			setDeployError(message)
			setDeployLoading(false)
			refreshTimeline(variables.path.definitionId)
		},
	})

	const handleInstall = async () => {
		if (selectedVersionIdx === null) return
		const version = versions[selectedVersionIdx]
		if (!version?.id) return
		installMutation.mutate({
			path: { definitionId: version.id },
			body: { peer_ids: Array.from(selectedPeers).map(Number) },
		})
	}

	const handleEdit = (idx: number) => {
		setEditIdx(idx)
		const v = versions[idx]
		editForm.reset({
			endorsementPolicy: v.endorsement_policy,
			dockerImage: v.docker_image,
			version: v.version,
			sequence: v.sequence,
			chaincodeAddress: v.chaincode_address || '',
		})
	}

	const onEditSubmit = async (data: VersionFormValues) => {
		if (editIdx === null) return
		setEditFormError(null)
		await editDefinitionMutation.mutateAsync({
			path: { definitionId: versions[editIdx].id },
			body: {
				docker_image: data.dockerImage,
				endorsement_policy: data.endorsementPolicy,
				version: data.version,
				sequence: data.sequence,
				chaincode_address: data.chaincodeAddress,
			},
		})
	}

	const refreshTimeline = (definitionId: number) => {
		queryClient.invalidateQueries({
			queryKey: getScFabricDefinitionsByDefinitionIdTimelineOptions({ path: { definitionId } }).queryKey,
		})
	}

	if (isLoading) {
		return <Card className="p-6">Loading chaincode details...</Card>
	}
	if (error || !def) {
		return (
			<Card className="p-6">
				Failed to load chaincode definition.{' '}
				<Button variant="link" onClick={() => navigate(-1)}>
					Back
				</Button>
			</Card>
		)
	}

	return (
		<div className="flex-1 p-8 w-full">
			<Button variant="link" onClick={() => navigate(-1)} className="mb-4">
				Back
			</Button>
			<Card className="p-6 mb-6">
				<div className="font-semibold text-lg mb-2">{def.name}</div>
				<div className="text-sm text-muted-foreground mb-1">Network Name: {def.network_name}</div>
			</Card>
			<div className="flex items-center justify-between mb-4">
				<div className="font-semibold">Chaincode Definitions</div>
				<Dialog open={isAddDialogOpen} onOpenChange={setIsAddDialogOpen}>
					<DialogTrigger asChild>
						<Button size="sm" variant="secondary">
							<Plus className="w-4 h-4 mr-2" />
							Add Definition
						</Button>
					</DialogTrigger>
					<DialogContent>
						<DialogHeader>
							<DialogTitle>Add Chaincode Definition</DialogTitle>
							<DialogDescription>Create a new chaincode definition with version and sequence.</DialogDescription>
						</DialogHeader>
						<Form {...form}>
							<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
								<div className="grid grid-cols-2 gap-4">
									<FormField
										control={form.control}
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
										control={form.control}
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
									control={form.control}
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
									control={form.control}
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
									control={form.control}
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
									<Button type="submit">Add Definition</Button>
								</DialogFooter>
							</form>
						</Form>
					</DialogContent>
				</Dialog>
			</div>
			{versions.length === 0 ? (
				<Card className="p-6 text-center text-muted-foreground">No chaincode definitions yet.</Card>
			) : (
				versions.map((v, idx) => (
					<Card key={v.id} className="p-4 mb-4">
						<div className="flex items-center gap-4 mb-2">
							<Badge variant="outline">Version {v.version}</Badge>
							<Badge variant="outline">Sequence {v.sequence}</Badge>
							<DropdownMenu>
								<DropdownMenuTrigger asChild>
									<Button variant="ghost" size="icon">
										<MoreVertical className="w-5 h-5" />
									</Button>
								</DropdownMenuTrigger>
								<DropdownMenuContent>
									<DropdownMenuItem onClick={() => handleEdit(idx)}>Edit</DropdownMenuItem>
									<DropdownMenuItem onClick={() => setConfirmDeleteIdx(idx)} disabled={deletingId === v.id}>
										{deletingId === v.id ? 'Deleting...' : 'Delete'}
									</DropdownMenuItem>
								</DropdownMenuContent>
							</DropdownMenu>
							<AlertDialog open={confirmDeleteIdx === idx} onOpenChange={(open) => !open && setConfirmDeleteIdx(null)}>
								<AlertDialogContent>
									<AlertDialogHeader>
										<AlertDialogTitle>Delete Chaincode Definition</AlertDialogTitle>
										<AlertDialogDescription>Are you sure you want to delete this chaincode definition? This action cannot be undone.</AlertDialogDescription>
									</AlertDialogHeader>
									{deleteError && deletingId === v.id && <div className="text-red-500 text-sm mb-2">{deleteError}</div>}
									<AlertDialogFooter>
										<AlertDialogCancel onClick={() => setConfirmDeleteIdx(null)}>Cancel</AlertDialogCancel>
										<AlertDialogAction
											disabled={deletingId === v.id}
											onClick={() => {
												deleteDefinitionMutation.mutate(v.id)
												setConfirmDeleteIdx(null)
											}}
										>
											{deletingId === v.id ? 'Deleting...' : 'Delete'}
										</AlertDialogAction>
									</AlertDialogFooter>
								</AlertDialogContent>
							</AlertDialog>
						</div>
						<div className="mb-1 text-sm">
							<span className="font-medium">Endorsement Policy:</span> {v.endorsement_policy}
						</div>
						<div className="mb-1 text-sm">
							<span className="font-medium">Docker Image:</span> {v.docker_image}
						</div>
						<div className="mb-1 text-sm">
							<span className="font-medium">Chaincode Address:</span> {v.chaincode_address}
						</div>
						<div className="mt-2 flex gap-2">
							<Button size="sm" variant="outline" onClick={() => handleEdit(idx)}>
								Edit
							</Button>
							{LIFECYCLE_ACTIONS.map((action) => (
								<Button
									key={action}
									size="sm"
									variant="default"
									onClick={() => {
										if (action === 'install') {
											setSelectedVersionIdx(idx)
											setInstallDialogOpen(true)
										} else if (action === 'approve') {
											setSelectedVersionIdx(idx)
											setApproveDialogOpen(true)
										} else if (action === 'commit') {
											setSelectedVersionIdx(idx)
											setCommitDialogOpen(true)
										} else if (action === 'deploy') {
											deployMutation.mutate({ path: { definitionId: v.id }, body: {} })
										}
									}}
								>
									{actionLabels[action]}
								</Button>
							))}
						</div>
						<Dialog open={editIdx === idx} onOpenChange={(open) => !open && setEditIdx(null)}>
							<DialogContent>
								<DialogHeader>
									<DialogTitle>Edit Chaincode Definition</DialogTitle>
								</DialogHeader>
								<Form {...editForm}>
									<form onSubmit={editForm.handleSubmit(onEditSubmit)} className="space-y-4">
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
											<Button type="submit">Save</Button>
										</DialogFooter>
									</form>
								</Form>
							</DialogContent>
						</Dialog>
						<div className="mt-4">
							<div className="text-sm font-medium mb-2">Timeline</div>
							<DefinitionTimeline definitionId={v.id} />
						</div>
					</Card>
				))
			)}
			<Dialog open={installDialogOpen} onOpenChange={setInstallDialogOpen}>
				<DialogContent className="max-w-lg">
					<DialogHeader>
						<DialogTitle>Install Chaincode</DialogTitle>
						<DialogDescription>Select the peers where you want to install the chaincode.</DialogDescription>
					</DialogHeader>
					<div className="space-y-4 max-h-[50vh] overflow-y-auto pr-2">
						{availablePeers.map((peer) => (
							<div key={peer.nodeId} className="flex items-center space-x-2">
								<Checkbox
									id={`peer-${peer.nodeId}`}
									checked={selectedPeers.has(peer.nodeId!.toString())}
									onCheckedChange={(checked) => {
										setSelectedPeers((prev) => {
											const next = new Set(prev)
											if (checked) {
												next.add(peer.nodeId!.toString())
											} else {
												next.delete(peer.nodeId!.toString())
											}
											return next
										})
									}}
								/>
								<label htmlFor={`peer-${peer.nodeId}`} className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
									{peer.node?.name} ({peer.node?.fabricPeer?.mspId})
								</label>
							</div>
						))}
					</div>
					{installError && <div className="text-red-500 text-sm mt-2 break-words max-w-full">{installError}</div>}
					<DialogFooter>
						<Button onClick={handleInstall} disabled={selectedPeers.size === 0 || installMutation.isPending}>
							{installMutation.isPending ? 'Installing...' : 'Install'}
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
			<Dialog open={approveDialogOpen} onOpenChange={setApproveDialogOpen}>
				<DialogContent className="max-w-lg">
					<DialogHeader>
						<DialogTitle>Approve Chaincode</DialogTitle>
						<DialogDescription>Select the peer to approve the chaincode.</DialogDescription>
					</DialogHeader>
					<div className="space-y-4 max-h-[50vh] overflow-y-auto pr-2">
						{availablePeers.map((peer) => (
							<div key={peer.nodeId} className="flex items-center space-x-2">
								<Checkbox
									id={`peer-${peer.nodeId}`}
									checked={selectedPeerId === peer.nodeId!.toString()}
									onCheckedChange={(checked) => {
										setSelectedPeerId(checked ? peer.nodeId!.toString() : null)
									}}
								/>
								<label htmlFor={`peer-${peer.nodeId}`} className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
									{peer.node?.name} ({peer.node?.fabricPeer?.mspId})
								</label>
							</div>
						))}
					</div>
					{approveError && <div className="text-red-500 text-sm mt-2 break-words max-w-full">{approveError}</div>}
					<DialogFooter>
						<Button
							onClick={() => {
								if (selectedPeerId) {
									approveMutation.mutate({ path: { definitionId: versions[selectedVersionIdx!].id }, body: { peer_id: Number(selectedPeerId) } })
								}
							}}
							disabled={selectedPeerId === null || approveMutation.isPending}
						>
							{approveMutation.isPending ? 'Approving...' : 'Approve'}
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
			<Dialog open={commitDialogOpen} onOpenChange={setCommitDialogOpen}>
				<DialogContent className="max-w-lg">
					<DialogHeader>
						<DialogTitle>Commit Chaincode</DialogTitle>
						<DialogDescription>Select the peer to commit the chaincode.</DialogDescription>
					</DialogHeader>
					<div className="space-y-4 max-h-[50vh] overflow-y-auto pr-2">
						{availablePeers.map((peer) => (
							<div key={peer.nodeId} className="flex items-center space-x-2">
								<Checkbox
									id={`peer-${peer.nodeId}`}
									checked={selectedPeerId === peer.nodeId!.toString()}
									onCheckedChange={(checked) => {
										setSelectedPeerId(checked ? peer.nodeId!.toString() : null)
									}}
								/>
								<label htmlFor={`peer-${peer.nodeId}`} className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
									{peer.node?.name} ({peer.node?.fabricPeer?.mspId})
								</label>
							</div>
						))}
					</div>
					{commitError && <div className="text-red-500 text-sm mt-2 break-words max-w-full">{commitError}</div>}
					<DialogFooter>
						<Button
							onClick={() => {
								if (selectedPeerId) {
									commitMutation.mutate({ path: { definitionId: versions[selectedVersionIdx!].id }, body: { peer_id: Number(selectedPeerId) } })
								}
							}}
							disabled={selectedPeerId === null || commitMutation.isPending}
						>
							{commitMutation.isPending ? 'Committing...' : 'Commit'}
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</div>
	)
}

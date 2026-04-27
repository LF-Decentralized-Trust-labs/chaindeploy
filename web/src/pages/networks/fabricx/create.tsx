import { HandlerOrganizationResponse, HttpFabricXOrganization, HttpNodeResponse } from '@/api/client'
import {
	getNetworksFabricxByIdQueryKey,
	getNodesOptions,
	getOrganizationsOptions,
	postNetworksFabricxByIdNodesByNodeIdJoinMutation,
	postNetworksFabricxMutation,
} from '@/api/client/@tanstack/react-query.gen'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertCircle, CheckCircle2, Network, Plus, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import * as z from 'zod'

// FabricX two-stage flow (MVP):
// 1. User creates FabricXOrdererGroup + FabricXCommitter nodes via the existing
//    node creation flow. Those sit in a "created but not started" state with
//    certs/config generated.
// 2. This page creates the FabricX network referencing those nodes (generates
//    the genesis block), then joins each node (writes genesis + starts).

const orgRowSchema = z.object({
	organizationId: z.coerce.number().min(1, 'Pick an organization'),
	ordererNodeId: z.coerce.number().min(1, 'Pick an orderer group node'),
	committerNodeId: z.coerce.number().optional(),
})

const FABRICX_CHANNEL_ID = 'arma'

const formSchema = z.object({
	name: z.string().min(1, 'Network name is required'),
	description: z.string().optional(),
	localDev: z.boolean().optional(),
	organizations: z.array(orgRowSchema).min(1, 'Add at least one organization'),
})

type FabricXFormValues = z.infer<typeof formSchema>

type FabricXNode = HttpNodeResponse

export default function FabricXCreatePage() {
	const navigate = useNavigate()
	const queryClient = useQueryClient()
	const [createdNetworkId, setCreatedNetworkId] = useState<number | null>(null)
	const [joinedNodes, setJoinedNodes] = useState<Record<number, 'pending' | 'joining' | 'joined' | 'error'>>({})

	const orgsQuery = useQuery({ ...getOrganizationsOptions() })
	const fabricXNodesQuery = useQuery({
		...getNodesOptions({ query: { platform: 'FABRICX' } }),
	})

	const organizations = (orgsQuery.data as unknown as { items?: HandlerOrganizationResponse[] } | undefined)?.items ?? []
	const allFabricXNodes: FabricXNode[] = (fabricXNodesQuery.data as unknown as { items?: FabricXNode[] } | undefined)?.items ?? []

	// TODO(node-groups): both committer and orderer are now node_groups, not
	// monolithic nodes. The picker should query getNodeGroups() instead of
	// getNodes() + filter by groupType. Today this filter resolves to empty
	// for new installs because no FABRICX_COMMITTER / FABRICX_ORDERER_GROUP
	// node-type rows are created anymore. New deployments should use the
	// quickstart flow which already speaks node_groups end-to-end.
	const ordererGroupNodes = useMemo(
		() => allFabricXNodes.filter((n) => n.nodeType === 'FABRICX_ORDERER_GROUP'),
		[allFabricXNodes]
	)
	const committerNodes = useMemo(
		() => allFabricXNodes.filter((n) => n.nodeType === 'FABRICX_COMMITTER'),
		[allFabricXNodes]
	)

	const form = useForm<FabricXFormValues>({
		resolver: zodResolver(formSchema),
		defaultValues: {
			name: '',
			description: '',
			localDev: false,
			organizations: [{ organizationId: 0, ordererNodeId: 0, committerNodeId: undefined }],
		},
	})

	const createNetwork = useMutation({
		...postNetworksFabricxMutation(),
	})
	const joinNode = useMutation({
		...postNetworksFabricxByIdNodesByNodeIdJoinMutation(),
	})

	const onSubmit = async (values: FabricXFormValues) => {
		try {
			const orgs: HttpFabricXOrganization[] = values.organizations.map((r) => ({
				id: r.organizationId,
				ordererNodeId: r.ordererNodeId,
				committerNodeId: r.committerNodeId || undefined,
			}))

			const resp = await createNetwork.mutateAsync({
				body: {
					name: values.name,
					description: values.description,
					config: {
						channelName: FABRICX_CHANNEL_ID,
						organizations: orgs,
						localDev: values.localDev ?? false,
					},
				},
			})

			const networkId = (resp as { id?: number }).id
			if (!networkId) {
				toast.error('Network created but response missing id')
				return
			}

			setCreatedNetworkId(networkId)
			const initialStatus: Record<number, 'pending'> = {}
			for (const r of values.organizations) {
				initialStatus[r.ordererNodeId] = 'pending'
				if (r.committerNodeId) initialStatus[r.committerNodeId] = 'pending'
			}
			setJoinedNodes(initialStatus)
			toast.success('FabricX network created — now join each node to start it.')
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Unknown error'
			toast.error(`Failed to create network: ${msg}`)
		}
	}

	const handleJoin = async (nodeId: number) => {
		if (!createdNetworkId) return
		setJoinedNodes((s) => ({ ...s, [nodeId]: 'joining' }))
		try {
			await joinNode.mutateAsync({
				path: { id: createdNetworkId, nodeId },
			})
			setJoinedNodes((s) => ({ ...s, [nodeId]: 'joined' }))
			await queryClient.invalidateQueries({
				queryKey: getNetworksFabricxByIdQueryKey({ path: { id: createdNetworkId } }),
			})
			toast.success(`Node ${nodeId} joined and started`)
		} catch (err) {
			setJoinedNodes((s) => ({ ...s, [nodeId]: 'error' }))
			const msg = err instanceof Error ? err.message : 'Unknown error'
			toast.error(`Failed to join node ${nodeId}: ${msg}`)
		}
	}

	const orgRows = form.watch('organizations')

	const loading = orgsQuery.isLoading || fabricXNodesQuery.isLoading

	if (loading) {
		return (
			<div className="p-6 space-y-4">
				<Skeleton className="h-10 w-64" />
				<Skeleton className="h-48 w-full" />
			</div>
		)
	}

	const needsNodes = ordererGroupNodes.length === 0
	const allJoined =
		createdNetworkId !== null &&
		Object.values(joinedNodes).length > 0 &&
		Object.values(joinedNodes).every((s) => s === 'joined')

	return (
		<div className="container mx-auto p-6 space-y-6 max-w-4xl">
			<div className="flex items-center gap-3">
				<Network className="h-8 w-8 text-primary" />
				<div>
					<h1 className="text-2xl font-semibold">Create FabricX Network</h1>
					<p className="text-sm text-muted-foreground">
						Two-stage deployment: create network from existing orderer-group / committer nodes, then join each node to
						start it.
					</p>
				</div>
			</div>

			{needsNodes && (
				<Alert variant="destructive">
					<AlertCircle className="h-4 w-4" />
					<AlertTitle>No FabricX orderer group nodes found</AlertTitle>
					<AlertDescription>
						You must first create at least one FabricXOrdererGroup node (stage 1). Certs and config are generated, but
						the containers won't start until the network is created and joined.{' '}
						<Link to="/nodes/create" className="underline">
							Create a node
						</Link>
						.
					</AlertDescription>
				</Alert>
			)}

			{createdNetworkId === null ? (
				<Form {...form}>
					<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
						<Card>
							<CardHeader>
								<CardTitle>Network basics</CardTitle>
								<CardDescription>Name, description, and channel ID for the Arma-consensus network.</CardDescription>
							</CardHeader>
							<CardContent className="space-y-4">
								<FormField
									control={form.control}
									name="name"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Name</FormLabel>
											<FormControl>
												<Input placeholder="fabricx-mvp" {...field} />
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
												<Textarea placeholder="Optional" {...field} />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
								<div className="space-y-2">
									<FormLabel>Channel ID</FormLabel>
									<div className="rounded-md border bg-muted/40 px-3 py-2 text-sm font-mono">{FABRICX_CHANNEL_ID}</div>
									<FormDescription>
										FabricX requires the channel ID to be <code>{FABRICX_CHANNEL_ID}</code>. This cannot be changed.
									</FormDescription>
								</div>
								<FormField
									control={form.control}
									name="localDev"
									render={({ field }) => (
										<FormItem className="flex flex-row items-start gap-3 rounded-md border p-3">
											<FormControl>
												<Checkbox
													checked={field.value ?? false}
													onCheckedChange={(v) => field.onChange(v === true)}
												/>
											</FormControl>
											<div className="space-y-1 leading-none">
												<FormLabel>Local development mode</FormLabel>
												<FormDescription>
													Enable when ChainLaunch runs on macOS or Windows with Docker Desktop. Swaps the external
													IP for <code>host.docker.internal</code> in the genesis block so containers can reach each
													other, and routes host-originated dials (namespace creation, explorer) through{' '}
													<code>127.0.0.1</code>. Leave unchecked on Linux or when ChainLaunch runs on the same host
													as the containers reachable via the external IP.
												</FormDescription>
											</div>
										</FormItem>
									)}
								/>
							</CardContent>
						</Card>

						<Card>
							<CardHeader>
								<div className="flex items-center justify-between">
									<div>
										<CardTitle>Party / organization mapping</CardTitle>
										<CardDescription>
											Each row binds an organization to its orderer group (required) and optional committer node.
										</CardDescription>
									</div>
									<Button
										type="button"
										variant="outline"
										size="sm"
										onClick={() =>
											form.setValue('organizations', [
												...form.getValues('organizations'),
												{ organizationId: 0, ordererNodeId: 0, committerNodeId: undefined },
											])
										}
									>
										<Plus className="h-4 w-4 mr-1" /> Add party
									</Button>
								</div>
							</CardHeader>
							<CardContent className="space-y-4">
								{orgRows.map((_, idx) => (
									<div key={idx} className="grid grid-cols-1 md:grid-cols-12 gap-3 items-end border rounded-md p-3">
										<FormField
											control={form.control}
											name={`organizations.${idx}.organizationId`}
											render={({ field }) => (
												<FormItem className="md:col-span-4">
													<FormLabel>Organization</FormLabel>
													<Select
														value={field.value?.toString() ?? ''}
														onValueChange={(v) => field.onChange(Number(v))}
													>
														<FormControl>
															<SelectTrigger>
																<SelectValue placeholder="Select org" />
															</SelectTrigger>
														</FormControl>
														<SelectContent>
															{organizations.map((o) => (
																<SelectItem key={o.id} value={String(o.id)}>
																	{o.mspId} ({o.id})
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
											name={`organizations.${idx}.ordererNodeId`}
											render={({ field }) => (
												<FormItem className="md:col-span-4">
													<FormLabel>Orderer group node</FormLabel>
													<Select
														value={field.value?.toString() ?? ''}
														onValueChange={(v) => field.onChange(Number(v))}
													>
														<FormControl>
															<SelectTrigger>
																<SelectValue placeholder="Select orderer" />
															</SelectTrigger>
														</FormControl>
														<SelectContent>
															{ordererGroupNodes.map((n) => (
																<SelectItem key={n.id} value={String(n.id)}>
																	{n.name} ({n.id})
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
											name={`organizations.${idx}.committerNodeId`}
											render={({ field }) => (
												<FormItem className="md:col-span-3">
													<FormLabel>Committer node</FormLabel>
													<Select
														value={field.value?.toString() ?? ''}
														onValueChange={(v) => field.onChange(v ? Number(v) : undefined)}
													>
														<FormControl>
															<SelectTrigger>
																<SelectValue placeholder="Optional" />
															</SelectTrigger>
														</FormControl>
														<SelectContent>
															{committerNodes.map((n) => (
																<SelectItem key={n.id} value={String(n.id)}>
																	{n.name} ({n.id})
																</SelectItem>
															))}
														</SelectContent>
													</Select>
													<FormMessage />
												</FormItem>
											)}
										/>
										<div className="md:col-span-1 flex justify-end">
											<Button
												type="button"
												variant="ghost"
												size="icon"
												disabled={orgRows.length === 1}
												onClick={() =>
													form.setValue(
														'organizations',
														orgRows.filter((_, i) => i !== idx)
													)
												}
											>
												<Trash2 className="h-4 w-4" />
											</Button>
										</div>
									</div>
								))}
							</CardContent>
						</Card>

						<div className="flex justify-end gap-3">
							<Button type="button" variant="outline" onClick={() => navigate('/networks')}>
								Cancel
							</Button>
							<Button type="submit" disabled={createNetwork.isPending || needsNodes}>
								{createNetwork.isPending ? 'Creating...' : 'Create network (stage 1)'}
							</Button>
						</div>
					</form>
				</Form>
			) : (
				<Card>
					<CardHeader>
						<CardTitle>Stage 2: Join nodes</CardTitle>
						<CardDescription>
							Network #{createdNetworkId} created with genesis block. Now write genesis + start each node.
						</CardDescription>
					</CardHeader>
					<CardContent className="space-y-3">
						{Object.entries(joinedNodes).map(([nodeIdStr, status]) => {
							const nodeId = Number(nodeIdStr)
							const node = allFabricXNodes.find((n) => n.id === nodeId)
							return (
								<div key={nodeId} className="flex items-center justify-between border rounded-md p-3">
									<div>
										<div className="font-medium">
											{node?.name ?? `Node ${nodeId}`}{' '}
											<Badge variant="outline">{node?.nodeType}</Badge>
										</div>
										<div className="text-xs text-muted-foreground">ID: {nodeId}</div>
									</div>
									<div className="flex items-center gap-2">
										{status === 'joined' && <CheckCircle2 className="h-5 w-5 text-green-600" />}
										<Button
											size="sm"
											variant={status === 'joined' ? 'outline' : 'default'}
											disabled={status === 'joining' || status === 'joined'}
											onClick={() => handleJoin(nodeId)}
										>
											{status === 'joining'
												? 'Joining...'
												: status === 'joined'
													? 'Joined'
													: status === 'error'
														? 'Retry'
														: 'Join & start'}
										</Button>
									</div>
								</div>
							)
						})}
						{allJoined && (
							<Alert>
								<CheckCircle2 className="h-4 w-4" />
								<AlertTitle>All nodes joined</AlertTitle>
								<AlertDescription>
									FabricX network is up.{' '}
									<Link to={`/networks`} className="underline">
										Back to networks
									</Link>
								</AlertDescription>
							</Alert>
						)}
					</CardContent>
				</Card>
			)}
		</div>
	)
}

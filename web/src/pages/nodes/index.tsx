import { HttpNodeResponse } from '@/api/client'
import { deleteNodesByIdMutation, getNodesOptions, postNodesByIdRestartMutation, postNodesByIdStartMutation, postNodesByIdStopMutation } from '@/api/client/@tanstack/react-query.gen'
import { BesuIcon } from '@/components/icons/besu-icon'
import { FabricIcon } from '@/components/icons/fabric-icon'
import { PageHeader, PageShell } from '@/components/layout/page-shell'
import { NodeListItem } from '@/components/nodes/node-list-item'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Pagination } from '@/components/ui/pagination'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useMutation, useQuery } from '@tanstack/react-query'
import { ChevronDown, MoreVertical, ScrollText, Server } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { toast } from 'sonner'

type PlatformFilter = 'ALL' | 'FABRIC' | 'FABRICX' | 'BESU'

type BulkAction = 'start' | 'stop' | 'restart' | 'delete'

const ACTION_VERBS = {
	start: { present: 'start', progressive: 'starting', past: 'started' },
	stop: { present: 'stop', progressive: 'stopping', past: 'stopped' },
	restart: { present: 'restart', progressive: 'restarting', past: 'restarted' },
	delete: { present: 'delete', progressive: 'deleting', past: 'deleted' },
} as const

interface BulkActionDetails {
	action: BulkAction
	nodes: HttpNodeResponse[]
}

function getNodeActions(status: string) {
	switch (status.toLowerCase()) {
		case 'running':
			return [
				{ label: 'Stop', action: 'stop' },
				{ label: 'Restart', action: 'restart' },
			]
		case 'stopped':
			return [
				{ label: 'Start', action: 'start' },
				{ label: 'Delete', action: 'delete' },
			]
		case 'stopping':
			return [{ label: 'Stop', action: 'stop' }]
		case 'error':
			return [
				{ label: 'Start', action: 'start' },
				{ label: 'Restart', action: 'restart' },
				{ label: 'Delete', action: 'delete' },
			]
		case 'starting':
		case 'stopping':
			return [
				{ label: 'Stop', action: 'stop' },
				{ label: 'Delete', action: 'delete' },
			] // No actions while transitioning
		default:
			return [
				{ label: 'Start', action: 'start' },
				{ label: 'Stop', action: 'stop' },
				{ label: 'Restart', action: 'restart' },
			]
	}
}

export default function NodesPage() {
	const [page, setPage] = useState(1)
	const [platform, setPlatform] = useState<PlatformFilter>('ALL')
	const pageSize = 10

	const { data: nodes, refetch } = useQuery({
		...getNodesOptions({
			query: {
				page,
				limit: pageSize,
				...(platform !== 'ALL' ? { platform } : {}),
			},
		}),
	})

	// Per-platform totals for the summary strip. limit=1 so we only pay for the
	// `total` field — the `items` payload is a single row per query.
	const fabricTotals = useQuery({
		...getNodesOptions({ query: { page: 1, limit: 1, platform: 'FABRIC' } }),
	})
	const fabricxTotals = useQuery({
		...getNodesOptions({ query: { page: 1, limit: 1, platform: 'FABRICX' } }),
	})
	const besuTotals = useQuery({
		...getNodesOptions({ query: { page: 1, limit: 1, platform: 'BESU' } }),
	})
	const allTotals = useQuery({
		...getNodesOptions({ query: { page: 1, limit: 1 } }),
	})

	const stats = useMemo(
		() => ({
			all: allTotals.data?.total ?? 0,
			fabric: fabricTotals.data?.total ?? 0,
			fabricx: fabricxTotals.data?.total ?? 0,
			besu: besuTotals.data?.total ?? 0,
		}),
		[allTotals.data?.total, fabricTotals.data?.total, fabricxTotals.data?.total, besuTotals.data?.total]
	)

	const handlePlatformChange = (value: string) => {
		setPlatform(value as PlatformFilter)
		setPage(1)
	}

	const refetchAll = () => {
		refetch()
		allTotals.refetch()
		fabricTotals.refetch()
		fabricxTotals.refetch()
		besuTotals.refetch()
	}

	const [nodeToDelete, setNodeToDelete] = useState<HttpNodeResponse | null>(null)
	const [selectedNodes, setSelectedNodes] = useState<HttpNodeResponse[]>([])
	const startNodeBulk = useMutation(postNodesByIdStartMutation())
	const stopNodeBulk = useMutation(postNodesByIdStopMutation())
	const restartNodeBulk = useMutation(postNodesByIdRestartMutation())
	const deleteNodeBulk = useMutation(deleteNodesByIdMutation())
	const [bulkActionDetails, setBulkActionDetails] = useState<BulkActionDetails | null>(null)

	const startNode = useMutation({
		...postNodesByIdStartMutation(),
		onSuccess: () => {
			toast.success('Node started')
			refetchAll()
		},
	})
	const stopNode = useMutation({
		...postNodesByIdStopMutation(),
		onSuccess: () => {
			toast.success('Node stopped')
			refetchAll()
		},
	})
	const restartNode = useMutation({
		...postNodesByIdRestartMutation(),
		onSuccess: () => {
			toast.success('Node restarted')
			refetchAll()
		},
	})
	const deleteNode = useMutation({
		...deleteNodesByIdMutation(),
		onSuccess: () => {
			toast.success('Node deleted')
			refetchAll()
		},
	})
	const handleBulkAction = async (action: BulkAction) => {
		if (action === 'delete') {
			setBulkActionDetails({
				action,
				nodes: selectedNodes,
			})
		} else {
			const actionMutation = {
				start: startNodeBulk,
				stop: stopNodeBulk,
				restart: restartNodeBulk,
			}[action]
			const promise = Promise.all(
				selectedNodes.map((node) =>
					actionMutation.mutateAsync({
						path: { id: node.id! },
					})
				)
			)
			await toast.promise(promise, {
				loading: `${ACTION_VERBS[action].progressive} ${selectedNodes.length} node${selectedNodes.length > 1 ? 's' : ''}...`,
				success: `Successfully ${ACTION_VERBS[action].past} ${selectedNodes.length} node${selectedNodes.length > 1 ? 's' : ''}`,
				error: (error: any) => `Failed to ${ACTION_VERBS[action].present} nodes: ${error.message}`,
			})
			await promise

			setSelectedNodes([])
			refetchAll()
		}
	}

	const handleBulkActionConfirm = async () => {
		if (!bulkActionDetails) return

		const { action, nodes } = bulkActionDetails
		const actionMutation = {
			start: startNodeBulk,
			stop: stopNodeBulk,
			restart: restartNodeBulk,
			delete: deleteNodeBulk,
		}[action]
		const promise = Promise.all(
			nodes.map((node) =>
				actionMutation.mutateAsync({
					path: { id: node.id! },
				})
			)
		)
		await toast.promise(
			promise,
			{
				loading: `${ACTION_VERBS[action].progressive} ${nodes.length} node${nodes.length > 1 ? 's' : ''}...`,
				success: `Successfully ${ACTION_VERBS[action].past} ${nodes.length} node${nodes.length > 1 ? 's' : ''}`,
				error: (error: any) => `Failed to ${ACTION_VERBS[action].present} nodes: ${error.message}`,
			}
		)
		await promise

		setSelectedNodes([])
		refetchAll()
		setBulkActionDetails(null)
	}

	const handleNodeAction = async (nodeId: number, action: string) => {
		try {
			switch (action) {
				case 'start':
					await startNode.mutateAsync({ path: { id: nodeId } })
					break
				case 'stop':
					await stopNode.mutateAsync({ path: { id: nodeId } })
					break
				case 'restart':
					await restartNode.mutateAsync({ path: { id: nodeId } })
					break
				case 'delete':
					// Find the node to delete
					const node = nodes?.items?.find((n) => n.id === nodeId)
					if (node) {
						setNodeToDelete(node)
					}
					break
			}
		} catch (error) {
			// Error handling is done in the mutation callbacks
		}
	}

	const handleDeleteConfirm = async () => {
		if (!nodeToDelete) return

		try {
			await deleteNode.mutateAsync({ path: { id: nodeToDelete.id! } })
		} finally {
			setNodeToDelete(null)
		}
	}

	const handleSelectAll = (checked: boolean) => {
		if (checked) {
			// Filter out nodes that are in transitional states
			const selectableNodes = nodes?.items?.filter((node) => !['starting', 'stopping'].includes(node.status?.toLowerCase() || '')) || []
			setSelectedNodes(selectableNodes)
		} else {
			setSelectedNodes([])
		}
	}

	// Empty-state welcome only fires when the whole system has zero nodes.
	// Filter-scoped empties (e.g. "no Besu nodes yet") render inside the list
	// so the stats strip and tabs stay visible.
	const hasAnyNodes = (allTotals.data?.total ?? 0) > 0
	if (!hasAnyNodes && !allTotals.isLoading) {
		return (
			<PageShell maxWidth="detail">
				<div className="py-12 text-center">
					<Server className="mx-auto mb-4 h-12 w-12 text-muted-foreground" />
					<h1 className="text-2xl font-semibold tracking-tight">Create your first node</h1>
					<p className="mt-2 text-sm text-muted-foreground">Get started by creating a blockchain node.</p>
				</div>

				<div className="space-y-4">
						<Card className="p-6">
							<div className="flex items-center justify-between">
								<div className="flex items-center gap-4">
									<FabricIcon className="h-8 w-8" />
									<div>
										<h3 className="font-semibold">Fabric Node</h3>
										<p className="text-sm text-muted-foreground">Create a Hyperledger Fabric peer or orderer node</p>
									</div>
								</div>
								<div className="flex gap-2">
									<Button variant="outline" asChild>
										<Link to="/nodes/fabric/bulk">
											<Server className="h-4 w-4 mr-2" />
											Bulk Create
										</Link>
									</Button>
									<Button asChild>
										<Link to="/nodes/create">Create Node</Link>
									</Button>
								</div>
							</div>
						</Card>

						<Card className="p-6">
							<div className="flex items-center justify-between">
								<div className="flex items-center gap-4">
									<BesuIcon className="h-8 w-8" />
									<div>
										<h3 className="font-semibold">Besu Node</h3>
										<p className="text-sm text-muted-foreground">Create a Hyperledger Besu node</p>
									</div>
								</div>
								<Button asChild>
									<Link to="/nodes/besu/create">Create Node</Link>
								</Button>
							</div>
						</Card>
					</div>
				</PageShell>
			)
	}

	return (
		<>
		<PageShell maxWidth="dashboard">
			<PageHeader title="Nodes" description="Manage your blockchain nodes">
				{selectedNodes.length > 0 && (
					<DropdownMenu>
						<DropdownMenuTrigger asChild>
							<Button variant="outline">
								Bulk Actions ({selectedNodes.length})
								<ChevronDown className="ml-2 h-4 w-4" />
							</Button>
						</DropdownMenuTrigger>
						<DropdownMenuContent align="end">
							{getNodeActions(selectedNodes[0].status || '').map(({ label, action }) => (
								<DropdownMenuItem
									key={action}
									onClick={() => handleBulkAction(action as BulkAction)}
									disabled={startNode.isPending || stopNode.isPending || restartNode.isPending}
								>
									{label}
								</DropdownMenuItem>
							))}
						</DropdownMenuContent>
					</DropdownMenu>
				)}
				<Button asChild variant="outline">
					<Link to="/nodes/logs">
						<ScrollText className="mr-2 h-4 w-4" />
						View Logs
					</Link>
				</Button>
				<Button asChild variant="outline">
					<Link to="/nodes/fabric/bulk">
						<Server className="mr-2 h-4 w-4" />
						Bulk Create Fabric
					</Link>
				</Button>
				<Button asChild>
					<Link to="/nodes/create">Create Node</Link>
				</Button>
			</PageHeader>

			<dl className="mb-6 grid grid-cols-2 border-y border-border sm:grid-cols-4">
					{(
						[
							{ key: 'ALL', label: 'Total nodes', value: stats.all },
							{ key: 'FABRIC', label: 'Fabric', value: stats.fabric },
							{ key: 'FABRICX', label: 'FabricX', value: stats.fabricx },
							{ key: 'BESU', label: 'Besu', value: stats.besu },
						] as { key: PlatformFilter; label: string; value: number }[]
					).map((tile, idx) => {
						const selected = platform === tile.key
						return (
							<button
								key={tile.key}
								type="button"
								onClick={() => handlePlatformChange(tile.key)}
								aria-pressed={selected}
								className={[
									'relative text-left cursor-pointer px-4 py-4 sm:py-5 transition-colors',
									// First column has no left border/padding on desktop so the
									// outer `border-y` frames the strip cleanly.
									idx === 0 ? 'sm:pl-0 sm:pr-4' : 'sm:border-l border-border',
									// Row 2 on mobile (cols 3 & 4) sits below a divider.
									idx >= 2 ? 'border-t sm:border-t-0' : '',
									'hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset',
									selected ? 'bg-muted/60' : '',
								].join(' ')}
							>
								<dt className="truncate text-sm font-medium text-muted-foreground">{tile.label}</dt>
								<dd className="mt-1 text-3xl font-semibold tracking-tight tabular-nums">{tile.value}</dd>
								{selected && <span aria-hidden className="absolute inset-x-0 -bottom-px h-0.5 bg-primary" />}
							</button>
						)
					})}
				</dl>

				<Tabs value={platform} onValueChange={handlePlatformChange} className="mb-4">
					<TabsList>
						<TabsTrigger value="ALL" className="gap-1.5">
							All <span className="text-muted-foreground tabular-nums">{stats.all}</span>
						</TabsTrigger>
						<TabsTrigger value="FABRIC" className="gap-1.5">
							Fabric <span className="text-muted-foreground tabular-nums">{stats.fabric}</span>
						</TabsTrigger>
						<TabsTrigger value="FABRICX" className="gap-1.5">
							FabricX <span className="text-muted-foreground tabular-nums">{stats.fabricx}</span>
						</TabsTrigger>
						<TabsTrigger value="BESU" className="gap-1.5">
							Besu <span className="text-muted-foreground tabular-nums">{stats.besu}</span>
						</TabsTrigger>
					</TabsList>
				</Tabs>

				<div className="grid gap-4">
					{nodes?.items && nodes.items.length > 0 && (
						<div className="flex items-center px-4 py-2 border rounded-lg bg-background">
							<Checkbox
								checked={nodes.items.length > 0 && selectedNodes.length === nodes.items.filter((node) => !['starting', 'stopping'].includes(node.status?.toLowerCase() || '')).length}
								onCheckedChange={handleSelectAll}
								className="mr-4"
							/>
							<span className="text-sm text-muted-foreground">Select all on this page</span>
						</div>
					)}
					{(!nodes?.items || nodes.items.length === 0) && (
						<div className="flex flex-col items-center justify-center rounded-lg border border-dashed px-4 py-12 text-center">
							<Server className="h-8 w-8 text-muted-foreground mb-3" />
							<p className="text-sm font-medium">No {platform === 'ALL' ? '' : platform === 'FABRICX' ? 'FabricX ' : platform === 'FABRIC' ? 'Fabric ' : 'Besu '}nodes on this page</p>
							<p className="text-sm text-muted-foreground">Try another filter or create one.</p>
						</div>
					)}
					{nodes?.items?.map((node) => (
						<div key={node.id} className="group relative rounded-lg border">
							<NodeListItem
								node={node}
								isSelected={selectedNodes.some((n) => n.id === node.id)}
								onSelectionChange={(checked) => {
									if (checked) {
										setSelectedNodes([...selectedNodes, node])
									} else {
										setSelectedNodes(selectedNodes.filter((n) => n.id !== node.id))
									}
								}}
								disabled={
									['starting', 'stopping'].includes(node.status?.toLowerCase() || '') || startNode.isPending || stopNode.isPending || restartNode.isPending || deleteNode.isPending
								}
							/>
							<div className="absolute right-4 top-4">
								<DropdownMenu>
									<DropdownMenuTrigger asChild>
										<Button variant="ghost" size="icon">
											<MoreVertical className="h-4 w-4" />
										</Button>
									</DropdownMenuTrigger>
									<DropdownMenuContent align="end">
										{getNodeActions(node.status || '').map(({ label, action }) => (
											<DropdownMenuItem key={action} onClick={() => handleNodeAction(node.id!, action)}>
												{label}
											</DropdownMenuItem>
										))}
									</DropdownMenuContent>
								</DropdownMenu>
							</div>
						</div>
					))}
				</div>

			{(nodes?.total || 0) > pageSize && (
				<div className="mt-6 flex justify-center">
					<Pagination currentPage={page} pageSize={pageSize} totalItems={nodes?.total || 0} onPageChange={setPage} />
				</div>
			)}
		</PageShell>

		<AlertDialog open={!!nodeToDelete} onOpenChange={(open) => !open && setNodeToDelete(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Are you sure?</AlertDialogTitle>
						<AlertDialogDescription>
							This action cannot be undone. This will permanently delete the node <span className="font-medium">{nodeToDelete?.name}</span> and remove all associated data.
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction onClick={handleDeleteConfirm} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
							Delete
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>

			<AlertDialog open={!!bulkActionDetails} onOpenChange={(open) => !open && setBulkActionDetails(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Confirm Bulk Action</AlertDialogTitle>
						<AlertDialogDescription className="space-y-2">
							<p>Are you sure you want to {bulkActionDetails?.action} the following nodes?</p>
							<ul className="list-disc pl-4 space-y-1">
								{bulkActionDetails?.nodes.map((node) => (
									<li key={node.id} className="text-sm">
										{node.name}
									</li>
								))}
							</ul>
							{bulkActionDetails?.action === 'delete' && (
								<p className="text-destructive mt-2">This action cannot be undone. This will permanently delete the selected nodes and remove all associated data.</p>
							)}
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction
							onClick={handleBulkActionConfirm}
							className={bulkActionDetails?.action === 'delete' ? 'bg-destructive text-destructive-foreground hover:bg-destructive/90' : ''}
						>
							{bulkActionDetails?.action?.charAt(0).toUpperCase() + (bulkActionDetails?.action?.slice(1) || '')}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</>
	)
}

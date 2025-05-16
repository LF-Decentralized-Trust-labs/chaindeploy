import { HttpNodeResponse } from '@/api/client'
import { deleteNodesByIdMutation, getNodesOptions, postNodesByIdRestartMutation, postNodesByIdStartMutation, postNodesByIdStopMutation } from '@/api/client/@tanstack/react-query.gen'
import { BesuIcon } from '@/components/icons/besu-icon'
import { FabricIcon } from '@/components/icons/fabric-icon'
import { NodeListItem } from '@/components/nodes/node-list-item'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Pagination } from '@/components/ui/pagination'
import { useMutation, useQuery } from '@tanstack/react-query'
import { ChevronDown, MoreVertical, ScrollText, Server } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { toast } from 'sonner'

type BulkAction = 'start' | 'stop' | 'restart' | 'delete'

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
	const pageSize = 10

	const { data: nodes, refetch } = useQuery({
		...getNodesOptions({
			query: {
				page,
				limit: pageSize,
			},
		}),
	})

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
			refetch()
		},
	})
	const stopNode = useMutation({
		...postNodesByIdStopMutation(),
		onSuccess: () => {
			toast.success('Node stopped')
			refetch()
		},
	})
	const restartNode = useMutation({
		...postNodesByIdRestartMutation(),
		onSuccess: () => {
			toast.success('Node restarted')
			refetch()
		},
	})
	const deleteNode = useMutation({
		...deleteNodesByIdMutation(),
		onSuccess: () => {
			toast.success('Node deleted')
			refetch()
		},
	})
	const handleBulkAction = async (action: BulkAction) => {
		if (action === 'delete') {
			setBulkActionDetails({
				action,
				nodes: selectedNodes,
			})
		} else {
			try {
				const actionMutation = {
					start: startNodeBulk,
					stop: stopNodeBulk,
					restart: restartNodeBulk,
				}[action]

				await Promise.all(
					selectedNodes.map((node) =>
						actionMutation.mutateAsync({
							path: { id: node.id! },
						})
					)
				)
				toast.success(`Successfully ${action}ed selected nodes`)
				setSelectedNodes([])
				refetch()
			} catch (error: any) {
				toast.error(`Failed to ${action} nodes`, {
					description: error.message,
				})
			}
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

		try {
			await Promise.all(
				nodes.map((node) =>
					actionMutation.mutateAsync({
						path: { id: node.id! },
					})
				)
			)
			toast.success(`Successfully ${action}ed selected nodes`)
			setSelectedNodes([])
			refetch()
		} catch (error: any) {
			toast.error(`Failed to ${action} nodes`, {
				description: error.message,
			})
		} finally {
			setBulkActionDetails(null)
		}
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

	if (!nodes?.items?.length) {
		return (
			<div className="flex-1 p-8">
				<div className="max-w-4xl mx-auto">
					<div className="text-center mb-12">
						<div className="flex justify-center mb-4">
							<Server className="h-12 w-12 text-muted-foreground" />
						</div>
						<h1 className="text-2xl font-semibold mb-2">Create your first node</h1>
						<p className="text-muted-foreground">Get started by creating a blockchain node.</p>
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
				</div>
			</div>
		)
	}

	return (
		<div className="flex-1 space-y-4 p-8">
			<div className="max-w-4xl mx-auto">
				<div className="flex items-center justify-between mb-8">
					<div>
						<h1 className="text-2xl font-semibold">Nodes</h1>
						<p className="text-muted-foreground">Manage your blockchain nodes</p>
					</div>
					<div className="flex items-center gap-2">
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
						<div className="flex items-center gap-2">
							<Button asChild variant="outline">
								<Link to="/nodes/fabric/bulk">
									<Server className="mr-2 h-4 w-4" />
									Bulk Create Fabric
								</Link>
							</Button>
							<Button asChild>
								<Link to="/nodes/create">Create Node</Link>
							</Button>
						</div>
					</div>
				</div>

				<div className="grid gap-4">
					<div className="flex items-center px-4 py-2 border rounded-lg bg-background">
						<Checkbox
							checked={nodes?.items?.length > 0 && selectedNodes.length === nodes.items.filter((node) => !['starting', 'stopping'].includes(node.status?.toLowerCase() || '')).length}
							onCheckedChange={handleSelectAll}
							className="mr-4"
						/>
						<span className="text-sm text-muted-foreground">Select All</span>
					</div>
					{nodes.items.map((node) => (
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
					<div className="mt-4 flex justify-center">
						<Pagination currentPage={page} pageSize={pageSize} totalItems={nodes?.total || 0} onPageChange={setPage} />
					</div>
				)}
			</div>

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
		</div>
	)
}

import { ServiceNetworkNode } from '@/api/client'
import {
	postNetworksFabricByIdOrderersByOrdererIdJoinMutation,
	postNetworksFabricByIdOrderersByOrdererIdUnjoinMutation,
	postNetworksFabricByIdPeersByPeerIdJoinMutation,
	postNetworksFabricByIdPeersByPeerIdUnjoinMutation,
} from '@/api/client/@tanstack/react-query.gen'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { useMutation } from '@tanstack/react-query'
import { Activity, EllipsisVertical, Network } from 'lucide-react'
import { Link } from 'react-router-dom'
import { toast } from 'sonner'
import { FabricIcon } from '../icons/fabric-icon'
import { Badge } from '../ui/badge'

interface NodeCardProps {
	networkNode: ServiceNetworkNode
	networkId: number
	onJoined: () => void
	onUnjoined: () => void
}

function getStatusColor(status: string) {
	switch (status?.toLowerCase()) {
		case 'running':
			return 'default'
		case 'stopped':
		case 'error':
			return 'destructive'
		case 'starting':
		case 'stopping':
			return 'outline'
		default:
			return 'secondary'
	}
}

export function NodeCard({ networkNode, networkId, onJoined, onUnjoined }: NodeCardProps) {
	const { node } = networkNode
	// const {
	// 	data: node,
	// 	isLoading,
	// 	refetch,
	// } = useQuery({
	// 	...getNodesByIdOptions({
	// 		path: { id: node.id! },
	// 	}),
	// })

	const joinPeerNode = useMutation({
		...postNetworksFabricByIdPeersByPeerIdJoinMutation(),
		onSuccess: () => {
			toast.success('Peer node joined successfully')
			onJoined()
		},
		onError: (error: any) => {
			toast.error('Failed to join peer node', {
				description: error.message,
			})
		},
	})

	const joinOrdererNode = useMutation({
		...postNetworksFabricByIdOrderersByOrdererIdJoinMutation(),
		onSuccess: () => {
			toast.success('Orderer node joined successfully')
			onJoined()
		},
		onError: (error: any) => {
			toast.error('Failed to join orderer node', {
				description: error.message,
			})
		},
	})

	const unjoinPeerNode = useMutation({
		...postNetworksFabricByIdPeersByPeerIdUnjoinMutation(),
		onSuccess: () => {
			toast.success('Peer node unjoined successfully')
			onUnjoined()
		},
		onError: (error: any) => {
			toast.error('Failed to unjoin peer node', {
				description: error.message,
			})
		},
	})

	const unjoinOrdererNode = useMutation({
		...postNetworksFabricByIdOrderersByOrdererIdUnjoinMutation(),
		onSuccess: () => {
			toast.success('Orderer node unjoined successfully')
			onUnjoined()
		},
		onError: (error: any) => {
			toast.error('Failed to unjoin orderer node', {
				description: error.message,
			})
		},
	})

	// if (isLoading) {
	// 	return (
	// 		<Card className="p-3">
	// 			<div className="flex items-center gap-3">
	// 				<Skeleton className="h-8 w-8 rounded-full" />
	// 				<div className="space-y-2">
	// 					<Skeleton className="h-4 w-32" />
	// 					<Skeleton className="h-3 w-24" />
	// 				</div>
	// 			</div>
	// 		</Card>
	// 	)
	// }

	if (!node) return null

	return (
		<Card className="p-3">
			<div className="flex items-center justify-between">
				<div className="flex items-center gap-3">
					<div className="h-8 w-8 rounded-full bg-primary/10 flex items-center justify-center">
						<FabricIcon className="h-4 w-4 text-primary" />
					</div>
					<div>
						<div className="flex items-center gap-2">
							<Link to={`/nodes/${node.id}`} className="font-medium hover:underline">
								{node.name}
							</Link>
							<Badge variant={getStatusColor(node.status || '')}>
								<Activity className="mr-1 h-3 w-3" />
								{node.status}
							</Badge>
						</div>
						<div className="flex items-center gap-2 text-sm text-muted-foreground">
							<span className="flex items-center gap-1">
								<Network className="h-3 w-3" />
								{node.nodeType}
							</span>
						</div>
					</div>
				</div>

				<DropdownMenu>
					<DropdownMenuTrigger asChild>
						<Button variant="ghost" size="icon">
							<EllipsisVertical className="h-4 w-4" />
						</Button>
					</DropdownMenuTrigger>
					<DropdownMenuContent align="end">
						{networkNode.status === 'joined' ? (
							<DropdownMenuItem
								className="text-destructive"
								onClick={() => {
									if (node.nodeType === 'FABRIC_PEER') {
										unjoinPeerNode.mutate({
											path: { id: networkId, peerId: node.id! },
										})
									} else if (node.nodeType === 'FABRIC_ORDERER') {
										unjoinOrdererNode.mutate({
											path: { id: networkId, ordererId: node.id! },
										})
									}
								}}
							>
								Unjoin Node
							</DropdownMenuItem>
						) : (
							<DropdownMenuItem
								onClick={() => {
									if (node.nodeType === 'FABRIC_PEER') {
										joinPeerNode.mutate({
											path: { id: networkId, peerId: node.id! },
										})
									} else if (node.nodeType === 'FABRIC_ORDERER') {
										joinOrdererNode.mutate({
											path: { id: networkId, ordererId: node.id! },
										})
									}
								}}
							>
								Join Node
							</DropdownMenuItem>
						)}
					</DropdownMenuContent>
				</DropdownMenu>
			</div>
		</Card>
	)
}

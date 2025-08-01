import { deleteNetworksFabricByIdMutation, deleteNetworksBesuByIdMutation, getNetworksFabricOptions, getNetworksBesuOptions } from '@/api/client/@tanstack/react-query.gen'
import { HttpNetworkResponse } from '@/api/client'
import { BesuIcon } from '@/components/icons/besu-icon'
import { FabricIcon } from '@/components/icons/fabric-icon'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Skeleton } from '@/components/ui/skeleton'
import { TimeAgo } from '@/components/ui/time-ago'
import { useMutation, useQuery } from '@tanstack/react-query'
import { MoreVertical, Network, Trash, ChevronDown, Upload } from 'lucide-react'
import { useState, useMemo, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { toast } from 'sonner'

export default function NetworksPage() {
	const [networkToDelete, setNetworkToDelete] = useState<HttpNetworkResponse | null>(null)

	const {
		data: fabricNetworks,
		isLoading: isLoadingFabric,
		refetch: refetchFabric,
	} = useQuery({
		...getNetworksFabricOptions(),
	})

	const {
		data: besuNetworks,
		isLoading: isLoadingBesu,
		refetch: refetchBesu,
	} = useQuery({
		...getNetworksBesuOptions(),
	})

	// Combine networks from both sources
	const networks = useMemo(
		() => ({
			networks: [...(fabricNetworks?.networks || []), ...(besuNetworks?.networks || [])].sort((a, b) => new Date(b.createdAt!).getTime() - new Date(a.createdAt!).getTime()),
		}),
		[fabricNetworks?.networks, besuNetworks?.networks]
	)

	const isLoading = useMemo(() => isLoadingFabric || isLoadingBesu, [isLoadingFabric, isLoadingBesu])

	const refetch = useCallback(() => {
		refetchFabric()
		refetchBesu()
	}, [refetchFabric, refetchBesu])

	const deleteFabricNetwork = useMutation({
		...deleteNetworksFabricByIdMutation(),
		onSuccess: () => {
			toast.success('Network deleted successfully')
			refetch()
			setNetworkToDelete(null)
		},
		onError: (error: any) => {
			toast.error('Failed to delete network', {
				description: error.message,
			})
		},
	})

	const deleteBesuNetwork = useMutation({
		...deleteNetworksBesuByIdMutation(),
		onSuccess: () => {
			toast.success('Network deleted successfully')
			refetch()
			setNetworkToDelete(null)
		},
		onError: (error: any) => {
			toast.error('Failed to delete network', {
				description: error.message,
			})
		},
	})

	const deleteNetwork = useCallback(
		(network: HttpNetworkResponse) => {
			if (network.platform === 'fabric') {
				deleteFabricNetwork.mutate({ path: { id: network.id! } })
			} else {
				deleteBesuNetwork.mutate({ path: { id: network.id! } })
			}
		},
		[deleteFabricNetwork, deleteBesuNetwork]
	)

	const handleDelete = useCallback((network: HttpNetworkResponse) => {
		setNetworkToDelete(network)
	}, [])

	if (isLoading) {
		return (
			<div className="flex-1 p-8">
				<div className="max-w-4xl mx-auto">
					<div className="space-y-4">
						{[1, 2].map((i) => (
							<Card key={i} className="p-6">
								<div className="flex items-center justify-between">
									<div className="flex items-center gap-4">
										<Skeleton className="h-8 w-8" />
										<div>
											<Skeleton className="h-5 w-32 mb-1" />
											<Skeleton className="h-4 w-48" />
										</div>
									</div>
									<Skeleton className="h-9 w-24" />
								</div>
							</Card>
						))}
					</div>
				</div>
			</div>
		)
	}
	if (!networks?.networks?.length) {
		return (
			<div className="flex-1 p-8">
				<div className="max-w-4xl mx-auto">
					<div className="text-center mb-12">
						<div className="flex justify-center mb-4">
							<Network className="h-12 w-12 text-muted-foreground" />
						</div>
						<h1 className="text-2xl font-semibold mb-2">Configure your network</h1>
						<p className="text-muted-foreground">Create blockchain nodes that you can connect to your network infrastructure.</p>
					</div>

					<div className="space-y-4">
						<Card className="p-6">
							<div className="flex items-center justify-between">
								<div className="flex items-center gap-4">
									<FabricIcon className="h-8 w-8" />
									<div>
										<h3 className="font-semibold">Fabric network</h3>
										<p className="text-sm text-muted-foreground">Enterprise-grade permissioned network</p>
									</div>
								</div>
								<DropdownMenu>
									<DropdownMenuTrigger asChild>
										<Button>
											Create Network
											<ChevronDown className="ml-2 h-4 w-4" />
										</Button>
									</DropdownMenuTrigger>
									<DropdownMenuContent align="end">
										<DropdownMenuItem asChild>
											<Link to="/networks/fabric/create">
												<FabricIcon className="mr-2 h-4 w-4" />
												Create Channel
											</Link>
										</DropdownMenuItem>
										<DropdownMenuItem asChild>
											<Link to="/networks/import">
												<Upload className="mr-2 h-4 w-4" />
												Import Network
											</Link>
										</DropdownMenuItem>
									</DropdownMenuContent>
								</DropdownMenu>
							</div>
						</Card>

						<Card className="p-6">
							<div className="flex items-center justify-between">
								<div className="flex items-center gap-4">
									<BesuIcon className="h-8 w-8" />
									<div>
										<h3 className="font-semibold">Besu network</h3>
										<p className="text-sm text-muted-foreground">Private network compatible with Ethereum</p>
									</div>
								</div>
								<DropdownMenu>
									<DropdownMenuTrigger asChild>
										<Button>
											Create Network
											<ChevronDown className="ml-2 h-4 w-4" />
										</Button>
									</DropdownMenuTrigger>
									<DropdownMenuContent align="end">
										<DropdownMenuItem asChild>
											<Link to="/networks/besu/create">
												<BesuIcon className="mr-2 h-4 w-4" />
												Create Network
											</Link>
										</DropdownMenuItem>
										<DropdownMenuItem asChild>
											<Link to="/networks/besu/bulk-create">
												<Upload className="mr-2 h-4 w-4" />
												Bulk Create Network
											</Link>
										</DropdownMenuItem>
										<DropdownMenuItem asChild>
											<Link to="/networks/import">
												<Upload className="mr-2 h-4 w-4" />
												Import Network
											</Link>
										</DropdownMenuItem>
									</DropdownMenuContent>
								</DropdownMenu>
							</div>
						</Card>
					</div>
				</div>
			</div>
		)
	}

	return (
		<div className="flex-1 p-8">
			<div className="max-w-4xl mx-auto">
				<div className="flex items-center justify-between mb-8">
					<div>
						<h1 className="text-2xl font-semibold">Networks</h1>
						<p className="text-muted-foreground">Manage your blockchain networks</p>
					</div>
					<div className="flex gap-2">
						<DropdownMenu>
							<DropdownMenuTrigger asChild>
								<Button>
									Create Network
									<ChevronDown className="ml-2 h-4 w-4" />
								</Button>
							</DropdownMenuTrigger>
							<DropdownMenuContent align="end">
								<DropdownMenuItem asChild>
									<Link to="/networks/fabric/create">
										<FabricIcon className="mr-2 h-4 w-4" />
										Fabric Channel
									</Link>
								</DropdownMenuItem>
								<DropdownMenuItem asChild>
									<Link to="/networks/besu/create">
										<BesuIcon className="mr-2 h-4 w-4" />
										Besu Network
									</Link>
								</DropdownMenuItem>
								<DropdownMenuItem asChild>
									<Link to="/networks/besu/bulk-create">
										<Upload className="mr-2 h-4 w-4" />
										Besu Network (Bulk)
									</Link>
								</DropdownMenuItem>
								<DropdownMenuItem asChild>
									<Link to="/networks/import">
										<Upload className="mr-2 h-4 w-4" />
										Import Network
									</Link>
								</DropdownMenuItem>
							</DropdownMenuContent>
						</DropdownMenu>
					</div>
				</div>

				<div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
					{networks?.networks?.map((network) => (
						<Card key={network.id} className="p-6">
							<div className="flex items-center justify-between">
								<Link to={`/networks/${network.id}/${network.platform === 'fabric' ? 'fabric' : 'besu'}`} className="flex items-center gap-4 flex-1">
									{network.platform === 'fabric' ? <FabricIcon className="h-8 w-8" /> : <BesuIcon className="h-8 w-8" />}
									<div>
										<h3 className="font-semibold">{network.name}</h3>
										<p className="text-sm text-muted-foreground">
											<TimeAgo date={network.createdAt!} />
										</p>
									</div>
								</Link>
								<DropdownMenu>
									<DropdownMenuTrigger asChild>
										<Button variant="ghost" size="icon">
											<MoreVertical className="h-4 w-4" />
										</Button>
									</DropdownMenuTrigger>
									<DropdownMenuContent align="end">
										<DropdownMenuItem
											className="text-destructive"
											onClick={(e) => {
												e.preventDefault()
												handleDelete(network)
											}}
										>
											<Trash className="h-4 w-4 mr-2" />
											Delete
										</DropdownMenuItem>
									</DropdownMenuContent>
								</DropdownMenu>
							</div>
						</Card>
					))}
				</div>
			</div>

			<AlertDialog open={!!networkToDelete} onOpenChange={(open) => !open && setNetworkToDelete(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Delete Network</AlertDialogTitle>
						<AlertDialogDescription>Are you sure you want to delete this network? This action cannot be undone and will remove all associated data.</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction
							onClick={() => networkToDelete && deleteNetwork(networkToDelete)}
							disabled={deleteFabricNetwork.isPending || deleteBesuNetwork.isPending}
							className="bg-destructive hover:bg-destructive/90"
						>
							{deleteFabricNetwork.isPending || deleteBesuNetwork.isPending ? 'Deleting...' : 'Delete'}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	)
}

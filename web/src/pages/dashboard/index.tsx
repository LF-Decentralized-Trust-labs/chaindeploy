import { getNodesOptions, getNetworksFabricOptions, getNetworksBesuOptions } from '@/api/client/@tanstack/react-query.gen'
import { BesuIcon } from '@/components/icons/besu-icon'
import { FabricIcon } from '@/components/icons/fabric-icon'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { usePageTitle } from '@/hooks/use-page-title'
import { useQuery } from '@tanstack/react-query'
import { Activity, ArrowRight, Network, Package, Plus, Server } from 'lucide-react'
import { useMemo } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import NetworksList from '@/components/dashboard/NetworksList'
import NodesList from '@/components/dashboard/NodesList'

export default function DashboardPage() {
	usePageTitle('Dashboard')
	const navigate = useNavigate()

	const { data: nodes } = useQuery({
		...getNodesOptions({
			query: {
				limit: 100,
				page: 1,
			},
		}),
	})

	const { data: fabricNetworks } = useQuery({
		...getNetworksFabricOptions(),
	})

	const { data: besuNetworks } = useQuery({
		...getNetworksBesuOptions(),
	})

	const stats = useMemo(() => {
		const totalNodes = nodes?.items?.length ?? 0
		const runningNodes = nodes?.items?.filter(n => n.status === 'RUNNING').length ?? 0
		const stoppedNodes = nodes?.items?.filter(n => n.status === 'STOPPED').length ?? 0
		const fabricNodes = nodes?.items?.filter(n => n.platform === 'FABRIC').length ?? 0
		const besuNodes = nodes?.items?.filter(n => n.platform === 'BESU').length ?? 0
		const totalNetworks = (fabricNetworks?.networks?.length ?? 0) + (besuNetworks?.networks?.length ?? 0)

		return {
			totalNodes,
			runningNodes,
			stoppedNodes,
			fabricNodes,
			besuNodes,
			totalNetworks,
			fabricNetworks: fabricNetworks?.networks?.length ?? 0,
			besuNetworks: besuNetworks?.networks?.length ?? 0,
			hasResources: totalNodes > 0 || totalNetworks > 0,
		}
	}, [nodes, fabricNetworks, besuNetworks])

	if (!stats.hasResources) {
		return (
			<div className="flex-1 p-4 md:p-8">
				<div className="max-w-6xl mx-auto">
					<div className="text-center mb-8 md:mb-12">
						<h1 className="text-2xl md:text-4xl font-bold mb-4">Welcome to ChainLaunch</h1>
						<p className="text-base md:text-lg text-muted-foreground mb-8">
							Start your blockchain journey by choosing your preferred platform
						</p>
					</div>

					<div className="space-y-6 md:space-y-8">
						<div>
							<h2 className="text-lg md:text-xl font-semibold mb-4 text-center">Create New Network</h2>
							<div className="grid gap-6 md:grid-cols-2 max-w-4xl mx-auto">
								<Card className="relative overflow-hidden hover:shadow-lg transition-shadow cursor-pointer group"
									onClick={() => navigate('/networks/fabric/create')}
								>
									<div className="absolute inset-0 bg-gradient-to-br from-blue-500/10 to-transparent opacity-0 group-hover:opacity-100 transition-opacity" />
									<CardHeader className="pb-4">
										<div className="flex items-center justify-between mb-4">
											<FabricIcon className="h-12 w-12" />
											<ArrowRight className="h-5 w-5 text-muted-foreground group-hover:translate-x-1 transition-transform" />
										</div>
										<CardTitle className="text-xl">Hyperledger Fabric</CardTitle>
										<CardDescription>
											Enterprise-grade permissioned blockchain platform
										</CardDescription>
									</CardHeader>
									<CardContent>
										<ul className="space-y-2 text-sm text-muted-foreground mb-4">
											<li className="flex items-center gap-2">
												<div className="h-1.5 w-1.5 rounded-full bg-primary" />
												Private & permissioned networks
											</li>
											<li className="flex items-center gap-2">
												<div className="h-1.5 w-1.5 rounded-full bg-primary" />
												Channel-based privacy
											</li>
											<li className="flex items-center gap-2">
												<div className="h-1.5 w-1.5 rounded-full bg-primary" />
												Pluggable consensus
											</li>
											<li className="flex items-center gap-2">
												<div className="h-1.5 w-1.5 rounded-full bg-primary" />
												Smart contracts in multiple languages
											</li>
										</ul>
										<Button className="w-full" size="lg">
											<Plus className="mr-2 h-4 w-4" />
											Create Fabric Network
										</Button>
									</CardContent>
								</Card>

								<Card className="relative overflow-hidden hover:shadow-lg transition-shadow cursor-pointer group"
									onClick={() => navigate('/networks/besu/create')}
								>
									<div className="absolute inset-0 bg-gradient-to-br from-purple-500/10 to-transparent opacity-0 group-hover:opacity-100 transition-opacity" />
									<CardHeader className="pb-4">
										<div className="flex items-center justify-between mb-4">
											<BesuIcon className="h-12 w-12" />
											<ArrowRight className="h-5 w-5 text-muted-foreground group-hover:translate-x-1 transition-transform" />
										</div>
										<CardTitle className="text-xl">Hyperledger Besu</CardTitle>
										<CardDescription>
											Ethereum-compatible enterprise blockchain
										</CardDescription>
									</CardHeader>
									<CardContent>
										<ul className="space-y-2 text-sm text-muted-foreground mb-4">
											<li className="flex items-center gap-2">
												<div className="h-1.5 w-1.5 rounded-full bg-primary" />
												EVM compatible
											</li>
											<li className="flex items-center gap-2">
												<div className="h-1.5 w-1.5 rounded-full bg-primary" />
												Public & private networks
											</li>
											<li className="flex items-center gap-2">
												<div className="h-1.5 w-1.5 rounded-full bg-primary" />
												Proof of Authority consensus
											</li>
											<li className="flex items-center gap-2">
												<div className="h-1.5 w-1.5 rounded-full bg-primary" />
												Solidity smart contracts
											</li>
										</ul>
										<Button className="w-full" size="lg">
											<Plus className="mr-2 h-4 w-4" />
											Create Besu Network
										</Button>
									</CardContent>
								</Card>
							</div>
						</div>
					</div>

					<div className="mt-8 md:mt-12 text-center">
						<p className="text-sm text-muted-foreground mb-4">Looking for other options?</p>
						<div className="flex flex-col sm:flex-row flex-wrap gap-3 sm:gap-4 justify-center">
							<Button variant="outline" asChild>
								<Link to="/nodes/create">
									Create Single Node
								</Link>
							</Button>
							<Button variant="outline" asChild>
								<Link to="/networks/import">
									Import Existing Network
								</Link>
							</Button>
						</div>
					</div>
				</div>
			</div>
		)
	}

	return (
		<div className="flex-1 p-4 md:p-8">
			<div className="max-w-7xl mx-auto">
				<div className="mb-6 md:mb-8">
					<h1 className="text-2xl md:text-3xl font-bold mb-2">Dashboard</h1>
					<p className="text-sm md:text-base text-muted-foreground">Overview of your blockchain infrastructure</p>
				</div>

				<div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3 mb-8">
					<Card>
						<CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
							<CardTitle className="text-sm font-medium">Total Nodes</CardTitle>
							<Server className="h-4 w-4 text-muted-foreground" />
						</CardHeader>
						<CardContent>
							<div className="text-2xl font-bold">{stats.totalNodes}</div>
							<div className="flex gap-4 mt-2">
								<p className="text-xs text-muted-foreground">
									<span className="text-green-600 dark:text-green-400">{stats.runningNodes}</span> running
								</p>
								<p className="text-xs text-muted-foreground">
									<span className="text-orange-600 dark:text-orange-400">{stats.stoppedNodes}</span> stopped
								</p>
							</div>
						</CardContent>
					</Card>

					<Card>
						<CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
							<CardTitle className="text-sm font-medium">Networks</CardTitle>
							<Network className="h-4 w-4 text-muted-foreground" />
						</CardHeader>
						<CardContent>
							<div className="text-2xl font-bold">{stats.totalNetworks}</div>
							<div className="flex gap-4 mt-2">
								<p className="text-xs text-muted-foreground">
									<span className="text-blue-600 dark:text-blue-400">{stats.fabricNetworks}</span> Fabric
								</p>
								<p className="text-xs text-muted-foreground">
									<span className="text-purple-600 dark:text-purple-400">{stats.besuNetworks}</span> Besu
								</p>
							</div>
						</CardContent>
					</Card>

					<Card>
						<CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
							<CardTitle className="text-sm font-medium">Platform Distribution</CardTitle>
							<Package className="h-4 w-4 text-muted-foreground" />
						</CardHeader>
						<CardContent>
							<div className="flex items-center gap-4">
								<div className="flex items-center gap-2">
									<FabricIcon className="h-5 w-5" />
									<span className="text-lg font-bold">{stats.fabricNodes}</span>
								</div>
								<div className="flex items-center gap-2">
									<BesuIcon className="h-5 w-5" />
									<span className="text-lg font-bold">{stats.besuNodes}</span>
								</div>
							</div>
							<p className="text-xs text-muted-foreground mt-2">
								Nodes by platform
							</p>
						</CardContent>
					</Card>
				</div>

				<Tabs defaultValue="all" className="space-y-4">
					<div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
						<TabsList className="w-full sm:w-auto">
							<TabsTrigger value="all" className="flex items-center gap-1 sm:gap-2 text-xs sm:text-sm flex-1 sm:flex-initial">
								<Activity className="h-3 w-3 sm:h-4 sm:w-4" />
								<span className="hidden sm:inline">All Resources</span>
								<span className="sm:hidden">All</span>
							</TabsTrigger>
							<TabsTrigger value="fabric" className="flex items-center gap-1 sm:gap-2 text-xs sm:text-sm flex-1 sm:flex-initial">
								<FabricIcon className="h-3 w-3 sm:h-4 sm:w-4" />
								Fabric
							</TabsTrigger>
							<TabsTrigger value="besu" className="flex items-center gap-1 sm:gap-2 text-xs sm:text-sm flex-1 sm:flex-initial">
								<BesuIcon className="h-3 w-3 sm:h-4 sm:w-4" />
								Besu
							</TabsTrigger>
						</TabsList>
						<div className="flex gap-2 w-full sm:w-auto">
							<Button variant="outline" asChild className="flex-1 sm:flex-initial text-xs sm:text-sm">
								<Link to="/nodes">
									<span className="hidden sm:inline">View All Nodes</span>
									<span className="sm:hidden">Nodes</span>
								</Link>
							</Button>
							<Button asChild className="flex-1 sm:flex-initial text-xs sm:text-sm">
								<Link to="/networks">
									<span className="hidden sm:inline">View All Networks</span>
									<span className="sm:hidden">Networks</span>
								</Link>
							</Button>
						</div>
					</div>

					<TabsContent value="all" className="space-y-4 md:space-y-6">
						<div>
							<h2 className="text-base md:text-lg font-semibold mb-3 md:mb-4">Networks</h2>
							<NetworksList
								fabricNetworks={fabricNetworks?.networks}
								besuNetworks={besuNetworks?.networks}
								limit={6}
							/>
						</div>
						<div>
							<h2 className="text-base md:text-lg font-semibold mb-3 md:mb-4">Nodes</h2>
							<NodesList
								nodes={nodes?.items}
								limit={6}
							/>
						</div>
					</TabsContent>

					<TabsContent value="fabric" className="space-y-4 md:space-y-6">
						<div>
							<h2 className="text-base md:text-lg font-semibold mb-3 md:mb-4">Fabric Networks</h2>
							<NetworksList
								fabricNetworks={fabricNetworks?.networks}
								limit={6}
							/>
						</div>
						<div>
							<h2 className="text-base md:text-lg font-semibold mb-3 md:mb-4">Fabric Nodes</h2>
							<NodesList
								nodes={nodes?.items?.filter(n => n.platform === 'FABRIC')}
								limit={6}
							/>
						</div>
					</TabsContent>

					<TabsContent value="besu" className="space-y-4 md:space-y-6">
						<div>
							<h2 className="text-base md:text-lg font-semibold mb-3 md:mb-4">Besu Networks</h2>
							<NetworksList
								besuNetworks={besuNetworks?.networks}
								limit={6}
							/>
						</div>
						<div>
							<h2 className="text-base md:text-lg font-semibold mb-3 md:mb-4">Besu Nodes</h2>
							<NodesList
								nodes={nodes?.items?.filter(n => n.platform === 'BESU')}
								limit={6}
							/>
						</div>
					</TabsContent>
				</Tabs>
			</div>
		</div>
	)
}

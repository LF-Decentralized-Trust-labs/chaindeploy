import { getNodesOptions, getNetworksFabricOptions, getNetworksBesuOptions, getNetworksFabricxOptions } from '@/api/client/@tanstack/react-query.gen'
import { BesuIcon } from '@/components/icons/besu-icon'
import { FabricIcon } from '@/components/icons/fabric-icon'
import { PageHeader, PageShell } from '@/components/layout/page-shell'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { usePageTitle } from '@/hooks/use-page-title'
import { useQuery } from '@tanstack/react-query'
import { Activity, ArrowRight, Plus, Rocket } from 'lucide-react'
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

	const { data: fabricxNetworks } = useQuery({
		...getNetworksFabricxOptions(),
	})

	const stats = useMemo(() => {
		const totalNodes = nodes?.items?.length ?? 0
		const runningNodes = nodes?.items?.filter(n => n.status === 'RUNNING').length ?? 0
		const stoppedNodes = nodes?.items?.filter(n => n.status === 'STOPPED').length ?? 0
		const fabricNodes = nodes?.items?.filter(n => n.platform === 'FABRIC').length ?? 0
		const besuNodes = nodes?.items?.filter(n => n.platform === 'BESU').length ?? 0
		const fabricxNodes = nodes?.items?.filter(n => n.platform === 'FABRICX').length ?? 0
		const totalNetworks = (fabricNetworks?.networks?.length ?? 0) + (besuNetworks?.networks?.length ?? 0) + (fabricxNetworks?.networks?.length ?? 0)

		return {
			totalNodes,
			runningNodes,
			stoppedNodes,
			fabricNodes,
			besuNodes,
			fabricxNodes,
			totalNetworks,
			fabricNetworks: fabricNetworks?.networks?.length ?? 0,
			besuNetworks: besuNetworks?.networks?.length ?? 0,
			fabricxNetworks: fabricxNetworks?.networks?.length ?? 0,
			hasResources: totalNodes > 0 || totalNetworks > 0,
		}
	}, [nodes, fabricNetworks, besuNetworks, fabricxNetworks])

	if (!stats.hasResources) {
		return (
			<PageShell maxWidth="dashboard">
				<div className="mb-10 text-center">
					<h1 className="text-2xl font-semibold tracking-tight">Welcome to ChainLaunch</h1>
					<p className="mt-2 text-sm text-muted-foreground">
						Start your blockchain journey by choosing your preferred platform
					</p>
				</div>

				<div className="space-y-8">
						<div className="max-w-lg mx-auto">
							<Card className="border-2 border-primary/20 bg-gradient-to-br from-primary/5 to-transparent">
								<CardContent className="pt-6 text-center space-y-4">
									<div className="flex items-center justify-center gap-2">
										<Rocket className="h-5 w-5 text-primary" />
										<h2 className="text-lg font-semibold">New here? Try Quick Start</h2>
									</div>
									<p className="text-sm text-muted-foreground">
										Create a fully working blockchain network in under a minute. We'll set up organizations, keys, nodes, and network configuration automatically.
									</p>
									<Button asChild size="lg" className="w-full sm:w-auto">
										<Link to="/quick-start">
											<Rocket className="mr-2 h-4 w-4" />
											Quick Start
										</Link>
									</Button>
								</CardContent>
							</Card>
						</div>

						<div className="relative flex items-center max-w-lg mx-auto">
							<div className="flex-grow border-t border-muted-foreground/20" />
							<span className="mx-4 text-xs text-muted-foreground">or create manually</span>
							<div className="flex-grow border-t border-muted-foreground/20" />
						</div>

						<div>
							<h2 className="mb-4 text-center text-lg font-semibold">Create New Network</h2>
							<div className="mx-auto grid max-w-4xl gap-6 sm:grid-cols-2">
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

				<div className="mt-10 text-center">
					<p className="mb-4 text-sm text-muted-foreground">Looking for other options?</p>
					<div className="flex flex-col flex-wrap justify-center gap-3 sm:flex-row sm:gap-4">
						<Button variant="outline" asChild>
							<Link to="/nodes/create">Create Single Node</Link>
						</Button>
						<Button variant="outline" asChild>
							<Link to="/networks/import">Import Existing Network</Link>
						</Button>
					</div>
				</div>
			</PageShell>
		)
	}

	return (
		<PageShell maxWidth="wide">
			<PageHeader title="Dashboard" description="Overview of your blockchain infrastructure" />

			<div className="space-y-8">
				<div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
					<Card>
						<CardHeader className="pb-2">
							<CardTitle className="truncate text-sm font-medium text-muted-foreground">Total Nodes</CardTitle>
						</CardHeader>
						<CardContent>
							<div className="text-3xl font-semibold tabular-nums">{stats.totalNodes}</div>
							<div className="mt-2 flex gap-4">
								<p className="text-xs text-muted-foreground tabular-nums">
									<span className="text-green-600 dark:text-green-400">{stats.runningNodes}</span> running
								</p>
								<p className="text-xs text-muted-foreground tabular-nums">
									<span className="text-orange-600 dark:text-orange-400">{stats.stoppedNodes}</span> stopped
								</p>
							</div>
						</CardContent>
					</Card>

					<Card>
						<CardHeader className="pb-2">
							<CardTitle className="truncate text-sm font-medium text-muted-foreground">Networks</CardTitle>
						</CardHeader>
						<CardContent>
							<div className="text-3xl font-semibold tabular-nums">{stats.totalNetworks}</div>
							<div className="mt-2 flex flex-wrap gap-x-4 gap-y-1">
								<p className="text-xs text-muted-foreground tabular-nums">
									<span className="text-blue-600 dark:text-blue-400">{stats.fabricNetworks}</span> Fabric
								</p>
								<p className="text-xs text-muted-foreground tabular-nums">
									<span className="text-purple-600 dark:text-purple-400">{stats.besuNetworks}</span> Besu
								</p>
								<p className="text-xs text-muted-foreground tabular-nums">
									<span className="text-cyan-600 dark:text-cyan-400">{stats.fabricxNetworks}</span> FabricX
								</p>
							</div>
						</CardContent>
					</Card>

					<Card>
						<CardHeader className="pb-2">
							<CardTitle className="truncate text-sm font-medium text-muted-foreground">Platform Distribution</CardTitle>
						</CardHeader>
						<CardContent>
							<div className="flex flex-wrap items-center gap-x-6 gap-y-2">
								<div className="flex items-center gap-2">
									<FabricIcon className="h-5 w-5" />
									<span className="text-2xl font-semibold tabular-nums">{stats.fabricNodes}</span>
								</div>
								<div className="flex items-center gap-2">
									<BesuIcon className="h-5 w-5" />
									<span className="text-2xl font-semibold tabular-nums">{stats.besuNodes}</span>
								</div>
								<div className="flex items-center gap-2">
									<FabricIcon className="h-5 w-5 text-cyan-600 dark:text-cyan-400" />
									<span className="text-2xl font-semibold tabular-nums">{stats.fabricxNodes}</span>
								</div>
							</div>
							<p className="mt-2 text-xs text-muted-foreground">Nodes by platform</p>
						</CardContent>
					</Card>
				</div>

				<Tabs defaultValue="all" className="space-y-6">
					<div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
						<TabsList className="w-full sm:w-auto">
							<TabsTrigger value="all" className="flex flex-1 items-center gap-2 sm:flex-initial">
								<Activity className="h-4 w-4" />
								<span className="hidden sm:inline">All Resources</span>
								<span className="sm:hidden">All</span>
							</TabsTrigger>
							<TabsTrigger value="fabric" className="flex flex-1 items-center gap-2 sm:flex-initial">
								<FabricIcon className="h-4 w-4" />
								Fabric
							</TabsTrigger>
							<TabsTrigger value="besu" className="flex flex-1 items-center gap-2 sm:flex-initial">
								<BesuIcon className="h-4 w-4" />
								Besu
							</TabsTrigger>
							<TabsTrigger value="fabricx" className="flex flex-1 items-center gap-2 sm:flex-initial">
								<FabricIcon className="h-4 w-4 text-cyan-600 dark:text-cyan-400" />
								FabricX
							</TabsTrigger>
						</TabsList>
						<div className="flex w-full gap-2 sm:w-auto">
							<Button variant="outline" asChild className="flex-1 sm:flex-initial">
								<Link to="/nodes">
									<span className="hidden sm:inline">View All Nodes</span>
									<span className="sm:hidden">Nodes</span>
								</Link>
							</Button>
							<Button asChild className="flex-1 sm:flex-initial">
								<Link to="/networks">
									<span className="hidden sm:inline">View All Networks</span>
									<span className="sm:hidden">Networks</span>
								</Link>
							</Button>
						</div>
					</div>

					<TabsContent value="all" className="space-y-6">
						<section className="space-y-4">
							<h2 className="text-lg font-semibold">Networks</h2>
							<NetworksList fabricNetworks={fabricNetworks?.networks} besuNetworks={besuNetworks?.networks} fabricxNetworks={fabricxNetworks?.networks} limit={6} />
						</section>
						<section className="space-y-4">
							<h2 className="text-lg font-semibold">Nodes</h2>
							<NodesList nodes={nodes?.items} limit={6} />
						</section>
					</TabsContent>

					<TabsContent value="fabric" className="space-y-6">
						<section className="space-y-4">
							<h2 className="text-lg font-semibold">Fabric Networks</h2>
							<NetworksList fabricNetworks={fabricNetworks?.networks} limit={6} />
						</section>
						<section className="space-y-4">
							<h2 className="text-lg font-semibold">Fabric Nodes</h2>
							<NodesList nodes={nodes?.items?.filter(n => n.platform === 'FABRIC')} limit={6} />
						</section>
					</TabsContent>

					<TabsContent value="besu" className="space-y-6">
						<section className="space-y-4">
							<h2 className="text-lg font-semibold">Besu Networks</h2>
							<NetworksList besuNetworks={besuNetworks?.networks} limit={6} />
						</section>
						<section className="space-y-4">
							<h2 className="text-lg font-semibold">Besu Nodes</h2>
							<NodesList nodes={nodes?.items?.filter(n => n.platform === 'BESU')} limit={6} />
						</section>
					</TabsContent>

					<TabsContent value="fabricx" className="space-y-6">
						<section className="space-y-4">
							<h2 className="text-lg font-semibold">FabricX Networks</h2>
							<NetworksList fabricxNetworks={fabricxNetworks?.networks} limit={6} />
						</section>
						<section className="space-y-4">
							<h2 className="text-lg font-semibold">FabricX Nodes</h2>
							<NodesList nodes={nodes?.items?.filter(n => n.platform === 'FABRICX')} limit={6} />
						</section>
					</TabsContent>
				</Tabs>
			</div>
		</PageShell>
	)
}

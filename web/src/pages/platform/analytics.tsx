import { getMetricsStatusOptions, getNodesOptions, postMetricsDeployMutation, getMetricsDefaultsOptions, postMetricsUndeployMutation, postMetricsReloadMutation } from '@/api/client/@tanstack/react-query.gen'
import { HttpNodeResponse, TypesDeployPrometheusRequest } from '@/api/client/types.gen'
import { BesuIcon } from '@/components/icons/besu-icon'
import { FabricIcon } from '@/components/icons/fabric-icon'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { BarChart3, Loader2, Plus, Info, MoreVertical, Play, Square } from 'lucide-react'
import { Suspense, useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import * as z from 'zod'
import BesuMetricsPage from '../metrics/besu/[nodeId]'
import OrdererMetricsPage from '../metrics/orderer/[nodeId]'
import PeerMetricsPage from '../metrics/peer/[nodeId]'

const prometheusSetupSchema = z.object({
	prometheus_port: z.number().min(1).max(65535),
	prometheus_version: z.string(),
	scrape_interval: z.number().min(1),
	deployment_mode: z.enum(['docker', 'service']),
	network_mode: z.enum(['bridge', 'host']).optional(),
})

type PrometheusSetupForm = z.infer<typeof prometheusSetupSchema>

export default function AnalyticsPage() {
	const [isSetupDialogOpen, setIsSetupDialogOpen] = useState(false)
	const [isStatusDialogOpen, setIsStatusDialogOpen] = useState(false)
	const [selectedNode, setSelectedNode] = useState<HttpNodeResponse>()
	const [portSelection, setPortSelection] = useState<'available' | 'custom'>('available')
	const [customPort, setCustomPort] = useState<number>(9090)

	const { data: prometheusDefaults, isLoading: isDefaultsLoading } = useQuery({
		...getMetricsDefaultsOptions({}),
	})

	const form = useForm<PrometheusSetupForm>({
		resolver: zodResolver(prometheusSetupSchema),
		defaultValues: {
			prometheus_port: prometheusDefaults?.available_ports?.[0] || 9090,
			prometheus_version: prometheusDefaults?.prometheus_version || 'v3.5.0',
			scrape_interval: prometheusDefaults?.scrape_interval || 15,
			deployment_mode: prometheusDefaults?.deployment_mode || 'docker',
			network_mode: prometheusDefaults?.docker_config?.network_mode || 'bridge',
		},
	})

	// Update form values when defaults are loaded
	useEffect(() => {
		if (prometheusDefaults) {
			const firstAvailablePort = prometheusDefaults.available_ports?.[0] || 9090
			form.reset({
				prometheus_port: firstAvailablePort,
				prometheus_version: prometheusDefaults.prometheus_version || 'v3.5.0',
				scrape_interval: prometheusDefaults.scrape_interval || 15,
				deployment_mode: prometheusDefaults.deployment_mode || 'docker',
				network_mode: prometheusDefaults.docker_config?.network_mode || 'bridge',
			})
			setCustomPort(firstAvailablePort)
		}
	}, [prometheusDefaults, form])

	// Function to initialize form with current status data
	const initializeFormWithCurrentStatus = () => {
		if (prometheusStatus) {
			// Convert TimeDuration to seconds
			const convertTimeDurationToSeconds = (duration: number): number => {
				switch (duration) {
					case 1: return 0.000000001 // 1 nanosecond
					case 1000: return 0.000001 // 1 microsecond
					case 1000000: return 0.001 // 1 millisecond
					case 1000000000: return 1 // 1 second
					case 60000000000: return 60 // 1 minute
					case 3600000000000: return 3600 // 1 hour
					default: return Math.floor(duration / 1000000000) // fallback to nanosecond conversion
				}
			}

			const scrapeIntervalInSeconds = prometheusStatus.scrape_interval 
				? convertTimeDurationToSeconds(prometheusStatus.scrape_interval)
				: 15

			const currentPort = prometheusStatus.port || 9090
			
			// Check if current port is in available ports
			const isCurrentPortAvailable = prometheusDefaults?.available_ports?.includes(currentPort)
			setPortSelection(isCurrentPortAvailable ? 'available' : 'custom')
			setCustomPort(currentPort)

			form.reset({
				prometheus_port: currentPort,
				prometheus_version: prometheusStatus.version || 'v3.5.0',
				scrape_interval: scrapeIntervalInSeconds,
				deployment_mode: prometheusStatus.deployment_mode || 'docker',
				network_mode: 'bridge', // Default since status doesn't include network mode
			})
		}
	}

	const { data: prometheusStatus, isLoading: isStatusLoading } = useQuery({
		...getMetricsStatusOptions({}),
	})

	const { data: nodes } = useQuery({
		...getNodesOptions({
			query: {
				limit: 1000,
				page: 1,
			},
		}),
	})
	useEffect(() => {
		if (nodes?.items && nodes.items.length > 0) {
			setSelectedNode(nodes.items[0])
		}
	}, [nodes])
	
	const deployPrometheus = useMutation({
		...postMetricsDeployMutation(),
		onSuccess: () => {
			toast.success('Prometheus deployed successfully')
			setIsSetupDialogOpen(false)
			form.reset()
			// Refresh the page to show the metrics interface
			window.location.reload()
		},
		onError: (error: any) => {
			toast.error('Failed to deploy Prometheus', {
				description: error.message,
			})
		},
	})

	const stopPrometheus = useMutation({
		...postMetricsUndeployMutation(),
		onSuccess: () => {
			toast.success('Prometheus stopped successfully')
			// Refresh the page to show the setup interface
			window.location.reload()
		},
		onError: (error: any) => {
			toast.error('Failed to stop Prometheus', {
				description: error.message,
			})
		},
	})

	const reloadPrometheus = useMutation({
		...postMetricsReloadMutation(),
		onSuccess: () => {
			toast.success('Prometheus reloaded successfully')
		},
		onError: (error: any) => {
			toast.error('Failed to reload Prometheus', {
				description: error.message,
			})
		},
	})

	const onSubmit = (data: PrometheusSetupForm) => {
		// Use custom port if custom is selected, otherwise use the selected available port
		const finalPort = portSelection === 'custom' ? customPort : data.prometheus_port
		
		const request: TypesDeployPrometheusRequest = {
			prometheus_port: finalPort,
			prometheus_version: data.prometheus_version,
			scrape_interval: data.scrape_interval,
			deployment_mode: data.deployment_mode,
			docker_config:
				data.deployment_mode === 'docker' && data.network_mode
					? {
							network_mode: data.network_mode,
						}
					: undefined,
		}
		deployPrometheus.mutate({
			body: request,
		})
	}

	if (isStatusLoading || isDefaultsLoading) {
		return (
			<div className="flex-1 p-8">
				<div className="max-w-4xl mx-auto">
					<div className="flex items-center justify-center h-[400px]">
						<Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
					</div>
				</div>
			</div>
		)
	}

	// Show setup page only if Prometheus is not running
	if (!prometheusStatus?.status || prometheusStatus.status !== 'running') {
		return (
			<div className="flex-1 p-8">
				<div className="max-w-4xl mx-auto">
					<Card className="p-6 flex flex-col items-center justify-center text-center">
						<BarChart3 className="h-12 w-12 text-muted-foreground mb-4" />
						<h2 className="text-2xl font-semibold mb-2">Analytics Not Set Up</h2>
						<p className="text-muted-foreground mb-6">Set up Prometheus to start collecting and visualizing metrics from your nodes.</p>
						<Dialog open={isSetupDialogOpen} onOpenChange={setIsSetupDialogOpen}>
							<DialogTrigger asChild>
								<Button>
									<Plus className="mr-2 h-4 w-4" />
									Set Up Analytics
								</Button>
							</DialogTrigger>
							<DialogContent>
								<DialogHeader>
									<DialogTitle>Set Up Prometheus</DialogTitle>
									<DialogDescription>Configure Prometheus to collect metrics from your nodes.</DialogDescription>
								</DialogHeader>
								<Form {...form}>
									<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
										<FormField
											control={form.control}
											name="prometheus_port"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Prometheus Port</FormLabel>
													<FormControl>
														<Select 
															onValueChange={(value) => {
																if (value === 'custom') {
																	setPortSelection('custom')
																	field.onChange(customPort)
																} else {
																	setPortSelection('available')
																	field.onChange(parseInt(value))
																}
															}} 
															value={portSelection === 'custom' ? 'custom' : field.value?.toString()}
														>
															<SelectTrigger>
																<SelectValue placeholder="Select a port" />
															</SelectTrigger>
															<SelectContent>
																{prometheusDefaults?.available_ports?.map((port) => (
																	<SelectItem key={port} value={port.toString()}>
																		{port}
																	</SelectItem>
																))}
																<SelectItem value="custom">Custom Port</SelectItem>
															</SelectContent>
														</Select>
													</FormControl>
													{portSelection === 'custom' && (
														<Input 
															type="number" 
															placeholder="Enter custom port (1-65535)"
															min="1"
															max="65535"
															value={customPort}
															onChange={(e) => {
																const port = parseInt(e.target.value)
																if (!isNaN(port) && port >= 1 && port <= 65535) {
																	setCustomPort(port)
																	field.onChange(port)
																}
															}}
															className="mt-2"
														/>
													)}
													<FormMessage />
												</FormItem>
											)}
										/>
										<FormField
											control={form.control}
											name="prometheus_version"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Prometheus Version</FormLabel>
													<FormControl>
														<Input {...field} />
													</FormControl>
													<FormMessage />
												</FormItem>
											)}
										/>
										<FormField
											control={form.control}
											name="scrape_interval"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Scrape Interval (seconds)</FormLabel>
													<FormControl>
														<Input type="number" {...field} onChange={(e) => field.onChange(parseInt(e.target.value))} />
													</FormControl>
													<FormMessage />
												</FormItem>
											)}
										/>
										<FormField
											control={form.control}
											name="deployment_mode"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Deployment Mode</FormLabel>
													<Select onValueChange={field.onChange} defaultValue={field.value}>
														<FormControl>
															<SelectTrigger>
																<SelectValue placeholder="Select deployment mode" />
															</SelectTrigger>
														</FormControl>
														<SelectContent>
															<SelectItem value="docker">Docker</SelectItem>
															<SelectItem value="service">Service</SelectItem>
														</SelectContent>
													</Select>
													<FormMessage />
												</FormItem>
											)}
										/>
										{form.watch('deployment_mode') === 'docker' && (
											<FormField
												control={form.control}
												name="network_mode"
												render={({ field }) => (
													<FormItem>
														<FormLabel>Network Mode</FormLabel>
														<Select onValueChange={field.onChange} defaultValue={field.value}>
															<FormControl>
																<SelectTrigger>
																	<SelectValue placeholder="Select network mode" />
																</SelectTrigger>
															</FormControl>
															<SelectContent>
																<SelectItem value="bridge">Bridge</SelectItem>
																<SelectItem value="host">Host</SelectItem>
															</SelectContent>
														</Select>
														<FormMessage />
													</FormItem>
												)}
											/>
										)}
										<DialogFooter>
											<Button type="submit" disabled={deployPrometheus.isPending}>
												{deployPrometheus.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
												Deploy Prometheus
											</Button>
										</DialogFooter>
									</form>
								</Form>
							</DialogContent>
						</Dialog>
					</Card>
				</div>
			</div>
		)
	}

	if (!nodes?.items || nodes.items.length === 0) {
		return (
			<div className="flex-1 p-8">
				<Card>
					<CardContent className="pt-6">
						<p className="text-center text-muted-foreground">No nodes available</p>
					</CardContent>
				</Card>
			</div>
		)
	}

	return (
		<div className="flex-1 p-8">
			<div className="mb-6">
				<div className="flex items-center justify-between">
					<div>
						<h1 className="text-2xl font-semibold">Node Metrics</h1>
						<p className="text-muted-foreground">Monitor your nodes performance and health</p>
					</div>
					<div className="flex gap-2">
						<Dialog open={isStatusDialogOpen} onOpenChange={setIsStatusDialogOpen}>
							<DialogTrigger asChild>
								<Button variant="outline">
									<Info className="mr-2 h-4 w-4" />
									Status
								</Button>
							</DialogTrigger>
							<DialogContent>
								<DialogHeader>
									<DialogTitle>Prometheus Configuration Status</DialogTitle>
									<DialogDescription>Current Prometheus deployment configuration</DialogDescription>
								</DialogHeader>
								<div className="space-y-4">
									<div className="grid grid-cols-2 gap-4">
										<div>
											<p className="text-sm font-medium text-muted-foreground">Status</p>
											<p className="text-sm">{prometheusStatus?.status || 'Unknown'}</p>
										</div>
										<div>
											<p className="text-sm font-medium text-muted-foreground">Port</p>
											<p className="text-sm">{prometheusStatus?.port || 'N/A'}</p>
										</div>
										<div>
											<p className="text-sm font-medium text-muted-foreground">Version</p>
											<p className="text-sm">{prometheusStatus?.version || 'N/A'}</p>
										</div>
										<div>
											<p className="text-sm font-medium text-muted-foreground">Deployment Mode</p>
											<p className="text-sm capitalize">{prometheusStatus?.deployment_mode || 'N/A'}</p>
										</div>
										<div>
											<p className="text-sm font-medium text-muted-foreground">Scrape Interval</p>
											<p className="text-sm">
												{prometheusStatus?.scrape_interval 
													? `${Math.floor(prometheusStatus.scrape_interval / 1000000000)}s`
													: 'N/A'}
											</p>
										</div>
										<div>
											<p className="text-sm font-medium text-muted-foreground">Started At</p>
											<p className="text-sm">
												{prometheusStatus?.started_at 
													? new Date(prometheusStatus.started_at).toLocaleString()
													: 'N/A'}
											</p>
										</div>
									</div>
									{prometheusStatus?.error && (
										<div className="rounded-md bg-destructive/10 p-3">
											<p className="text-sm text-destructive">{prometheusStatus.error}</p>
										</div>
									)}
								</div>
								<DialogFooter>
									<Button variant="outline" onClick={() => setIsStatusDialogOpen(false)}>
										Close
									</Button>
								</DialogFooter>
							</DialogContent>
						</Dialog>

						<DropdownMenu>
							<DropdownMenuTrigger asChild>
								<Button variant="outline">
									<MoreVertical className="mr-2 h-4 w-4" />
									Actions
								</Button>
							</DropdownMenuTrigger>
							<DropdownMenuContent align="end">
								<DropdownMenuItem 
									onClick={() => reloadPrometheus.mutate({})}
									disabled={reloadPrometheus.isPending}
								>
									<Play className="mr-2 h-4 w-4" />
									{reloadPrometheus.isPending ? 'Reloading...' : 'Reload'}
								</DropdownMenuItem>
								<DropdownMenuItem 
									onClick={() => stopPrometheus.mutate({})}
									disabled={stopPrometheus.isPending}
									className="text-destructive focus:text-destructive"
								>
									<Square className="mr-2 h-4 w-4" />
									{stopPrometheus.isPending ? 'Stopping...' : 'Stop'}
								</DropdownMenuItem>
							</DropdownMenuContent>
						</DropdownMenu>

						<Dialog open={isSetupDialogOpen} onOpenChange={(open) => {
							setIsSetupDialogOpen(open)
							if (open) {
								initializeFormWithCurrentStatus()
							}
						}}>
							<DialogTrigger asChild>
								<Button variant="outline">
									<BarChart3 className="mr-2 h-4 w-4" />
									Refresh Deployment
								</Button>
							</DialogTrigger>
						<DialogContent>
							<DialogHeader>
								<DialogTitle>Refresh Prometheus Deployment</DialogTitle>
								<DialogDescription>Update Prometheus configuration with new parameters.</DialogDescription>
							</DialogHeader>
							<Form {...form}>
								<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
									<FormField
										control={form.control}
										name="prometheus_port"
										render={({ field }) => (
											<FormItem>
												<FormLabel>Prometheus Port</FormLabel>
												<FormControl>
													<Select 
														onValueChange={(value) => {
															if (value === 'custom') {
																setPortSelection('custom')
																field.onChange(customPort)
															} else {
																setPortSelection('available')
																field.onChange(parseInt(value))
															}
														}} 
														value={portSelection === 'custom' ? 'custom' : field.value?.toString()}
													>
														<SelectTrigger>
															<SelectValue placeholder="Select a port" />
														</SelectTrigger>
														<SelectContent>
															{prometheusDefaults?.available_ports?.map((port) => (
																<SelectItem key={port} value={port.toString()}>
																	{port}
																</SelectItem>
															))}
															{/* Add current port if it's not in available ports */}
															{prometheusStatus?.port && !prometheusDefaults?.available_ports?.includes(prometheusStatus.port) && (
																<SelectItem key={prometheusStatus.port} value={prometheusStatus.port.toString()}>
																	{prometheusStatus.port} (Current)
																</SelectItem>
															)}
															<SelectItem value="custom">Custom Port</SelectItem>
														</SelectContent>
													</Select>
												</FormControl>
												{portSelection === 'custom' && (
													<Input 
														type="number" 
														placeholder="Enter custom port (1-65535)"
														min="1"
														max="65535"
														value={customPort}
														onChange={(e) => {
															const port = parseInt(e.target.value)
															if (!isNaN(port) && port >= 1 && port <= 65535) {
																setCustomPort(port)
																field.onChange(port)
															}
														}}
														className="mt-2"
													/>
												)}
												<FormMessage />
											</FormItem>
										)}
									/>
									<FormField
										control={form.control}
										name="prometheus_version"
										render={({ field }) => (
											<FormItem>
												<FormLabel>Prometheus Version</FormLabel>
												<FormControl>
													<Input {...field} />
												</FormControl>
												<FormMessage />
											</FormItem>
										)}
									/>
									<FormField
										control={form.control}
										name="scrape_interval"
										render={({ field }) => (
											<FormItem>
												<FormLabel>Scrape Interval (seconds)</FormLabel>
												<FormControl>
													<Input type="number" {...field} onChange={(e) => field.onChange(parseInt(e.target.value))} />
												</FormControl>
												<FormMessage />
											</FormItem>
										)}
									/>
									<FormField
										control={form.control}
										name="deployment_mode"
										render={({ field }) => (
											<FormItem>
												<FormLabel>Deployment Mode</FormLabel>
												<Select onValueChange={field.onChange} defaultValue={field.value}>
													<FormControl>
														<SelectTrigger>
															<SelectValue placeholder="Select deployment mode" />
														</SelectTrigger>
													</FormControl>
													<SelectContent>
														<SelectItem value="docker">Docker</SelectItem>
														<SelectItem value="service">Service</SelectItem>
													</SelectContent>
												</Select>
												<FormMessage />
											</FormItem>
										)}
									/>
									{form.watch('deployment_mode') === 'docker' && (
										<FormField
											control={form.control}
											name="network_mode"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Network Mode</FormLabel>
													<Select onValueChange={field.onChange} defaultValue={field.value}>
														<FormControl>
															<SelectTrigger>
																<SelectValue placeholder="Select network mode" />
															</SelectTrigger>
														</FormControl>
														<SelectContent>
															<SelectItem value="bridge">Bridge</SelectItem>
															<SelectItem value="host">Host</SelectItem>
														</SelectContent>
													</Select>
													<FormMessage />
												</FormItem>
											)}
										/>
									)}
									<DialogFooter>
										<Button type="submit" disabled={deployPrometheus.isPending}>
											{deployPrometheus.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
											Update Deployment
										</Button>
									</DialogFooter>
								</form>
							</Form>
						</DialogContent>
					</Dialog>
					</div>
				</div>
			</div>

			{/* Mobile View */}
			<div className="md:hidden mb-4">
				<Select value={selectedNode?.id!.toString()} onValueChange={(value) => setSelectedNode(nodes.items.find((n) => n.id!.toString() === value))}>
					<SelectTrigger>
						<SelectValue placeholder="Select a node" />
					</SelectTrigger>
					<SelectContent>
						{nodes.items.map((node) => (
							<SelectItem key={node.id} value={node.id!.toString()}>
								<div className="flex items-center gap-2">
									{node.fabricPeer || node.fabricOrderer ? <FabricIcon className="h-4 w-4" /> : <BesuIcon className="h-4 w-4" />}
									{node.name}
								</div>
							</SelectItem>
						))}
					</SelectContent>
				</Select>
			</div>

			{/* Desktop View */}
			<div className="hidden md:block">
				<Tabs value={selectedNode?.id!.toString()} onValueChange={(value) => setSelectedNode(nodes.items.find((n) => n.id!.toString() === value))}>
					<TabsList className="w-full justify-start">
						{nodes.items.map((node) => (
							<TabsTrigger key={node.id} value={node.id!.toString()} className="flex items-center gap-2">
								{node.fabricPeer || node.fabricOrderer ? <FabricIcon className="h-4 w-4" /> : <BesuIcon className="h-4 w-4" />}
								{node.name}
							</TabsTrigger>
						))}
					</TabsList>
					{nodes.items.map((node) => (
						<TabsContent key={node.id} value={node.id!.toString()}>
							<Card>
								<CardHeader>
									<CardTitle>Metrics for {node.name}</CardTitle>
									<CardDescription>Real-time node metrics</CardDescription>
								</CardHeader>
								<CardContent>
									{selectedNode?.id === node.id && (
										<Suspense
											fallback={
												<div className="flex items-center justify-center h-[400px]">
													<Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
												</div>
											}
										>
											{node.besuNode ? (
												<BesuMetricsPage node={node} />
											) : node.fabricOrderer ? (
												<OrdererMetricsPage node={node} />
											) : node.fabricPeer ? (
												<PeerMetricsPage node={node} />
											) : (
												<>
													<p>Unsupported node</p>
												</>
											)}
										</Suspense>
									)}
								</CardContent>
							</Card>
						</TabsContent>
					))}
				</Tabs>
			</div>
		</div>
	)
}

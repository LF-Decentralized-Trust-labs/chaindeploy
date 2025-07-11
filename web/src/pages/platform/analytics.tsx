import { getMetricsStatusOptions, getNodesOptions, postMetricsDeployMutation } from '@/api/client/@tanstack/react-query.gen'
import { HttpNodeResponse, TypesDeployPrometheusRequest } from '@/api/client/types.gen'
import { BesuIcon } from '@/components/icons/besu-icon'
import { FabricIcon } from '@/components/icons/fabric-icon'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { BarChart3, Loader2, Plus } from 'lucide-react'
import { Suspense, useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import * as z from 'zod'
import BesuMetricsPage from '../metrics/besu/[nodeId]'
import OrdererMetricsPage from '../metrics/orderer/[nodeId]'
import PeerMetricsPage from '../metrics/peer/[nodeId]'

const prometheusSetupSchema = z.object({
	prometheus_port: z.number().min(1).max(65535),
	prometheus_version: z.string(),
	scrape_interval: z.number().min(1),
})

type PrometheusSetupForm = z.infer<typeof prometheusSetupSchema>

export default function AnalyticsPage() {
	const [isSetupDialogOpen, setIsSetupDialogOpen] = useState(false)
	const [selectedNode, setSelectedNode] = useState<HttpNodeResponse>()

	const form = useForm<PrometheusSetupForm>({
		resolver: zodResolver(prometheusSetupSchema),
		defaultValues: {
			prometheus_port: 9090,
			prometheus_version: 'v3.4.0',
			scrape_interval: 15,
		},
	})

	const { data: prometheusStatus, isLoading: isStatusLoading, refetch: refetchPrometheusStatus } = useQuery({
		...getMetricsStatusOptions({}),
	})

	const { data: nodes, refetch: refetchNodes } = useQuery({
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
			refetchPrometheusStatus()
			refetchNodes()
		},
		onError: (error: any) => {
			toast.error('Failed to deploy Prometheus', {
				description: error.message,
			})
		},
	})

	const onSubmit = (data: PrometheusSetupForm) => {
		const request: TypesDeployPrometheusRequest = {
			prometheus_port: data.prometheus_port,
			prometheus_version: data.prometheus_version,
			scrape_interval: data.scrape_interval,
		}
		deployPrometheus.mutate({
			body: request,
		})
	}

	if (isStatusLoading) {
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
														<Input type="number" {...field} onChange={(e) => field.onChange(parseInt(e.target.value))} />
													</FormControl>
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

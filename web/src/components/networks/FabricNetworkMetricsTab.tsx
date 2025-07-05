import { ServiceNetworkNode } from '@/api/client/types.gen'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { MetricsCard, MetricsDataPoint } from '@/components/metrics/MetricsCard'
import { MetricsGrid } from '@/components/metrics/MetricsGrid'
import { Loader2 } from 'lucide-react'
import { useMemo, useEffect, useState, useRef, useCallback } from 'react'
import { getMetricsNodeByIdLabelByLabelValues, postMetricsNodeByIdQuery } from '@/api/client/sdk.gen'

// Utility to filter out NaN values from metric data
function filterNaN(data: MetricsDataPoint[] | undefined): MetricsDataPoint[] {
	return (data || []).filter((point: MetricsDataPoint) => !isNaN(point.value))
}

// Color palette for node lines (extend as needed)
const nodeColors = [
	'#2563eb', // blue
	'#eab308', // yellow
	'#9333ea', // purple
	'#16a34a', // green
	'#be185d', // pink
	'#0891b2', // cyan
	'#f59e42', // orange
	'#dc2626', // red
	'#059669', // teal
	'#7c3aed', // indigo
]

interface FabricNetworkMetricsTabProps {
	nodes: ServiceNetworkNode[]
	nodeChannels: Record<string, string | undefined>
	isLoading?: boolean
	networkName: string
}

// New AggregatedMetricsCard component
interface AggregatedMetricsCardProps {
	metric: {
		key: string
		title: string
		query: (channel: string) => string
		color: string
		unit: string
		valueFormatter: (v: number) => string
		chartType: string
	}
	nodes: { node: ServiceNetworkNode['node']; channel: string; color: string }[]
}

const AggregatedMetricsCard = ({ metric, nodes }: AggregatedMetricsCardProps) => {
	const [metricsData, setMetricsData] = useState<Record<string, MetricsDataPoint[]>>({})
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState<string | null>(null)

	// Time range: last 1 hour
	const end = Date.now()
	const start = end - 3600000
	const step = '1m'

	useEffect(() => {
		let cancelled = false
		setLoading(true)
		setError(null)
		const fetchAll = async () => {
			try {
				const allResults: Record<string, MetricsDataPoint[]> = {}
				await Promise.all(
					nodes.map(async ({ node, channel }) => {
						const nodeId = node?.id?.toString() || ''
						if (!channel) return
						const response = await postMetricsNodeByIdQuery({
							path: { id: nodeId },
							body: {
								query: metric.query(channel),
								start: new Date(start).toISOString(),
								end: new Date(end).toISOString(),
								step,
							},
						})
						const data: MetricsDataPoint[] =
							response.data?.data?.result?.flatMap((item: any) =>
								item.values
									? item.values.map(([timestamp, value]: [number, string]) => ({
											timestamp: timestamp * 1000,
											value: parseFloat(value),
										}))
									: []
							) || []
						allResults[nodeId] = data
					})
				)
				if (!cancelled) {
					setMetricsData(allResults)
					setLoading(false)
				}
			} catch (err) {
				if (!cancelled) {
					setError('Failed to load metrics')
					setLoading(false)
				}
			}
		}
		fetchAll()
		return () => {
			cancelled = true
		}
	}, [nodes, metric])

	// Prepare series for multi-line chart
	const series = nodes.map((n) => ({
		name: n.node?.name || n.node?.id?.toString() || 'Node',
		color: n.color,
		data: metricsData[n.node?.id?.toString() || ''] || [],
	}))

	if (loading) {
		return (
			<MetricsCard
				key={metric.key}
				title={metric.title}
				series={series}
				unit={metric.unit}
				valueFormatter={metric.valueFormatter}
				chartType={metric.chartType as any}
				description={metric.title + ' (loading...)'}
			/>
		)
	}
	if (error) {
		return (
			<MetricsCard
				key={metric.key}
				title={metric.title}
				series={series}
				unit={metric.unit}
				valueFormatter={metric.valueFormatter}
				chartType={metric.chartType as any}
				description={metric.title + ' (error)'}
			/>
		)
	}
	return (
		<MetricsCard key={metric.key} title={metric.title} series={series} unit={metric.unit} valueFormatter={metric.valueFormatter} chartType={metric.chartType as any} description={metric.title} />
	)
}

function FabricNetworkMetricsTab({ nodes, nodeChannels, isLoading, networkName }: FabricNetworkMetricsTabProps) {
	const joinedNodes = useMemo(() => nodes.filter((n) => n.status === 'joined' && n.node && (n.node.fabricPeer || n.node.fabricOrderer)), [nodes])

	const peerMetrics = useMemo(
		() => [
			{
				key: 'blockHeight',
				title: 'Block Height',
				query: (channel: string) => `ledger_blockchain_height{channel="${channel}"}`,
				color: '#9333ea',
				unit: 'blocks',
				valueFormatter: (v: number) => v.toFixed(0),
				chartType: 'area',
			},
			{
				key: 'transactionRate',
				title: 'Transaction Rate',
				query: (channel: string) => `rate(ledger_transaction_count{channel="${channel}"}[1m])`,
				color: '#eab308',
				unit: 'tx/s',
				valueFormatter: (v: number) => v.toFixed(2),
				chartType: 'line',
			},
			{
				key: 'blockProcessingTime',
				title: 'Block Processing Time (95th percentile)',
				query: (channel: string) => `histogram_quantile(0.95, sum(rate(ledger_block_processing_time_bucket{channel="${channel}"}[1m])) by (le))`,
				color: '#f59e42',
				unit: 'seconds',
				valueFormatter: (v: number) => v.toFixed(3),
				chartType: 'area',
			},
		],
		[]
	)
	const ordererMetrics = useMemo(
		() => [
			{
				key: 'blockHeight',
				title: 'Block Height',
				query: (channel: string) => `consensus_etcdraft_committed_block_number{channel="${channel}"}`,
				color: '#9333ea',
				unit: 'blocks',
				valueFormatter: (v: number) => v.toFixed(0),
				chartType: 'area',
			},
			{
				key: 'clusterSize',
				title: 'Cluster Size',
				query: (channel: string) => `consensus_etcdraft_cluster_size{channel="${channel}"}`,
				color: '#16a34a',
				unit: 'nodes',
				valueFormatter: (v: number) => v.toFixed(0),
				chartType: 'line',
			},
			{
				key: 'leaderChanges',
				title: 'Leader Changes',
				query: (channel: string) => `consensus_etcdraft_leader_changes{channel="${channel}"}`,
				color: '#eab308',
				unit: 'changes',
				valueFormatter: (v: number) => v.toFixed(0),
				chartType: 'line',
			},
		],
		[]
	)

	const allMetrics = useMemo(() => [...peerMetrics, ...ordererMetrics.filter((om) => !peerMetrics.some((pm) => pm.key === om.key))], [peerMetrics, ordererMetrics])
	console.log('allMetrics', allMetrics)

	if (isLoading) {
		return (
			<div className="flex items-center justify-center h-[400px]">
				<Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
			</div>
		)
	}

	if (!joinedNodes.length) {
		return (
			<Card>
				<CardContent className="pt-6">
					<p className="text-center text-muted-foreground">No joined peer or orderer nodes available in this network.</p>
				</CardContent>
			</Card>
		)
	}

	return (
		<div>
			<Card className="mb-4">
				<CardHeader>
					<CardTitle>Fabric Network Metrics</CardTitle>
				</CardHeader>
				<CardContent>
					<p className="text-muted-foreground text-sm">Metrics are shown for all joined peer and orderer nodes. Each card shows a metric for all nodes.</p>
				</CardContent>
			</Card>
			<MetricsGrid>
				{allMetrics.map((metric) => {
					// Prepare nodes for this metric
					const metricNodes = joinedNodes
						.map((n, idx) => {
							// const nodeId = n.node?.id?.toString() || ''
							// const channel = nodeChannels[nodeId]
							// if (!channel) return null
							return { node: n.node, channel: networkName, color: nodeColors[idx % nodeColors.length] }
						})
						.filter(Boolean)
					console.log('metricNodes', metricNodes)
					if (!metricNodes.length) return null
					return <AggregatedMetricsCard key={metric.key} metric={metric} nodes={metricNodes} />
				})}
			</MetricsGrid>
		</div>
	)
}

// Loader component for node channels
interface FabricNodeChannelsLoaderProps {
	nodes: ServiceNetworkNode[]
	isLoading?: boolean
	networkName: string
}

const FabricNodeChannelsLoader = ({ nodes, isLoading, networkName }: FabricNodeChannelsLoaderProps) => {
	const joinedNodes = useMemo(() => nodes.filter((n) => n.status === 'joined' && n.node && (n.node.fabricPeer || n.node.fabricOrderer)), [nodes])
	const [nodeChannels, setNodeChannels] = useState<Record<string, string | undefined>>({})
	const [channelsLoading, setChannelsLoading] = useState(true)
	const [channelsError, setChannelsError] = useState<string | null>(null)
	const lastNodeIdsRef = useRef<string[]>([])
	useEffect(() => {
		const currentNodeIds = joinedNodes.map((n) => n.node?.id?.toString() || '')
		if (joinedNodes.length === 0) {
			setNodeChannels({})
			setChannelsLoading(false)
			return
		}
		if (currentNodeIds.length === lastNodeIdsRef.current.length && currentNodeIds.every((id, idx) => id === lastNodeIdsRef.current[idx])) {
			setChannelsLoading(false)
			return
		}
		lastNodeIdsRef.current = currentNodeIds
		setChannelsLoading(true)
		setChannelsError(null)
		let cancelled = false
		const fetchChannels = async () => {
			try {
				const results = await Promise.all(
					joinedNodes.map(async (n) => {
						const nodeId = n.node?.id?.toString() || ''
						let metric = ''
						if (n.node?.fabricPeer) metric = 'ledger_blockchain_height'
						else if (n.node?.fabricOrderer) metric = 'consensus_etcdraft_committed_block_number'
						if (!metric) return [nodeId, 'mychannel']
						const response = await getMetricsNodeByIdLabelByLabelValues({
							path: { id: nodeId, label: 'channel' },
							query: { match: [metric] },
						})
						const channel = response.data?.data?.[0] || 'mychannel'
						return [nodeId, channel]
					})
				)
				if (!cancelled) {
					setNodeChannels(Object.fromEntries(results))
					setChannelsLoading(false)
				}
			} catch (err) {
				if (!cancelled) {
					setChannelsError('Failed to load channels')
					setChannelsLoading(false)
				}
			}
		}
		fetchChannels()
		return () => {
			cancelled = true
		}
	}, [joinedNodes])
	if (isLoading || channelsLoading) {
		return (
			<div className="flex items-center justify-center h-[400px]">
				<Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
			</div>
		)
	}

	if (channelsError) {
		return (
			<Card>
				<CardContent className="pt-6">
					<p className="text-center text-destructive">Error loading node channels. Please try again.</p>
				</CardContent>
			</Card>
		)
	}

	return <FabricNetworkMetricsTab nodes={nodes} nodeChannels={nodeChannels} isLoading={isLoading} networkName={networkName} />
}

export default FabricNodeChannelsLoader

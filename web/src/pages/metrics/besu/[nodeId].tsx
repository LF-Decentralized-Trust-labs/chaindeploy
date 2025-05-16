import { HttpNodeResponse } from '@/api/client/types.gen'
import { MetricsCard, MetricsDataPoint } from '@/components/metrics/MetricsCard'
import { MetricsGrid } from '@/components/metrics/MetricsGrid'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useCustomMetrics } from '@/hooks/useCustomMetrics'
import { useState } from 'react'

interface BesuMetricsPageProps {
	node: HttpNodeResponse
}

function filterNaN(data: MetricsDataPoint[] | undefined): MetricsDataPoint[] {
	return (data || []).filter((point: MetricsDataPoint) => !isNaN(point.value))
}

export default function BesuMetricsPage({ node }: BesuMetricsPageProps) {
	const [timeRange, setTimeRange] = useState({ start: Date.now() - 3600000, end: Date.now() })
	const [selectedTimeRangeLabel, setSelectedTimeRangeLabel] = useState('Last 1 hour')

	const timeRanges = [
		{ label: 'Last 1 hour', value: 3600000 },
		{ label: 'Last 6 hours', value: 21600000 },
		{ label: 'Last 24 hours', value: 86400000 },
	]
	const nodeId = node.id?.toString() ?? ''

	// Blockchain Metrics
	const { data: besuBlockchainDifficultyData } = useCustomMetrics({
		nodeId,
		query: 'besu_blockchain_difficulty{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: chainHeadGasLimitData } = useCustomMetrics({
		nodeId,
		query: 'besu_blockchain_chain_head_gas_limit{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: chainHeadGasUsedData } = useCustomMetrics({
		nodeId,
		query: 'besu_blockchain_chain_head_gas_used{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	// Network Metrics
	const { data: p2pMessagesInboundData } = useCustomMetrics({
		nodeId,
		query: 'besu_network_p2p_messages_inbound_total{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: p2pMessagesOutboundData } = useCustomMetrics({
		nodeId,
		query: 'besu_network_p2p_messages_outbound_total{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	// Peer Metrics
	const { data: peersConnectedData } = useCustomMetrics({
		nodeId,
		query: 'besu_peers_connected_total{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	// Transaction Pool Metrics
	const { data: transactionPoolSizeData } = useCustomMetrics({
		nodeId,
		query: 'besu_transaction_pool_transactions{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	// Synchronization Metrics
	const { data: synchronizerInSyncData } = useCustomMetrics({
		nodeId,
		query: 'besu_synchronizer_in_sync{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	// JVM Metrics
	const { data: jvmMemoryUsedData } = useCustomMetrics({
		nodeId,
		query: 'jvm_memory_used_bytes{area="heap",job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: jvmThreadsData } = useCustomMetrics({
		nodeId,
		query: 'jvm_threads_current{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	// Additional Blockchain Metrics
	const { data: chainHeadTransactionCountData } = useCustomMetrics({
		nodeId,
		query: 'besu_blockchain_chain_head_transaction_count{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: chainHeadOmmerCountData } = useCustomMetrics({
		nodeId,
		query: 'besu_blockchain_chain_head_ommer_count{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	// Executor Metrics
	const { data: bftExecutorTasksData } = useCustomMetrics({
		nodeId,
		query: 'besu_executors_bfttimerexecutor_qbft_completed_tasks_total{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: blockCreationTasksData } = useCustomMetrics({
		nodeId,
		query: 'besu_executors_ethscheduler_blockcreation_completed_tasks_total{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	// Cache Metrics
	const { data: cacheHitData } = useCustomMetrics({
		nodeId,
		query: 'guava_cache_hit_total{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: cacheMissData } = useCustomMetrics({
		nodeId,
		query: 'guava_cache_miss_total{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	// Process Metrics
	const { data: processCpuData } = useCustomMetrics({
		nodeId,
		query: 'process_cpu_seconds_total{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: processOpenFdsData } = useCustomMetrics({
		nodeId,
		query: 'process_open_fds{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	// Additional Blockchain Height Metrics
	const { data: blockchainHeightData } = useCustomMetrics({
		nodeId,
		query: 'ethereum_blockchain_height{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: bestKnownBlockData } = useCustomMetrics({
		nodeId,
		query: 'ethereum_best_known_block_number{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: finalizedBlockData } = useCustomMetrics({
		nodeId,
		query: 'ethereum_blockchain_finalized_block{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: safeBlockData } = useCustomMetrics({
		nodeId,
		query: 'ethereum_blockchain_safe_block{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: minGasPriceData } = useCustomMetrics({
		nodeId,
		query: 'ethereum_min_gas_price{job="{jobName}"}',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	// Transaction Rate Metrics
	const { data: transactionCountData } = useCustomMetrics({
		nodeId,
		query: 'rate(besu_blockchain_chain_head_transaction_count_counter_total{job="{jobName}"}[1m])',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: transactionPoolAddedData } = useCustomMetrics({
		nodeId,
		query: 'rate(besu_transaction_pool_transactions_added_total{job="{jobName}"}[1m])',
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	if (!node.id) {
		return null
	}

	return (
		<Card>
			<CardHeader>
				<div className="flex items-center justify-between w-full">
					<CardTitle>Besu Metrics</CardTitle>
					<div className="w-[180px]">
						<Select
							value={selectedTimeRangeLabel}
							onValueChange={(label) => {
								const range = timeRanges.find((r) => r.label === label)
								if (range) {
									setTimeRange({ start: Date.now() - range.value, end: Date.now() })
									setSelectedTimeRangeLabel(label)
								}
							}}
						>
							<SelectTrigger>
								<SelectValue placeholder="Select time range" />
							</SelectTrigger>
							<SelectContent>
								{timeRanges.map((range) => (
									<SelectItem key={range.label} value={range.label}>
										{range.label}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</div>
				</div>
			</CardHeader>
			<CardContent>
				<div className="container mx-auto py-6 space-y-8">
					{/* Transaction Rate Metrics Section */}
					<div>
						<h2 className="text-xl font-semibold mb-4">Transaction Rate Metrics</h2>
						<MetricsGrid>
							<MetricsCard title="Transactions Per Second" data={filterNaN(transactionCountData)} color="#0284c7" unit="tx/s" valueFormatter={(value: number) => value.toFixed(2)} />
							<MetricsCard
								title="Transaction Pool Additions Per Second"
								data={filterNaN(transactionPoolAddedData)}
								color="#0369a1"
								unit="tx/s"
								valueFormatter={(value: number) => value.toFixed(2)}
							/>
						</MetricsGrid>
					</div>

					{/* Blockchain Height Metrics Section */}
					<div>
						<h2 className="text-xl font-semibold mb-4">Blockchain Height Metrics</h2>
						<MetricsGrid>
							<MetricsCard title="Current Block Height" data={filterNaN(blockchainHeightData)} color="#2563eb" unit="blocks" valueFormatter={(value: number) => value.toFixed(0)} />
							<MetricsCard title="Best Known Block" data={filterNaN(bestKnownBlockData)} color="#7c3aed" unit="blocks" valueFormatter={(value: number) => value.toFixed(0)} />
							<MetricsCard title="Finalized Block" data={filterNaN(finalizedBlockData)} color="#059669" unit="blocks" valueFormatter={(value: number) => value.toFixed(0)} />
							<MetricsCard title="Safe Block" data={filterNaN(safeBlockData)} color="#d97706" unit="blocks" valueFormatter={(value: number) => value.toFixed(0)} />
						</MetricsGrid>
					</div>

					{/* Blockchain Metrics Section */}
					<div>
						<h2 className="text-xl font-semibold mb-4">Blockchain Metrics</h2>
						<MetricsGrid>
							<MetricsCard
								title="Blockchain Difficulty"
								data={filterNaN(besuBlockchainDifficultyData)}
								color="#9333ea"
								unit="difficulty"
								valueFormatter={(value: number) => value.toFixed(0)}
							/>
							<MetricsCard title="Chain Head Gas Limit" data={filterNaN(chainHeadGasLimitData)} color="#3b82f6" unit="gas" valueFormatter={(value: number) => value.toFixed(0)} />
							<MetricsCard title="Chain Head Gas Used" data={filterNaN(chainHeadGasUsedData)} color="#10b981" unit="gas" valueFormatter={(value: number) => value.toFixed(0)} />
							<MetricsCard
								title="Chain Head Transaction Count"
								data={filterNaN(chainHeadTransactionCountData)}
								color="#f97316"
								unit="transactions"
								valueFormatter={(value: number) => value.toFixed(0)}
							/>
							<MetricsCard title="Chain Head Ommer Count" data={filterNaN(chainHeadOmmerCountData)} color="#a855f7" unit="ommers" valueFormatter={(value: number) => value.toFixed(0)} />
							<MetricsCard title="Minimum Gas Price" data={filterNaN(minGasPriceData)} color="#eab308" unit="wei" valueFormatter={(value: number) => value.toFixed(0)} />
						</MetricsGrid>
					</div>

					{/* Network Metrics Section */}
					<div>
						<h2 className="text-xl font-semibold mb-4">Network Metrics</h2>
						<MetricsGrid>
							<MetricsCard title="P2P Messages Inbound" data={filterNaN(p2pMessagesInboundData)} color="#f59e0b" unit="messages" valueFormatter={(value: number) => value.toFixed(0)} />
							<MetricsCard title="P2P Messages Outbound" data={filterNaN(p2pMessagesOutboundData)} color="#ef4444" unit="messages" valueFormatter={(value: number) => value.toFixed(0)} />
						</MetricsGrid>
					</div>

					{/* Peer Metrics Section */}
					<div>
						<h2 className="text-xl font-semibold mb-4">Peer Metrics</h2>
						<MetricsGrid>
							<MetricsCard title="Connected Peers" data={filterNaN(peersConnectedData)} color="#8b5cf6" unit="peers" valueFormatter={(value: number) => value.toFixed(0)} />
						</MetricsGrid>
					</div>

					{/* Transaction Pool Metrics Section */}
					<div>
						<h2 className="text-xl font-semibold mb-4">Transaction Pool Metrics</h2>
						<MetricsGrid>
							<MetricsCard
								title="Transaction Pool Size"
								data={filterNaN(transactionPoolSizeData)}
								color="#ec4899"
								unit="transactions"
								valueFormatter={(value: number) => value.toFixed(0)}
							/>
						</MetricsGrid>
					</div>

					{/* Synchronization Metrics Section */}
					<div>
						<h2 className="text-xl font-semibold mb-4">Synchronization Metrics</h2>
						<MetricsGrid>
							<MetricsCard
								title="Synchronization Status"
								data={filterNaN(synchronizerInSyncData)}
								color="#14b8a6"
								unit="status"
								valueFormatter={(value: number) => (value === 1 ? 'In Sync' : 'Out of Sync')}
							/>
						</MetricsGrid>
					</div>

					{/* Executor Metrics Section */}
					<div>
						<h2 className="text-xl font-semibold mb-4">Executor Metrics</h2>
						<MetricsGrid>
							<MetricsCard title="BFT Executor Tasks" data={filterNaN(bftExecutorTasksData)} color="#06b6d4" unit="tasks" valueFormatter={(value: number) => value.toFixed(0)} />
							<MetricsCard title="Block Creation Tasks" data={filterNaN(blockCreationTasksData)} color="#0ea5e9" unit="tasks" valueFormatter={(value: number) => value.toFixed(0)} />
						</MetricsGrid>
					</div>

					{/* Cache Metrics Section */}
					<div>
						<h2 className="text-xl font-semibold mb-4">Cache Metrics</h2>
						<MetricsGrid>
							<MetricsCard title="Cache Hits" data={filterNaN(cacheHitData)} color="#22c55e" unit="hits" valueFormatter={(value: number) => value.toFixed(0)} />
							<MetricsCard title="Cache Misses" data={filterNaN(cacheMissData)} color="#ef4444" unit="misses" valueFormatter={(value: number) => value.toFixed(0)} />
						</MetricsGrid>
					</div>

					{/* Process Metrics Section */}
					<div>
						<h2 className="text-xl font-semibold mb-4">Process Metrics</h2>
						<MetricsGrid>
							<MetricsCard title="CPU Usage" data={filterNaN(processCpuData)} color="#f59e0b" unit="seconds" valueFormatter={(value: number) => value.toFixed(2)} />
							<MetricsCard title="Open File Descriptors" data={filterNaN(processOpenFdsData)} color="#8b5cf6" unit="fds" valueFormatter={(value: number) => value.toFixed(0)} />
						</MetricsGrid>
					</div>

					{/* JVM Metrics Section */}
					<div>
						<h2 className="text-xl font-semibold mb-4">JVM Metrics</h2>
						<MetricsGrid>
							<MetricsCard
								title="JVM Memory Used"
								data={filterNaN(jvmMemoryUsedData)}
								color="#6366f1"
								unit="bytes"
								valueFormatter={(value: number) => (value / 1024 / 1024).toFixed(2) + ' MB'}
							/>
							<MetricsCard title="JVM Threads" data={filterNaN(jvmThreadsData)} color="#f43f5e" unit="threads" valueFormatter={(value: number) => value.toFixed(0)} />
						</MetricsGrid>
					</div>
				</div>
			</CardContent>
		</Card>
	)
}

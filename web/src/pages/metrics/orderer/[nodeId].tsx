import { HttpNodeResponse } from '@/api/client/types.gen'
import { MetricsCard, MetricsDataPoint } from '@/components/metrics/MetricsCard'
import { MetricsGrid } from '@/components/metrics/MetricsGrid'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useCustomMetrics } from '@/hooks/useCustomMetrics'
import { useMetricLabels } from '@/hooks/useMetricLabels'
import { useEffect, useState } from 'react'

interface OrdererMetricsPageProps {
	node: HttpNodeResponse
}

function filterNaN(data: MetricsDataPoint[] | undefined): MetricsDataPoint[] {
	return (data || []).filter((point: MetricsDataPoint) => !isNaN(point.value))
}

export default function OrdererMetricsPage({ node }: OrdererMetricsPageProps) {
	const [timeRange, setTimeRange] = useState({ start: Date.now() - 3600000, end: Date.now() })
	const [selectedChannel, setSelectedChannel] = useState<string>('')
	const [selectedTimeRangeLabel, setSelectedTimeRangeLabel] = useState('Last 1 hour')

	const timeRanges = [
		{ label: 'Last 1 hour', value: 3600000 },
		{ label: 'Last 6 hours', value: 21600000 },
		{ label: 'Last 24 hours', value: 86400000 },
	]
	const nodeId = node.id?.toString() || ''
	// Get available channels
	const { data: channels } = useMetricLabels({
		nodeId: nodeId,
		metric: 'consensus_etcdraft_committed_block_number',
		label: 'channel',
	})

	// Auto-select the first channel when channels are loaded
	useEffect(() => {
		if (channels && channels.length > 0 && !selectedChannel) {
			setSelectedChannel(channels[0])
		}
	}, [channels, selectedChannel])

	// Metrics
	const { data: blockHeightData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `consensus_etcdraft_committed_block_number{job="{jobName}", channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: blockFillDurationData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `rate(blockcutter_block_fill_duration_sum{channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: broadcastEnqueueDurationData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `rate(broadcast_enqueue_duration_sum{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: broadcastValidateDurationData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `rate(broadcast_validate_duration_sum{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: processedCountData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `rate(broadcast_processed_count{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: clusterSizeData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `consensus_etcdraft_cluster_size{job="{jobName}",channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: activeNodesData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `consensus_etcdraft_active_nodes{job="{jobName}",channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: leaderChangesData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `consensus_etcdraft_leader_changes{job="{jobName}",channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: participationStatusData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `participation_status{job="{jobName}",channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: participationConsensusRelationData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `participation_consensus_relation{job="{jobName}",channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	// SmartBFT Metrics
	const { data: smartbftBlacklistCountData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `consensus_smartbft_blacklist_count{job="{jobName}",channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: smartbftConsensusLatencySyncData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `rate(consensus_smartbft_consensus_latency_sync_sum{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: smartbftConsensusReconfigData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `rate(consensus_smartbft_consensus_reconfig{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: smartbftNodeIdInBlacklistData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `consensus_smartbft_node_id_in_blacklist{job="{jobName}",channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: smartbftPoolCountLeaderForwardRequestData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `rate(consensus_smartbft_pool_count_leader_forward_request{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: smartbftPoolCountOfElementsData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `consensus_smartbft_pool_count_of_elements{job="{jobName}",channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: smartbftPoolLatencyOfElementsData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `rate(consensus_smartbft_pool_latency_of_elements_sum{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: smartbftViewCountBatchAllData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `rate(consensus_smartbft_view_count_batch_all{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: smartbftViewCountTxsAllData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `rate(consensus_smartbft_view_count_txs_all{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: smartbftViewLatencyBatchProcessingData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `rate(consensus_smartbft_view_latency_batch_processing_sum{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	// etcdraft Metrics
	const { data: etcdraftActiveNodesData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `consensus_etcdraft_active_nodes{job="{jobName}",channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: etcdraftClusterSizeData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `consensus_etcdraft_cluster_size{job="{jobName}",channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: etcdraftCommittedBlockNumberData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `consensus_etcdraft_committed_block_number{job="{jobName}",channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: etcdraftConfigProposalsReceivedData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `rate(consensus_etcdraft_config_proposals_received{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: etcdraftDataPersistDurationData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `rate(consensus_etcdraft_data_persist_duration_sum{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: etcdraftIsLeaderData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `consensus_etcdraft_is_leader{job="{jobName}",channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: etcdraftLeaderChangesData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `consensus_etcdraft_leader_changes{job="{jobName}",channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: etcdraftNormalProposalsReceivedData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `rate(consensus_etcdraft_normal_proposals_received{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: etcdraftProposalFailuresData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `rate(consensus_etcdraft_proposal_failures{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: etcdraftSnapshotBlockNumberData } = useCustomMetrics({
		nodeId: nodeId.toString(),
		query: `consensus_etcdraft_snapshot_block_number{job="{jobName}",channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	// If no channels, show a message and do not render the metrics grid
	const noChannels = !channels || channels.length === 0

	if (!nodeId) {
		return null
	}

	return (
		<Card>
			<CardHeader>
				<div className="flex items-center justify-between w-full">
					<CardTitle>Orderer Metrics</CardTitle>
					<div className="flex gap-4 items-center">
						<div className="w-[200px]">
							<Select value={selectedChannel} onValueChange={setSelectedChannel} disabled={noChannels}>
								<SelectTrigger>
									<SelectValue placeholder="Select channel" />
								</SelectTrigger>
								<SelectContent>
									{channels?.map((channel: string) => (
										<SelectItem key={channel} value={channel}>
											{channel}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
						</div>
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
				</div>
			</CardHeader>
			<CardContent>
				{noChannels ? (
					<div className="text-center text-muted-foreground py-12">
						<p>No channels found. Channel-dependent metrics cannot be displayed.</p>
					</div>
				) : (
					<MetricsGrid>
						<MetricsCard
							title="Block Fill Duration (rate)"
							data={filterNaN(blockFillDurationData)}
							color="#0891b2"
							unit="seconds"
							valueFormatter={(value: number) => value.toFixed(3)}
							chartType="area"
						/>
						<MetricsCard
							title="Broadcast Enqueue Duration (rate)"
							data={filterNaN(broadcastEnqueueDurationData)}
							color="#be185d"
							unit="seconds"
							valueFormatter={(value: number) => value.toFixed(3)}
							chartType="area"
						/>
						<MetricsCard title="Transaction Rate" data={filterNaN(processedCountData)} color="#eab308" unit="tx/s" valueFormatter={(value: number) => value.toFixed(2)} />
						<MetricsCard
							title="Broadcast Validate Duration (rate)"
							data={filterNaN(broadcastValidateDurationData)}
							color="#dc2626"
							unit="seconds"
							valueFormatter={(value: number) => value.toFixed(3)}
							chartType="area"
						/>
						<MetricsCard title="Block Height" data={filterNaN(blockHeightData)} color="#9333ea" unit="blocks" valueFormatter={(value: number) => value.toFixed(0)} />
						<MetricsCard title="Cluster Size" data={filterNaN(clusterSizeData)} color="#16a34a" unit="nodes" valueFormatter={(value: number) => value.toFixed(0)} />
						<MetricsCard title="Active Nodes" data={filterNaN(activeNodesData)} color="#9333ea" unit="nodes" valueFormatter={(value: number) => value.toFixed(0)} />
						<MetricsCard title="Leader Changes" data={filterNaN(leaderChangesData)} color="#eab308" unit="changes" valueFormatter={(value: number) => value.toFixed(0)} />
						<div>
							<MetricsCard title="Participation Status" data={filterNaN(participationStatusData)} color="#be185d" unit="status" valueFormatter={(value: number) => value.toFixed(0)} />
							<div className="mt-2 text-xs text-muted-foreground text-center">
								<span className="inline-block mr-2">
									<b>0</b>: Inactive
								</span>
								<span className="inline-block mr-2">
									<b>1</b>: Active
								</span>
								<span className="inline-block mr-2">
									<b>2</b>: Onboarding
								</span>
								<span className="inline-block">
									<b>3</b>: Failed
								</span>
							</div>
						</div>
						<div>
							<MetricsCard
								title="Participation Consensus Relation"
								data={filterNaN(participationConsensusRelationData)}
								color="#2563eb"
								unit="relation"
								valueFormatter={(value: number) => value.toFixed(0)}
							/>
							<div className="mt-2 text-xs text-muted-foreground text-center">
								<span className="inline-block mr-2">
									<b>0</b>: Other
								</span>
								<span className="inline-block mr-2">
									<b>1</b>: Consenter
								</span>
								<span className="inline-block mr-2">
									<b>2</b>: Follower
								</span>
								<span className="inline-block">
									<b>3</b>: Config-tracker
								</span>
							</div>
						</div>

						{/* etcdraft Section */}
						<div className="col-span-full mt-8 mb-2">
							<h2 className="text-lg font-semibold">etcdraft</h2>
						</div>
						<MetricsCard title="Active Nodes" data={filterNaN(etcdraftActiveNodesData)} color="#9333ea" unit="nodes" valueFormatter={(value: number) => value.toFixed(0)} />
						<MetricsCard title="Cluster Size" data={filterNaN(etcdraftClusterSizeData)} color="#16a34a" unit="nodes" valueFormatter={(value: number) => value.toFixed(0)} />
						<MetricsCard
							title="Committed Block Number"
							data={filterNaN(etcdraftCommittedBlockNumberData)}
							color="#9333ea"
							unit="blocks"
							valueFormatter={(value: number) => value.toFixed(0)}
						/>
						<MetricsCard
							title="Config Proposals Received (rate)"
							data={filterNaN(etcdraftConfigProposalsReceivedData)}
							color="#eab308"
							unit="proposals/s"
							valueFormatter={(value: number) => value.toFixed(2)}
						/>
						<MetricsCard
							title="Data Persist Duration (rate)"
							data={filterNaN(etcdraftDataPersistDurationData)}
							color="#0891b2"
							unit="seconds"
							valueFormatter={(value: number) => value.toFixed(3)}
							chartType="area"
						/>
						<MetricsCard title="Is Leader" data={filterNaN(etcdraftIsLeaderData)} color="#059669" unit="leader" valueFormatter={(value: number) => value.toFixed(0)} />
						<MetricsCard title="Leader Changes" data={filterNaN(etcdraftLeaderChangesData)} color="#eab308" unit="changes" valueFormatter={(value: number) => value.toFixed(0)} />
						<MetricsCard
							title="Normal Proposals Received (rate)"
							data={filterNaN(etcdraftNormalProposalsReceivedData)}
							color="#2563eb"
							unit="proposals/s"
							valueFormatter={(value: number) => value.toFixed(2)}
						/>
						<MetricsCard
							title="Proposal Failures (rate)"
							data={filterNaN(etcdraftProposalFailuresData)}
							color="#dc2626"
							unit="failures/s"
							valueFormatter={(value: number) => value.toFixed(2)}
						/>
						<MetricsCard
							title="Snapshot Block Number"
							data={filterNaN(etcdraftSnapshotBlockNumberData)}
							color="#be185d"
							unit="blocks"
							valueFormatter={(value: number) => value.toFixed(0)}
						/>

						{/* SmartBFT Section */}
						<div className="col-span-full mt-8 mb-2">
							<h2 className="text-lg font-semibold">SmartBFT</h2>
						</div>
						<MetricsCard title="Blacklist Count" data={filterNaN(smartbftBlacklistCountData)} color="#dc2626" unit="nodes" valueFormatter={(value: number) => value.toFixed(0)} />
						<MetricsCard
							title="Consensus Latency Sync (rate)"
							data={filterNaN(smartbftConsensusLatencySyncData)}
							color="#0891b2"
							unit="seconds"
							valueFormatter={(value: number) => value.toFixed(3)}
							chartType="area"
						/>
						<MetricsCard
							title="Consensus Reconfig (rate)"
							data={filterNaN(smartbftConsensusReconfigData)}
							color="#eab308"
							unit="reconfigs/s"
							valueFormatter={(value: number) => value.toFixed(2)}
						/>
						<MetricsCard title="Node ID in Blacklist" data={filterNaN(smartbftNodeIdInBlacklistData)} color="#9333ea" unit="id" valueFormatter={(value: number) => value.toFixed(0)} />
						<MetricsCard
							title="Pool Count Leader Forward Request (rate)"
							data={filterNaN(smartbftPoolCountLeaderForwardRequestData)}
							color="#16a34a"
							unit="req/s"
							valueFormatter={(value: number) => value.toFixed(2)}
						/>
						<MetricsCard
							title="Pool Count of Elements"
							data={filterNaN(smartbftPoolCountOfElementsData)}
							color="#059669"
							unit="elements"
							valueFormatter={(value: number) => value.toFixed(0)}
						/>
						<MetricsCard
							title="Pool Latency of Elements (rate)"
							data={filterNaN(smartbftPoolLatencyOfElementsData)}
							color="#be185d"
							unit="seconds"
							valueFormatter={(value: number) => value.toFixed(3)}
							chartType="area"
						/>
						<MetricsCard
							title="View Count Batch All (rate)"
							data={filterNaN(smartbftViewCountBatchAllData)}
							color="#2563eb"
							unit="batches/s"
							valueFormatter={(value: number) => value.toFixed(2)}
						/>
						<MetricsCard
							title="View Count TXs All (rate)"
							data={filterNaN(smartbftViewCountTxsAllData)}
							color="#eab308"
							unit="txs/s"
							valueFormatter={(value: number) => value.toFixed(2)}
						/>
						<MetricsCard
							title="View Latency Batch Processing (rate)"
							data={filterNaN(smartbftViewLatencyBatchProcessingData)}
							color="#7c3aed"
							unit="seconds"
							valueFormatter={(value: number) => value.toFixed(3)}
							chartType="area"
						/>
					</MetricsGrid>
				)}
			</CardContent>
		</Card>
	)
}

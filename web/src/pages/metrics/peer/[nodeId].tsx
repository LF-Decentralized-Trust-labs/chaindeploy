import { HttpNodeResponse } from '@/api/client/types.gen'
import { MetricsCard, MetricsDataPoint } from '@/components/metrics/MetricsCard'
import { MetricsGrid } from '@/components/metrics/MetricsGrid'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useCustomMetrics } from '@/hooks/useCustomMetrics'
import { useMetricLabels } from '@/hooks/useMetricLabels'
import { useState } from 'react'

interface PeerMetricsPageProps {
	node: HttpNodeResponse
}

// Utility to filter out NaN values from metric data
function filterNaN(data: MetricsDataPoint[] | undefined): MetricsDataPoint[] {
	return (data || []).filter((point: MetricsDataPoint) => !isNaN(point.value))
}

export default function PeerMetricsPage({ node }: PeerMetricsPageProps) {
	const [timeRange, setTimeRange] = useState({ start: Date.now() - 3600000, end: Date.now() })
	const [selectedChannel, setSelectedChannel] = useState<string>('demo')
	const [selectedTimeRangeLabel, setSelectedTimeRangeLabel] = useState('Last 1 hour')

	const timeRanges = [
		{ label: 'Last 1 hour', value: 3600000 },
		{ label: 'Last 6 hours', value: 21600000 },
		{ label: 'Last 24 hours', value: 86400000 },
	]

	// Get available channels
	const { data: channels } = useMetricLabels({
		nodeId: node.id!.toString(),
		metric: 'ledger_blockchain_height',
		label: 'channel',
	})

	// Blockchain Metrics
	const { data: blockHeightData } = useCustomMetrics({
		nodeId: node.id!.toString(),
		query: `ledger_blockchain_height{job="{jobName}",channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
		refetchInterval: 15 * 1000,
	})

	const { data: transactionCountData } = useCustomMetrics({
		nodeId: node.id!.toString(),
		query: `rate(ledger_transaction_count{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
		refetchInterval: 15 * 1000,
	})

	const { data: blockProcessingTimeP95Data } = useCustomMetrics({
		nodeId: node.id!.toString(),
		query: `histogram_quantile(0.95, sum(rate(ledger_block_processing_time_bucket{job="{jobName}",channel="${selectedChannel}"}[1m])) by (le))`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: gossipStateHeightData } = useCustomMetrics({
		nodeId: node.id!.toString(),
		query: `gossip_state_height{job="{jobName}",channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: blockStorageCommitTimeData } = useCustomMetrics({
		nodeId: node.id!.toString(),
		// query: `avg(rate(ledger_blockstorage_commit_time_sum{job="{jobName}",channel="${selectedChannel}"}[1m])) / avg(rate(ledger_blockstorage_commit_time_count{job="{jobName}",channel="${selectedChannel}"}[1m]))`,
		query: `ledger_blockstorage_commit_time_sum{job="{jobName}",channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	// Endorsement Metrics

	const { data: endorsementFailuresData } = useCustomMetrics({
		nodeId: node.id!.toString(),
		query: `rate(endorser_endorsement_failures{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: proposalsReceivedData } = useCustomMetrics({
		nodeId: node.id!.toString(),
		query: `rate(endorser_proposals_received{job="{jobName}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: successfulProposalsData } = useCustomMetrics({
		nodeId: node.id!.toString(),
		query: `rate(endorser_successful_proposals{job="{jobName}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	// Gossip Metrics
	const { data: gossipMessagesReceivedData } = useCustomMetrics({
		nodeId: node.id!.toString(),
		query: `rate(gossip_comm_messages_received{job="{jobName}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: gossipMessagesSentData } = useCustomMetrics({
		nodeId: node.id!.toString(),
		query: `rate(gossip_comm_messages_sent{job="{jobName}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: gossipPeersKnownData } = useCustomMetrics({
		nodeId: node.id!.toString(),
		query: `gossip_membership_total_peers_known{job="{jobName}",channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: gossipPayloadBufferSizeData } = useCustomMetrics({
		nodeId: node.id!.toString(),
		query: `gossip_payload_buffer_size{job="{jobName}",channel="${selectedChannel}"}`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	// Gossip Histograms (rate)
	const { data: gossipPrivdataCommitBlockDurationData } = useCustomMetrics({
		nodeId: node.id!.toString(),
		query: `rate(gossip_privdata_commit_block_duration_sum{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: gossipPrivdataListMissingDurationData } = useCustomMetrics({
		nodeId: node.id!.toString(),
		query: `rate(gossip_privdata_list_missing_duration_sum{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: gossipPrivdataPurgeDurationData } = useCustomMetrics({
		nodeId: node.id!.toString(),
		query: `rate(gossip_privdata_purge_duration_sum{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: gossipPrivdataReconciliationDurationData } = useCustomMetrics({
		nodeId: node.id!.toString(),
		query: `rate(gossip_privdata_reconciliation_duration_sum{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: gossipPrivdataSendDurationData } = useCustomMetrics({
		nodeId: node.id!.toString(),
		query: `rate(gossip_privdata_send_duration_sum{job="{jobName}",channel="${selectedChannel}"}[1m])`,
		start: timeRange.start,
		end: timeRange.end,
		step: '1m',
	})

	const { data: gossipPrivdataValidationDurationData } = useCustomMetrics({
		nodeId: node.id!.toString(),
		query: `rate(gossip_privdata_validation_duration_sum{job="{jobName}",channel="${selectedChannel}"}[1m])`,
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
					<CardTitle>Peer Metrics</CardTitle>
					<div className="flex gap-4 items-center">
						<div className="w-[200px]">
							<Select value={selectedChannel} onValueChange={setSelectedChannel}>
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
				<div className="container mx-auto py-6">
					<MetricsGrid>
						<MetricsCard title="Block Height" data={filterNaN(blockHeightData)} color="#9333ea" unit="blocks" valueFormatter={(value: number) => value.toFixed(0)} chartType="area" />

						<MetricsCard
							title="Transaction Rate"
							data={filterNaN(transactionCountData)}
							color="#eab308"
							unit="tx/s"
							valueFormatter={(value: number) => value.toFixed(2)}
							chartType="line"
						/>

						<MetricsCard
							title="Block Processing Time (95th percentile)"
							data={filterNaN(blockProcessingTimeP95Data)}
							color="#f59e42"
							unit="seconds"
							valueFormatter={(value: number) => value.toFixed(3)}
							chartType="area"
						/>

						<MetricsCard
							title="Block Storage Commit Time"
							data={filterNaN(blockStorageCommitTimeData)}
							color="#7c3aed"
							unit="seconds"
							valueFormatter={(value: number) => value.toFixed(3)}
							chartType="area"
						/>

						<MetricsCard
							title="Gossip State Height"
							data={filterNaN(gossipStateHeightData)}
							color="#9333ea"
							unit="blocks"
							valueFormatter={(value: number) => value.toFixed(0)}
							chartType="area"
						/>

						{/* Endorsement Section */}
						<div className="col-span-full mt-8 mb-2">
							<h2 className="text-lg font-semibold">Endorsement</h2>
						</div>

						<MetricsCard
							title="Endorsement Failures"
							data={filterNaN(endorsementFailuresData)}
							color="#be185d"
							unit="failures/s"
							valueFormatter={(value: number) => value.toFixed(2)}
							chartType="line"
						/>
						<MetricsCard
							title="Proposals Received"
							data={filterNaN(proposalsReceivedData)}
							color="#2563eb"
							unit="proposals/s"
							valueFormatter={(value: number) => value.toFixed(2)}
							chartType="line"
						/>
						<MetricsCard
							title="Successful Proposals"
							data={filterNaN(successfulProposalsData)}
							color="#059669"
							unit="proposals/s"
							valueFormatter={(value: number) => value.toFixed(2)}
							chartType="line"
						/>

						{/* Gossip Section */}
						<div className="col-span-full mt-8 mb-2">
							<h2 className="text-lg font-semibold">Gossip</h2>
						</div>
						<MetricsCard
							title="Messages Received"
							data={filterNaN(gossipMessagesReceivedData)}
							color="#2563eb"
							unit="msgs/s"
							valueFormatter={(value: number) => value.toFixed(2)}
							chartType="line"
						/>
						<MetricsCard
							title="Messages Sent"
							data={filterNaN(gossipMessagesSentData)}
							color="#eab308"
							unit="msgs/s"
							valueFormatter={(value: number) => value.toFixed(2)}
							chartType="line"
						/>
						<MetricsCard
							title="Total Peers Known"
							data={filterNaN(gossipPeersKnownData)}
							color="#9333ea"
							unit="peers"
							valueFormatter={(value: number) => value.toFixed(0)}
							chartType="line"
						/>
						<MetricsCard
							title="Payload Buffer Size"
							data={filterNaN(gossipPayloadBufferSizeData)}
							color="#be185d"
							unit="buffer"
							valueFormatter={(value: number) => value.toFixed(0)}
							chartType="line"
						/>
						<MetricsCard
							title="Privdata Commit Block Duration (avg)"
							data={filterNaN(gossipPrivdataCommitBlockDurationData)}
							color="#0891b2"
							unit="seconds"
							valueFormatter={(value: number) => value.toFixed(3)}
							chartType="area"
						/>
						<MetricsCard
							title="Privdata List Missing Duration (avg)"
							data={filterNaN(gossipPrivdataListMissingDurationData)}
							color="#059669"
							unit="seconds"
							valueFormatter={(value: number) => value.toFixed(3)}
							chartType="area"
						/>
						<MetricsCard
							title="Privdata Purge Duration (avg)"
							data={filterNaN(gossipPrivdataPurgeDurationData)}
							color="#f59e42"
							unit="seconds"
							valueFormatter={(value: number) => value.toFixed(3)}
							chartType="area"
						/>
						<MetricsCard
							title="Privdata Reconciliation Duration (avg)"
							data={filterNaN(gossipPrivdataReconciliationDurationData)}
							color="#be185d"
							unit="seconds"
							valueFormatter={(value: number) => value.toFixed(3)}
							chartType="area"
						/>
						<MetricsCard
							title="Privdata Send Duration (avg)"
							data={filterNaN(gossipPrivdataSendDurationData)}
							color="#2563eb"
							unit="seconds"
							valueFormatter={(value: number) => value.toFixed(3)}
							chartType="area"
						/>
						<MetricsCard
							title="Privdata Validation Duration (avg)"
							data={filterNaN(gossipPrivdataValidationDurationData)}
							color="#eab308"
							unit="seconds"
							valueFormatter={(value: number) => value.toFixed(3)}
							chartType="area"
						/>
					</MetricsGrid>
				</div>
			</CardContent>
		</Card>
	)
}

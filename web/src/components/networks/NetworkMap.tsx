import type { ServiceNetworkMap } from '@/api/client/types.gen'
import type { ServiceNodeMapInfo } from '@/api/client/types.gen'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { BaseEdge, getSmoothStepPath, type EdgeProps } from '@xyflow/react'
import React, { useCallback, useEffect } from 'react'
import ReactFlow, { addEdge, Background, Controls, Edge, EdgeLabelRenderer, Handle, MiniMap, Node, Position, useEdgesState, useNodesState } from 'reactflow'
import 'reactflow/dist/style.css'

// Type alias for node params used in PeerNode/OrdererNode
export type NetworkMapNodeParams = ServiceNodeMapInfo

interface NetworkMapProps {
	map: ServiceNetworkMap | undefined
	isLoading?: boolean
}

const getNodeColor = (role: string | undefined, healthy: boolean | undefined) => {
	if (!healthy) return 'bg-destructive text-destructive-foreground'
	if (role === 'orderer') return 'bg-primary text-primary-foreground'
	if (role === 'peer') return 'bg-secondary text-secondary-foreground'
	return 'bg-muted text-muted-foreground'
}

export function AnimatedSVGEdge({ id, sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition, label }: EdgeProps) {
	const [edgePath] = getSmoothStepPath({
		sourceX,
		sourceY,
		sourcePosition,
		targetX,
		targetY,
		targetPosition,
	})

	// Offset label to the right of the target node
	const labelOffsetX = -50
	const labelOffsetY = 0

	// Try to extract latencyNs from label (expects label like '123.45 ms' or '123.45 ms (123456789 ns)')
	let latencyNs: number | undefined = undefined
	if (label && typeof label === 'string') {
		const match = label.match(/([0-9]+(?:\.[0-9]+)?)\s*ms/)
		if (match) {
			// If label is in ms, convert to ns
			latencyNs = parseFloat(match[1]) * 1_000_000
		}
	}
	// Set duration based on latencyNs
	let duration = 2
	if (latencyNs) {
		const latencyMs = latencyNs / 1_000_000
		if (latencyMs <= 100) {
			// Fast connections (≤100ms): keep duration fast
			duration = Math.max(2, Math.min(4, 2 + latencyMs / 50))
		} else {
			// Slower connections (>100ms): make it slower by 100ms
			const adjustedLatency = latencyMs - 100
			duration = Math.max(2, Math.min(8, 2 + adjustedLatency / 100))
		}
	}
	console.log('duration', duration)
	return (
		<>
			<BaseEdge id={id} path={edgePath} />
			<circle r="10" fill="#ff0073">
				<animateMotion dur={`${duration}s`} repeatCount="indefinite" path={edgePath} />
			</circle>
			{label && (
				<EdgeLabelRenderer>
					<div
						style={{
							position: 'absolute',
							transform: `translate(-50%, -50%) translate(${targetX + labelOffsetX}px,${targetY + labelOffsetY}px)`,
							background: 'white',
							padding: '2px 8px',
							borderRadius: 4,
							fontSize: 12,
							border: '1px solid #ddd',
							pointerEvents: 'all',
							color: '#222',
							fontWeight: 500,
							boxShadow: '0 1px 4px rgba(0,0,0,0.08)',
						}}
						className="nodrag nopan"
					>
						{label}
					</div>
				</EdgeLabelRenderer>
			)}
		</>
	)
}

// Peer node
const PeerNode = ({ data }: { data: NetworkMapNodeParams }) => (
	<div className="rounded-lg border bg-background shadow p-2 min-w-[120px] max-w-[160px] flex flex-col items-center">
		<Handle type="target" position={Position.Left} id="peer-in" className="!bg-primary" isConnectable={true} />
		<div className="font-bold text-sm mb-0.5 truncate w-full text-center">{data.id || data.host}</div>
		<div className="text-[10px] mb-1 truncate w-full text-center">
			{data.host}:{data.port}
		</div>
		<div className="text-[10px] mb-1 truncate w-full text-center">{data.mspId}</div>
		<Badge variant="outline" className="mb-0.5 capitalize text-xs px-2 py-0.5">
			Peer
		</Badge>
		<Badge className={data.healthy ? 'bg-green-500' : 'bg-red-500'}>
			<span className="text-xs">{data.healthy ? 'Healthy' : 'Unhealthy'}</span>
		</Badge>
		<Handle type="source" position={Position.Right} id="peer-out" className="!bg-primary" isConnectable={true} />
	</div>
)

// Orderer node
const OrdererNode = ({ data }: { data: NetworkMapNodeParams }) => (
	<div className="rounded-lg border bg-background shadow p-2 min-w-[120px] max-w-[160px] flex flex-col items-center relative">
		<Handle type="target" position={Position.Left} id="orderer-in" className="!bg-black" isConnectable={true} />
		<div className="font-bold text-sm mb-0.5 truncate w-full text-center">{data.id || data.host}</div>
		<div className="text-[10px] mb-1 truncate w-full text-center">
			{data.host}:{data.port}
		</div>
		<div className="text-[10px] mb-1 truncate w-full text-center">{data.mspId}</div>
		<Badge variant="outline" className="mb-0.5 capitalize text-xs px-2 py-0.5">
			Orderer
		</Badge>
		<Badge className={data.healthy ? 'bg-green-500' : 'bg-red-500'}>
			<span className="text-xs">{data.healthy ? 'Healthy' : 'Unhealthy'}</span>
		</Badge>
		<Handle type="source" position={Position.Right} id="orderer-out" className="!bg-black" isConnectable={true} />
	</div>
)

// Chainlaunch node
const ChainlaunchNode = () => (
	<div className="rounded-lg border-2 border-primary bg-background shadow-lg p-4 min-w-[140px] max-w-[180px] flex flex-col items-center relative">
		<Handle type="target" position={Position.Left} id="chainlaunch-in" className="!bg-primary" isConnectable={true} />
		<span className="font-bold text-lg text-primary mb-1">Chainlaunch</span>
		<span className="text-xs text-muted-foreground">Instance</span>
		<Handle type="source" position={Position.Right} id="chainlaunch-out" className="!bg-primary" isConnectable={true} />
	</div>
)

const nodeTypes = {
	peer: PeerNode,
	orderer: OrdererNode,
	chainlaunch: ChainlaunchNode,
}

const edgeTypes = {
	animatedSvg: AnimatedSVGEdge,
}

export const NetworkMap: React.FC<NetworkMapProps> = ({ map, isLoading }) => {
	const [nodes, setNodes, onNodesChange] = useNodesState([])
	const [edges, setEdges, onEdgesChange] = useEdgesState([])

	// Compute nodes/edges from map
	const computedNodes = React.useMemo(() => {
		if (!map?.nodes) return []
		// Sort all nodes: Peer first, then Orderer, then by host/port
		const allNodes = map.nodes.slice().sort((a, b) => {
			if (a.role !== b.role) {
				return a.role === 'peer' ? -1 : 1
			}
			return (a.host || '').localeCompare(b.host || '')
		})
		let nodesArr: Node[] = []
		// Increase horizontal spacing
		const nodeX = 800
		const nodeYStart = 100
		const nodeSpacing = 140
		const totalHeight = (allNodes.length - 1) * nodeSpacing
		const centerY = nodeYStart + totalHeight / 2
		// Place Chainlaunch node vertically centered, but to the left
		nodesArr.push({
			id: 'chainlaunch',
			position: { x: nodeX - 600, y: centerY },
			data: {},
			type: 'chainlaunch',
		})
		// Lay out sorted nodes vertically
		allNodes.forEach((node, idx) => {
			const nodeType = node.role === 'orderer' ? 'orderer' : 'peer'
			nodesArr.push({
				id: `${node.mspId}-${node.id || idx}`,
				position: {
					x: nodeX,
					y: nodeYStart + idx * nodeSpacing,
				},
				data: node,
				type: nodeType,
			})
		})
		return nodesArr
	}, [map])

	const computedEdges = React.useMemo(() => {
		if (!map?.nodes) return []
		// Use the same sorted order as nodes
		const allNodes = map.nodes.slice().sort((a, b) => {
			if (a.role !== b.role) {
				return a.role === 'peer' ? -1 : 1
			}
			return (a.host || '').localeCompare(b.host || '')
		})
		// Build a set of valid node ids
		const validNodeIds = new Set(allNodes.map((node, idx) => `${node.mspId}-${node.id || idx}`))
		let edgesArr: Edge[] = []
		allNodes.forEach((node, idx) => {
			const targetId = `${node.mspId}-${node.id || idx}`
			if (!validNodeIds.has(targetId)) return // Only add edge if target exists
			const latency = typeof node.latency === 'string' ? node.latency : typeof node.latency === 'number' ? `${node.latency} μs` : undefined
			const latencyNS = node.latencyNs !== undefined ? node.latencyNs : undefined
			let label = latency
			if (latencyNS !== undefined && latency !== undefined) {
				const latencyMs = (latencyNS / 1000000).toFixed(2)
				label = `${latencyMs} ms`
			} else if (latencyNS !== undefined) {
				const latencyMs = (latencyNS / 1000000).toFixed(2)
				label = `${latencyMs} ms`
			}
			edgesArr.push({
				id: `chainlaunch-to-${targetId}`,
				source: 'chainlaunch',
				target: targetId,
				type: 'animatedSvg',
				label,
				labelBgPadding: [6, 2],
				labelBgBorderRadius: 4,
				labelBgStyle: { fill: '#fff', color: '#222', fillOpacity: 0.9 },
				style: { stroke: '#6366f1', strokeWidth: 2 },
			})
		})
		return edgesArr
	}, [map])

	// Update state when map changes
	useEffect(() => {
		setNodes(computedNodes)
		setEdges(computedEdges)
	}, [computedNodes, computedEdges, setNodes, setEdges])

	const onConnect = useCallback((params) => setEdges((eds) => addEdge(params, eds)), [setEdges])

	if (isLoading) {
		return (
			<Card className="p-6 flex flex-col items-center justify-center">
				<Skeleton className="h-96 w-full max-w-3xl" />
			</Card>
		)
	}

	if (!map || !map.nodes || map.nodes.length === 0) {
		return (
			<Card className="p-6 flex flex-col items-center justify-center">
				<p className="text-muted-foreground">No network map data available.</p>
			</Card>
		)
	}

	return (
		<div className="h-[500px] w-full flex items-center justify-center bg-background rounded-lg border relative">
			<div className="h-full w-full max-w-5xl flex items-center justify-center">
				<ReactFlow
					nodes={nodes}
					edges={edges}
					onNodesChange={onNodesChange}
					onEdgesChange={onEdgesChange}
					onConnect={onConnect}
					nodeTypes={nodeTypes}
					edgeTypes={edgeTypes}
					fitView
					minZoom={0.2}
					maxZoom={2}
					className="bg-muted"
				>
					<Background />
					<MiniMap />
					<Controls />
					<Background gap={16} size={1} />
				</ReactFlow>
			</div>
		</div>
	)
}

export default NetworkMap

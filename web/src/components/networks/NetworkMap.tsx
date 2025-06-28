import React, { useMemo } from 'react'
import ReactFlow, { Background, Controls, MiniMap, Node, Edge, Position } from 'reactflow'
import 'reactflow/dist/style.css'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import type { ServiceNetworkMap, ServiceNodeMapInfo } from '@/api/client/types.gen'
import { BaseEdge, getSmoothStepPath, type EdgeProps } from '@xyflow/react'

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

export function AnimatedSVGEdge({ id, sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition }: EdgeProps) {
	const [edgePath] = getSmoothStepPath({
		sourceX,
		sourceY,
		sourcePosition,
		targetX,
		targetY,
		targetPosition,
	})
	return (
		<>
			<BaseEdge id={id} path={edgePath} />
			<circle r="10" fill="#ff0073">
				<animateMotion dur="2s" repeatCount="indefinite" path={edgePath} />
			</circle>
		</>
	)
}

// Custom org group node
const OrgGroupNode = ({ data }: any) => {
	return (
		<div className="rounded-xl border-2 border-primary/30 bg-background/80 shadow-md px-4 py-2 min-w-[200px]">
			<div className="font-bold text-primary mb-2 text-center">{data.orgId}</div>
			<div className="flex flex-col items-center gap-2">{data.children}</div>
		</div>
	)
}

const nodeTypes = {
	'org-group': OrgGroupNode,
}
const edgeTypes = {
	animatedSvg: AnimatedSVGEdge,
}
export const NetworkMap: React.FC<NetworkMapProps> = ({ map, isLoading }) => {
	// Group nodes by org (using node.id as orgId for now)
	const { nodes, edges } = useMemo(() => {
		if (!map?.nodes) return { nodes: [], edges: [] }
		// Group nodes by mspId
		const grouped: Record<string, ServiceNodeMapInfo[]> = {}
		map.nodes.forEach((node) => {
			const orgId = node.mspId // Use mspId as org identifier
			if (!orgId) return
			if (!grouped[orgId]) grouped[orgId] = []
			grouped[orgId].push(node)
		})
		const orgIds = Object.keys(grouped)
		const orgBoxWidth = 220
		const orgBoxSpacing = 80
		const totalWidth = orgIds.length * orgBoxWidth + (orgIds.length - 1) * orgBoxSpacing
		let nodesArr: Node[] = []
		// Create one org-group node per org
		orgIds.forEach((orgId, orgIdx) => {
			const orgNodes = grouped[orgId]
			const x = orgIdx * (orgBoxWidth + orgBoxSpacing) - totalWidth / 2 + orgBoxWidth / 2
			const orgHeight = Math.max(80, orgNodes.length * 90)
			const y = 250 - orgHeight / 2
			// Children node elements for org group
			const children = orgNodes.map((node, idx) => {
				const nodeId = `${orgId}-${idx}`
				return (
					<div
						key={nodeId}
						className={`rounded-lg shadow p-2 min-w-[120px] max-w-[160px] flex flex-col items-center border ${getNodeColor(node.role, node.healthy)} mb-2`}
						tabIndex={0}
						aria-label={`${node.id || 'Node'} (${node.role}) ${node.healthy ? 'Healthy' : 'Unhealthy'}`}
						style={{ marginBottom: idx === orgNodes.length - 1 ? 0 : 8 }}
					>
						<div className="font-bold text-sm mb-0.5 truncate w-full text-center">{node.id || node.host}</div>
						<div className="text-[10px] mb-1 truncate w-full text-center">
							{node.host}:{node.port}
						</div>
						<Badge variant="outline" className="mb-0.5 capitalize text-xs px-2 py-0.5">
							{node.role}
						</Badge>
						<Badge className={node.healthy ? 'bg-green-500' : 'bg-red-500'}>
							<span className="text-xs">{node.healthy ? 'Healthy' : 'Unhealthy'}</span>
						</Badge>
						{typeof node.latency === 'number' && <div className="text-[10px] mt-1">{`Latency: ${node.latency} μs`}</div>}
					</div>
				)
			})
			nodesArr.push({
				id: `org-group-${orgId}`,
				position: { x, y },
				data: { orgId, children },
				type: 'org-group',
				draggable: false,
				selectable: false,
				zIndex: 10,
				width: orgBoxWidth,
				height: orgHeight,
			})
		})
		// Edges: connect only org-group nodes (not individual nodes)
		let edgesArr: Edge[] = []
		const nodeIdsSet = new Set(nodesArr.map(n => n.id))
		for (let i = 0; i < orgIds.length; i++) {
			for (let j = i + 1; j < orgIds.length; j++) {
				const fromId = `org-group-${orgIds[i]}`
				const toId = `org-group-${orgIds[j]}`
				if (nodeIdsSet.has(fromId) && nodeIdsSet.has(toId)) {
					edgesArr.push({
						id: `org-connect-${orgIds[i]}-to-${orgIds[j]}`,
						source: fromId,
						target: toId,
						type: 'animatedSvg',
					})
				}
			}
		}
		return { nodes: nodesArr, edges: edgesArr }
	}, [map])
	console.log(nodes, edges)
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
					fitView
					nodeTypes={nodeTypes}
					edgeTypes={edgeTypes}
					minZoom={0.2}
					defaultEdgeOptions={{
						style: {
							stroke: '#888',
							strokeWidth: 1,
						},
					}}
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

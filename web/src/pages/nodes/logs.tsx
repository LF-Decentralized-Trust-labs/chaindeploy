import { getNodesOptions } from '@/api/client/@tanstack/react-query.gen'
import { BesuIcon } from '@/components/icons/besu-icon'
import { FabricIcon } from '@/components/icons/fabric-icon'
import { LogViewer } from '@/components/nodes/LogViewer'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'

interface NodeLogs {
	nodeId: number
	logs: string
}

export default function NodesLogsPage() {
	const navigate = useNavigate()
	const [searchParams] = useSearchParams()
	const [selectedNode, setSelectedNode] = useState<string>(searchParams.get('node') || undefined)
	const [nodeLogs, setNodeLogs] = useState<NodeLogs[]>([])
	const logsRef = useRef<HTMLPreElement>(null)
	const abortControllers = useRef<{ [key: string]: AbortController }>({})

	const { data: nodes } = useQuery({
		...getNodesOptions({
			query: {
				limit: 1000,
				page: 1,
			},
		}),
	})

	const scrollToBottom = (_: number | string) => {
		setTimeout(() => {
			const ref = logsRef.current
			if (ref) {
				ref.scrollTop = ref.scrollHeight
			}
		}, 50)
	}

	const fetchNodeLogs = async (nodeId: number) => {
		try {
			// Cancel previous request if exists
			if (abortControllers.current[nodeId]) {
				abortControllers.current[nodeId].abort()
			}

			const abortController = new AbortController()
			abortControllers.current[nodeId] = abortController

			const eventSource = new EventSource(`/api/v1/nodes/${nodeId}/logs?follow=true`, {
				withCredentials: true,
			})

			eventSource.onmessage = (event) => {
				setNodeLogs((prev) => {
					const existing = prev.find((nl) => nl.nodeId === nodeId)
					if (existing) {
						return prev.map((nl) => (nl.nodeId === nodeId ? { ...nl, logs: nl.logs + event.data + '\n' } : nl))
					}
					return [...prev, { nodeId, logs: event.data + '\n' }]
				})
				scrollToBottom(nodeId)
			}

			eventSource.onerror = (error) => {
				console.error('EventSource error:', error)
				eventSource.close()
			}

			// Store the EventSource in the abort controller for cleanup
			abortControllers.current[nodeId] = {
				abort: () => {
					eventSource.close()
				},
			} as AbortController
		} catch (error) {
			console.error('Error fetching logs:', error)
		}
	}

	const handleNodeChange = (nodeId: string) => {
		setSelectedNode(nodeId)
		// Update URL with the selected node
		const params = new URLSearchParams(searchParams.toString())
		params.set('node', nodeId)
		navigate(`/nodes/logs?${params.toString()}`)
	}

	useEffect(() => {
		if (nodes?.items && !selectedNode && nodes.items.length > 0) {
			const nodeFromUrl = searchParams.get('node')
			if (nodeFromUrl && nodes.items.some(node => node.id!.toString() === nodeFromUrl)) {
				setSelectedNode(nodeFromUrl)
			} else {
				setSelectedNode(nodes.items[0].id!.toString())
			}
		}
	}, [nodes, searchParams])

	useEffect(() => {
		if (selectedNode) {
			fetchNodeLogs(parseInt(selectedNode))
		}
	}, [selectedNode])

	useEffect(() => {
		return () => {
			// Cleanup all abort controllers
			Object.values(abortControllers.current).forEach((controller) => {
				controller.abort()
			})
		}
	}, [])

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
				<h1 className="text-2xl font-semibold">Node Logs</h1>
				<p className="text-muted-foreground">View logs from your blockchain nodes</p>
			</div>

			{/* Mobile View */}
			<div className="md:hidden mb-4">
				<Select value={selectedNode} onValueChange={handleNodeChange}>
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
			<div className="hidden md:block mb-4">
				<Select value={selectedNode} onValueChange={handleNodeChange}>
					<SelectTrigger className="w-full max-w-md">
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

			{/* Desktop Content */}
			<div className="hidden md:block">
				{selectedNode && (
					<Card>
						<CardHeader>
							<CardTitle>Logs for {nodes.items.find((n) => n.id!.toString() === selectedNode)?.name}</CardTitle>
							<CardDescription>Real-time node logs</CardDescription>
						</CardHeader>
						<CardContent>
							<LogViewer
								logs={nodeLogs.find((nl) => nl.nodeId.toString() === selectedNode)?.logs || ''}
								onScroll={(isScrolledToBottom) => {
									if (isScrolledToBottom) {
										scrollToBottom(selectedNode!)
									}
								}}
							/>
						</CardContent>
					</Card>
				)}
			</div>

			{/* Mobile Content */}
			<div className="md:hidden">
				{selectedNode && (
					<Card>
						<CardHeader>
							<CardTitle>Logs for {nodes.items.find((n) => n.id!.toString() === selectedNode)?.name}</CardTitle>
							<CardDescription>Real-time node logs</CardDescription>
						</CardHeader>
						<CardContent>
							<LogViewer
								logs={nodeLogs.find((nl) => nl.nodeId.toString() === selectedNode)?.logs || ''}
								onScroll={(isScrolledToBottom) => {
									if (isScrolledToBottom) {
										scrollToBottom(selectedNode!)
									}
								}}
							/>
						</CardContent>
					</Card>
				)}
			</div>
		</div>
	)
}

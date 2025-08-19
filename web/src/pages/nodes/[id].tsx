import { HttpNodeResponse } from '@/api/client'
import {
	deleteNodesByIdMutation,
	getNodesByIdEventsOptions,
	getNodesByIdOptions,
	postNodesByIdCertificatesRenewMutation,
	postNodesByIdRestartMutation,
	postNodesByIdStartMutation,
	postNodesByIdStopMutation,
} from '@/api/client/@tanstack/react-query.gen'
import { FabricNodeDetails } from '@/components/nodes/FabricNodeDetails'
import { BesuNodeDetails } from '@/components/nodes/BesuNodeDetails'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { DropdownMenu, DropdownMenuContent, DropdownMenuGroup, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { TimeAgo } from '@/components/ui/time-ago'
import { cn } from '@/lib/utils'
import { useMutation, useQuery } from '@tanstack/react-query'
import { format } from 'date-fns/format'
import { AlertCircle, CheckCircle2, ChevronDown, Clock, KeyRound, Pencil, Play, PlayCircle, RefreshCcw, RefreshCw, Square, StopCircle, Trash, XCircle } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'

interface DeploymentConfig {
	type?: string
	mode?: string
	organizationId?: number
	mspId?: string
	signKeyId?: number
	tlsKeyId?: number
	signCert?: string
	tlsCert?: string
	caCert?: string
	tlsCaCert?: string
	listenAddress?: string
	chaincodeAddress?: string
	eventsAddress?: string
	operationsListenAddress?: string
	externalEndpoint?: string
	adminAddress?: string
	domainNames?: string[]
}

function isFabricNode(node: HttpNodeResponse): node is HttpNodeResponse & { deploymentConfig: DeploymentConfig } {
	return node.platform === 'FABRIC' && (node.fabricPeer !== undefined || node.fabricOrderer !== undefined)
}

function isBesuNode(node: HttpNodeResponse): node is HttpNodeResponse & { deploymentConfig: DeploymentConfig } {
	return node.platform === 'BESU' && node.besuNode !== undefined
}

function getNodeActions(status: string) {
	switch (status.toLowerCase()) {
		case 'running':
			return [
				{ label: 'Stop', action: 'stop', icon: Square },
				{ label: 'Restart', action: 'restart', icon: RefreshCw },
			]
		case 'stopped':
			return [
				{ label: 'Start', action: 'start', icon: Play },
				{ label: 'Delete', action: 'delete', icon: Trash },
			]
		case 'stopping':
			return [{ label: 'Stop', action: 'stop', icon: Square }]
		case 'error':
			return [
				{ label: 'Start', action: 'start', icon: Play },
				{ label: 'Restart', action: 'restart', icon: RefreshCw },
				{ label: 'Delete', action: 'delete', icon: Trash },
			]
		case 'starting':
		case 'stopping':
			return [] // No actions while transitioning
		default:
			return [
				{ label: 'Start', action: 'start', icon: Play },
				{ label: 'Stop', action: 'stop', icon: Square },
				{ label: 'Restart', action: 'restart', icon: RefreshCw },
			]
	}
}

function getEventIcon(type: string) {
	switch (type.toUpperCase()) {
		case 'START':
			return PlayCircle
		case 'STOP':
			return StopCircle
		case 'RESTART':
			return RefreshCcw
		default:
			return Clock
	}
}

function getEventStatusIcon(status: string) {
	switch (status.toUpperCase()) {
		case 'SUCCESS':
		case 'COMPLETED':
			return CheckCircle2
		case 'FAILED':
			return XCircle
		case 'PENDING':
			return Clock
		default:
			return AlertCircle
	}
}

function getEventStatusColor(status: string) {
	switch (status.toUpperCase()) {
		case 'SUCCESS':
		case 'COMPLETED':
			return 'text-green-500'
		case 'FAILED':
			return 'text-red-500'
		case 'PENDING':
			return 'text-yellow-500'
		default:
			return 'text-gray-500'
	}
}

export default function NodeDetailPage() {
	const { id } = useParams<{ id: string }>()
	const navigate = useNavigate()
	const [searchParams, setSearchParams] = useSearchParams()
	const [logs, setLogs] = useState<string>('')
	const logsRef = useRef<HTMLTextAreaElement>(null)
	const eventSourceRef = useRef<EventSource | null>(null)
	const [showRenewCertDialog, setShowRenewCertDialog] = useState(false)
	const [showDeleteDialog, setShowDeleteDialog] = useState(false)
	const [logRefreshKey, setLogRefreshKey] = useState(0)

	// Get the active tab from URL or default to 'logs'
	const activeTab = searchParams.get('tab') || 'logs'

	// Update URL when tab changes
	const handleTabChange = (value: string) => {
		searchParams.set('tab', value)
		setSearchParams(searchParams)
	}

	const {
		data: node,
		isLoading,
		refetch,
		error,
	} = useQuery({
		...getNodesByIdOptions({
			path: { id: parseInt(id!) },
		}),
	})

	const startNode = useMutation({
		...postNodesByIdStartMutation(),
		onSuccess: () => {
			refetchEvents()
			refetch()
			setLogRefreshKey(prev => prev + 1)
		},
	})

	const stopNode = useMutation({
		...postNodesByIdStopMutation(),
		onSuccess: () => {
			refetchEvents()
			refetch()
			setLogRefreshKey(prev => prev + 1)
		},
	})

	const restartNode = useMutation({
		...postNodesByIdRestartMutation(),
		onSuccess: () => {
			refetchEvents()
			refetch()
			setLogRefreshKey(prev => prev + 1)
		},
	})

	const deleteNode = useMutation({
		...deleteNodesByIdMutation(),
		onSuccess: () => {
			toast.success('Node deleted successfully')
			navigate('/nodes')
		},
		onError: (error: any) => {
			toast.error(`Failed to delete node: ${(error as any).error.message}`)
		},
	})

	const renewCertificates = useMutation({
		...postNodesByIdCertificatesRenewMutation(),
		onSuccess: () => {
			refetchEvents()
			refetch()
			setShowRenewCertDialog(false)
		},
	})

	const { data: events, refetch: refetchEvents } = useQuery({
		...getNodesByIdEventsOptions({
			path: { id: parseInt(id!) },
		}),
	})

	// Function to refresh logs by restarting the EventSource connection
	const refreshLogs = () => {
		setLogRefreshKey(prev => prev + 1)
		setLogs('') // Clear existing logs
	}

	const handleAction = async (action: string) => {
		if (!node) return

		try {
			switch (action) {
				case 'start':
					await toast.promise(
						startNode.mutateAsync({ path: { id: node.id! } }),
						{
							loading: 'Starting node...',
							success: 'Node started successfully',
							error: (error: any) => `Failed to start node: ${error?.error?.message || error.message}`,
						}
					)
					break
				case 'stop':
					await toast.promise(
						stopNode.mutateAsync({ path: { id: node.id! } }),
						{
							loading: 'Stopping node...',
							success: 'Node stopped successfully',
							error: (error: any) => `Failed to stop node: ${error?.error?.message || error.message}`,
						}
					)
					break
				case 'restart':
					await toast.promise(
						restartNode.mutateAsync({ path: { id: node.id! } }),
						{
							loading: 'Restarting node...',
							success: 'Node restarted successfully',
							error: (error: any) => `Failed to restart node: ${error?.error?.message || error.message}`,
						}
					)
					break
				case 'delete':
					setShowDeleteDialog(true)
					break
				case 'renew-certificates':
					setShowRenewCertDialog(true)
					break
			}
		} catch (error) {
			// Error handling is now done by toast.promise
		}
	}

	const handleRenewCertificates = async () => {
		if (!node) return
		try {
			await toast.promise(renewCertificates.mutateAsync({ path: { id: node.id! } }), {
				loading: 'Renewing certificates...',
				success: 'Certificates renewed successfully',
				error: (e) => `Failed to renew certificates: ${e.message}`,
			})
		} catch (error) {
			// Error handling is done in the mutation callbacks
		}
	}

	useEffect(() => {
		// Close existing EventSource if it exists
		if (eventSourceRef.current) {
			eventSourceRef.current.close()
		}

		const eventSource = new EventSource(`/api/v1/nodes/${id}/logs?follow=true`, {
			withCredentials: true,
		})

		eventSourceRef.current = eventSource
		let fullText = ''

		eventSource.onmessage = (event) => {
			fullText += event.data + '\n'
			setLogs(fullText)

			// Scroll to bottom after new logs arrive
			if (logsRef.current) {
				setTimeout(() => {
					if (logsRef.current) {
						logsRef.current.scrollTop = logsRef.current.scrollHeight
					}
				}, 100)
			}
		}

		eventSource.onerror = (error) => {
			console.error('EventSource error:', error)
			eventSource.close()
		}

		return () => {
			eventSource.close()
			if (eventSourceRef.current === eventSource) {
				eventSourceRef.current = null
			}
		}
	}, [id, logRefreshKey])

	if (isLoading) {
		return <div>Loading...</div>
	}

	if (error) {
		return <div>Error loading node: {(error as any)?.error?.message || error.message}</div>
	}
	if (!node) {
		return <div>Node not found</div>
	}
	// Events rendering component
	const renderEvents = () => {
		if (!events?.items?.length) {
			return (
				<div className="flex flex-col items-center justify-center py-8 text-center">
					<Clock className="h-12 w-12 text-muted-foreground mb-4" />
					<h3 className="text-lg font-medium mb-2">No Events Found</h3>
					<p className="text-sm text-muted-foreground max-w-md">
						There are no events recorded for this node yet. Events will appear here when you perform operations like start, stop, or restart.
					</p>
				</div>
			)
		}

		return (
			<div className="space-y-8">
				{events.items.map((event) => {
					const EventIcon = getEventIcon(event.type!)
					const StatusIcon = getEventStatusIcon(event.type!)
					return (
						<div key={event.id} className="flex gap-4">
							<div className="mt-1">
								<EventIcon className="h-5 w-5 text-muted-foreground" />
							</div>
							<div className="flex-1 space-y-1">
								<div className="flex items-center justify-between">
									<div className="flex items-center gap-2">
										<span className="font-medium">{event.type}</span>
										<StatusIcon className={cn('h-4 w-4', getEventStatusColor(event.type!))} />
										<span className="text-sm text-muted-foreground">{event.type}</span>
									</div>
									<time className="text-sm text-muted-foreground">{format(new Date(event.created_at!), 'PPp')}</time>
								</div>
								{event.data && typeof event.data === 'object' ? (
									<div className="rounded-md bg-muted p-2 text-sm">
										<pre className="whitespace-pre-wrap font-mono text-xs">{JSON.stringify(event.data, null, 2)}</pre>
									</div>
								) : null}
							</div>
						</div>
					)
				})}
			</div>
		)
	}

	return (
		<div className="flex-1 space-y-6 p-8">
			<div className="flex items-center justify-between">
				<div>
					<h1 className="text-2xl font-semibold">{node.name}</h1>
					<div className="flex items-center gap-4 mt-1">
						<p className="text-muted-foreground">
							{isFabricNode(node) ? 'Hyperledger Fabric' : 'Hyperledger Besu'} Node
						</p>
						<Badge variant="default">{node.status}</Badge>
						<span className="text-sm text-muted-foreground">
							Created <TimeAgo date={node.createdAt!} />
						</span>
					</div>
				</div>
				<div className="flex gap-2">
					<DropdownMenu>
						<DropdownMenuTrigger asChild>
							<Button variant="outline" size="sm">
								Actions <ChevronDown className="ml-2 h-4 w-4" />
							</Button>
						</DropdownMenuTrigger>
						<DropdownMenuContent align="end">
							<DropdownMenuGroup>
								{getNodeActions(node.status!).map(({ label, action, icon: Icon }) => (
									<DropdownMenuItem
										key={action}
										onClick={() => handleAction(action)}
										disabled={['starting', 'stopping'].includes(node.status!.toLowerCase()) || startNode.isPending || stopNode.isPending || restartNode.isPending}
									>
										<Icon className="mr-2 h-4 w-4" />
										{label}
									</DropdownMenuItem>
								))}

								{isFabricNode(node) && (
									<>
										<DropdownMenuSeparator />
										<DropdownMenuItem onClick={() => handleAction('renew-certificates')} disabled={renewCertificates.isPending}>
											<KeyRound className="mr-2 h-4 w-4" />
											Renew Certificates
										</DropdownMenuItem>
										<DropdownMenuItem onClick={() => navigate(`/nodes/fabric/edit/${node.id}`)}>
											<Pencil className="mr-2 h-4 w-4" />
											Edit
										</DropdownMenuItem>
									</>
								)}

								{isBesuNode(node) && (
									<>
										<DropdownMenuSeparator />
										<DropdownMenuItem onClick={() => navigate(`/nodes/besu/edit/${node.id}`)}>
											<Pencil className="mr-2 h-4 w-4" />
											Edit
										</DropdownMenuItem>
									</>
								)}
							</DropdownMenuGroup>
						</DropdownMenuContent>
					</DropdownMenu>
				</div>
			</div>

			{/* Error Message Card - only show if there's an error */}
			{node.errorMessage && (
				<Card className="border-destructive">
					<CardHeader className="pb-3">
						<CardTitle className="text-base text-destructive">Error Message</CardTitle>
					</CardHeader>
					<CardContent>
						<p className="text-sm">{node.errorMessage}</p>
					</CardContent>
				</Card>
			)}

			{/* Platform-specific content */}
			{isFabricNode(node) && (
				<FabricNodeDetails
					node={node}
					logs={logs}
					events={renderEvents()}
					activeTab={activeTab}
					onTabChange={handleTabChange}
				/>
			)}

			{isBesuNode(node) && (
				<BesuNodeDetails
					node={node}
					logs={logs}
					events={renderEvents()}
					activeTab={activeTab}
					onTabChange={handleTabChange}
				/>
			)}

			<AlertDialog open={showRenewCertDialog} onOpenChange={setShowRenewCertDialog}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Renew Certificates</AlertDialogTitle>
						<AlertDialogDescription>Are you sure you want to renew the certificates for this node? This will generate new TLS and signing certificates.</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction onClick={handleRenewCertificates} disabled={renewCertificates.isPending}>
							Renew Certificates
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>

			<AlertDialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Delete Node</AlertDialogTitle>
						<AlertDialogDescription>Are you sure you want to delete this node? This action cannot be undone.</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction
							onClick={async () => {
								if (node) {
									await deleteNode.mutateAsync({ path: { id: node.id! } })
									setShowDeleteDialog(false)
								}
							}}
							disabled={deleteNode.isPending}
							className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
						>
							Delete
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	)
}

import { getNodesByIdRpcSyncing, HttpNodeResponse } from '@/api/client'
import { getKeysByIdOptions, getNetworksBesuByIdOptions, getNodesByIdRpcSyncingOptions } from '@/api/client/@tanstack/react-query.gen'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { TimeAgo } from '@/components/ui/time-ago'
import { BesuNodeConfig } from '@/components/nodes/BesuNodeConfig'
import { BesuNodeNetwork } from '@/components/nodes/BesuNodeNetwork'
import { LogViewer } from '@/components/nodes/LogViewer'
import BesuMetricsPage from '@/pages/metrics/besu/[nodeId]'
import { Activity, Blocks, GitBranch, Globe, Shield, Key, Copy, Check, ExternalLink } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Link } from 'react-router-dom'

interface BesuNodeDetailsProps {
	node: HttpNodeResponse
	logs: string
	events: any
	activeTab: string
	onTabChange: (value: string) => void
}

export function BesuNodeDetails({ node, logs, events, activeTab, onTabChange }: BesuNodeDetailsProps) {
	const besuNode = node.besuNode!
	const [copiedAddress, setCopiedAddress] = useState(false)
	const [copiedPublicKey, setCopiedPublicKey] = useState(false)

	// Fetch key details if keyId exists
	const { data: keyData } = useQuery({
		...getKeysByIdOptions({
			path: { id: besuNode.keyId! },
		}),
		enabled: !!besuNode.keyId,
	})
	const { data: syncingData } = useQuery({
		...getNodesByIdRpcSyncingOptions({
			path: { id: node.id! },
		}),
		enabled: !!node.id,
	})

	// Fetch network details if networkId exists
	const { data: networkData } = useQuery({
		...getNetworksBesuByIdOptions({
			path: { id: besuNode.networkId! },
		}),
		enabled: !!besuNode.networkId,
	})

	const handleCopyAddress = () => {
		if (keyData?.ethereumAddress) {
			navigator.clipboard.writeText(keyData.ethereumAddress)
			setCopiedAddress(true)
			setTimeout(() => setCopiedAddress(false), 2000)
		}
	}

	const handleCopyPublicKey = () => {
		if (keyData?.publicKey) {
			navigator.clipboard.writeText(keyData.publicKey)
			setCopiedPublicKey(true)
			setTimeout(() => setCopiedPublicKey(false), 2000)
		}
	}

	const trimPublicKey = (key: string) => {
		if (key.length <= 20) return key
		return `${key.slice(0, 10)}...${key.slice(-10)}`
	}

	return (
		<div className="space-y-6">
			{/* Besu-specific header cards */}
			<div className="grid gap-6 md:grid-cols-4">
				<Card>
					<CardHeader className="pb-3">
						<div className="flex items-center gap-2">
							<Blocks className="h-4 w-4 text-muted-foreground" />
							<CardTitle className="text-base">Node Type</CardTitle>
						</div>
					</CardHeader>
					<CardContent>
						<div className="flex items-center gap-2">
							<Badge variant="secondary" className="font-mono">
								VALIDATOR
							</Badge>
							<span className="text-sm text-muted-foreground">{besuNode.mode || 'Standard'}</span>
						</div>
					</CardContent>
				</Card>

				<Card>
					<CardHeader className="pb-3">
						<div className="flex items-center gap-2">
							<GitBranch className="h-4 w-4 text-muted-foreground" />
							<CardTitle className="text-base">Consensus</CardTitle>
						</div>
					</CardHeader>
					<CardContent>
						<div className="space-y-1">
							<p className="font-medium">QBFT</p>
							<p className="text-xs text-muted-foreground">Algorithm</p>
						</div>
					</CardContent>
				</Card>

				<Card>
					<CardHeader className="pb-3">
						<div className="flex items-center gap-2">
							<Activity className="h-4 w-4 text-muted-foreground" />
							<CardTitle className="text-base">Sync Status</CardTitle>
						</div>
					</CardHeader>
					<CardContent>
						<div className="flex items-center gap-2">
							<div className={`h-2 w-2 rounded-full animate-pulse ${syncingData ? 'bg-yellow-400' : 'bg-green-500'}`} />
							<span className="text-sm">{syncingData ? 'Not Synchronized' : 'Synchronized'}</span>
						</div>
					</CardContent>
				</Card>

				<Card>
					<CardHeader className="pb-3">
						<div className="flex items-center gap-2">
							<Shield className="h-4 w-4 text-muted-foreground" />
							<CardTitle className="text-base">Privacy</CardTitle>
						</div>
					</CardHeader>
					<CardContent>
						<Badge variant="outline">Disabled</Badge>
					</CardContent>
				</Card>
			</div>

			{/* Configuration and Key Info */}
			<div className="grid gap-6 md:grid-cols-2">
				{/* Key Information Card */}
				{besuNode.keyId ? (
					<Card>
						<CardHeader>
							<div className="flex items-center gap-2">
								<Key className="h-4 w-4 text-muted-foreground" />
								<CardTitle>Node Key Information</CardTitle>
							</div>
							<CardDescription>Cryptographic key details for this node</CardDescription>
						</CardHeader>
						<CardContent className="space-y-3">
							<div>
								<p className="text-sm font-medium text-muted-foreground">Key IDs</p>
								<p>
									<Link to={`/settings/keys/${besuNode.keyId}`} className="text-blue-500 hover:underline">
										{besuNode.keyId}
									</Link>
								</p>
							</div>
							{keyData?.ethereumAddress && (
								<div>
									<p className="text-sm font-medium text-muted-foreground">Ethereum Address</p>
									<div className="flex items-start gap-1">
										<p className="font-mono text-sm break-all">{keyData.ethereumAddress}</p>
										<button onClick={handleCopyAddress} className="p-1 hover:bg-muted rounded transition-colors flex-shrink-0" title="Copy address">
											{copiedAddress ? <Check className="h-4 w-4 text-green-500" /> : <Copy className="h-4 w-4 text-muted-foreground" />}
										</button>
									</div>
								</div>
							)}
							{keyData?.publicKey && (
								<div>
									<p className="text-sm font-medium text-muted-foreground">Public Key</p>
									<div className="flex items-start gap-1">
										<p className="font-mono text-sm" title={keyData.publicKey}>
											{trimPublicKey(keyData.publicKey)}
										</p>
										<button onClick={handleCopyPublicKey} className="p-1 hover:bg-muted rounded transition-colors flex-shrink-0" title="Copy public key">
											{copiedPublicKey ? <Check className="h-4 w-4 text-green-500" /> : <Copy className="h-4 w-4 text-muted-foreground" />}
										</button>
									</div>
								</div>
							)}
							{keyData?.algorithm && (
								<div>
									<p className="text-sm font-medium text-muted-foreground">Algorithm</p>
									<p className="text-sm">{keyData.algorithm}</p>
								</div>
							)}
							{keyData?.status && (
								<div>
									<p className="text-sm font-medium text-muted-foreground">Status</p>
									<Badge variant={keyData.status === 'active' ? 'default' : 'secondary'} className="text-xs">
										{keyData.status}
									</Badge>
								</div>
							)}
						</CardContent>
					</Card>
				) : (
					<BesuNodeConfig config={besuNode} />
				)}

				{/* Node Configuration & Network Card */}
				<Card>
					<CardHeader>
						<div className="flex items-center gap-2">
							<Globe className="h-4 w-4 text-muted-foreground" />
							<CardTitle>Node Configuration & Network</CardTitle>
						</div>
						<CardDescription>Network settings and connection endpoints</CardDescription>
					</CardHeader>
					<CardContent className="space-y-4">
						{/* Network Information */}
						<div>
							<p className="text-sm font-medium text-muted-foreground mb-2">Network</p>
							<div className="flex items-center gap-2">
								<div>
									<p className="text-sm font-medium">{networkData?.name || `Network ${besuNode.networkId}`}</p>
									<p className="text-xs text-muted-foreground">ID: {besuNode.networkId}</p>
								</div>
								{networkData && (
									<Link to={`/networks/${besuNode.networkId}/besu`} className="p-1 hover:bg-muted rounded transition-colors" title="View network details">
										<ExternalLink className="h-4 w-4 text-muted-foreground" />
									</Link>
								)}
							</div>
						</div>

						{/* Connection Settings */}
						<div>
							<p className="text-sm font-medium text-muted-foreground mb-2">Connection Settings</p>
							<div className="grid grid-cols-2 gap-4">
								<div>
									<p className="text-sm font-medium text-muted-foreground">P2P</p>
									<p className="text-sm">
										{besuNode.p2pHost}:{besuNode.p2pPort}
									</p>
								</div>
								<div>
									<p className="text-sm font-medium text-muted-foreground">RPC</p>
									<p className="text-sm">
										{besuNode.rpcHost}:{besuNode.rpcPort}
									</p>
								</div>
								<div>
									<p className="text-sm font-medium text-muted-foreground">External IP</p>
									<p className="text-sm">{besuNode.externalIp}</p>
								</div>
								<div>
									<p className="text-sm font-medium text-muted-foreground">Internal IP</p>
									<p className="text-sm">{besuNode.internalIp}</p>
								</div>
							</div>
						</div>

						{/* Metrics */}
						{besuNode.metricsEnabled && (
							<div>
								<p className="text-sm font-medium text-muted-foreground mb-2">Metrics</p>
								<div className="flex items-center gap-4">
									<Badge variant="outline" className="text-xs">
										Enabled
									</Badge>
									<span className="text-sm">
										{besuNode.metricsHost}:{besuNode.metricsPort}
									</span>
									{besuNode.metricsProtocol && <span className="text-xs text-muted-foreground">({besuNode.metricsProtocol})</span>}
								</div>
							</div>
						)}

						{/* Bootnodes */}
						{besuNode.bootNodes && besuNode.bootNodes.length > 0 && (
							<div>
								<p className="text-sm font-medium text-muted-foreground mb-2">Bootnodes</p>
								<div className="space-y-1">
									{besuNode.bootNodes.map((bootnode, idx) => (
										<p key={idx} className="font-mono text-xs text-muted-foreground truncate">
											{bootnode}
										</p>
									))}
								</div>
							</div>
						)}

						{/* Enode URL */}
						{besuNode.enodeUrl && (
							<div>
								<p className="text-sm font-medium text-muted-foreground mb-2">Enode URL</p>
								<p className="font-mono text-xs break-all text-muted-foreground">{besuNode.enodeUrl}</p>
							</div>
						)}
					</CardContent>
				</Card>
			</div>

			{/* Tabs */}
			<Tabs value={activeTab} onValueChange={onTabChange} className="space-y-4">
				<TabsList className="grid w-full grid-cols-4">
					<TabsTrigger value="logs">Logs</TabsTrigger>
					<TabsTrigger value="metrics">Metrics</TabsTrigger>
					<TabsTrigger value="network">Network</TabsTrigger>
					<TabsTrigger value="events">Events</TabsTrigger>
				</TabsList>

				<TabsContent value="logs" className="space-y-4">
					<Card>
						<CardHeader>
							<CardTitle>Node Logs</CardTitle>
							<CardDescription>Real-time logs from the Besu node</CardDescription>
						</CardHeader>
						<CardContent>
							<LogViewer logs={logs} onScroll={() => {}} />
						</CardContent>
					</Card>
				</TabsContent>

				<TabsContent value="metrics" className="space-y-4">
					<BesuMetricsPage node={node} />
				</TabsContent>

				<TabsContent value="network">
					{node && <BesuNodeNetwork nodeId={node.id} node={node} />}
				</TabsContent>

				<TabsContent value="events">
					<Card>
						<CardHeader>
							<CardTitle>Event History</CardTitle>
							<CardDescription>Node operations and state changes</CardDescription>
						</CardHeader>
						<CardContent>
							{/* Events content will be passed from parent */}
							{events}
						</CardContent>
					</Card>
				</TabsContent>
			</Tabs>
		</div>
	)
}

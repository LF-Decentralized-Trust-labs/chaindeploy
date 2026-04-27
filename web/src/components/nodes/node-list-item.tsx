import { HttpNodeResponse } from '@/api/client'
import { BesuIcon } from '@/components/icons/besu-icon'
import { FabricIcon } from '@/components/icons/fabric-icon'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Activity, Network } from 'lucide-react'
import { Link } from 'react-router-dom'
import { formatDistanceToNow } from 'date-fns'
import { format } from 'date-fns'

interface NodeListItemProps {
	node: HttpNodeResponse
	isSelected: boolean
	onSelectionChange: (checked: boolean) => void
	disabled?: boolean
	showCheckbox?: boolean
}

function isFabricNode(node: HttpNodeResponse): node is HttpNodeResponse & { platform: 'FABRIC' } {
	return node.platform === 'FABRIC'
}

function isFabricXNode(node: HttpNodeResponse): boolean {
	return node.platform === 'FABRICX'
}

function getStatusColor(status: string) {
	switch (status?.toLowerCase()) {
		case 'running':
			return 'default'
		case 'stopped':
		case 'error':
			return 'destructive'
		case 'starting':
		case 'stopping':
			return 'outline'
		default:
			return 'secondary'
	}
}

export function NodeListItem({ node, isSelected, onSelectionChange, disabled = false, showCheckbox = true }: NodeListItemProps) {
	// Fetch organization data if organizationId exists

	return (
		<div className="flex items-center gap-4 p-4 min-w-0">
			{showCheckbox && <Checkbox checked={isSelected} onCheckedChange={onSelectionChange} disabled={disabled || ['starting', 'stopping'].includes(node.status?.toLowerCase() || '')} />}
			<Link to={`/nodes/${node.id}`} className="flex-1 min-w-0 flex items-center justify-between hover:bg-muted/50 transition-colors rounded-lg">
				<div className="flex items-center gap-4 min-w-0">
					<div className="h-10 w-10 shrink-0 rounded-full bg-primary/10 flex items-center justify-center" aria-label={node.platform}>
						{isFabricNode(node) ? (
							<FabricIcon className="h-5 w-5 text-primary" />
						) : isFabricXNode(node) ? (
							<span className="text-xs font-semibold tracking-tight text-primary">FX</span>
						) : (
							<BesuIcon className="h-5 w-5 text-primary" />
						)}
					</div>
					<div className="min-w-0">
						<div className="flex flex-wrap items-center gap-x-2 gap-y-1">
							<h3 className="font-medium truncate">{node.name}</h3>
							{node.createdAt && (
								<span className="text-xs text-muted-foreground" title={format(new Date(node.createdAt), 'PPP p')}>
									Created {formatDistanceToNow(new Date(node.createdAt), { addSuffix: true })}
								</span>
							)}
							<Badge variant={getStatusColor(node.status || '')}>
								<Activity className="mr-1 h-3 w-3" />
								{node.status}
							</Badge>
						</div>
						<div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted-foreground">
							<span className="flex items-center gap-1">
								<Network className="h-3 w-3" />
								{node.platform}
							</span>

							{node.fabricPeer && (
								<>
									<span>•</span>
									<span>{node.fabricPeer?.mspId}</span>
									<span>•</span>
									<span>{node.nodeType}</span>
									{node.fabricPeer?.mode && (
										<>
											<span>•</span>
											<span className="capitalize">{node.fabricPeer.mode}</span>
										</>
									)}
									{node.fabricPeer?.listenAddress && (
										<>
											<span>•</span>
											<span className="font-mono text-xs">{node.fabricPeer.listenAddress}</span>
										</>
									)}
									{node.fabricPeer?.version && (
										<>
											<span>•</span>
											<span>v{node.fabricPeer.version}</span>
										</>
									)}
								</>
							)}
							{node.fabricOrderer && (
								<>
									<span>•</span>
									<span>{node.fabricOrderer?.mspId}</span>
									<span>•</span>
									<span>{node.nodeType}</span>
									{node.fabricOrderer?.mode && (
										<>
											<span>•</span>
											<span className="capitalize">{node.fabricOrderer.mode}</span>
										</>
									)}
									{node.fabricOrderer?.listenAddress && (
										<>
											<span>•</span>
											<span className="font-mono text-xs">{node.fabricOrderer.listenAddress}</span>
										</>
									)}
									{node.fabricOrderer?.version && (
										<>
											<span>•</span>
											<span>v{node.fabricOrderer.version}</span>
										</>
									)}
								</>
							)}
							{node.fabricXOrdererGroup && (
								<>
									<span>•</span>
									<span>{node.fabricXOrdererGroup.mspId}</span>
									<span>•</span>
									<span>{node.nodeType}</span>
									{node.fabricXOrdererGroup.partyId !== undefined && (
										<>
											<span>•</span>
											<span>Party {node.fabricXOrdererGroup.partyId}</span>
										</>
									)}
									{node.fabricXOrdererGroup.externalIp && node.fabricXOrdererGroup.routerPort && (
										<>
											<span>•</span>
											<span className="font-mono text-xs">
												{node.fabricXOrdererGroup.externalIp}:{node.fabricXOrdererGroup.routerPort}
											</span>
										</>
									)}
									{node.fabricXOrdererGroup.version && (
										<>
											<span>•</span>
											<span>v{node.fabricXOrdererGroup.version}</span>
										</>
									)}
								</>
							)}
							{node.fabricXCommitter && (
								<>
									<span>•</span>
									<span>{node.fabricXCommitter.mspId}</span>
									<span>•</span>
									<span>{node.nodeType}</span>
									{node.fabricXCommitter.partyId !== undefined && (
										<>
											<span>•</span>
											<span>Party {node.fabricXCommitter.partyId}</span>
										</>
									)}
									{node.fabricXCommitter.externalIp && node.fabricXCommitter.sidecarPort && (
										<>
											<span>•</span>
											<span className="font-mono text-xs">
												{node.fabricXCommitter.externalIp}:{node.fabricXCommitter.sidecarPort}
											</span>
										</>
									)}
									{node.fabricXCommitter.version && (
										<>
											<span>•</span>
											<span>v{node.fabricXCommitter.version}</span>
										</>
									)}
								</>
							)}
							{node.besuNode && (
								<>
									<span>•</span>
									<span>
										RPC:{' '}
										<span className="font-mono text-xs">
											{node.besuNode.rpcHost}:{node.besuNode.rpcPort}
										</span>
									</span>
									<span>•</span>
									<span>
										P2P:{' '}
										<span className="font-mono text-xs">
											{node.besuNode.p2pHost}:{node.besuNode.p2pPort}
										</span>
									</span>
									{node.besuNode?.mode && (
										<>
											<span>•</span>
											<span className="capitalize">{node.besuNode.mode}</span>
										</>
									)}
									{node.besuNode?.version && (
										<>
											<span>•</span>
											<span>v{node.besuNode.version}</span>
										</>
									)}
								</>
							)}
						</div>
					</div>
				</div>
			</Link>
		</div>
	)
}

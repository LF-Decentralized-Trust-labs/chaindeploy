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
		<div className="flex items-center gap-4 p-4">
			{showCheckbox && <Checkbox checked={isSelected} onCheckedChange={onSelectionChange} disabled={disabled || ['starting', 'stopping'].includes(node.status?.toLowerCase() || '')} />}
			<Link to={`/nodes/${node.id}`} className="flex-1 flex items-center justify-between hover:bg-muted/50 transition-colors rounded-lg">
				<div className="flex items-center gap-4">
					<div className="h-10 w-10 rounded-full bg-primary/10 flex items-center justify-center">
						{isFabricNode(node) ? <FabricIcon className="h-5 w-5 text-primary" /> : <BesuIcon className="h-5 w-5 text-primary" />}
					</div>
					<div>
						<div className="flex items-center gap-2">
							<h3 className="font-medium">{node.name}</h3>
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
						<div className="flex items-center gap-2 text-sm text-muted-foreground">
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

import { HttpNodeResponse } from '@/api/client'
import { BesuIcon } from '@/components/icons/besu-icon'
import { FabricIcon } from '@/components/icons/fabric-icon'
import { Card } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Link } from 'react-router-dom'
import { Server, ArrowUpRight, Circle } from 'lucide-react'
import { cn } from '@/lib/utils'

interface NodesListProps {
	nodes?: HttpNodeResponse[]
	limit?: number
}

function getStatusColor(status?: string) {
	switch (status?.toLowerCase()) {
		case 'running':
			return 'text-green-600 dark:text-green-400'
		case 'stopped':
			return 'text-gray-600 dark:text-gray-400'
		case 'error':
			return 'text-red-600 dark:text-red-400'
		case 'starting':
		case 'stopping':
			return 'text-yellow-600 dark:text-yellow-400'
		default:
			return 'text-gray-600 dark:text-gray-400'
	}
}

function getStatusBadgeVariant(status?: string): "default" | "secondary" | "destructive" | "outline" {
	switch (status?.toLowerCase()) {
		case 'running':
			return 'default'
		case 'stopped':
			return 'secondary'
		case 'error':
			return 'destructive'
		default:
			return 'outline'
	}
}

export default function NodesList({ nodes = [], limit }: NodesListProps) {
	const displayNodes = limit ? nodes.slice(0, limit) : nodes

	if (displayNodes.length === 0) {
		return (
			<div className="text-center py-8 border-2 border-dashed rounded-lg">
				<Server className="h-8 w-8 text-muted-foreground mx-auto mb-2" />
				<p className="text-sm text-muted-foreground">No nodes created yet</p>
			</div>
		)
	}

	return (
		<div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
			{displayNodes.map((node) => (
				<Link
					key={node.id}
					to={`/nodes/${node.id}`}
					className="block"
				>
					<Card className="p-4 hover:shadow-md transition-shadow cursor-pointer group">
						<div className="flex items-start justify-between mb-2">
							<div className="flex items-center gap-2 flex-1 min-w-0">
								{node.platform?.toUpperCase() === 'FABRIC' ? (
									<FabricIcon className="h-5 w-5 shrink-0" />
								) : (
									<BesuIcon className="h-5 w-5 shrink-0" />
								)}
								<h3 className="font-medium truncate">{node.name}</h3>
							</div>
							<ArrowUpRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity shrink-0" />
						</div>
						<div className="flex items-center justify-between gap-2">
							<div className="flex items-center gap-2">
								<Circle className={cn("h-2 w-2 fill-current", getStatusColor(node.status))} />
								<Badge variant={getStatusBadgeVariant(node.status)} className="text-xs">
									{node.status}
								</Badge>
							</div>
							<Badge variant="outline" className="text-xs">
								{node.nodeType}
							</Badge>
						</div>
						{node.endpoint && (
							<p className="text-xs text-muted-foreground mt-2 font-mono">
								{node.endpoint}
							</p>
						)}
					</Card>
				</Link>
			))}
		</div>
	)
}

import { HttpNetworkResponse } from '@/api/client'
import { BesuIcon } from '@/components/icons/besu-icon'
import { FabricIcon } from '@/components/icons/fabric-icon'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { TimeAgo } from '@/components/ui/time-ago'
import { Badge } from '@/components/ui/badge'
import { Link } from 'react-router-dom'
import { Network, ArrowUpRight } from 'lucide-react'

interface NetworksListProps {
	fabricNetworks?: HttpNetworkResponse[]
	besuNetworks?: HttpNetworkResponse[]
	fabricxNetworks?: HttpNetworkResponse[]
	limit?: number
}

function detailPath(network: HttpNetworkResponse): string {
	switch (network.platform) {
		case 'fabricx':
			return `/networks/${network.id}/fabricx`
		case 'besu':
			return `/networks/${network.id}/besu`
		default:
			return `/networks/${network.id}/fabric`
	}
}

function PlatformIcon({ platform }: { platform?: string }) {
	if (platform === 'besu') return <BesuIcon className="h-5 w-5" />
	if (platform === 'fabricx') return <FabricIcon className="h-5 w-5 text-cyan-600 dark:text-cyan-400" />
	return <FabricIcon className="h-5 w-5" />
}

export default function NetworksList({ fabricNetworks = [], besuNetworks = [], fabricxNetworks = [], limit }: NetworksListProps) {
	const allNetworks = [...fabricNetworks, ...besuNetworks, ...fabricxNetworks]
		.sort((a, b) => new Date(b.createdAt!).getTime() - new Date(a.createdAt!).getTime())
		.slice(0, limit)

	if (allNetworks.length === 0) {
		return (
			<div className="text-center py-8 border-2 border-dashed rounded-lg">
				<Network className="h-8 w-8 text-muted-foreground mx-auto mb-2" />
				<p className="text-sm text-muted-foreground mb-3">No networks created yet</p>
				<div className="flex flex-wrap gap-2 justify-center">
					<Button variant="outline" size="sm" asChild>
						<Link to="/networks/fabric/create">Create Fabric Network</Link>
					</Button>
					<Button variant="outline" size="sm" asChild>
						<Link to="/networks/besu/create">Create Besu Network</Link>
					</Button>
					<Button variant="outline" size="sm" asChild>
						<Link to="/networks/fabricx/create">Create FabricX Network</Link>
					</Button>
				</div>
			</div>
		)
	}

	return (
		<div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
			{allNetworks.map((network) => (
				<Link
					key={`${network.platform}-${network.id}`}
					to={detailPath(network)}
					className="block"
				>
					<Card className="p-4 hover:shadow-md transition-shadow cursor-pointer group">
						<div className="flex items-start justify-between mb-2">
							<div className="flex items-center gap-2">
								<PlatformIcon platform={network.platform} />
								<h3 className="font-medium truncate">{network.name}</h3>
							</div>
							<ArrowUpRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity" />
						</div>
						<div className="flex items-center justify-between">
							<Badge variant="secondary" className="text-xs">
								{network.platform}
							</Badge>
							<p className="text-xs text-muted-foreground">
								<TimeAgo date={network.createdAt!} />
							</p>
						</div>
					</Card>
				</Link>
			))}
		</div>
	)
}

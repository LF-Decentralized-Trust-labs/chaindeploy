import { getNodesByIdChannelsOptions } from '@/api/client/@tanstack/react-query.gen'
import { HttpChannelResponse } from '@/api/client/types.gen'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useQuery } from '@tanstack/react-query'

interface FabricNodeChannelsProps {
	nodeId: number
}

export function FabricNodeChannels({ nodeId }: FabricNodeChannelsProps) {
	const {
		data: channels,
		isLoading,
		error,
	} = useQuery({
		...getNodesByIdChannelsOptions({
			path: { id: nodeId },
		}),
	})
	if (isLoading) {
		return <div>Loading channels...</div>
	}

	if (error) {
		return <div>Error loading channels: {(error as any).error.message}</div>
	}

	if (!channels?.channels?.length) {
		return (
			<Card>
				<CardHeader>
					<CardTitle>Channels</CardTitle>
					<CardDescription>No channels found for this node</CardDescription>
				</CardHeader>
			</Card>
		)
	}

	return (
		<Card>
			<CardHeader>
				<CardTitle>Channels</CardTitle>
				<CardDescription>List of channels this node is part of</CardDescription>
			</CardHeader>
			<CardContent>
				<div className="space-y-4">
					{channels.channels.map((channel: HttpChannelResponse) => (
						<div key={channel.name} className="flex items-center justify-between rounded-lg border p-4">
							<div className="space-y-1">
								<h3 className="font-medium">{channel.name}</h3>
								<p className="text-sm text-muted-foreground">Block Number: {channel.blockNum || 'N/A'}</p>
								<p className="text-sm text-muted-foreground">Created: {channel.createdAt ? new Date(channel.createdAt).toLocaleString() : 'N/A'}</p>
							</div>
						</div>
					))}
				</div>
			</CardContent>
		</Card>
	)
}

import { getNetworksFabricByIdBlocksOptions } from '@/api/client/@tanstack/react-query.gen'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { useQuery } from '@tanstack/react-query'
import { formatDistanceToNow } from 'date-fns'
import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'

interface BlockExplorerProps {
	networkId: number
}

export function BlockExplorer({ networkId }: BlockExplorerProps) {
	const limit = 3
	const [offset, setOffset] = useState(0)
	const { data: blocksResponse, isLoading: blocksLoading, isFetching } = useQuery({
		...getNetworksFabricByIdBlocksOptions({
			path: { id: networkId },
			query: {
				limit,
				offset,
				reverse: true,
			},
		}),
	})
	// Sort blocks in descending order by block number
	const sortedBlocks = useMemo(
		() => [...(blocksResponse?.blocks || [])].sort((a, b) => (b.number ?? 0) - (a.number ?? 0)),
		[blocksResponse?.blocks]
	)
	const hasBlockZero = sortedBlocks.some((block) => block.number === 0)
	const isFirstPage = offset === 0
	const isLastPage = hasBlockZero || (sortedBlocks.length < limit)

	if (blocksLoading && !blocksResponse) {
		return (
			<div className="space-y-4">
				<Skeleton className="h-8 w-32" />
				<div className="space-y-2">
					{[1, 2, 3].map((i) => (
						<Skeleton key={i} className="h-20 w-full" />
					))}
				</div>
			</div>
		)
	}

	return (
		<div className="space-y-4">
			<div className="flex items-center justify-between">
				<h3 className="text-lg font-medium">Recent Blocks</h3>
				<Button variant="outline" size="sm" asChild>
					<Link to={`/networks/${networkId}/blocks`}>View All Blocks</Link>
				</Button>
			</div>
			<div className="space-y-2">
				{sortedBlocks.map((block) => (
					<Card key={block.number} className="p-4">
						<div className="flex items-center justify-between">
							<div>
								<p className="font-medium">Block #{block.number}</p>
								<p className="text-sm text-muted-foreground">
									{block.transactions?.length} {block.transactions?.length === 1 ? 'transaction' : 'transactions'} • {formatDistanceToNow(new Date(block.createdAt || ''), { addSuffix: true })}
								</p>
							</div>
							<Button variant="ghost" size="sm" asChild>
								<Link to={`/networks/${networkId}/blocks/${block.number}`}>View Details</Link>
							</Button>
						</div>
					</Card>
				))}
			</div>
			<div className="flex justify-end gap-2 pt-2">
				<Button
					variant="outline"
					size="sm"
					onClick={() => setOffset((o) => Math.max(0, o - limit))}
					disabled={isFirstPage || isFetching}
				>
					Previous
				</Button>
				<Button
					variant="outline"
					size="sm"
					onClick={() => setOffset((o) => o + limit)}
					disabled={isLastPage || isFetching}
				>
					Next
				</Button>
			</div>
		</div>
	)
}

import { getNetworksFabricByIdBlocksOptions, getNetworksFabricByIdInfoOptions, getNetworksFabricByIdOptions } from '@/api/client/@tanstack/react-query.gen'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { useQuery } from '@tanstack/react-query'
import { formatDistanceToNow } from 'date-fns'
import { Link, useParams } from 'react-router-dom'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useMemo, useState } from 'react'

export function BlocksOverview() {
	const { id } = useParams<{ id: string }>()
	const networkId = parseInt(id || '0')

	const { data: chainInfo, isLoading: chainLoading } = useQuery({
		...getNetworksFabricByIdInfoOptions({
			path: { id: networkId },
		}),
	})
	const { data: networkInfo, isLoading: networkLoading } = useQuery({
		...getNetworksFabricByIdOptions({
			path: { id: networkId },
		}),
	})

	const limit = 10
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
	const blocks = useMemo(() => blocksResponse?.blocks?.sort((a, b) => (b.number || 0) - (a.number || 0)) || [], [blocksResponse?.blocks])
	const hasBlockZero = blocks.some((block) => block.number === 0)
	const isFirstPage = offset === 0
	const isLastPage = hasBlockZero || (blocks.length < limit)
	const lastBlock = useMemo(() => blocks[0], [blocks])
	const transactions = useMemo(() => lastBlock?.transactions || [], [lastBlock])

	if (chainLoading || networkLoading || (blocksLoading && !blocksResponse)) {
		return (
			<div className="space-y-6 p-4">
				<div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
					{[1, 2, 3].map((i) => (
						<Card key={i}>
							<CardHeader>
								<CardTitle><Skeleton className="h-5 w-32" /></CardTitle>
							</CardHeader>
							<CardContent>
								<Skeleton className="h-8 w-24 mb-1" />
								<Skeleton className="h-4 w-48" />
							</CardContent>
						</Card>
					))}
				</div>

				<Card>
					<CardHeader>
						<CardTitle>Recent Blocks</CardTitle>
					</CardHeader>
					<CardContent>
						<Table>
							<TableHeader>
								<TableRow>
									<TableHead>Block</TableHead>
									<TableHead>Hash</TableHead>
									<TableHead>Time</TableHead>
									<TableHead>Transactions</TableHead>
									<TableHead className="text-right">Actions</TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{[1, 2, 3, 4, 5].map((i) => (
									<TableRow key={i}>
										<TableCell><Skeleton className="h-4 w-16" /></TableCell>
										<TableCell><Skeleton className="h-4 w-48" /></TableCell>
										<TableCell><Skeleton className="h-4 w-24" /></TableCell>
										<TableCell><Skeleton className="h-4 w-8" /></TableCell>
										<TableCell className="text-right"><Skeleton className="h-8 w-24 ml-auto" /></TableCell>
									</TableRow>
								))}
							</TableBody>
						</Table>
					</CardContent>
				</Card>
			</div>
		)
	}

	return (
		<div className="space-y-6 p-4">
			<div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
				<Card>
					<CardHeader>
						<CardTitle>Latest Block</CardTitle>
					</CardHeader>
					<CardContent>
						<p className="text-2xl font-bold">#{chainInfo?.height}</p>
						<p className="text-sm text-muted-foreground">Latest block height</p>
					</CardContent>
				</Card>
				<Card>
					<CardHeader>
						<CardTitle>Chain ID</CardTitle>
					</CardHeader>
					<CardContent>
						<p className="text-2xl font-bold">{networkInfo?.name}</p>
						<p className="text-sm text-muted-foreground">Network Identifier</p>
					</CardContent>
				</Card>
				<Card>
					<CardHeader>
						<CardTitle>Total Transactions</CardTitle>
					</CardHeader>
					<CardContent>
						<p className="text-2xl font-bold">{transactions.length}</p>
						<p className="text-sm text-muted-foreground">In Latest Block</p>
					</CardContent>
				</Card>
			</div>

			<Card>
				<CardHeader>
					<CardTitle>Recent Blocks</CardTitle>
				</CardHeader>
				<CardContent>
					<Table>
						<TableHeader>
							<TableRow>
								<TableHead>Block</TableHead>
								<TableHead>Hash</TableHead>
								<TableHead>Time</TableHead>
								<TableHead>Transactions</TableHead>
								<TableHead className="text-right">Actions</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{blocks.map((block) => (
								<TableRow key={block.number}>
									<TableCell className="font-medium">#{block.number}</TableCell>
									<TableCell className="font-mono text-sm">
										{block.dataHash && (
											<>
												{block.dataHash.substring(0, 10)}...{block.dataHash.substring(block.dataHash.length - 10)}
											</>
										)}
									</TableCell>
									<TableCell>{formatDistanceToNow(new Date(block.createdAt || ''), { addSuffix: true })}</TableCell>
									<TableCell>{block.transactions?.length || 0}</TableCell>
									<TableCell className="text-right">
										<Button variant="ghost" size="sm" asChild>
											<Link to={`/networks/${id}/blocks/${block.number}`}>View Details</Link>
										</Button>
									</TableCell>
								</TableRow>
							))}
						</TableBody>
					</Table>
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
				</CardContent>
			</Card>
		</div>
	)
}

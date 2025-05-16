import { getNetworksFabricByIdBlocksByBlockNumOptions } from '@/api/client/@tanstack/react-query.gen'
import { BlockTransaction } from '@/api/client/types.gen'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@radix-ui/react-dropdown-menu'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, EllipsisVertical } from 'lucide-react'
import { Fragment, useMemo } from 'react'
import { Link, useParams } from 'react-router-dom'

export function BlockDetails() {
	const { id, blockNumber } = useParams<{ id: string; blockNumber: string }>()
	const networkId = parseInt(id || '0')
	const blockNum = parseInt(blockNumber || '0')

	const { data: blockResponse, isLoading } = useQuery({
		...getNetworksFabricByIdBlocksByBlockNumOptions({
			path: { id: networkId, blockNum },
		}),
	})

	const transactions = useMemo(() => blockResponse?.block?.transactions || [], [blockResponse?.block?.transactions])
	if (isLoading) {
		return (
			<div className="space-y-6 p-4">
				<div className="flex items-center justify-between">
					<div className="space-y-1">
						<Skeleton className="h-8 w-32" />
					</div>
					<Skeleton className="h-10 w-32" />
				</div>

				<div className="grid gap-6">
					<Card>
						<CardHeader>
							<CardTitle>Block Information</CardTitle>
						</CardHeader>
						<CardContent>
							<dl className="grid gap-4 sm:grid-cols-2">
								<div>
									<dt className="text-sm font-medium text-muted-foreground">Block Hash</dt>
									<dd className="mt-1">
										<Skeleton className="h-5 w-full" />
									</dd>
								</div>
								<div>
									<dt className="text-sm font-medium text-muted-foreground">Previous Block Hash</dt>
									<dd className="mt-1">
										<Skeleton className="h-5 w-full" />
									</dd>
								</div>
								<div>
									<dt className="text-sm font-medium text-muted-foreground">Timestamp</dt>
									<dd className="mt-1">
										<Skeleton className="h-5 w-32" />
									</dd>
								</div>
								<div>
									<dt className="text-sm font-medium text-muted-foreground">Transactions</dt>
									<dd className="mt-1">
										<Skeleton className="h-5 w-16" />
									</dd>
								</div>
							</dl>
						</CardContent>
					</Card>
				</div>

				<Card>
					<CardHeader>
						<CardTitle>Transactions</CardTitle>
					</CardHeader>
					<CardContent>
						<div className="space-y-4">
							{[1, 2].map((i) => (
								<div key={i} className="flex flex-col p-4 border rounded-lg">
									<div className="flex items-center justify-between mb-4">
										<div className="space-y-1">
											<p className="text-sm font-medium">Transaction ID</p>
											<Skeleton className="h-5 w-64" />
										</div>
										<Skeleton className="h-8 w-8" />
									</div>

									<div className="grid gap-4 md:grid-cols-2">
										<div>
											<p className="text-sm font-medium mb-2">Basic Information</p>
											<dl className="grid grid-cols-2 gap-2 text-sm">
												{['Type', 'Channel ID', 'Chaincode ID', 'Created At'].map((label) => (
													<Fragment key={label}>
														<dt className="text-muted-foreground">{label}</dt>
														<dd>
															<Skeleton className="h-4 w-24" />
														</dd>
													</Fragment>
												))}
											</dl>
										</div>

										<div>
											<p className="text-sm font-medium mb-2">Event</p>
											<dl className="grid grid-cols-2 gap-2 text-sm">
												{['Name', 'Value'].map((label) => (
													<Fragment key={label}>
														<dt className="text-muted-foreground">{label}</dt>
														<dd>
															<Skeleton className="h-4 w-24" />
														</dd>
													</Fragment>
												))}
											</dl>
										</div>

										<div className="md:col-span-2">
											<p className="text-sm font-medium mb-2">Read Set</p>
											<div className="overflow-x-auto">
												<table className="min-w-full text-sm">
													<thead>
														<tr className="border-b">
															<th className="text-left py-2 px-4">Key</th>
															<th className="text-left py-2 px-4">Chaincode ID</th>
															<th className="text-left py-2 px-4">Block Version</th>
															<th className="text-left py-2 px-4">Tx Version</th>
														</tr>
													</thead>
													<tbody>
														{[1, 2].map((row) => (
															<tr key={row} className="border-b">
																{[1, 2, 3, 4].map((col) => (
																	<td key={col} className="py-2 px-4">
																		<Skeleton className="h-4 w-24" />
																	</td>
																))}
															</tr>
														))}
													</tbody>
												</table>
											</div>
										</div>

										<div className="md:col-span-2">
											<p className="text-sm font-medium mb-2">Write Set</p>
											<div className="overflow-x-auto">
												<table className="min-w-full text-sm">
													<thead>
														<tr className="border-b">
															<th className="text-left py-2 px-4">Key</th>
															<th className="text-left py-2 px-4">Chaincode ID</th>
															<th className="text-left py-2 px-4">Value</th>
															<th className="text-left py-2 px-4">Is Deleted</th>
														</tr>
													</thead>
													<tbody>
														{[1, 2].map((row) => (
															<tr key={row} className="border-b">
																{[1, 2, 3, 4].map((col) => (
																	<td key={col} className="py-2 px-4">
																		<Skeleton className="h-4 w-24" />
																	</td>
																))}
															</tr>
														))}
													</tbody>
												</table>
											</div>
										</div>
									</div>
								</div>
							))}
						</div>
					</CardContent>
				</Card>
			</div>
		)
	}

	if (!transactions) {
		return (
			<div className="flex flex-col items-center justify-center space-y-4 py-12 p-4">
				<h2 className="text-2xl font-bold">Block Not Found</h2>
				<p className="text-muted-foreground">The requested block could not be found.</p>
				<Button asChild>
					<Link to={`/networks/${id}/blocks`}>
						<ArrowLeft className="mr-2 h-4 w-4" />
						Back to Blocks
					</Link>
				</Button>
			</div>
		)
	}

	return (
		<div className="space-y-6 p-4">
			<div className="flex items-center justify-between">
				<div className="space-y-1">
					<h2 className="text-2xl font-bold">Block #{blockNum}</h2>
				</div>
				<Button variant="outline" asChild>
					<Link to={`/networks/${id}/blocks`}>
						<ArrowLeft className="mr-2 h-4 w-4" />
						Back to Blocks
					</Link>
				</Button>
			</div>

			<div className="grid gap-6">
				<Card>
					<CardHeader>
						<CardTitle>Block Information</CardTitle>
					</CardHeader>
					<CardContent>
						<dl className="grid gap-4 sm:grid-cols-2">
							<div>
								<dt className="text-sm font-medium text-muted-foreground">Block Hash</dt>
								<dd className="mt-1 font-mono text-sm break-all">{blockResponse?.block?.dataHash}</dd>
							</div>
							<div>
								<dt className="text-sm font-medium text-muted-foreground">Previous Block Hash</dt>
								<dd className="mt-1 font-mono text-sm break-all">{blockResponse?.block?.dataHash}</dd>
							</div>
							<div>
								<dt className="text-sm font-medium text-muted-foreground">Timestamp</dt>
								<dd className="mt-1 text-sm">{new Date(blockResponse?.block?.createdAt || '').toLocaleString()}</dd>
							</div>
							<div>
								<dt className="text-sm font-medium text-muted-foreground">Transactions</dt>
								<dd className="mt-1 text-sm">{blockResponse?.block?.transactions?.length}</dd>
							</div>
						</dl>
					</CardContent>
				</Card>
			</div>
			<Card>
				<CardHeader>
					<CardTitle>Transactions</CardTitle>
				</CardHeader>
				<CardContent>
					{transactions && transactions.length > 0 ? (
						<div className="space-y-4">
							{transactions.map((tx: BlockTransaction, index: number) => (
								<div key={tx.id || index} className="flex flex-col p-4 border rounded-lg">
									<div className="flex items-center justify-between mb-4">
										<div className="space-y-1">
											<p className="text-sm font-medium">Transaction ID</p>
											<p className="font-mono text-sm text-muted-foreground break-all">{tx.id}</p>
										</div>
										<DropdownMenu>
											<DropdownMenuTrigger asChild>
												<Button variant="ghost" size="icon">
													<EllipsisVertical className="h-4 w-4" />
												</Button>
											</DropdownMenuTrigger>
											<DropdownMenuContent align="end">
												<DropdownMenuItem>View Details</DropdownMenuItem>
												<DropdownMenuItem>Copy ID</DropdownMenuItem>
											</DropdownMenuContent>
										</DropdownMenu>
									</div>

									<div className="grid gap-4 md:grid-cols-2">
										<div>
											<p className="text-sm font-medium mb-2">Basic Information</p>
											<dl className="grid grid-cols-2 gap-2 text-sm">
												<dt className="text-muted-foreground">Type</dt>
												<dd>{tx.type}</dd>
												<dt className="text-muted-foreground">Channel ID</dt>
												<dd>{tx.channelId}</dd>
												<dt className="text-muted-foreground">Chaincode ID</dt>
												<dd>{tx.chaincodeId}</dd>
												<dt className="text-muted-foreground">Created At</dt>
												<dd>{new Date(tx.createdAt || '').toLocaleString()}</dd>
											</dl>
										</div>

										{tx.event && tx.event.name && tx.event.value && (
											<div>
												<p className="text-sm font-medium mb-2">Event</p>
												<dl className="grid grid-cols-2 gap-2 text-sm">
													<dt className="text-muted-foreground">Name</dt>
													<dd>{tx.event.name}</dd>
													<dt className="text-muted-foreground">Value</dt>
													<dd className="break-all">{tx.event.value}</dd>
												</dl>
											</div>
										)}

										{tx.reads && tx.reads.length > 0 && (
											<div className="md:col-span-2">
												<p className="text-sm font-medium mb-2">Read Set</p>
												<div className="overflow-x-auto">
													<table className="min-w-full text-sm">
														<thead>
															<tr className="border-b">
																<th className="text-left py-2 px-4">Key</th>
																<th className="text-left py-2 px-4">Chaincode ID</th>
																<th className="text-left py-2 px-4">Block Version</th>
																<th className="text-left py-2 px-4">Tx Version</th>
															</tr>
														</thead>
														<tbody>
															{tx.reads.map((read, i) => (
																<tr key={i} className="border-b">
																	<td className="py-2 px-4 font-mono">{read.key}</td>
																	<td className="py-2 px-4">{read.chaincodeId}</td>
																	<td className="py-2 px-4">{read.blockNumVersion}</td>
																	<td className="py-2 px-4">{read.txNumVersion}</td>
																</tr>
															))}
														</tbody>
													</table>
												</div>
											</div>
										)}

										{tx.writes && tx.writes.length > 0 && (
											<div className="md:col-span-2">
												<p className="text-sm font-medium mb-2">Write Set</p>
												<div className="overflow-x-auto">
													<table className="min-w-full text-sm">
														<thead>
															<tr className="border-b">
																<th className="text-left py-2 px-4">Key</th>
																<th className="text-left py-2 px-4">Chaincode ID</th>
																<th className="text-left py-2 px-4">Value</th>
																<th className="text-left py-2 px-4">Is Deleted</th>
															</tr>
														</thead>
														<tbody>
															{tx.writes.map((write, i) => (
																<tr key={i} className="border-b">
																	<td className="py-2 px-4 font-mono">{write.key}</td>
																	<td className="py-2 px-4">{write.chaincodeId}</td>
																	<td className="py-2 px-4 break-all font-mono">{write.value}</td>
																	<td className="py-2 px-4">{write.deleted ? 'Yes' : 'No'}</td>
																</tr>
															))}
														</tbody>
													</table>
												</div>
											</div>
										)}
									</div>
								</div>
							))}
						</div>
					) : (
						<div className="text-center py-6">
							<p className="text-sm text-muted-foreground">No transactions found in this block</p>
						</div>
					)}
				</CardContent>
			</Card>
		</div>
	)
}

import { getNetworksFabricxByIdBlocksByBlockNumOptions } from '@/api/client/@tanstack/react-query.gen'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, Copy, Hash, Package } from 'lucide-react'
import { Link, useParams } from 'react-router-dom'
import { toast } from 'sonner'

export default function FabricXBlockDetailPage() {
	const { id: idParam, blockNumber: blockParam } = useParams<{ id: string; blockNumber: string }>()
	const id = Number(idParam)
	const blockNum = Number(blockParam)

	const { data: block, isLoading } = useQuery({
		...getNetworksFabricxByIdBlocksByBlockNumOptions({ path: { id, blockNum } } as any),
		enabled: Number.isFinite(id) && Number.isFinite(blockNum),
	})

	const copy = (text: string) => {
		if (!text) return
		navigator.clipboard.writeText(text)
		toast.success('Copied to clipboard')
	}

	const formatHash = (hash?: string) => {
		if (!hash) return '—'
		if (hash.length <= 14) return hash
		return `${hash.slice(0, 8)}…${hash.slice(-6)}`
	}

	const txs: any[] = Array.isArray((block as any)?.transactions) ? (block as any).transactions : []

	return (
		<div className="flex-1 p-8">
			<div className="max-w-5xl mx-auto">
				<Button variant="ghost" size="sm" asChild className="mb-4">
					<Link to={`/networks/${id}/fabricx`}>
						<ArrowLeft className="mr-2 h-4 w-4" />
						Back to Network
					</Link>
				</Button>

				<div className="mb-6 flex items-center justify-between">
					<div>
						<h1 className="text-2xl font-semibold">Block #{blockParam}</h1>
						<p className="text-muted-foreground">FabricX channel block details</p>
					</div>
					<Badge variant="outline">Network #{id}</Badge>
				</div>

				{isLoading ? (
					<Card>
						<CardContent className="p-6 space-y-3">
							<Skeleton className="h-6 w-48" />
							<Skeleton className="h-4 w-full" />
							<Skeleton className="h-4 w-5/6" />
						</CardContent>
					</Card>
				) : !block ? (
					<Card>
						<CardContent className="p-6">
							<p className="text-sm text-muted-foreground">Block not found.</p>
						</CardContent>
					</Card>
				) : (
					<div className="space-y-6">
						<Card>
							<CardHeader>
								<CardTitle className="text-base">Block overview</CardTitle>
								<CardDescription>Header fields from the committer sidecar</CardDescription>
							</CardHeader>
							<CardContent>
								<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
									<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
										<div className="flex items-center gap-3">
											<Hash className="h-4 w-4 text-muted-foreground" />
											<div>
												<p className="text-sm font-medium">Number</p>
												<p className="text-sm text-muted-foreground font-mono">{(block as any).number ?? blockParam}</p>
											</div>
										</div>
									</div>
									<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
										<div className="flex items-center gap-3">
											<Package className="h-4 w-4 text-muted-foreground" />
											<div>
												<p className="text-sm font-medium">Tx count</p>
												<p className="text-sm text-muted-foreground">{(block as any).txCount ?? txs.length}</p>
											</div>
										</div>
									</div>
									<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
										<div className="flex items-center gap-3">
											<Hash className="h-4 w-4 text-muted-foreground" />
											<div>
												<p className="text-sm font-medium">Data hash</p>
												<p className="text-sm text-muted-foreground font-mono">{formatHash((block as any).dataHash)}</p>
											</div>
										</div>
										<Button variant="ghost" size="sm" className="h-8 w-8 p-0" onClick={() => copy((block as any).dataHash)}>
											<Copy className="h-4 w-4" />
										</Button>
									</div>
									<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
										<div className="flex items-center gap-3">
											<Hash className="h-4 w-4 text-muted-foreground" />
											<div>
												<p className="text-sm font-medium">Previous hash</p>
												<p className="text-sm text-muted-foreground font-mono">{formatHash((block as any).previousHash)}</p>
											</div>
										</div>
										<Button variant="ghost" size="sm" className="h-8 w-8 p-0" onClick={() => copy((block as any).previousHash)}>
											<Copy className="h-4 w-4" />
										</Button>
									</div>
								</div>
							</CardContent>
						</Card>

						<Card>
							<CardHeader>
								<CardTitle className="text-base">Transactions</CardTitle>
								<CardDescription>{txs.length} transaction{txs.length === 1 ? '' : 's'} in this block</CardDescription>
							</CardHeader>
							<CardContent>
								{txs.length === 0 ? (
									<p className="text-sm text-muted-foreground">No transactions.</p>
								) : (
									<Table>
										<TableHeader>
											<TableRow>
												<TableHead className="w-16">#</TableHead>
												<TableHead>Tx ID</TableHead>
												<TableHead className="w-28">Type</TableHead>
												<TableHead>Timestamp</TableHead>
											</TableRow>
										</TableHeader>
										<TableBody>
											{txs.map((tx: any) => (
												<TableRow
													key={`${tx.index}-${tx.txId}`}
													className={tx.txId ? 'cursor-pointer hover:bg-muted/50' : ''}
													onClick={() => {
														if (tx.txId) window.location.assign(`/networks/${id}/fabricx/transactions/${tx.txId}`)
													}}
												>
													<TableCell className="font-mono">{tx.index}</TableCell>
													<TableCell className="font-mono text-xs">{tx.txId || '—'}</TableCell>
													<TableCell>
														<Badge variant="outline">{tx.type || '—'}</Badge>
													</TableCell>
													<TableCell className="text-sm text-muted-foreground">
														{tx.timestamp ? new Date(tx.timestamp).toLocaleString() : '—'}
													</TableCell>
												</TableRow>
											))}
										</TableBody>
									</Table>
								)}
							</CardContent>
						</Card>

						<Card>
							<CardHeader>
								<CardTitle className="text-base">Raw block JSON</CardTitle>
								<CardDescription>Decoded block payload</CardDescription>
							</CardHeader>
							<CardContent>
								<div className="bg-muted/50 p-4 rounded-md overflow-auto max-h-96">
									<pre className="text-xs font-mono">
										<code>{JSON.stringify(block, null, 2)}</code>
									</pre>
								</div>
							</CardContent>
						</Card>
					</div>
				)}
			</div>
		</div>
	)
}

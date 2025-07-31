import { Activity, Copy, Hash, Package, TrendingUp, ArrowLeft, ExternalLink, Clock, Zap } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { useParams, Link } from 'react-router-dom'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { toast } from 'sonner'
import {
	getNodesByIdRpcBlockByNumberOptions,
	getNodesByIdRpcBlockTransactionCountByNumberOptions,
	getNodesByIdRpcTransactionByBlockNumberAndIndexOptions,
} from '@/api/client/@tanstack/react-query.gen'

export default function BesuBlockDetailsPage() {
	const { nodeId, blockNumber } = useParams<{ nodeId: string; blockNumber: string }>()
	const nodeIdNumber = nodeId ? parseInt(nodeId) : 0
	const blockNumberInt = blockNumber ? parseInt(blockNumber) : 0

	// Get block details
	const { data: block, isLoading: blockLoading } = useQuery({
		...getNodesByIdRpcBlockByNumberOptions({
			path: { id: nodeIdNumber },
			query: { number: blockNumberInt.toString(), tag: blockNumberInt.toString() },
		}),
		enabled: !!nodeIdNumber && !!blockNumberInt,
	})

	// Get transaction count for the block
	const { data: transactionCount, isLoading: transactionCountLoading } = useQuery({
		...getNodesByIdRpcBlockTransactionCountByNumberOptions({
			path: { id: nodeIdNumber },
			query: { number: blockNumberInt.toString(), tag: blockNumberInt.toString() },
		}),
		enabled: !!nodeIdNumber && !!blockNumberInt,
	})

	// Get transactions for the block (we'll fetch them one by one for now)
	const { data: transactions, isLoading: transactionsLoading } = useQuery({
		...getNodesByIdRpcTransactionByBlockNumberAndIndexOptions({
			path: { id: nodeIdNumber },
			query: { number: blockNumberInt.toString(), tag: blockNumberInt.toString(), index: '0x0' },
		}),
		enabled: !!nodeIdNumber && !!blockNumberInt && !!transactionCount && parseInt(String(transactionCount)) > 0,
	})

	const copyToClipboard = (text: string) => {
		navigator.clipboard.writeText(text)
		toast.success('Copied to clipboard')
	}

	// Format block hash for better readability
	const formatHash = (hash: string) => {
		if (hash.length <= 10) return hash
		return `${hash.slice(0, 6)}...${hash.slice(-4)}`
	}

	// Format block number to hex
	const formatBlockNumberToHex = (number: number) => {
		return `0x${number.toString(16)}`
	}

	// Format timestamp
	const formatTimestamp = (timestamp: string) => {
		if (!timestamp) return 'N/A'
		const date = new Date(parseInt(timestamp) * 1000)
		return date.toLocaleString()
	}

	// Generate array of transaction indices for the block
	const getTransactionIndices = () => {
		if (!transactionCount) return []
		const count = parseInt(String(transactionCount))
		const indices = []
		for (let i = 0; i < count; i++) {
			indices.push(i)
		}
		return indices
	}

	const transactionIndices = getTransactionIndices()

	return (
		<div className="flex-1 p-8">
			<div className="max-w-7xl mx-auto">
				<div className="flex items-center gap-2 text-muted-foreground mb-8">
					<Button variant="ghost" size="sm" asChild>
						<Link to={`/networks/besu/${nodeId}/blocks`}>
							<ArrowLeft className="mr-2 h-4 w-4" />
							Back to Blocks
						</Link>
					</Button>
				</div>

				<div className="mb-6">
					<div className="flex items-center justify-between">
						<div>
							<h1 className="text-2xl font-semibold">Block #{blockNumber}</h1>
							<p className="text-muted-foreground">Detailed information about block {blockNumber}</p>
						</div>
						<Badge variant="outline" className="text-sm">
							Node {nodeId}
						</Badge>
					</div>
				</div>

				{blockLoading ? (
					<Card>
						<CardContent className="text-center py-12">
							<div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary mx-auto mb-4"></div>
							<p className="text-muted-foreground">Loading block details...</p>
						</CardContent>
					</Card>
				) : (
					<div className="space-y-6">
						{/* Block Overview */}
						<Card>
							<CardHeader>
								<CardTitle className="text-base">Block Overview</CardTitle>
								<CardDescription>Basic information about this block</CardDescription>
							</CardHeader>
							<CardContent>
								<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
									<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
										<div className="flex items-center gap-3">
											<Hash className="h-4 w-4 text-muted-foreground" />
											<div>
												<p className="text-sm font-medium">Block Number</p>
												<p className="text-sm text-muted-foreground">{blockNumber}</p>
											</div>
										</div>
										<Button
											variant="ghost"
											size="sm"
											onClick={() => copyToClipboard(blockNumber)}
											className="h-8 w-8 p-0"
										>
											<Copy className="h-4 w-4" />
										</Button>
									</div>

									<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
										<div className="flex items-center gap-3">
											<Hash className="h-4 w-4 text-muted-foreground" />
											<div>
												<p className="text-sm font-medium">Block Hash</p>
												<p className="text-sm text-muted-foreground font-mono">
													{formatHash(String(block?.hash || 'N/A'))}
												</p>
											</div>
										</div>
										<Button
											variant="ghost"
											size="sm"
											onClick={() => copyToClipboard(String(block?.hash || ''))}
											className="h-8 w-8 p-0"
										>
											<Copy className="h-4 w-4" />
										</Button>
									</div>

									<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
										<div className="flex items-center gap-3">
											<Hash className="h-4 w-4 text-muted-foreground" />
											<div>
												<p className="text-sm font-medium">Parent Hash</p>
												<p className="text-sm text-muted-foreground font-mono">
													{formatHash(String(block?.parentHash || 'N/A'))}
												</p>
											</div>
										</div>
										<Button
											variant="ghost"
											size="sm"
											onClick={() => copyToClipboard(String(block?.parentHash || ''))}
											className="h-8 w-8 p-0"
										>
											<Copy className="h-4 w-4" />
										</Button>
									</div>

									<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
										<div className="flex items-center gap-3">
											<Package className="h-4 w-4 text-muted-foreground" />
											<div>
												<p className="text-sm font-medium">Transactions</p>
												<p className="text-sm text-muted-foreground">
													{transactionCountLoading ? 'Loading...' : (String(transactionCount) || '0')} transactions
												</p>
											</div>
										</div>
									</div>

									<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
										<div className="flex items-center gap-3">
											<Clock className="h-4 w-4 text-muted-foreground" />
											<div>
												<p className="text-sm font-medium">Timestamp</p>
												<p className="text-sm text-muted-foreground">
													{formatTimestamp(String(block?.timestamp || 'N/A'))}
												</p>
											</div>
										</div>
									</div>

									<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
										<div className="flex items-center gap-3">
											<Zap className="h-4 w-4 text-muted-foreground" />
											<div>
												<p className="text-sm font-medium">Gas Used</p>
												<p className="text-sm text-muted-foreground">
													{String(block?.gasUsed || 'N/A')}
												</p>
											</div>
										</div>
									</div>
								</div>
							</CardContent>
						</Card>

						{/* Transactions */}
						<Card>
							<CardHeader>
								<CardTitle className="text-base">Transactions</CardTitle>
								<CardDescription>
									{transactionCountLoading ? 'Loading...' : `${transactionCount || 0} transactions in this block`}
								</CardDescription>
							</CardHeader>
							<CardContent>
								{transactionsLoading ? (
									<div className="text-center py-8">
										<div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary mx-auto mb-3"></div>
										<p className="text-sm text-muted-foreground">Loading transactions...</p>
									</div>
								) : transactionIndices.length > 0 ? (
									<div className="space-y-3">
										{transactionIndices.map((index) => (
											<div key={index} className="flex items-center justify-between p-4 rounded-lg border bg-muted/30 hover:bg-muted/50 transition-colors">
												<div className="flex items-center gap-4">
													<div className="w-3 h-3 rounded-full bg-green-500"></div>
													<div>
														<p className="font-medium">Transaction #{index}</p>
														<p className="text-sm text-muted-foreground">
															Index: {index} • Block: {blockNumber}
														</p>
													</div>
												</div>
												<div className="flex items-center gap-2">
													<Button variant="outline" size="sm" asChild>
														<Link to={`/networks/besu/${nodeId}/blocks/${blockNumber}/transactions/${index}`}>
															View Details
														</Link>
													</Button>
												</div>
											</div>
										))}
									</div>
								) : (
									<div className="text-center py-8">
										<Package className="h-8 w-8 text-muted-foreground mx-auto mb-3" />
										<p className="text-sm text-muted-foreground">No transactions in this block</p>
									</div>
								)}
							</CardContent>
						</Card>

						{/* Raw Block Data */}
						<Card>
							<CardHeader>
								<CardTitle className="text-base">Raw Block Data</CardTitle>
								<CardDescription>Complete block information in JSON format</CardDescription>
							</CardHeader>
							<CardContent>
								<div className="bg-muted/50 p-4 rounded-md overflow-auto">
									<pre className="text-sm font-mono">
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
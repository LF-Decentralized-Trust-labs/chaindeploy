import { Activity, Copy, Hash, Package, TrendingUp, ArrowLeft, ExternalLink, Clock, Zap, Send, Receipt } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { useParams, Link } from 'react-router-dom'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { toast } from 'sonner'
import {
	getNodesByIdRpcTransactionByBlockNumberAndIndexOptions,
} from '@/api/client/@tanstack/react-query.gen'

export default function BesuTransactionDetailsPage() {
	const { nodeId, blockNumber, index } = useParams<{ nodeId: string; blockNumber: string; index: string }>()
	const nodeIdNumber = nodeId ? parseInt(nodeId) : 0
	const blockNumberInt = blockNumber ? parseInt(blockNumber) : 0
	const indexInt = index ? parseInt(index) : 0

	// Get transaction details
	const { data: transaction, isLoading: transactionLoading } = useQuery({
		...getNodesByIdRpcTransactionByBlockNumberAndIndexOptions({
			path: { id: nodeIdNumber },
			query: { number: blockNumberInt.toString(), tag: blockNumberInt.toString(), index: indexInt.toString() },
		}),
		enabled: !!nodeIdNumber && !!blockNumberInt && indexInt >= 0,
	})

	const copyToClipboard = (text: string) => {
		navigator.clipboard.writeText(text)
		toast.success('Copied to clipboard')
	}

	// Format hash for better readability
	const formatHash = (hash: string) => {
		if (hash.length <= 10) return hash
		return `${hash.slice(0, 6)}...${hash.slice(-4)}`
	}

	// Format value from wei to ether
	const formatValue = (value: string) => {
		if (!value) return '0'
		const wei = BigInt(value)
		const eth = Number(wei) / Math.pow(10, 18)
		return eth.toFixed(6)
	}

	// Format gas price
	const formatGasPrice = (gasPrice: string) => {
		if (!gasPrice) return 'N/A'
		const wei = BigInt(gasPrice)
		const gwei = Number(wei) / Math.pow(10, 9)
		return `${gwei.toFixed(2)} Gwei`
	}

	return (
		<div className="flex-1 p-8">
			<div className="max-w-7xl mx-auto">
				<div className="flex items-center gap-2 text-muted-foreground mb-8">
					<Button variant="ghost" size="sm" asChild>
						<Link to={`/networks/besu/${nodeId}/blocks/${blockNumber}`}>
							<ArrowLeft className="mr-2 h-4 w-4" />
							Back to Block
						</Link>
					</Button>
				</div>

				<div className="mb-6">
					<div className="flex items-center justify-between">
						<div>
							<h1 className="text-2xl font-semibold">Transaction #{index}</h1>
							<p className="text-muted-foreground">Transaction details from block {blockNumber}</p>
						</div>
						<Badge variant="outline" className="text-sm">
							Node {nodeId}
						</Badge>
					</div>
				</div>

				{transactionLoading ? (
					<Card>
						<CardContent className="text-center py-12">
							<div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary mx-auto mb-4"></div>
							<p className="text-muted-foreground">Loading transaction details...</p>
						</CardContent>
					</Card>
				) : (
					<div className="space-y-6">
						{/* Transaction Overview */}
						<Card>
							<CardHeader>
								<CardTitle className="text-base">Transaction Overview</CardTitle>
								<CardDescription>Basic information about this transaction</CardDescription>
							</CardHeader>
							<CardContent>
								<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
									<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
										<div className="flex items-center gap-3">
											<Hash className="h-4 w-4 text-muted-foreground" />
											<div>
												<p className="text-sm font-medium">Transaction Hash</p>
												<p className="text-sm text-muted-foreground font-mono">
													{formatHash(String(transaction?.hash || 'N/A'))}
												</p>
											</div>
										</div>
										<Button
											variant="ghost"
											size="sm"
											onClick={() => copyToClipboard(String(transaction?.hash || ''))}
											className="h-8 w-8 p-0"
										>
											<Copy className="h-4 w-4" />
										</Button>
									</div>

									<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
										<div className="flex items-center gap-3">
											<Package className="h-4 w-4 text-muted-foreground" />
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
											<Send className="h-4 w-4 text-muted-foreground" />
											<div>
												<p className="text-sm font-medium">From</p>
												<p className="text-sm text-muted-foreground font-mono">
													{formatHash(String(transaction?.from || 'N/A'))}
												</p>
											</div>
										</div>
										<Button
											variant="ghost"
											size="sm"
											onClick={() => copyToClipboard(String(transaction?.from || ''))}
											className="h-8 w-8 p-0"
										>
											<Copy className="h-4 w-4" />
										</Button>
									</div>

									<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
										<div className="flex items-center gap-3">
											<Receipt className="h-4 w-4 text-muted-foreground" />
											<div>
												<p className="text-sm font-medium">To</p>
												<p className="text-sm text-muted-foreground font-mono">
													{formatHash(String(transaction?.to || 'N/A'))}
												</p>
											</div>
										</div>
										<Button
											variant="ghost"
											size="sm"
											onClick={() => copyToClipboard(String(transaction?.to || ''))}
											className="h-8 w-8 p-0"
										>
											<Copy className="h-4 w-4" />
										</Button>
									</div>

									<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
										<div className="flex items-center gap-3">
											<TrendingUp className="h-4 w-4 text-muted-foreground" />
											<div>
												<p className="text-sm font-medium">Value</p>
												<p className="text-sm text-muted-foreground">
													{formatValue(String(transaction?.value || '0'))} ETH
												</p>
											</div>
										</div>
									</div>

									<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
										<div className="flex items-center gap-3">
											<Zap className="h-4 w-4 text-muted-foreground" />
											<div>
												<p className="text-sm font-medium">Gas Price</p>
												<p className="text-sm text-muted-foreground">
													{formatGasPrice(String(transaction?.gasPrice || 'N/A'))}
												</p>
											</div>
										</div>
									</div>

									<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
										<div className="flex items-center gap-3">
											<Zap className="h-4 w-4 text-muted-foreground" />
											<div>
												<p className="text-sm font-medium">Gas Limit</p>
												<p className="text-sm text-muted-foreground">
													{String(transaction?.gas || 'N/A')}
												</p>
											</div>
										</div>
									</div>

									<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
										<div className="flex items-center gap-3">
											<Hash className="h-4 w-4 text-muted-foreground" />
											<div>
												<p className="text-sm font-medium">Nonce</p>
												<p className="text-sm text-muted-foreground">
													{String(transaction?.nonce || 'N/A')}
												</p>
											</div>
										</div>
									</div>

									<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
										<div className="flex items-center gap-3">
											<Hash className="h-4 w-4 text-muted-foreground" />
											<div>
												<p className="text-sm font-medium">Input Data</p>
												<p className="text-sm text-muted-foreground font-mono">
													{String(transaction?.input || '0x').length > 66 
														? `${String(transaction?.input || '0x').slice(0, 66)}...` 
														: String(transaction?.input || '0x')}
												</p>
											</div>
										</div>
										<Button
											variant="ghost"
											size="sm"
											onClick={() => copyToClipboard(String(transaction?.input || ''))}
											className="h-8 w-8 p-0"
										>
											<Copy className="h-4 w-4" />
										</Button>
									</div>
								</div>
							</CardContent>
						</Card>

						{/* Raw Transaction Data */}
						<Card>
							<CardHeader>
								<CardTitle className="text-base">Raw Transaction Data</CardTitle>
								<CardDescription>Complete transaction information in JSON format</CardDescription>
							</CardHeader>
							<CardContent>
								<div className="bg-muted/50 p-4 rounded-md overflow-auto">
									<pre className="text-sm font-mono">
										<code>{JSON.stringify(transaction, null, 2)}</code>
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
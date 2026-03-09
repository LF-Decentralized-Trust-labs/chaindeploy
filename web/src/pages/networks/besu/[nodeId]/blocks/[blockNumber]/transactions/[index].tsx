import { Activity, Copy, Hash, Package, TrendingUp, ArrowLeft, Zap, Send, Receipt, FileText } from 'lucide-react'
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

	const copyToClipboard = async (text: string, label?: string) => {
		try {
			await navigator.clipboard.writeText(text)
			toast.success(`${label || 'Value'} copied to clipboard`)
		} catch (error) {
			toast.error('Failed to copy to clipboard')
		}
	}

	// Format value from wei to ether (handles hex strings)
	const formatWei = (value: string | number | undefined) => {
		if (!value) return '0 ETH'
		const wei = typeof value === 'string' ? parseInt(value, 16) : value
		if (wei === 0) return '0 ETH'
		const eth = wei / Math.pow(10, 18)
		return `${eth.toFixed(6)} ETH`
	}

	// Format gas value (handles hex strings)
	const formatGasValue = (value: string | number | undefined) => {
		if (!value) return '0'
		const num = typeof value === 'string' ? parseInt(value, 16) : value
		return num.toLocaleString()
	}

	// Format gas price to Gwei
	const formatGasPrice = (gasPrice: string | number | undefined) => {
		if (!gasPrice) return 'N/A'
		const wei = typeof gasPrice === 'string' ? parseInt(gasPrice, 16) : gasPrice
		const gwei = wei / Math.pow(10, 9)
		return `${gwei.toFixed(2)} Gwei`
	}

	// Format a number from hex
	const formatNumber = (value: string | number | undefined) => {
		if (!value) return '0'
		return typeof value === 'string' ? parseInt(value, 16) : value
	}

	// Get transaction type label
	const getTransactionType = (type: string | undefined) => {
		if (!type) return 'Legacy'
		const typeNum = typeof type === 'string' ? parseInt(type, 16) : type
		switch (typeNum) {
			case 0: return 'Legacy'
			case 1: return 'EIP-2930'
			case 2: return 'EIP-1559'
			default: return `Type ${typeNum}`
		}
	}

	// Decode input data client-side
	const decodeInputData = (hex: string) => {
		if (!hex || hex === '0x' || hex === '0x0') return null

		try {
			const cleanHex = hex.startsWith('0x') ? hex.slice(2) : hex

			// Function calls are at least 4 bytes (function selector)
			if (cleanHex.length >= 8) {
				const functionSelector = '0x' + cleanHex.slice(0, 8)
				const parameters = cleanHex.length > 8 ? '0x' + cleanHex.slice(8) : null

				return {
					type: 'function_call',
					functionSelector,
					parameters,
					parameterCount: parameters ? (cleanHex.length - 8) / 64 : 0
				}
			}

			// Try to decode as string if it's mostly printable
			let result = ''
			let printableCount = 0
			for (let i = 0; i < cleanHex.length; i += 2) {
				const hexPair = cleanHex.substring(i, i + 2)
				const charCode = parseInt(hexPair, 16)
				if (charCode >= 32 && charCode <= 126) {
					result += String.fromCharCode(charCode)
					printableCount++
				} else {
					result += '.'
				}
			}

			const totalBytes = cleanHex.length / 2
			if (printableCount / totalBytes >= 0.5) {
				return { type: 'string', value: result }
			}

			return { type: 'binary', value: hex }
		} catch (error) {
			return null
		}
	}

	const tx = transaction as any
	const inputDecoded = tx?.input ? decodeInputData(tx.input) : null

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
						<div className="flex items-center gap-2">
							{tx && (
								<Badge variant="outline" className="text-sm">
									{getTransactionType(tx.type)}
								</Badge>
							)}
							<Badge variant="outline" className="text-sm">
								Node {nodeId}
							</Badge>
						</div>
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
								<div className="grid gap-4">
									{/* Transaction Hash */}
									<div className="flex items-center justify-between py-3 border-b">
										<div className="flex items-center gap-2">
											<Hash className="h-4 w-4 text-muted-foreground" />
											<span className="font-medium">Transaction Hash</span>
										</div>
										<div className="flex items-center gap-2">
											<code className="text-sm font-mono bg-muted px-2 py-1 rounded">
												{String(tx?.hash || 'N/A')}
											</code>
											{tx?.hash && (
												<Button
													variant="ghost"
													size="sm"
													onClick={() => copyToClipboard(String(tx.hash), 'Transaction hash')}
												>
													<Copy className="h-4 w-4" />
												</Button>
											)}
										</div>
									</div>

									{/* Block Number */}
									<div className="flex items-center justify-between py-3 border-b">
										<div className="flex items-center gap-2">
											<Package className="h-4 w-4 text-muted-foreground" />
											<span className="font-medium">Block Number</span>
										</div>
										<div className="flex items-center gap-2">
											<span className="text-sm font-mono">{blockNumber}</span>
											<Button
												variant="ghost"
												size="sm"
												onClick={() => copyToClipboard(blockNumber!)}
											>
												<Copy className="h-4 w-4" />
											</Button>
										</div>
									</div>

									{/* From */}
									<div className="flex items-center justify-between py-3 border-b">
										<div className="flex items-center gap-2">
											<Send className="h-4 w-4 text-muted-foreground" />
											<span className="font-medium">From</span>
										</div>
										<div className="flex items-center gap-2">
											<code className="text-sm font-mono bg-muted px-2 py-1 rounded">
												{String(tx?.from || 'N/A')}
											</code>
											{tx?.from && (
												<Button
													variant="ghost"
													size="sm"
													onClick={() => copyToClipboard(String(tx.from), 'From address')}
												>
													<Copy className="h-4 w-4" />
												</Button>
											)}
										</div>
									</div>

									{/* To */}
									<div className="flex items-center justify-between py-3 border-b">
										<div className="flex items-center gap-2">
											<Receipt className="h-4 w-4 text-muted-foreground" />
											<span className="font-medium">To</span>
										</div>
										<div className="flex items-center gap-2">
											<code className="text-sm font-mono bg-muted px-2 py-1 rounded">
												{tx?.to ? String(tx.to) : 'Contract Creation'}
											</code>
											{tx?.to && (
												<Button
													variant="ghost"
													size="sm"
													onClick={() => copyToClipboard(String(tx.to), 'To address')}
												>
													<Copy className="h-4 w-4" />
												</Button>
											)}
										</div>
									</div>

									{/* Value */}
									<div className="flex items-center justify-between py-3 border-b">
										<div className="flex items-center gap-2">
											<TrendingUp className="h-4 w-4 text-muted-foreground" />
											<span className="font-medium">Value</span>
										</div>
										<span className="text-sm font-mono">
											{formatWei(tx?.value)}
										</span>
									</div>

									{/* Transaction Type */}
									<div className="flex items-center justify-between py-3 border-b">
										<div className="flex items-center gap-2">
											<Activity className="h-4 w-4 text-muted-foreground" />
											<span className="font-medium">Transaction Type</span>
										</div>
										<Badge variant="secondary">
											{getTransactionType(tx?.type)}
										</Badge>
									</div>

									{/* Gas Price */}
									<div className="flex items-center justify-between py-3 border-b">
										<div className="flex items-center gap-2">
											<Zap className="h-4 w-4 text-muted-foreground" />
											<span className="font-medium">Gas Price</span>
										</div>
										<span className="text-sm font-mono">
											{formatGasPrice(tx?.gasPrice)}
										</span>
									</div>

									{/* Gas Limit */}
									<div className="flex items-center justify-between py-3 border-b">
										<div className="flex items-center gap-2">
											<Zap className="h-4 w-4 text-muted-foreground" />
											<span className="font-medium">Gas Limit</span>
										</div>
										<span className="text-sm font-mono">
											{formatGasValue(tx?.gas)}
										</span>
									</div>

									{/* Nonce */}
									<div className="flex items-center justify-between py-3 border-b">
										<div className="flex items-center gap-2">
											<Hash className="h-4 w-4 text-muted-foreground" />
											<span className="font-medium">Nonce</span>
										</div>
										<span className="text-sm font-mono">
											{formatNumber(tx?.nonce)}
										</span>
									</div>

									{/* Chain ID */}
									{tx?.chainId && (
										<div className="flex items-center justify-between py-3 border-b">
											<div className="flex items-center gap-2">
												<Hash className="h-4 w-4 text-muted-foreground" />
												<span className="font-medium">Chain ID</span>
											</div>
											<span className="text-sm font-mono">
												{formatNumber(tx.chainId)}
											</span>
										</div>
									)}
								</div>
							</CardContent>
						</Card>

						{/* Signature Details */}
						{tx && (tx.r || tx.s || tx.v) && (
							<Card>
								<CardHeader>
									<CardTitle className="text-base flex items-center gap-2">
										<FileText className="h-4 w-4" />
										Signature
									</CardTitle>
									<CardDescription>ECDSA signature components</CardDescription>
								</CardHeader>
								<CardContent>
									<div className="grid gap-4">
										{/* R Value */}
										<div className="flex items-center justify-between py-3 border-b">
											<span className="font-medium text-muted-foreground">R:</span>
											<div className="flex items-center gap-2">
												<code className="text-xs font-mono bg-muted px-2 py-1 rounded truncate max-w-[400px]">
													{tx.r || 'N/A'}
												</code>
												{tx.r && (
													<Button
														variant="ghost"
														size="sm"
														onClick={() => copyToClipboard(tx.r, 'R value')}
													>
														<Copy className="h-4 w-4" />
													</Button>
												)}
											</div>
										</div>

										{/* S Value */}
										<div className="flex items-center justify-between py-3 border-b">
											<span className="font-medium text-muted-foreground">S:</span>
											<div className="flex items-center gap-2">
												<code className="text-xs font-mono bg-muted px-2 py-1 rounded truncate max-w-[400px]">
													{tx.s || 'N/A'}
												</code>
												{tx.s && (
													<Button
														variant="ghost"
														size="sm"
														onClick={() => copyToClipboard(tx.s, 'S value')}
													>
														<Copy className="h-4 w-4" />
													</Button>
												)}
											</div>
										</div>

										{/* V Value */}
										<div className="flex items-center justify-between py-3">
											<span className="font-medium text-muted-foreground">V:</span>
											<span className="font-mono">{tx.v || 'N/A'}</span>
										</div>
									</div>
								</CardContent>
							</Card>
						)}

						{/* Input Data */}
						{tx?.input && tx.input !== '0x' && (
							<Card>
								<CardHeader>
									<CardTitle className="text-base flex items-center gap-2">
										<Zap className="h-4 w-4" />
										Input Data
									</CardTitle>
									<CardDescription>Transaction calldata and decoded information</CardDescription>
								</CardHeader>
								<CardContent>
									<div className="space-y-4">
										{/* Raw Hex Data */}
										<div>
											<span className="text-sm font-medium text-muted-foreground mb-2 block">Raw Data:</span>
											<div className="flex items-start gap-2">
												<code className="text-xs font-mono bg-muted/50 p-3 rounded block w-full overflow-x-auto">
													{tx.input}
												</code>
												<Button
													variant="ghost"
													size="sm"
													onClick={() => copyToClipboard(tx.input, 'Input data')}
												>
													<Copy className="h-4 w-4" />
												</Button>
											</div>
										</div>

										{/* Client-side Decoded Analysis */}
										{inputDecoded && inputDecoded.type === 'function_call' && (
											<div className="space-y-3 pt-3 border-t">
												<div>
													<span className="text-sm font-medium text-muted-foreground mb-2 block">Function Selector:</span>
													<div className="flex items-center gap-2">
														<code className="text-sm font-mono bg-muted px-2 py-1 rounded">
															{inputDecoded.functionSelector}
														</code>
														<Button
															variant="ghost"
															size="sm"
															onClick={() => copyToClipboard(inputDecoded.functionSelector, 'Function selector')}
														>
															<Copy className="h-4 w-4" />
														</Button>
													</div>
												</div>
												{inputDecoded.parameters && (
													<div>
														<span className="text-sm font-medium text-muted-foreground mb-2 block">
															Raw Parameters ({Math.floor(inputDecoded.parameterCount)} words):
														</span>
														<div className="flex items-start gap-2">
															<code className="text-xs font-mono bg-muted/50 p-3 rounded block w-full overflow-x-auto">
																{inputDecoded.parameters}
															</code>
															<Button
																variant="ghost"
																size="sm"
																onClick={() => copyToClipboard(inputDecoded.parameters!, 'Parameters')}
															>
																<Copy className="h-4 w-4" />
															</Button>
														</div>
													</div>
												)}
												{!inputDecoded.parameters && (
													<p className="text-sm text-muted-foreground">
														No parameters (function takes no arguments)
													</p>
												)}
											</div>
										)}

										{inputDecoded && inputDecoded.type === 'string' && (
											<div className="pt-3 border-t">
												<span className="text-sm font-medium text-muted-foreground mb-2 block">Decoded String:</span>
												<div className="flex items-start gap-2">
													<code className="text-xs font-mono bg-muted/50 p-3 rounded block w-full overflow-x-auto break-all">
														{inputDecoded.value}
													</code>
													<Button
														variant="ghost"
														size="sm"
														onClick={() => copyToClipboard(inputDecoded.value!, 'Decoded string')}
													>
														<Copy className="h-4 w-4" />
													</Button>
												</div>
											</div>
										)}

										{inputDecoded && inputDecoded.type === 'binary' && (
											<p className="text-sm text-muted-foreground pt-3 border-t">
												Binary data - no readable content detected
											</p>
										)}
									</div>
								</CardContent>
							</Card>
						)}

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

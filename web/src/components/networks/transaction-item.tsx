import { Button } from '@/components/ui/button'
import { useState } from 'react'
import { toast } from 'sonner'
import { ChevronDown, ChevronUp, Copy, Hash, ArrowRight, FileText, Zap } from 'lucide-react'

interface TransactionItemProps {
	tx: any
	index: number
}

export function TransactionItem({ tx }: TransactionItemProps) {
	const [isExpanded, setIsExpanded] = useState(false)

	const copyToClipboard = async (text: string, label?: string) => {
		try {
			await navigator.clipboard.writeText(text)
			toast.success(`${label || 'Value'} copied to clipboard`)
		} catch (error) {
			toast.error('Failed to copy to clipboard')
		}
	}

	const formatGasValue = (value: string | number | undefined) => {
		if (!value) return '0'
		const num = typeof value === 'string' ? parseInt(value, 16) : value
		return num.toLocaleString()
	}

	const formatNumber = (value: string | number | undefined) => {
		if (!value) return '0'
		return typeof value === 'string' ? parseInt(value, 16) : value
	}

	const formatWei = (value: string | number | undefined) => {
		if (!value) return '0 ETH'
		const wei = typeof value === 'string' ? parseInt(value, 16) : value
		if (wei === 0) return '0 ETH'
		const eth = wei / Math.pow(10, 18)
		return `${eth.toFixed(6)} ETH`
	}

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

	const decodeInputData = (hex: string) => {
		if (!hex || hex === '0x' || hex === '0x0') return null

		try {
			// Remove 0x prefix if present
			const cleanHex = hex.startsWith('0x') ? hex.slice(2) : hex

			// Function calls are at least 4 bytes (function selector)
			if (cleanHex.length >= 8) {
				const functionSelector = '0x' + cleanHex.slice(0, 8)
				const parameters = cleanHex.length > 8 ? '0x' + cleanHex.slice(8) : null

				return {
					type: 'function_call',
					functionSelector,
					parameters,
					parameterCount: parameters ? (cleanHex.length - 8) / 64 : 0 // Each parameter is typically 32 bytes
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

	const txHash = typeof tx === 'string' ? tx : tx.hash

	return (
		<div className="border rounded-lg overflow-hidden">
			{/* Compact View */}
			<div
				className="flex items-center justify-between p-4 hover:bg-muted/50 transition-colors cursor-pointer"
				onClick={() => setIsExpanded(!isExpanded)}
			>
				<div className="flex-1 min-w-0">
					<div className="flex items-center gap-2">
						<Hash className="h-4 w-4 text-muted-foreground flex-shrink-0" />
						<code className="text-sm font-mono truncate">
							{txHash}
						</code>
						{typeof tx === 'object' && (
							<>
								<span className="text-xs bg-muted px-2 py-1 rounded">
									{getTransactionType(tx.type)}
								</span>
								{tx.value && parseInt(tx.value, 16) > 0 && (
									<span className="text-xs bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200 px-2 py-1 rounded">
										{formatWei(tx.value)}
									</span>
								)}
							</>
						)}
					</div>
					{typeof tx === 'object' && (
						<div className="mt-2 text-sm text-muted-foreground">
							<span className="flex items-center gap-1">
								<span className="truncate max-w-[200px]">{tx.from || 'Unknown'}</span>
								<ArrowRight className="h-3 w-3 flex-shrink-0" />
								<span className="truncate max-w-[200px]">{tx.to || 'Contract Creation'}</span>
							</span>
						</div>
					)}
				</div>
				<div className="flex items-center gap-2 ml-4">
					<Button
						variant="ghost"
						size="sm"
						onClick={(e) => {
							e.stopPropagation()
							copyToClipboard(txHash, 'Transaction hash')
						}}
					>
						<Copy className="h-4 w-4" />
					</Button>
					{isExpanded ? (
						<ChevronUp className="h-4 w-4 text-muted-foreground" />
					) : (
						<ChevronDown className="h-4 w-4 text-muted-foreground" />
					)}
				</div>
			</div>

			{/* Expanded Detailed View */}
			{isExpanded && typeof tx === 'object' && (
				<div className="border-t bg-muted/20 p-4">
					<div className="grid gap-3 text-sm">
						{/* Transaction Details Grid */}
						<div className="grid md:grid-cols-2 gap-4">
							<div className="space-y-3">
								{/* From Address */}
								<div className="flex items-center justify-between">
									<span className="font-medium text-muted-foreground">From:</span>
									<div className="flex items-center gap-2">
										<code className="text-xs font-mono bg-background px-2 py-1 rounded">
											{tx.from || 'Unknown'}
										</code>
										{tx.from && (
											<Button
												variant="ghost"
												size="sm"
												onClick={() => copyToClipboard(tx.from, 'From address')}
											>
												<Copy className="h-3 w-3" />
											</Button>
										)}
									</div>
								</div>

								{/* To Address */}
								<div className="flex items-center justify-between">
									<span className="font-medium text-muted-foreground">To:</span>
									<div className="flex items-center gap-2">
										<code className="text-xs font-mono bg-background px-2 py-1 rounded">
											{tx.to || 'Contract Creation'}
										</code>
										{tx.to && (
											<Button
												variant="ghost"
												size="sm"
												onClick={() => copyToClipboard(tx.to, 'To address')}
											>
												<Copy className="h-3 w-3" />
											</Button>
										)}
									</div>
								</div>

								{/* Value */}
								<div className="flex items-center justify-between">
									<span className="font-medium text-muted-foreground">Value:</span>
									<span className="font-mono">{formatWei(tx.value)}</span>
								</div>

								{/* Gas Limit */}
								<div className="flex items-center justify-between">
									<span className="font-medium text-muted-foreground">Gas Limit:</span>
									<span className="font-mono">{formatGasValue(tx.gas)}</span>
								</div>

								{/* Gas Price */}
								<div className="flex items-center justify-between">
									<span className="font-medium text-muted-foreground">Gas Price:</span>
									<span className="font-mono">{formatGasValue(tx.gasPrice)} Gwei</span>
								</div>
							</div>

							<div className="space-y-3">
								{/* Nonce */}
								<div className="flex items-center justify-between">
									<span className="font-medium text-muted-foreground">Nonce:</span>
									<span className="font-mono">{formatNumber(tx.nonce)}</span>
								</div>

								{/* Transaction Index */}
								<div className="flex items-center justify-between">
									<span className="font-medium text-muted-foreground">Index:</span>
									<span className="font-mono">{formatNumber(tx.transactionIndex)}</span>
								</div>

								{/* Transaction Type */}
								<div className="flex items-center justify-between">
									<span className="font-medium text-muted-foreground">Type:</span>
									<span className="font-mono">{getTransactionType(tx.type)}</span>
								</div>

								{/* Chain ID */}
								{tx.chainId && (
									<div className="flex items-center justify-between">
										<span className="font-medium text-muted-foreground">Chain ID:</span>
										<span className="font-mono">{formatNumber(tx.chainId)}</span>
									</div>
								)}
							</div>
						</div>

						{/* Signature Details */}
						<div className="pt-3 border-t">
							<h4 className="font-medium mb-3 flex items-center gap-2">
								<FileText className="h-4 w-4" />
								Signature
							</h4>
							<div className="grid gap-2">
								{/* R Value */}
								<div className="flex items-center justify-between">
									<span className="font-medium text-muted-foreground">R:</span>
									<div className="flex items-center gap-2">
										<code className="text-xs font-mono bg-background px-2 py-1 rounded truncate max-w-[300px]">
											{tx.r || 'N/A'}
										</code>
										{tx.r && (
											<Button
												variant="ghost"
												size="sm"
												onClick={() => copyToClipboard(tx.r, 'R value')}
											>
												<Copy className="h-3 w-3" />
											</Button>
										)}
									</div>
								</div>

								{/* S Value */}
								<div className="flex items-center justify-between">
									<span className="font-medium text-muted-foreground">S:</span>
									<div className="flex items-center gap-2">
										<code className="text-xs font-mono bg-background px-2 py-1 rounded truncate max-w-[300px]">
											{tx.s || 'N/A'}
										</code>
										{tx.s && (
											<Button
												variant="ghost"
												size="sm"
												onClick={() => copyToClipboard(tx.s, 'S value')}
											>
												<Copy className="h-3 w-3" />
											</Button>
										)}
									</div>
								</div>

								{/* V Value */}
								<div className="flex items-center justify-between">
									<span className="font-medium text-muted-foreground">V:</span>
									<span className="font-mono">{tx.v || 'N/A'}</span>
								</div>
							</div>
						</div>

						{/* Input Data */}
						{tx.input && tx.input !== '0x' && (
							<div className="pt-3 border-t">
								<h4 className="font-medium mb-3 flex items-center gap-2">
									<Zap className="h-4 w-4" />
									Input Data
								</h4>
								<div className="space-y-3">
									{/* Raw Hex Data */}
									<div>
										<span className="text-xs font-medium text-muted-foreground mb-1 block">Raw Data:</span>
										<div className="flex items-start gap-2">
											<code className="text-xs font-mono bg-background p-3 rounded block w-full overflow-x-auto">
												{tx.input}
											</code>
											<Button
												variant="ghost"
												size="sm"
												onClick={() => copyToClipboard(tx.input, 'Input data')}
											>
												<Copy className="h-3 w-3" />
											</Button>
										</div>
									</div>

									{/* Client-side Decoded Analysis */}
									{(() => {
										const decoded = decodeInputData(tx.input)
										if (!decoded) return null

										if (decoded.type === 'function_call') {
											return (
												<div className="space-y-2">
													<div>
														<span className="text-xs font-medium text-muted-foreground mb-1 block">Function Selector:</span>
														<div className="flex items-start gap-2">
															<code className="text-xs font-mono bg-background p-2 rounded">
																{decoded.functionSelector}
															</code>
															<Button
																variant="ghost"
																size="sm"
																onClick={() => copyToClipboard(decoded.functionSelector, 'Function selector')}
															>
																<Copy className="h-3 w-3" />
															</Button>
														</div>
													</div>
													{decoded.parameters && (
														<div>
															<span className="text-xs font-medium text-muted-foreground mb-1 block">
																Raw Parameters ({Math.floor(decoded.parameterCount)} words):
															</span>
															<div className="flex items-start gap-2">
																<code className="text-xs font-mono bg-background p-2 rounded block w-full overflow-x-auto">
																	{decoded.parameters}
																</code>
																<Button
																	variant="ghost"
																	size="sm"
																	onClick={() => copyToClipboard(decoded.parameters!, 'Parameters')}
																>
																	<Copy className="h-3 w-3" />
																</Button>
															</div>
														</div>
													)}
													{!decoded.parameters && (
														<p className="text-xs text-muted-foreground">
															No parameters (function takes no arguments)
														</p>
													)}
												</div>
											)
										}

										if (decoded.type === 'string') {
											return (
												<div>
													<span className="text-xs font-medium text-muted-foreground mb-1 block">Decoded String:</span>
													<div className="flex items-start gap-2">
														<code className="text-xs font-mono bg-background p-3 rounded block w-full overflow-x-auto break-all">
															{decoded.value}
														</code>
														<Button
															variant="ghost"
															size="sm"
															onClick={() => copyToClipboard(decoded.value!, 'Decoded string')}
														>
															<Copy className="h-3 w-3" />
														</Button>
													</div>
												</div>
											)
										}

										return (
											<p className="text-xs text-muted-foreground">
												Binary data - no readable content
											</p>
										)
									})()}
								</div>
							</div>
						)}
					</div>
				</div>
			)}
		</div>
	)
}

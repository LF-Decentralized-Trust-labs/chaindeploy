import { Activity, Copy, ExternalLink, Hash, Package, Search, TrendingUp, ChevronLeft, ChevronRight, Server, Check } from 'lucide-react'
import { useState, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card'
import { Input } from '../ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import { toast } from 'sonner'
import { Link } from 'react-router-dom'
import {
	getNodesByIdRpcBlockNumberOptions,
	getNodesByIdRpcBlockByNumberOptions,
	getNodesByIdRpcBlockByHashOptions,
	getNodesByIdRpcBlockTransactionCountByNumberOptions,
} from '@/api/client/@tanstack/react-query.gen'

interface BesuBlockExplorerProps {
	nodesLoading: boolean
	networkNodes?: Array<{ id?: number; name?: string; status?: string }>
}

export function BesuBlockExplorer({ nodesLoading, networkNodes = [] }: BesuBlockExplorerProps) {
	const [selectedNodeId, setSelectedNodeId] = useState<number | null>(null)
	const [searchBlock, setSearchBlock] = useState<string>('')
	const [searchType, setSearchType] = useState<'number' | 'hash'>('number')
	const [currentPage, setCurrentPage] = useState(0)
	const blocksPerPage = 5

	// Get latest block number
	const { data: latestBlockNumber, isLoading: latestBlockLoading } = useQuery({
		...getNodesByIdRpcBlockNumberOptions({
			path: { id: selectedNodeId || 0 },
		}),
		enabled: !!selectedNodeId,
	})

	// Calculate the range of blocks to fetch
	const latestBlock = typeof latestBlockNumber === 'string' ? parseInt(latestBlockNumber, 16) : latestBlockNumber || 0
	const startBlock = latestBlock ? Math.max(0, latestBlock - (currentPage * blocksPerPage)) : 0
	const endBlock = latestBlock ? Math.max(0, latestBlock - ((currentPage + 1) * blocksPerPage) + 1) : 0

	// Get blocks for the current page
	const { data: blocks, isLoading: blocksLoading } = useQuery({
		...getNodesByIdRpcBlockByNumberOptions({
			path: { id: selectedNodeId || 0 },
			query: { number: startBlock.toString(), tag: startBlock.toString() },
		}),
		enabled: !!selectedNodeId && !!latestBlockNumber && startBlock >= 0,
	})

	// Get block by number (for search)
	const { data: blockByNumber, isLoading: blockByNumberLoading } = useQuery({
		...getNodesByIdRpcBlockByNumberOptions({
			path: { id: selectedNodeId || 0 },
			query: { number: searchBlock, tag: searchBlock },
		}),
		enabled: !!selectedNodeId && !!searchBlock && searchType === 'number',
	})

	// Get block by hash (for search)
	const { data: blockByHash, isLoading: blockByHashLoading } = useQuery({
		...getNodesByIdRpcBlockByHashOptions({
			path: { id: selectedNodeId || 0 },
			query: { hash: searchBlock },
		}),
		enabled: !!selectedNodeId && !!searchBlock && searchType === 'hash',
	})

	// Get transaction count for a block
	const { data: transactionCount, isLoading: transactionCountLoading } = useQuery({
		...getNodesByIdRpcBlockTransactionCountByNumberOptions({
			path: { id: selectedNodeId || 0 },
			query: { number: searchBlock, tag: searchBlock },
		}),
		enabled: !!selectedNodeId && !!searchBlock && searchType === 'number',
	})

	// Reset to first page when node changes
	useEffect(() => {
		setCurrentPage(0)
	}, [selectedNodeId])

	// Set initial selected node when networkNodes are available
	useEffect(() => {
		if (networkNodes.length > 0 && !selectedNodeId) {
			setSelectedNodeId(networkNodes[0].id!)
		}
	}, [networkNodes, selectedNodeId])

	const handleSearch = () => {
		if (!searchBlock.trim()) {
			toast.error('Please enter a block number or hash')
			return
		}
		// The search is handled by the queries above
	}

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

	// Generate array of block numbers for current page
	const getBlockNumbersForPage = () => {
		if (!latestBlock) return []
		const numbers = []
		for (let i = startBlock; i >= endBlock; i--) {
			numbers.push(i)
		}
		return numbers
	}

	const blockNumbers = getBlockNumbersForPage()
	const totalPages = latestBlock ? Math.ceil(latestBlock / blocksPerPage) : 0

	// Show loading state while nodes are being fetched
	if (nodesLoading) {
		return (
			<div className="space-y-6">
				<div className="flex items-center gap-4">
					<div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center">
						<Activity className="h-6 w-6 text-primary" />
					</div>
					<div>
						<h2 className="text-lg font-semibold">Block Explorer</h2>
						<p className="text-sm text-muted-foreground">Explore blocks and transactions</p>
					</div>
				</div>
				<Card>
					<CardContent className="text-center py-12">
						<div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary mx-auto mb-4"></div>
						<p className="text-muted-foreground">Loading network nodes...</p>
					</CardContent>
				</Card>
			</div>
		)
	}

	return (
		<div className="space-y-6">
			{selectedNodeId ? (
				<div className="space-y-6">
					{/* Header */}
					<div className="flex items-center gap-4">
						<div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center">
							<Package className="h-6 w-6 text-primary" />
						</div>
						<div className="flex-1">
							<h2 className="text-lg font-semibold">Block Explorer</h2>
							<p className="text-sm text-muted-foreground">Explore blocks and transactions on your Besu network</p>
						</div>
						<div className="flex items-center gap-3">
							{/* Node Selection */}
							{networkNodes.length > 0 && (
								<Select 
									value={selectedNodeId?.toString() || ''} 
									onValueChange={(value) => setSelectedNodeId(parseInt(value))}
								>
									<SelectTrigger className="w-auto min-w-[180px]">
										<SelectValue>
											<div className="flex items-center gap-2">
												<Server className="h-4 w-4" />
												<span className="text-sm">
													{selectedNodeId ? `Node ${selectedNodeId}` : 'Select node'}
												</span>
											</div>
										</SelectValue>
									</SelectTrigger>
									<SelectContent>
										{networkNodes.map((node) => (
											<SelectItem key={node.id} value={node.id?.toString() || ''}>
												<div className="flex items-center gap-2">
													<Server className="h-4 w-4" />
													<span>{node.name || `Node ${node.id}`}</span>
													{node.status && (
														<Badge variant="outline" className="text-xs">
															{node.status}
														</Badge>
													)}
												</div>
											</SelectItem>
										))}
									</SelectContent>
								</Select>
							)}
							<Button variant="outline" size="sm" asChild disabled={!selectedNodeId}>
								<Link to={`/networks/besu/${selectedNodeId || 0}/blocks`}>
									View All Blocks
									<ExternalLink className="h-4 w-4 ml-2" />
								</Link>
							</Button>
						</div>
					</div>

					{/* Latest Block Info */}
					<Card>
						<CardHeader>
							<CardTitle className="text-base">Latest Block</CardTitle>
							<CardDescription>Current blockchain state</CardDescription>
						</CardHeader>
						<CardContent>
							{latestBlockLoading ? (
								<div className="text-center py-8">
									<div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary mx-auto mb-3"></div>
									<p className="text-sm text-muted-foreground">Loading latest block...</p>
								</div>
							) : (
								<div className="flex items-center justify-between">
									<div className="flex items-center gap-3">
										<TrendingUp className="h-5 w-5 text-green-500" />
										<div>
											<p className="text-2xl font-bold">
												{latestBlockNumber !== undefined && latestBlockNumber !== null
													? Number(latestBlockNumber)
													: '0x0'}
											</p>
											<p className="text-sm text-muted-foreground">Current block height</p>
										</div>
									</div>
									<Badge variant="outline" className="text-sm">
										Latest
									</Badge>
								</div>
							)}
						</CardContent>
					</Card>

					{/* Recent Blocks */}
					<Card>
						<CardHeader>
							<div className="flex items-center justify-between">
								<div>
									<CardTitle className="text-base">Recent Blocks</CardTitle>
									<CardDescription>Latest 5 blocks from the blockchain</CardDescription>
								</div>
								<div className="flex items-center gap-2">
									<Button
										variant="outline"
										size="sm"
										onClick={() => setCurrentPage(Math.max(0, currentPage - 1))}
										disabled={currentPage === 0 || blocksLoading}
									>
										<ChevronLeft className="h-4 w-4" />
									</Button>
									<span className="text-sm text-muted-foreground">
										Page {currentPage + 1} of {totalPages}
									</span>
									<Button
										variant="outline"
										size="sm"
										onClick={() => setCurrentPage(currentPage + 1)}
										disabled={currentPage >= totalPages - 1 || blocksLoading}
									>
										<ChevronRight className="h-4 w-4" />
									</Button>
								</div>
							</div>
						</CardHeader>
						<CardContent>
							{blocksLoading ? (
								<div className="text-center py-8">
									<div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary mx-auto mb-3"></div>
									<p className="text-sm text-muted-foreground">Loading blocks...</p>
								</div>
							) : (
								<div className="space-y-3">
									{blockNumbers.map((blockNumber) => (
										<div key={blockNumber} className="flex items-center justify-between p-3 rounded-lg border bg-muted/30 hover:bg-muted/50 transition-colors">
											<div className="flex items-center gap-3">
												<div className="w-2 h-2 rounded-full bg-blue-500"></div>
												<div>
													<p className="font-medium">Block #{blockNumber}</p>
													<p className="text-sm text-muted-foreground">
														Hex: {formatBlockNumberToHex(blockNumber)}
													</p>
												</div>
											</div>
											<div className="flex items-center gap-2">
												<Button variant="ghost" size="sm" onClick={() => copyToClipboard(blockNumber.toString())} className="h-8 w-8 p-0">
													<Copy className="h-4 w-4" />
												</Button>
												<Button variant="outline" size="sm" asChild>
													<Link to={`/networks/besu/${selectedNodeId || 0}/blocks/${blockNumber}`}>
														View Details
													</Link>
												</Button>
											</div>
										</div>
									))}
									{blockNumbers.length === 0 && (
										<div className="text-center py-8">
											<Package className="h-8 w-8 text-muted-foreground mx-auto mb-3" />
											<p className="text-sm text-muted-foreground">No blocks found</p>
										</div>
									)}
								</div>
							)}
						</CardContent>
					</Card>

					{/* Block Search */}
					<Card>
						<CardHeader>
							<CardTitle className="text-base">Search Blocks</CardTitle>
							<CardDescription>Search for blocks by number or hash</CardDescription>
						</CardHeader>
						<CardContent>
							<div className="space-y-4">
								<div className="flex gap-2">
									<Button
										variant={searchType === 'number' ? 'default' : 'outline'}
										size="sm"
										onClick={() => setSearchType('number')}
									>
										By Number
									</Button>
									<Button
										variant={searchType === 'hash' ? 'default' : 'outline'}
										size="sm"
										onClick={() => setSearchType('hash')}
									>
										By Hash
									</Button>
								</div>
								<div className="flex gap-2">
									<Input
										placeholder={searchType === 'number' ? 'Enter block number' : 'Enter block hash'}
										value={searchBlock}
										onChange={(e) => setSearchBlock(e.target.value)}
										onKeyPress={(e) => e.key === 'Enter' && handleSearch()}
									/>
									<Button onClick={handleSearch} disabled={!searchBlock.trim()}>
										<Search className="h-4 w-4" />
									</Button>
								</div>
							</div>
						</CardContent>
					</Card>

					{/* Search Results */}
					{(blockByNumber || blockByHash) && (
						<Card>
							<CardHeader>
								<CardTitle className="text-base">Block Details</CardTitle>
								<CardDescription>Information about the searched block</CardDescription>
							</CardHeader>
							<CardContent>
								{(blockByNumberLoading || blockByHashLoading) ? (
									<div className="text-center py-8">
										<div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary mx-auto mb-3"></div>
										<p className="text-sm text-muted-foreground">Loading block details...</p>
									</div>
								) : (
									<div className="space-y-4">
										{/* Block Number */}
										<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
											<div className="flex items-center gap-3">
												<Hash className="h-4 w-4 text-muted-foreground" />
												<div>
													<p className="text-sm font-medium">Block Number</p>
													<p className="text-sm text-muted-foreground">
														{String(blockByNumber?.number || blockByHash?.number || 'N/A')}
													</p>
												</div>
											</div>
											<Button
												variant="ghost"
												size="sm"
												onClick={() => copyToClipboard(String(blockByNumber?.number || blockByHash?.number || ''))}
												className="h-8 w-8 p-0"
											>
												<Copy className="h-4 w-4" />
											</Button>
										</div>

										{/* Block Hash */}
										<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
											<div className="flex items-center gap-3">
												<Hash className="h-4 w-4 text-muted-foreground" />
												<div>
													<p className="text-sm font-medium">Block Hash</p>
													<p className="text-sm text-muted-foreground font-mono">
														{formatHash(String(blockByNumber?.hash || blockByHash?.hash || 'N/A'))}
													</p>
												</div>
											</div>
											<Button
												variant="ghost"
												size="sm"
												onClick={() => copyToClipboard(String(blockByNumber?.hash || blockByHash?.hash || ''))}
												className="h-8 w-8 p-0"
											>
												<Copy className="h-4 w-4" />
											</Button>
										</div>

										{/* Parent Hash */}
										<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
											<div className="flex items-center gap-3">
												<Hash className="h-4 w-4 text-muted-foreground" />
												<div>
													<p className="text-sm font-medium">Parent Hash</p>
													<p className="text-sm text-muted-foreground font-mono">
														{formatHash(String(blockByNumber?.parentHash || blockByHash?.parentHash || 'N/A'))}
													</p>
												</div>
											</div>
											<Button
												variant="ghost"
												size="sm"
												onClick={() => copyToClipboard(String(blockByNumber?.parentHash || blockByHash?.parentHash || ''))}
												className="h-8 w-8 p-0"
											>
												<Copy className="h-4 w-4" />
											</Button>
										</div>

										{/* Transaction Count */}
										{searchType === 'number' && (
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
										)}

										{/* Timestamp */}
										<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
											<div className="flex items-center gap-3">
												<Activity className="h-4 w-4 text-muted-foreground" />
												<div>
													<p className="text-sm font-medium">Timestamp</p>
													<p className="text-sm text-muted-foreground">
														{String(blockByNumber?.timestamp || blockByHash?.timestamp || 'N/A')}
													</p>
												</div>
											</div>
										</div>

										{/* Gas Info */}
										<div className="flex items-center justify-between p-3 rounded-lg border bg-muted/30">
											<div className="flex items-center gap-3">
												<TrendingUp className="h-4 w-4 text-muted-foreground" />
												<div>
													<p className="text-sm font-medium">Gas Used</p>
													<p className="text-sm text-muted-foreground">
														{String(blockByNumber?.gasUsed || blockByHash?.gasUsed || 'N/A')}
													</p>
												</div>
											</div>
										</div>
									</div>
								)}
							</CardContent>
						</Card>
					)}

					{/* No Results Message */}
					{searchBlock && !blockByNumber && !blockByHash && !blockByNumberLoading && !blockByHashLoading && (
						<Card>
							<CardContent className="text-center py-8">
								<Package className="h-8 w-8 text-muted-foreground mx-auto mb-3" />
								<p className="text-sm text-muted-foreground">No block found with the provided {searchType}</p>
							</CardContent>
						</Card>
					)}
				</div>
			) : (
				<Card>
					<CardContent className="text-center py-12">
						<Server className="h-8 w-8 text-muted-foreground mx-auto mb-3" />
						<p className="text-muted-foreground">
							{networkNodes.length > 0 
								? 'Please select a node to explore blocks' 
								: 'No nodes available for this network'
							}
						</p>
					</CardContent>
				</Card>
			)}
		</div>
	)
} 
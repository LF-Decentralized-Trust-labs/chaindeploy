import { Activity, ArrowLeft, ChevronLeft, ChevronRight, Copy, Package, Search, TrendingUp } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import {
	getNodesByIdRpcBlockByNumberOptions,
	getNodesByIdRpcBlockNumberOptions,
} from '@/api/client/@tanstack/react-query.gen'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'

export default function BesuBlocksPage() {
	const { nodeId } = useParams<{ nodeId: string }>()
	const [currentPage, setCurrentPage] = useState(0)
	const [copiedAddresses, setCopiedAddresses] = useState<Set<string>>(new Set())
	const [jumpToBlock, setJumpToBlock] = useState<string>('')
	const blocksPerPage = 10

	const nodeIdNumber = nodeId ? parseInt(nodeId) : 0

	// Get latest block number
	const { data: latestBlockNumber, isLoading: latestBlockLoading } = useQuery({
		...getNodesByIdRpcBlockNumberOptions({
			path: { id: nodeIdNumber },
		}),
		enabled: !!nodeIdNumber,
	})

	// Calculate the range of blocks to fetch
	const latestBlock = typeof latestBlockNumber === 'string' ? parseInt(latestBlockNumber, 16) : latestBlockNumber || 0
	const startBlock = latestBlock ? Math.max(0, latestBlock - (currentPage * blocksPerPage)) : 0
	const endBlock = latestBlock ? Math.max(0, latestBlock - ((currentPage + 1) * blocksPerPage) + 1) : 0

	// Get blocks for the current page
	const { data: blocks, isLoading: blocksLoading } = useQuery({
		...getNodesByIdRpcBlockByNumberOptions({
			path: { id: nodeIdNumber },
			query: { number: startBlock.toString(), tag: startBlock.toString() },
		}),
		enabled: !!nodeIdNumber && !!latestBlock && startBlock >= 0,
	})

	// Reset to first page when node changes
	useEffect(() => {
		setCurrentPage(0)
	}, [nodeId])

	const handleJumpToBlock = () => {
		if (!jumpToBlock.trim()) {
			toast.error('Please enter a block number')
			return
		}

		const blockNumber = parseInt(jumpToBlock)
		if (isNaN(blockNumber) || blockNumber < 0) {
			toast.error('Please enter a valid block number')
			return
		}

		if (blockNumber > latestBlock) {
			toast.error(`Block ${blockNumber} doesn't exist. Latest block is ${latestBlock}`)
			return
		}

		// Calculate which page this block would be on
		// Since we show blocks in descending order (latest first), we need to calculate the page
		const blockIndex = latestBlock - blockNumber
		const targetPage = Math.floor(blockIndex / blocksPerPage)
		
		setCurrentPage(targetPage)
		setJumpToBlock('')
		toast.success(`Jumped to page containing block ${blockNumber}`)
	}

	const copyToClipboard = (text: string) => {
		navigator.clipboard.writeText(text)
		setCopiedAddresses((prev) => new Set([...prev, text]))
		// Reset the checkmark after 2 seconds
		setTimeout(() => {
			setCopiedAddresses((prev) => {
				const newSet = new Set(prev)
				newSet.delete(text)
				return newSet
			})
		}, 2000)
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

	return (
		<div className="flex-1 p-8">
			<div className="max-w-7xl mx-auto">
				<div className="flex items-center gap-2 text-muted-foreground mb-8">
					<Button variant="ghost" size="sm" asChild>
						<Link to={`/networks/besu/${nodeId}`}>
							<ArrowLeft className="mr-2 h-4 w-4" />
							Back to Network
						</Link>
					</Button>
				</div>

				<div className="mb-6">
					<div className="flex items-center justify-between">
						<div>
							<h1 className="text-2xl font-semibold">All Blocks</h1>
							<p className="text-muted-foreground">Browse all blocks for node {nodeId}</p>
						</div>
						{!latestBlockLoading && (
							<Badge variant="outline" className="text-sm">
								Total: {latestBlock} blocks
							</Badge>
						)}
					</div>
				</div>

				{/* Latest Block Info */}
				<Card className="mb-6">
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
										<p className="text-2xl font-bold">{latestBlock}</p>
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

				{/* Blocks List */}
				<Card>
					<CardHeader>
						<div className="flex items-center justify-between">
							<div>
								<CardTitle className="text-base">Blocks</CardTitle>
								<CardDescription>Showing blocks {startBlock} to {endBlock}</CardDescription>
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
						{/* Jump to Block */}
						<div className="mb-6 p-4 border rounded-lg bg-muted/30">
							<div className="flex items-center gap-3 mb-2">
								<Search className="h-4 w-4 text-muted-foreground" />
								<h3 className="text-sm font-medium">Jump to Block</h3>
							</div>
							<div className="flex gap-2">
								<Input
									placeholder="Enter block number (e.g., 1000)"
									value={jumpToBlock}
									onChange={(e) => setJumpToBlock(e.target.value)}
									onKeyPress={(e) => e.key === 'Enter' && handleJumpToBlock()}
									className="flex-1"
								/>
								<Button 
									onClick={handleJumpToBlock} 
									disabled={!jumpToBlock.trim() || latestBlockLoading}
									size="sm"
								>
									Jump
								</Button>
							</div>
							<p className="text-xs text-muted-foreground mt-2">
								Enter a block number to jump to the page containing that block
							</p>
						</div>

						{blocksLoading ? (
							<div className="text-center py-8">
								<div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary mx-auto mb-3"></div>
								<p className="text-sm text-muted-foreground">Loading blocks...</p>
							</div>
						) : (
							<div className="space-y-3">
								{blockNumbers.map((blockNumber) => (
									<div key={blockNumber} className="flex items-center justify-between p-4 rounded-lg border bg-muted/30 hover:bg-muted/50 transition-colors">
										<div className="flex items-center gap-4">
											<div className="w-3 h-3 rounded-full bg-blue-500"></div>
											<div>
												<p className="font-medium text-lg">Block #{blockNumber}</p>
												<p className="text-sm text-muted-foreground">
													Hex: {formatBlockNumberToHex(blockNumber)}
												</p>
											</div>
										</div>
										<div className="flex items-center gap-2">
											<Button 
												variant="ghost" 
												size="sm" 
												onClick={() => copyToClipboard(blockNumber.toString())} 
												className="h-8 w-8 p-0"
											>
												{copiedAddresses.has(blockNumber.toString()) ? (
													<Activity className="h-4 w-4 text-green-500" />
												) : (
													<Copy className="h-4 w-4" />
												)}
											</Button>
											<Button variant="outline" size="sm" asChild>
												<Link to={`/networks/besu/${nodeId}/blocks/${blockNumber}`}>
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
			</div>
		</div>
	)
} 
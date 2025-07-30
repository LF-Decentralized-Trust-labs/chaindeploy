import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
	getNodesByIdRpcAccountsOptions,
	getNodesByIdRpcBalanceOptions,
	getNodesByIdRpcBlockNumberOptions,
	getNodesByIdRpcChainIdOptions,
	getNodesByIdRpcProtocolVersionOptions,
	getNodesByIdRpcSyncingOptions,
	getNodesByIdRpcTransactionCountOptions,
	getNodesByIdRpcBlockByNumberOptions,
	getNodesByIdRpcBlockByHashOptions,
	getNodesByIdRpcCodeOptions,
	getNodesByIdRpcStorageOptions,
	getNodesByIdRpcQbftRequestTimeoutOptions,
	getNodesByIdRpcQbftSignerMetricsOptions,
	getNodesByIdRpcQbftPendingVotesOptions,
	getNodesByIdRpcQbftValidatorsByBlockHashOptions,
	getNodesByIdRpcQbftValidatorsByBlockNumberOptions,
} from '@/api/client/@tanstack/react-query.gen'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { Copy, RefreshCw, Search, ExternalLink, Check } from 'lucide-react'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'

interface BesuNodeNetworkProps {
	nodeId: number
}

export function BesuNodeNetwork({ nodeId }: BesuNodeNetworkProps) {
	const [address, setAddress] = useState('')
	const [blockNumber, setBlockNumber] = useState('')
	const [blockHash, setBlockHash] = useState('')

	const [storageAddress, setStorageAddress] = useState('')
	const [storagePosition, setStoragePosition] = useState('')
	const [copiedAddresses, setCopiedAddresses] = useState<Set<string>>(new Set())

	const [selectedTab, setSelectedTab] = useState('overview')
	const [voteLoading, setVoteLoading] = useState(false)
	const [discardLoading, setDiscardLoading] = useState(false)

	const queryClient = useQueryClient()

	// Core network queries
	const { data: chainId, isLoading: chainIdLoading } = useQuery({
		...getNodesByIdRpcChainIdOptions({
			path: { id: nodeId },
		}),
	})

	const { data: protocolVersion, isLoading: protocolVersionLoading } = useQuery({
		...getNodesByIdRpcProtocolVersionOptions({
			path: { id: nodeId },
		}),
	})

	const { data: latestBlock, isLoading: latestBlockLoading } = useQuery({
		...getNodesByIdRpcBlockNumberOptions({
			path: { id: nodeId },
		}),
	})

	const { data: syncStatus, isLoading: syncStatusLoading } = useQuery({
		...getNodesByIdRpcSyncingOptions({
			path: { id: nodeId },
		}),
	})

	const { data: accounts, isLoading: accountsLoading } = useQuery({
		...getNodesByIdRpcAccountsOptions({
			path: { id: nodeId },
		}),
	})

	// Conditional queries based on user input
	const { data: balance, isLoading: balanceLoading } = useQuery({
		...getNodesByIdRpcBalanceOptions({
			path: { id: nodeId },
			query: { address, blockTag: 'latest' },
		}),
		enabled: !!address,
	})

	const { data: transactionCount, isLoading: transactionCountLoading } = useQuery({
		...getNodesByIdRpcTransactionCountOptions({
			path: { id: nodeId },
			query: { address, blockTag: 'latest' },
		}),
		enabled: !!address,
	})

	const { data: blockByNumber, isLoading: blockByNumberLoading } = useQuery({
		...getNodesByIdRpcBlockByNumberOptions({
			path: { id: nodeId },
			query: { number: blockNumber, tag: blockNumber },
		}),
		enabled: !!blockNumber,
	})

	const { data: blockByHash, isLoading: blockByHashLoading } = useQuery({
		...getNodesByIdRpcBlockByHashOptions({
			path: { id: nodeId },
			query: { hash: blockHash },
		}),
		enabled: !!blockHash,
	})



	const { data: contractCode, isLoading: contractCodeLoading } = useQuery({
		...getNodesByIdRpcCodeOptions({
			path: { id: nodeId },
			query: { address, blockTag: 'latest' },
		}),
		enabled: !!address,
	})

	const { data: storageValue, isLoading: storageValueLoading } = useQuery({
		...getNodesByIdRpcStorageOptions({
			path: { id: nodeId },
			query: { address: storageAddress, position: storagePosition, blockTag: 'latest' },
		}),
		enabled: !!storageAddress && !!storagePosition,
	})

	// QBFT queries
	const { data: qbftRequestTimeout, isLoading: qbftRequestTimeoutLoading } = useQuery({
		...getNodesByIdRpcQbftRequestTimeoutOptions({
			path: { id: nodeId },
		}),
	})

	const { data: qbftSignerMetrics, isLoading: qbftSignerMetricsLoading } = useQuery({
		...getNodesByIdRpcQbftSignerMetricsOptions({
			path: { id: nodeId },
		}),
	})

	// Additional QBFT queries
	const { data: qbftPendingVotes, isLoading: qbftPendingVotesLoading } = useQuery({
		...getNodesByIdRpcQbftPendingVotesOptions({
			path: { id: nodeId },
		}),
	})

	const { data: qbftValidatorsByBlockHash, isLoading: qbftValidatorsByBlockHashLoading } = useQuery({
		...getNodesByIdRpcQbftValidatorsByBlockHashOptions({
			path: { id: nodeId },
			query: { blockHash: 'latest' },
		}),
	})

	const { data: qbftValidatorsByBlockNumber, isLoading: qbftValidatorsByBlockNumberLoading } = useQuery({
		...getNodesByIdRpcQbftValidatorsByBlockNumberOptions({
			path: { id: nodeId },
			query: { blockNumber: 'latest' },
		}),
	})

	// QBFT voting mutations
	const handleVote = async (validator: string, vote: boolean) => {
		setVoteLoading(true)
		try {
			// TODO: Implement direct API call for voting
			console.log('Voting for validator:', validator, 'vote:', vote)
			// Refetch pending votes after successful vote
			queryClient.invalidateQueries({
				queryKey: getNodesByIdRpcQbftPendingVotesOptions({ path: { id: nodeId } }).queryKey,
			})
		} catch (error) {
			console.error('Error voting:', error)
		} finally {
			setVoteLoading(false)
		}
	}

	const handleDiscardVote = async (validator: string) => {
		setDiscardLoading(true)
		try {
			// TODO: Implement direct API call for discarding vote
			console.log('Discarding vote for validator:', validator)
			// Refetch pending votes after successful discard
			queryClient.invalidateQueries({
				queryKey: getNodesByIdRpcQbftPendingVotesOptions({ path: { id: nodeId } }).queryKey,
			})
		} catch (error) {
			console.error('Error discarding vote:', error)
		} finally {
			setDiscardLoading(false)
		}
	}

	const copyToClipboard = (text: string) => {
		navigator.clipboard.writeText(text)
		setCopiedAddresses(prev => new Set([...prev, text]))
		// Reset the checkmark after 2 seconds
		setTimeout(() => {
			setCopiedAddresses(prev => {
				const newSet = new Set(prev)
				newSet.delete(text)
				return newSet
			})
		}, 2000)
	}

	const formatWeiToEth = (wei: string) => {
		const weiBigInt = BigInt(wei)
		const eth = Number(weiBigInt) / Math.pow(10, 18)
		return eth.toFixed(6)
	}

	const formatHex = (hex: string) => {
		if (!hex) return 'N/A'
		return hex.startsWith('0x') ? hex : `0x${hex}`
	}

	const formatHexToDecimal = (hex: string) => {
		if (!hex) return 'N/A'
		try {
			return parseInt(hex, 16).toString()
		} catch {
			return hex
		}
	}

	return (
		<div className="space-y-6">
			<Tabs defaultValue="overview" className="space-y-4">
				<TabsList>
					<TabsTrigger value="overview">Overview</TabsTrigger>
					<TabsTrigger value="accounts">Accounts</TabsTrigger>
					<TabsTrigger value="blocks">Blocks</TabsTrigger>
					<TabsTrigger value="contracts">Contracts</TabsTrigger>
					<TabsTrigger value="validators">Validators</TabsTrigger>
				</TabsList>

				<TabsContent value="overview" className="space-y-4">
					<div className="grid gap-4 md:grid-cols-2">
						<Card>
							<CardHeader>
								<CardTitle className="flex items-center gap-2">
									Network Information
									<RefreshCw className={cn('h-4 w-4', { 'animate-spin': chainIdLoading || protocolVersionLoading || latestBlockLoading || syncStatusLoading || qbftRequestTimeoutLoading || qbftSignerMetricsLoading || qbftPendingVotesLoading || qbftValidatorsByBlockHashLoading || qbftValidatorsByBlockNumberLoading })} />
								</CardTitle>
							</CardHeader>
							<CardContent className="space-y-4">
								<div className="grid grid-cols-2 gap-4">
									<div>
										<Label className="text-sm font-medium text-muted-foreground">Chain ID</Label>
										<p className="text-lg font-mono">{chainIdLoading ? 'Loading...' : chainId ? formatHexToDecimal(chainId) : 'N/A'}</p>
									</div>
									<div>
										<Label className="text-sm font-medium text-muted-foreground">Protocol Version</Label>
										<p className="text-lg font-mono">{protocolVersionLoading ? 'Loading...' : protocolVersion ? formatHexToDecimal(protocolVersion) : 'N/A'}</p>
									</div>
									<div>
										<Label className="text-sm font-medium text-muted-foreground">Latest Block</Label>
										<p className="text-lg font-mono">{latestBlockLoading ? 'Loading...' : latestBlock ? formatHexToDecimal(latestBlock) : 'N/A'}</p>
									</div>
									<div>
										<Label className="text-sm font-medium text-muted-foreground mb-1 block">Sync Status</Label>
										<Badge variant={syncStatusLoading ? 'secondary' : syncStatus ? 'destructive' : 'default'}>
											{syncStatusLoading ? 'Loading...' : syncStatus ? 'Syncing' : 'Synced'}
										</Badge>
									</div>
									<div>
										<Label className="text-sm font-medium text-muted-foreground">QBFT Request Timeout</Label>
										<p className="text-lg font-mono">{qbftRequestTimeoutLoading ? 'Loading...' : qbftRequestTimeout ? `${qbftRequestTimeout}s` : 'N/A'}</p>
									</div>
									<div>
										<Label className="text-sm font-medium text-muted-foreground">QBFT Signer Metrics</Label>
										{qbftSignerMetricsLoading ? (
											<p className="text-lg font-mono">Loading...</p>
										) : qbftSignerMetrics ? (
											<div className="space-y-2">
												{Array.isArray(qbftSignerMetrics) ? (
													qbftSignerMetrics.map((signer, index) => (
														<div key={index} className="p-2 border rounded text-xs">
															<div className="flex justify-between mb-1">
																<span className="text-muted-foreground">Address:</span>
																<div className="flex items-center gap-1">
																	<span className="font-mono">{signer.address.length > 12 ? `${signer.address.slice(0, 8)}...${signer.address.slice(-4)}` : signer.address}</span>
																	<Button
																		variant="ghost"
																		size="sm"
																		onClick={() => copyToClipboard(signer.address)}
																		className="h-4 w-4 p-0"
																	>
																		{copiedAddresses.has(signer.address) ? (
																			<Check className="h-3 w-3 text-green-500" />
																		) : (
																			<Copy className="h-3 w-3" />
																		)}
																	</Button>
																</div>
															</div>
															<div className="flex justify-between mb-1">
																<span className="text-muted-foreground">Proposed Blocks:</span>
																<span className="font-mono">{parseInt(signer.proposedBlockCount, 16)}</span>
															</div>
															<div className="flex justify-between">
																<span className="text-muted-foreground">Last Proposed:</span>
																<span className="font-mono">{parseInt(signer.lastProposedBlockNumber, 16)}</span>
															</div>
														</div>
													))
												) : (
													<p className="text-lg font-mono">Invalid data format</p>
												)}
											</div>
										) : (
											<p className="text-lg font-mono">N/A</p>
										)}
									</div>
								</div>
								
								{syncStatus && typeof syncStatus === 'object' && (
									<div className="mt-4 p-3 bg-muted rounded-lg">
										<h4 className="text-sm font-medium mb-2">Sync Details</h4>
										<div className="grid grid-cols-2 gap-4 text-xs">
											<div>
												<span className="text-muted-foreground">Starting Block:</span>
												<p className="font-mono">{(syncStatus as any)?.startingBlock || 'N/A'}</p>
											</div>
											<div>
												<span className="text-muted-foreground">Current Block:</span>
												<p className="font-mono">{(syncStatus as any)?.currentBlock || 'N/A'}</p>
											</div>
											<div>
												<span className="text-muted-foreground">Highest Block:</span>
												<p className="font-mono">{(syncStatus as any)?.highestBlock || 'N/A'}</p>
											</div>
											<div>
												<span className="text-muted-foreground">Synced Accounts:</span>
												<p className="font-mono">{(syncStatus as any)?.syncedAccounts || 'N/A'}</p>
											</div>
										</div>
									</div>
								)}
							</CardContent>
						</Card>

						<Card>
							<CardHeader>
								<CardTitle>Managed Accounts</CardTitle>
								<CardDescription>Accounts managed by this node</CardDescription>
							</CardHeader>
							<CardContent>
								{accountsLoading ? (
									<p>Loading accounts...</p>
								) : accounts && accounts.length > 0 ? (
									<div className="space-y-2">
										{accounts.map((account, index) => (
											<div key={index} className="flex items-center justify-between p-2 rounded border">
												<span className="font-mono text-sm">{account}</span>
												<Button
													variant="ghost"
													size="sm"
													onClick={() => copyToClipboard(account)}
												>
													<Copy className="h-4 w-4" />
												</Button>
											</div>
										))}
									</div>
								) : (
									<p className="text-muted-foreground">No managed accounts found</p>
								)}
							</CardContent>
						</Card>
					</div>
				</TabsContent>

				<TabsContent value="accounts" className="space-y-4">
					<Card>
						<CardHeader>
							<CardTitle>Account Information</CardTitle>
							<CardDescription>Query account balances and transaction counts</CardDescription>
						</CardHeader>
						<CardContent className="space-y-4">
							<div className="space-y-2">
								<Label htmlFor="address">Address</Label>
								<div className="flex gap-2">
									<Input
										id="address"
										placeholder="0x..."
										value={address}
										onChange={(e) => setAddress(e.target.value)}
									/>
									<Button variant="outline" onClick={() => setAddress('')}>
										Clear
									</Button>
								</div>
							</div>

							{address && (
								<div className="grid gap-4 md:grid-cols-2">
									<Card>
										<CardHeader>
											<CardTitle className="text-sm">Balance</CardTitle>
										</CardHeader>
										<CardContent>
											{balanceLoading ? (
												<p>Loading...</p>
											) : balance ? (
												<div className="space-y-2">
													<p className="text-lg font-mono">{formatWeiToEth(balance)} ETH</p>
													<p className="text-sm text-muted-foreground">{balance} Wei</p>
													<Button
														variant="ghost"
														size="sm"
														onClick={() => copyToClipboard(balance)}
													>
														<Copy className="h-4 w-4" />
													</Button>
												</div>
											) : (
												<p className="text-muted-foreground">No balance data</p>
											)}
										</CardContent>
									</Card>

									<Card>
										<CardHeader>
											<CardTitle className="text-sm">Transaction Count</CardTitle>
										</CardHeader>
										<CardContent>
											{transactionCountLoading ? (
												<p>Loading...</p>
											) : transactionCount ? (
												<div className="space-y-2">
													<p className="text-lg font-mono">{transactionCount}</p>
													<Button
														variant="ghost"
														size="sm"
														onClick={() => copyToClipboard(transactionCount)}
													>
														<Copy className="h-4 w-4" />
													</Button>
												</div>
											) : (
												<p className="text-muted-foreground">No transaction count data</p>
											)}
										</CardContent>
									</Card>
								</div>
							)}
						</CardContent>
					</Card>
				</TabsContent>

				<TabsContent value="blocks" className="space-y-4">
					<div className="grid gap-4 md:grid-cols-2">
						<Card>
							<CardHeader>
								<CardTitle>Block by Number</CardTitle>
								<CardDescription>Get block details by block number</CardDescription>
							</CardHeader>
							<CardContent className="space-y-4">
								<div className="space-y-2">
									<Label htmlFor="blockNumber">Block Number</Label>
									<div className="flex gap-2">
										<Input
											id="blockNumber"
											placeholder="12345"
											value={blockNumber}
											onChange={(e) => setBlockNumber(e.target.value)}
										/>
										<Button variant="outline" onClick={() => setBlockNumber('')}>
											Clear
										</Button>
									</div>
								</div>

								{blockByNumber && (
									<div className="space-y-2">
										<Separator />
										<div className="space-y-2">
											<div className="flex justify-between">
												<span className="text-sm font-medium">Hash:</span>
												<span className="font-mono text-sm">{formatHex((blockByNumber as any)?.hash)}</span>
											</div>
											<div className="flex justify-between">
												<span className="text-sm font-medium">Number:</span>
												<span className="font-mono text-sm">{(blockByNumber as any)?.number}</span>
											</div>
											<div className="flex justify-between">
												<span className="text-sm font-medium">Transactions:</span>
												<span className="font-mono text-sm">{(blockByNumber as any)?.transactions?.length || 0}</span>
											</div>
										</div>
									</div>
								)}
							</CardContent>
						</Card>

						<Card>
							<CardHeader>
								<CardTitle>Block by Hash</CardTitle>
								<CardDescription>Get block details by block hash</CardDescription>
							</CardHeader>
							<CardContent className="space-y-4">
								<div className="space-y-2">
									<Label htmlFor="blockHash">Block Hash</Label>
									<div className="flex gap-2">
										<Input
											id="blockHash"
											placeholder="0x..."
											value={blockHash}
											onChange={(e) => setBlockHash(e.target.value)}
										/>
										<Button variant="outline" onClick={() => setBlockHash('')}>
											Clear
										</Button>
									</div>
								</div>

								{blockByHash && (
									<div className="space-y-2">
										<Separator />
										<div className="space-y-2">
											<div className="flex justify-between">
												<span className="text-sm font-medium">Hash:</span>
												<span className="font-mono text-sm">{formatHex((blockByHash as any)?.hash)}</span>
											</div>
											<div className="flex justify-between">
												<span className="text-sm font-medium">Number:</span>
												<span className="font-mono text-sm">{(blockByHash as any)?.number}</span>
											</div>
											<div className="flex justify-between">
												<span className="text-sm font-medium">Transactions:</span>
												<span className="font-mono text-sm">{(blockByHash as any)?.transactions?.length || 0}</span>
											</div>
										</div>
									</div>
								)}
							</CardContent>
						</Card>
					</div>
				</TabsContent>



				<TabsContent value="contracts" className="space-y-4">
					<div className="grid gap-4 md:grid-cols-2">
						<Card>
							<CardHeader>
								<CardTitle>Contract Code</CardTitle>
								<CardDescription>Get bytecode at an address</CardDescription>
							</CardHeader>
							<CardContent className="space-y-4">
								<div className="space-y-2">
									<Label htmlFor="contractAddress">Contract Address</Label>
									<div className="flex gap-2">
										<Input
											id="contractAddress"
											placeholder="0x..."
											value={address}
											onChange={(e) => setAddress(e.target.value)}
										/>
										<Button variant="outline" onClick={() => setAddress('')}>
											Clear
										</Button>
									</div>
								</div>

								{contractCode && (
									<div className="space-y-2">
										<Separator />
										<div className="space-y-2">
											<div className="flex justify-between">
												<span className="text-sm font-medium">Has Code:</span>
												<Badge variant={contractCode && contractCode !== '0x' ? 'default' : 'secondary'}>
													{contractCode && contractCode !== '0x' ? 'Yes' : 'No'}
												</Badge>
											</div>
											{contractCode && contractCode !== '0x' && (
												<div className="space-y-2">
													<span className="text-sm font-medium">Bytecode:</span>
													<div className="bg-muted p-2 rounded text-xs font-mono max-h-32 overflow-y-auto">
														{contractCode}
													</div>
													<Button
														variant="ghost"
														size="sm"
														onClick={() => copyToClipboard(contractCode)}
													>
														<Copy className="h-4 w-4" />
													</Button>
												</div>
											)}
										</div>
									</div>
								)}
							</CardContent>
						</Card>

						<Card>
							<CardHeader>
								<CardTitle>Storage Value</CardTitle>
								<CardDescription>Get storage value at a position</CardDescription>
							</CardHeader>
							<CardContent className="space-y-4">
								<div className="space-y-2">
									<Label htmlFor="storageAddress">Contract Address</Label>
									<Input
										id="storageAddress"
										placeholder="0x..."
										value={storageAddress}
										onChange={(e) => setStorageAddress(e.target.value)}
									/>
								</div>
								<div className="space-y-2">
									<Label htmlFor="storagePosition">Storage Position</Label>
									<div className="flex gap-2">
										<Input
											id="storagePosition"
											placeholder="0x..."
											value={storagePosition}
											onChange={(e) => setStoragePosition(e.target.value)}
										/>
										<Button variant="outline" onClick={() => {
											setStorageAddress('')
											setStoragePosition('')
										}}>
											Clear
										</Button>
									</div>
								</div>

								{storageValue && (
									<div className="space-y-2">
										<Separator />
										<div className="space-y-2">
											<div className="flex justify-between">
												<span className="text-sm font-medium">Storage Value:</span>
												<span className="font-mono text-sm">{formatHex(storageValue)}</span>
											</div>
											<Button
												variant="ghost"
												size="sm"
												onClick={() => copyToClipboard(storageValue)}
											>
												<Copy className="h-4 w-4" />
											</Button>
										</div>
									</div>
								)}
							</CardContent>
						</Card>
					</div>
				</TabsContent>

				<TabsContent value="validators" className="space-y-4">
					<div className="grid gap-4 md:grid-cols-2">
						<Card>
							<CardHeader>
								<CardTitle>Current Validators</CardTitle>
								<CardDescription>Validators for the latest block</CardDescription>
							</CardHeader>
							<CardContent>
								{qbftValidatorsByBlockNumberLoading ? (
									<p>Loading validators...</p>
								) : qbftValidatorsByBlockNumber && qbftValidatorsByBlockNumber.length > 0 ? (
									<div className="space-y-2">
										{qbftValidatorsByBlockNumber.map((validator, index) => (
											<div key={index} className="flex items-center justify-between p-2 rounded border">
												<span className="font-mono text-sm">{validator}</span>
												<Button
													variant="ghost"
													size="sm"
													onClick={() => copyToClipboard(validator)}
												>
													<Copy className="h-4 w-4" />
												</Button>
											</div>
										))}
									</div>
								) : (
									<p>No validators found</p>
								)}
							</CardContent>
						</Card>

						<Card>
							<CardHeader>
								<CardTitle>Pending Votes</CardTitle>
								<CardDescription>Votes waiting for consensus</CardDescription>
							</CardHeader>
							<CardContent>
								{qbftPendingVotesLoading ? (
									<p>Loading pending votes...</p>
								) : qbftPendingVotes && Object.keys(qbftPendingVotes).length > 0 ? (
									<div className="space-y-4">
										{Object.entries(qbftPendingVotes).map(([validator, votes]) => (
											<div key={validator} className="p-4 border rounded-lg">
												<div className="flex items-center justify-between mb-2">
													<span className="font-medium">Validator: {validator}</span>
													<div className="flex gap-2">
														<Button
															size="sm"
															variant="outline"
															onClick={() => handleVote(validator, true)}
															disabled={voteLoading}
														>
															Approve
														</Button>
														<Button
															size="sm"
															variant="outline"
															onClick={() => handleVote(validator, false)}
															disabled={voteLoading}
														>
															Reject
														</Button>
														<Button
															size="sm"
															variant="destructive"
															onClick={() => handleDiscardVote(validator)}
															disabled={discardLoading}
														>
															Discard
														</Button>
													</div>
												</div>
												<div className="space-y-1">
													{votes.map((vote, index) => (
														<div key={index} className="text-sm text-muted-foreground">
															Proposer: {vote.proposer} - Vote: {vote.vote ? 'Approve' : 'Reject'}
														</div>
													))}
												</div>
											</div>
										))}
									</div>
								) : (
									<p>No pending votes</p>
								)}
							</CardContent>
						</Card>
					</div>
				</TabsContent>
			</Tabs>
		</div>
	)
} 
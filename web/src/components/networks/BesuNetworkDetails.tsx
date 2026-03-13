import { HttpBesuNetworkResponse, HttpNodeResponse } from '@/api/client'
import { getNodesByIdRpcBlockNumberOptions, getNodesPlatformByPlatformOptions } from '@/api/client/@tanstack/react-query.gen'
import { BesuSmartContractTutorial } from '@/components/networks/BesuSmartContractTutorial'
import { BesuValidatorsTab } from '@/components/networks/BesuValidatorsTab'
import { BesuBlockExplorer } from '@/components/networks/BesuBlockExplorer'
import { ValidatorList } from '@/components/networks/validator-list'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { Activity, ArrowLeft, Blocks, Code, Copy, Edit, Network, Save, X } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'
import * as z from 'zod'
import rlp from 'rlp'
import { Buffer } from 'buffer'
import { BesuIcon } from '../icons/besu-icon'
import { Alert, AlertDescription } from '../ui/alert'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '../ui/form'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import { Textarea } from '../ui/textarea'
import { TimeAgo } from '../ui/time-ago'
import { BesuNetworkTabs, BesuTabValue } from './besu-network-tabs'

interface BesuConfig {
	type: string
	networkId: number
	chainId: number
	consensus: string
	initialValidators: number[]
	blockPeriod: number
	epochLength: number
	requestTimeout: number
	nonce: string
	timestamp: string
	gasLimit: string
	difficulty: string
	mixHash: string
	coinbase: string
}

interface BesuGenesisConfig {
	config: {
		chainId: number
		berlinBlock: number
		qbft: {
			blockperiodseconds: number
			epochlength: number
			requesttimeoutseconds: number
			startBlock: number
		}
	}
	nonce: string
	timestamp: string
	gasLimit: string
	difficulty: string
	mixHash: string
	coinbase: string
	alloc: Record<string, { balance: string }>
	extraData: string
	number: string
	gasUsed: string
	parentHash: string
}

interface BesuNetworkDetailsProps {
	network: HttpBesuNetworkResponse & {
		platform: string
		config: BesuConfig
		genesisConfig: BesuGenesisConfig
	}
}

// Form schema for genesis JSON validation
const genesisFormSchema = z.object({
	genesisJson: z
		.string()
		.refine(
			(val) => {
				try {
					JSON.parse(val)
					return true
				} catch {
					return false
				}
			},
			{
				message: 'Invalid JSON format',
			}
		)
		.refine(
			(val) => {
				try {
					const parsed = JSON.parse(val)
					// Basic validation for required genesis fields
					return parsed.config && typeof parsed.config.chainId === 'number' && parsed.nonce && parsed.timestamp && parsed.gasLimit && parsed.difficulty && parsed.mixHash && parsed.coinbase
				} catch {
					return false
				}
			},
			{
				message: 'Missing required genesis fields (config.chainId, nonce, timestamp, gasLimit, difficulty, mixHash, coinbase)',
			}
		),
})

type GenesisFormValues = z.infer<typeof genesisFormSchema>

// Utility function to parse validators from Besu QBFT genesis extraData using RLP
const parseValidatorsFromBesuQbftExtraData = (extraData: string): { vanity: string; proposer: string; validators: string[]; seals: string[] } => {
	if (!extraData || extraData === '0x') {
		return { vanity: '', proposer: '', validators: [], seals: [] }
	}

	try {
		// Remove the '0x' prefix and convert hex to a Buffer
		const encoded = Buffer.from(extraData.slice(2), 'hex')

		// Decode the RLP data
		const decoded = rlp.decode(encoded)

		// Helper function to convert Buffer/Uint8Array to hex string
		const bufferToHex = (buffer: Buffer | Uint8Array): string => {
			if (Buffer.isBuffer(buffer)) {
				return '0x' + buffer.toString('hex')
			} else if (buffer instanceof Uint8Array) {
				return '0x' + Array.from(buffer).map(b => b.toString(16).padStart(2, '0')).join('')
			}
			return '0x' + buffer.toString()
		}

		// Helper function to check if something is a buffer-like object
		const isBufferLike = (obj: unknown): obj is Buffer | Uint8Array => {
			return Buffer.isBuffer(obj) || obj instanceof Uint8Array
		}

		// Parse the decoded data
		// Besu QBFT extraData format: [vanity, validators, seals]
		const vanity = decoded[0] && isBufferLike(decoded[0]) ? bufferToHex(decoded[0]) : ''
		const validators = decoded[1] ? 
			(Array.isArray(decoded[1]) ? 
				decoded[1].filter(isBufferLike).map(v => bufferToHex(v)) : 
				isBufferLike(decoded[1]) ? [bufferToHex(decoded[1])] : []
			) : []
		const seals = decoded[2] ? 
			(Array.isArray(decoded[2]) ? 
				decoded[2].filter(isBufferLike).map(s => bufferToHex(s)) : 
				isBufferLike(decoded[2]) ? [bufferToHex(decoded[2])] : []
			) : []

		return { vanity, proposer: '', validators, seals }
	} catch (error) {
		console.error('Error parsing Besu QBFT extraData with RLP:', error)

		// Fallback: try to parse as raw hex if RLP fails
		try {
			const cleanData = extraData.startsWith('0x') ? extraData.slice(2) : extraData
			if (cleanData.length >= 40) {
				// Try to extract at least one validator address
				const validator = `0x${cleanData.slice(0, 40)}`
				return { vanity: '', proposer: '', validators: [validator], seals: [] }
			}
		} catch (fallbackError) {
			console.error('Fallback parsing also failed:', fallbackError)
		}

		return { vanity: '', proposer: '', validators: [], seals: [] }
	}
}

function BesuNodeCard({ node }: { node: HttpNodeResponse }) {
	const { data: blockNumberHex } = useQuery({
		...getNodesByIdRpcBlockNumberOptions({
			path: { id: node.id! },
		}),
		refetchInterval: 5000,
	})
	const blockHeight = blockNumberHex ? parseInt(blockNumberHex, 16) : undefined

	return (
		<Card className="p-3">
			<div className="flex items-center gap-3">
				<div className="h-8 w-8 rounded-full bg-primary/10 flex items-center justify-center">
					<BesuIcon className="h-4 w-4 text-primary" />
				</div>
				<div>
					<div className="flex items-center gap-2">
						<Link to={`/nodes/${node.id}`} className="font-medium hover:underline">
							{node.name}
						</Link>
						<Badge variant={node.status?.toLowerCase() === 'running' ? 'default' : node.status?.toLowerCase() === 'stopped' || node.status?.toLowerCase() === 'error' ? 'destructive' : 'secondary'}>
							<Activity className="mr-1 h-3 w-3" />
							{node.status}
						</Badge>
					</div>
					<div className="flex items-center gap-2 text-sm text-muted-foreground">
						<span className="flex items-center gap-1">
							<Network className="h-3 w-3" />
							{node.nodeType}
						</span>
						{blockHeight !== undefined && (
							<span className="flex items-center gap-1">
								<Blocks className="h-3 w-3" />
								Block #{blockHeight}
							</span>
						)}
					</div>
				</div>
			</div>
		</Card>
	)
}

export function BesuNetworkDetails({ network }: BesuNetworkDetailsProps) {
	const [searchParams, setSearchParams] = useSearchParams()
	const currentTab = (searchParams.get('tab') || 'details') as BesuTabValue
	const [isEditingGenesis, setIsEditingGenesis] = useState(false)
	const [selectedNodeId, setSelectedNodeId] = useState<number | null>(null)

	// Fetch BESU nodes for the network
	const { data: besuNodes, isLoading: nodesLoading } = useQuery({
		...getNodesPlatformByPlatformOptions({
			path: { platform: 'BESU' },
			query: { limit: 100 }, // Get all nodes
		}),
	})

	// Filter nodes by network ID using useMemo
	const networkNodes = useMemo(() => {
		return besuNodes?.items?.filter((node) => node.besuNode?.networkId === network.id) || []
	}, [besuNodes?.items, network.id])

	// Set the first node as selected by default when nodes are loaded
	useMemo(() => {
		if (networkNodes.length > 0 && !selectedNodeId) {
			setSelectedNodeId(networkNodes[0].id!)
		}
	}, [networkNodes, selectedNodeId])

	// Update the genesisConfig and initialConfig typing
	const genesisConfig = network.genesisConfig as BesuGenesisConfig
	const initialConfig = network.config as BesuConfig

	const handleTabChange = (newTab: BesuTabValue) => {
		setSearchParams({ tab: newTab })
	}

	const handleCopyGenesis = () => {
		navigator.clipboard.writeText(JSON.stringify(JSON.parse(genesisConfig as any), null, 2))
	}

	// Form setup for genesis editing
	const form = useForm<GenesisFormValues>({
		resolver: zodResolver(genesisFormSchema),
		defaultValues: {
			genesisJson: JSON.stringify(genesisConfig, null, 2),
		},
	})

	const onSubmit = async (data: GenesisFormValues) => {
		try {
			const parsedGenesis = JSON.parse(data.genesisJson)

			// Here you would typically make an API call to update the genesis
			// For now, we'll just show a success message
			void parsedGenesis // Genesis config parsed successfully

			toast.success('Genesis configuration updated successfully')
			setIsEditingGenesis(false)

			// In a real implementation, you would update the network state here
			// setNetwork(prev => ({ ...prev, genesisConfig: parsedGenesis }))
		} catch (error) {
			toast.error('Failed to update genesis configuration')
			console.error('Error updating genesis:', error)
		}
	}

	const handleCancelEdit = () => {
		setIsEditingGenesis(false)
		form.reset()
	}
	if (!network) {
		return (
			<div className="flex-1 p-8">
				<div className="mx-auto text-center">
					<Network className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
					<h1 className="text-2xl font-semibold mb-2">Network not found</h1>
					<p className="text-muted-foreground mb-8">The network you're looking for doesn't exist or you don't have access to it.</p>
					<Button asChild>
						<Link to="/networks">
							<ArrowLeft className="mr-2 h-4 w-4" />
							Back to Networks
						</Link>
					</Button>
				</div>
			</div>
		)
	}

	return (
		<div className="flex-1 p-8">
			<div className="max-w-7xlxl mx-auto">
				<div className="flex items-center gap-2 text-muted-foreground mb-8">
					<Button variant="ghost" size="sm" asChild>
						<Link to="/networks">
							<ArrowLeft className="mr-2 h-4 w-4" />
							Networks
						</Link>
					</Button>
				</div>

				<div className="mb-4">
					<div className="flex items-center justify-between">
						<div>
							<div className="flex items-center gap-3 mb-1">
								<h1 className="text-2xl font-semibold">{network.name}</h1>
								<Badge className="gap-1">
									<Activity className="h-3 w-3" />
									{network.status}
								</Badge>
								<Badge variant="outline" className="text-sm flex items-center gap-1">
									<BesuIcon className="h-3 w-3" />
									{network.platform}
								</Badge>
							</div>
							<p className="text-muted-foreground">
								Created <TimeAgo date={network.createdAt!} />
							</p>
						</div>

						<div className="flex items-center gap-2"> </div>
					</div>
				</div>

				<Card className="p-6">
					<BesuNetworkTabs
						tab={currentTab}
						setTab={handleTabChange}
						networkDetails={
							<div className="space-y-6">
								<div className="flex items-center gap-4 mb-6">
									<div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center">
										<BesuIcon className="h-6 w-6 text-primary" />
									</div>
									<div>
										<h2 className="text-lg font-semibold">Network Information</h2>
										<p className="text-sm text-muted-foreground">Details about your Besu network</p>
									</div>
								</div>

								<div>
									<h3 className="text-sm font-medium mb-2">Network ID</h3>
									<p className="text-sm text-muted-foreground">{genesisConfig?.config?.chainId || 'Not specified'}</p>
								</div>

								<div>
									<h3 className="text-sm font-medium mb-2">Consensus</h3>
									<p className="text-sm text-muted-foreground">{initialConfig?.consensus || 'Not specified'}</p>
								</div>

							{(() => {
								const { vanity, validators, seals } = parseValidatorsFromBesuQbftExtraData(genesisConfig?.extraData || '')
								return validators.length > 0 ? (
									<div>
										<h3 className="text-sm font-medium mb-2">Validators ({validators.length})</h3>
										{vanity && (
											<div className="mb-3">
												<h4 className="text-xs font-medium text-muted-foreground mb-1">Vanity Data</h4>
												<div className="flex items-center gap-2">
													<Badge variant="secondary" className="font-mono text-xs">
														{vanity}
													</Badge>
													<Button
														variant="ghost"
														size="sm"
														onClick={() => navigator.clipboard.writeText(vanity)}
														className="h-6 w-6 p-0"
													>
														<Copy className="h-3 w-3" />
													</Button>
												</div>
											</div>
										)}
										<div className="mb-3">
											<h4 className="text-xs font-medium text-muted-foreground mb-2">Validator Addresses</h4>
											<div className="space-y-2">
												{validators.map((validator) => (
													<div key={validator} className="flex items-center gap-2">
														<Badge variant="outline" className="font-mono text-xs">
															{validator}
														</Badge>
														<Button
															variant="ghost"
															size="sm"
															onClick={() => navigator.clipboard.writeText(validator)}
															className="h-6 w-6 p-0"
														>
															<Copy className="h-3 w-3" />
														</Button>
													</div>
												))}
											</div>
										</div>
										{seals.length > 0 && (
											<div>
												<h4 className="text-xs font-medium text-muted-foreground mb-2">Seals ({seals.length})</h4>
												<div className="space-y-2">
													{seals.map((seal) => (
														<div key={seal} className="flex items-center gap-2">
															<Badge variant="outline" className="font-mono text-xs">
																{seal}
															</Badge>
															<Button
																variant="ghost"
																size="sm"
																onClick={() => navigator.clipboard.writeText(seal)}
																className="h-6 w-6 p-0"
															>
																<Copy className="h-3 w-3" />
															</Button>
														</div>
													))}
												</div>
											</div>
										)}
									</div>
								) : initialConfig?.initialValidators ? (
									<div>
										<h3 className="text-sm font-medium mb-2">Validators</h3>
										<ValidatorList validatorIds={initialConfig.initialValidators} />
									</div>
								) : null
							})()}

								<div>
									<h3 className="text-sm font-medium mb-3">Nodes</h3>
									<div className="space-y-4">
										{networkNodes.map((node) => (
											<BesuNodeCard key={node.id} node={node} />
										))}
									</div>
								</div>
							</div>
						}
						genesis={
							<div className="space-y-4">
								<div className="flex items-center gap-4 mb-6">
									<div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center">
										<Code className="h-6 w-6 text-primary" />
									</div>
									<div className="flex-1">
										<h2 className="text-lg font-semibold">Genesis Configuration</h2>
										<p className="text-sm text-muted-foreground">Network genesis block configuration</p>
									</div>
									{!isEditingGenesis ? (
										<Button variant="outline" size="sm" onClick={() => setIsEditingGenesis(true)} className="gap-2">
											<Edit className="h-4 w-4" />
											Edit
										</Button>
									) : (
										<div className="flex gap-2">
											<Button variant="outline" size="sm" onClick={handleCancelEdit} className="gap-2">
												<X className="h-4 w-4" />
												Cancel
											</Button>
										</div>
									)}
								</div>

								{isEditingGenesis ? (
									<Card className="p-4">
										<Form {...form}>
											<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
												<FormField
													control={form.control}
													name="genesisJson"
													render={({ field }) => (
														<FormItem>
															<FormLabel className="text-sm font-medium">Genesis Configuration JSON</FormLabel>
															<FormControl>
																<Textarea {...field} placeholder="Enter genesis configuration JSON..." className="font-mono text-sm min-h-[400px]" rows={20} />
															</FormControl>
															<FormMessage />
														</FormItem>
													)}
												/>

												{form.formState.errors.genesisJson && (
													<Alert variant="destructive">
														<AlertDescription>{form.formState.errors.genesisJson.message}</AlertDescription>
													</Alert>
												)}

												<div className="flex gap-2">
													<Button type="submit" disabled={form.formState.isSubmitting} className="gap-2">
														<Save className="h-4 w-4" />
														{form.formState.isSubmitting ? 'Saving...' : 'Save Changes'}
													</Button>
													<Button type="button" variant="outline" onClick={handleCancelEdit}>
														Cancel
													</Button>
												</div>
											</form>
										</Form>
									</Card>
								) : (
									<Card className="p-4">
										<div className="flex justify-between items-center mb-2">
											<h3 className="text-sm font-medium">Genesis Configuration</h3>
											<Button variant="ghost" size="sm" onClick={handleCopyGenesis} className="h-8 w-8 p-0">
												<Copy className="h-4 w-4" />
											</Button>
										</div>
									<pre className="text-sm overflow-hidden bg-muted/50 p-4 rounded-md whitespace-pre-wrap break-words break-all">
										<code className="break-words break-all">{JSON.stringify(genesisConfig, null, 2)}</code>
									</pre>
									</Card>
								)}
							</div>
						}
						validators={
							<div className="space-y-6">
								{/* Node Selection */}
								{networkNodes.length > 0 && (
									<Card className="border-l-4 border-l-primary">
										<CardHeader className="pb-3">
											<div className="flex items-center justify-between">
												<div className="flex items-center gap-3">
													<Network className="h-5 w-5 text-primary" />
													<div>
														<CardTitle className="text-base">Active Node</CardTitle>
														<CardDescription>Selected node for validator operations</CardDescription>
													</div>
												</div>
												<Select value={selectedNodeId?.toString() || ''} onValueChange={(value) => setSelectedNodeId(parseInt(value))}>
													<SelectTrigger className="w-auto min-w-[200px]">
														<SelectValue>
															<div className="flex items-center gap-2">
																<span className="text-sm">Switch node</span>
															</div>
														</SelectValue>
													</SelectTrigger>
													<SelectContent>
														{networkNodes.map((node) => (
															<SelectItem key={node.id} value={node.id?.toString() || ''}>
																<div className="flex items-center gap-2">
																	<span>{node.name || `Node ${node.id}`}</span>
																</div>
															</SelectItem>
														))}
													</SelectContent>
												</Select>
											</div>
										</CardHeader>
										<CardContent>
											<div className="flex items-center gap-3">
												<div className="flex items-center gap-2">
													<span className="font-medium text-lg">{selectedNodeId}</span>
												</div>
											</div>
										</CardContent>
									</Card>
								)}

							{/* Validators Tab Content */}
							<BesuValidatorsTab 
								nodeId={selectedNodeId || 0} 
								nodesLoading={nodesLoading} 
								networkValidators={parseValidatorsFromBesuQbftExtraData(genesisConfig?.extraData || '').validators}
							/>
							</div>
						}
						explorer={
							<div className="space-y-6">

							{/* Explorer Tab Content */}
							<BesuBlockExplorer
								nodesLoading={nodesLoading}
								networkNodes={networkNodes}
								networkId={network.id}
							/>
							</div>
						}
						tutorial={
							<BesuSmartContractTutorial
								rpcEndpoint={networkNodes[0]?.besuNode?.rpcHost && networkNodes[0]?.besuNode?.rpcPort
									? `http://${networkNodes[0].besuNode.rpcHost}:${networkNodes[0].besuNode.rpcPort}`
									: undefined
								}
								chainId={genesisConfig?.config?.chainId}
							/>
						}
					/>
				</Card>
			</div>
		</div>
	)
}

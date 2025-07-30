import { HttpBesuNetworkResponse } from '@/api/client'
import { ValidatorList } from '@/components/networks/validator-list'
import { Activity, ArrowLeft, Code, Copy, Network, Edit, Save, X, Vote, Users, MoreHorizontal, Check } from 'lucide-react'
import { Link, useSearchParams } from 'react-router-dom'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import * as z from 'zod'
import { BesuIcon } from '../icons/besu-icon'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card'
import { TimeAgo } from '../ui/time-ago'
import { BesuNetworkTabs, BesuTabValue } from './besu-network-tabs'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '../ui/form'
import { Textarea } from '../ui/textarea'
import { Alert, AlertDescription } from '../ui/alert'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '../ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '../ui/dropdown-menu'
import { toast } from 'sonner'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
	getNodesPlatformByPlatformOptions,
	getNodesByIdRpcQbftPendingVotesOptions,
	getNodesByIdRpcQbftValidatorsByBlockNumberOptions,
	postNodesByIdRpcQbftProposeValidatorVoteMutation,
	postNodesByIdRpcQbftDiscardValidatorVoteMutation,
} from '@/api/client/@tanstack/react-query.gen'
import { useMutation } from '@tanstack/react-query'
import { Input } from '../ui/input'

// Add these interfaces to properly type the config and genesis config
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

export function BesuNetworkDetails({ network }: BesuNetworkDetailsProps) {
	const [searchParams, setSearchParams] = useSearchParams()
	const currentTab = (searchParams.get('tab') || 'details') as BesuTabValue
	const [isEditingGenesis, setIsEditingGenesis] = useState(false)
	const [selectedNodeId, setSelectedNodeId] = useState<string>('')
	const [addValidatorDialogOpen, setAddValidatorDialogOpen] = useState(false)
	const [addValidatorLoading, setAddValidatorLoading] = useState(false)
	const [copiedAddresses, setCopiedAddresses] = useState<Set<string>>(new Set())

	const queryClient = useQueryClient()

	// Fetch BESU nodes for the network
	const { data: besuNodes, isLoading: nodesLoading } = useQuery({
		...getNodesPlatformByPlatformOptions({
			path: { platform: 'BESU' },
			query: { limit: 100 }, // Get all nodes
		}),
	})

	// QBFT queries for the selected node
	const { data: qbftPendingVotes, isLoading: qbftPendingVotesLoading } = useQuery({
		...getNodesByIdRpcQbftPendingVotesOptions({
			path: { id: parseInt(selectedNodeId) || 0 },
		}),
		enabled: !!selectedNodeId,
	})

	const { data: qbftValidatorsByBlockNumber, isLoading: qbftValidatorsByBlockNumberLoading } = useQuery({
		...getNodesByIdRpcQbftValidatorsByBlockNumberOptions({
			path: { id: parseInt(selectedNodeId) || 0 },
			query: { blockNumber: 'latest' },
		}),
		enabled: !!selectedNodeId,
	})

	const handleVote = async (validatorAddress: string, vote: boolean) => {
		if (!selectedNodeId) {
			toast.error('Please select a node first')
			return
		}

		try {
			await proposeVoteMutation.mutateAsync({
				path: { id: parseInt(selectedNodeId) },
				body: {
					validatorAddress,
					vote
				}
			})
		} catch (error) {
			// Error is handled by the mutation's onError
			console.error('Error in handleVote:', error)
		}
	}

	const handleDiscardVote = async (validatorAddress: string) => {
		if (!selectedNodeId) {
			toast.error('Please select a node first')
			return
		}

		try {
			await discardVoteMutation.mutateAsync({
				path: { id: parseInt(selectedNodeId) },
				body: {
					validatorAddress
				}
			})
		} catch (error) {
			// Error is handled by the mutation's onError
			console.error('Error in handleDiscardVote:', error)
		}
	}

	// Form for adding validator
	const addValidatorForm = useForm<{ validatorAddress: string }>({
		resolver: zodResolver(z.object({
			validatorAddress: z.string().regex(/^0x[a-fA-F0-9]{40}$/, 'Must be a valid Ethereum address (0x followed by 40 hex characters)')
		})),
		defaultValues: {
			validatorAddress: ''
		}
	})

	// Add validator mutation
	const addValidatorMutation = useMutation({
		...postNodesByIdRpcQbftProposeValidatorVoteMutation({
			path: { id: parseInt(selectedNodeId) || 0 },
			body: {
				validatorAddress: '',
				vote: true
			}
		}),
		onSuccess: () => {
			toast.success('Validator vote proposed successfully')
			setAddValidatorDialogOpen(false)
			addValidatorForm.reset()
			queryClient.invalidateQueries({
				queryKey: getNodesByIdRpcQbftValidatorsByBlockNumberOptions({
					path: { id: parseInt(selectedNodeId) || 0 },
					query: { blockNumber: 'latest' }
				}).queryKey
			})
		},
		onError: (error) => {
			console.error('Error proposing validator vote:', error)
			toast.error('Failed to propose validator vote')
		}
	})

	const discardVoteMutation = useMutation({
		...postNodesByIdRpcQbftDiscardValidatorVoteMutation({
			path: { id: parseInt(selectedNodeId) || 0 },
			body: {
				validatorAddress: ''
			}
		}),
		onSuccess: () => {
			toast.success('Vote discarded successfully')
			queryClient.invalidateQueries({
				queryKey: getNodesByIdRpcQbftPendingVotesOptions({
					path: { id: parseInt(selectedNodeId) || 0 }
				}).queryKey
			})
		},
		onError: (error) => {
			console.error('Error discarding vote:', error)
			toast.error('Failed to discard vote')
		}
	})

	const proposeVoteMutation = useMutation({
		...postNodesByIdRpcQbftProposeValidatorVoteMutation({
			path: { id: parseInt(selectedNodeId) || 0 },
			body: {
				validatorAddress: '',
				vote: true
			}
		}),
		onSuccess: () => {
			toast.success('Vote proposed successfully')
			queryClient.invalidateQueries({
				queryKey: getNodesByIdRpcQbftPendingVotesOptions({
					path: { id: parseInt(selectedNodeId) || 0 }
				}).queryKey
			})
		},
		onError: (error) => {
			console.error('Error proposing vote:', error)
			toast.error('Failed to propose vote')
		}
	})

	const handleAddValidator = async (data: { validatorAddress: string }) => {
		if (!selectedNodeId) {
			toast.error('Please select a node first')
			return
		}

		setAddValidatorLoading(true)
		try {
			await addValidatorMutation.mutateAsync({
				path: { id: parseInt(selectedNodeId) },
				body: {
					validatorAddress: data.validatorAddress,
					vote: true // true means add validator, false means remove
				}
			})
		} catch (error) {
			// Error is handled by the mutation's onError
			console.error('Error in handleAddValidator:', error)
		} finally {
			setAddValidatorLoading(false)
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
			console.log('Updated genesis config:', parsedGenesis)

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
				<div className="max-w-4xl mx-auto text-center">
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
			<div className="max-w-4xl mx-auto">
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

								{initialConfig?.initialValidators && (
									<div>
										<h3 className="text-sm font-medium mb-2">Validators</h3>
										<ValidatorList validatorIds={initialConfig.initialValidators} />
									</div>
								)}
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
										<pre className="text-sm overflow-auto bg-muted/50 p-4 rounded-md">
											<code>{JSON.stringify(genesisConfig, null, 2)}</code>
										</pre>
									</Card>
								)}
							</div>
						}
						validators={
							<div className="space-y-4">
								<div className="flex items-center gap-4 mb-6">
									<div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center">
										<Activity className="h-6 w-6 text-primary" />
									</div>
									<div>
										<h2 className="text-lg font-semibold">Validator Management</h2>
										<p className="text-sm text-muted-foreground">Manage QBFT validators and voting</p>
									</div>
								</div>

								{/* Node Selector */}
								<Card>
									<CardHeader>
										<CardTitle className="text-sm">Select Node for Write Operations</CardTitle>
										<CardDescription>Choose a node to perform voting operations</CardDescription>
									</CardHeader>
									<CardContent>
										{nodesLoading ? (
											<p>Loading nodes...</p>
										) : besuNodes?.items && besuNodes.items.length > 0 ? (
											<Select value={selectedNodeId} onValueChange={setSelectedNodeId}>
												<SelectTrigger>
													<SelectValue placeholder="Select a node..." />
												</SelectTrigger>
												<SelectContent>
													{besuNodes.items.map((node) => (
														<SelectItem key={node.id} value={node.id?.toString() || ''}>
															<div className="flex items-center gap-2">
																<span>{node.name}</span>
																<Badge variant={node.status === 'running' ? 'default' : 'secondary'} className="text-xs">
																	{node.status}
																</Badge>
															</div>
														</SelectItem>
													))}
												</SelectContent>
											</Select>
										) : (
											<div className="text-center py-4">
												<p className="text-muted-foreground">No BESU nodes found</p>
												<p className="text-sm text-muted-foreground mt-1">
													Create BESU nodes to enable validator management.
												</p>
											</div>
										)}
									</CardContent>
								</Card>

								{selectedNodeId ? (
									<div className="space-y-4">
										<div className="grid gap-4 md:grid-cols-2">
											{/* Current Validators */}
											<Card>
												<CardHeader>
													<CardTitle className="text-sm">Current Validators</CardTitle>
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
																		onClick={() => navigator.clipboard.writeText(validator)}
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

											{/* Pending Votes */}
											<Card>
												<CardHeader>
													<CardTitle className="text-sm">Pending Votes</CardTitle>
													<CardDescription>Votes waiting for consensus</CardDescription>
												</CardHeader>
												<CardContent>
													{qbftPendingVotesLoading ? (
														<p>Loading pending votes...</p>
													) : qbftPendingVotes && Object.keys(qbftPendingVotes).length > 0 ? (
														<div className="space-y-4">
															{Object.entries(qbftPendingVotes || {}).map(([validator, votes]) => (
																<div key={validator} className="p-4 border rounded-lg">
																	{/* Address Line */}
																	<div className="flex items-center justify-between mb-3">
																		<div className="flex items-center gap-2 flex-1 min-w-0">
																			<span className="font-medium text-sm whitespace-nowrap">Validator:</span>
																			<span className="font-mono text-sm truncate">{validator}</span>
																			<Button
																				variant="ghost"
																				size="sm"
																				onClick={() => copyToClipboard(validator)}
																				className="h-6 w-6 p-0 flex-shrink-0"
																			>
																				{copiedAddresses.has(validator) ? (
																					<Check className="h-3 w-3 text-green-500" />
																				) : (
																					<Copy className="h-3 w-3" />
																				)}
																			</Button>
																		</div>
																	</div>
																	
																	{/* Actions Line */}
																	<div className="flex justify-end">
																		<DropdownMenu>
																			<DropdownMenuTrigger asChild>
																				<Button variant="outline" size="sm">
																					<MoreHorizontal className="h-4 w-4 mr-2" />
																					Actions
																				</Button>
																			</DropdownMenuTrigger>
																			<DropdownMenuContent align="end">
																				<DropdownMenuItem
																					onClick={() => handleVote(validator, true)}
																					disabled={proposeVoteMutation.isPending}
																				>
																					<Vote className="h-4 w-4 mr-2" />
																					Approve
																				</DropdownMenuItem>
																				<DropdownMenuItem
																					onClick={() => handleVote(validator, false)}
																					disabled={proposeVoteMutation.isPending}
																				>
																					<Vote className="h-4 w-4 mr-2" />
																					Reject
																				</DropdownMenuItem>
																				<DropdownMenuSeparator />
																				<DropdownMenuItem
																					onClick={() => handleDiscardVote(validator)}
																					disabled={discardVoteMutation.isPending}
																					className="text-destructive"
																				>
																					<X className="h-4 w-4 mr-2" />
																					Discard
																				</DropdownMenuItem>
																			</DropdownMenuContent>
																		</DropdownMenu>
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

										{/* Add Validator Dialog */}
										<Card>
											<CardHeader>
												<CardTitle className="text-sm">Add New Validator</CardTitle>
												<CardDescription>Propose a new validator to the QBFT consensus</CardDescription>
											</CardHeader>
											<CardContent>
												<Dialog open={addValidatorDialogOpen} onOpenChange={setAddValidatorDialogOpen}>
													<DialogTrigger asChild>
														<Button variant="outline">
															<Users className="h-4 w-4 mr-2" />
															Add Validator
														</Button>
													</DialogTrigger>
													<DialogContent>
														<DialogHeader>
															<DialogTitle>Add New Validator</DialogTitle>
															<DialogDescription>
																Add a new validator to the QBFT consensus. The validator address must be a valid Ethereum address.
															</DialogDescription>
														</DialogHeader>
														<Form {...addValidatorForm}>
															<form onSubmit={addValidatorForm.handleSubmit(handleAddValidator)} className="space-y-4">
																<FormField
																	control={addValidatorForm.control}
																	name="validatorAddress"
																	render={({ field }) => (
																		<FormItem>
																			<FormLabel>Validator Address</FormLabel>
																			<FormControl>
																				<Input
																					placeholder="0x1234567890abcdef1234567890abcdef12345678"
																					{...field}
																				/>
																			</FormControl>
																			<FormMessage />
																		</FormItem>
																	)}
																/>
																<DialogFooter>
																	<Button
																		type="button"
																		variant="outline"
																		onClick={() => setAddValidatorDialogOpen(false)}
																		disabled={addValidatorLoading}
																	>
																		Cancel
																	</Button>
																	<Button
																		type="submit"
																		disabled={addValidatorLoading}
																	>
																		{addValidatorLoading ? 'Adding...' : 'Add Validator'}
																	</Button>
																</DialogFooter>
															</form>
														</Form>
													</DialogContent>
												</Dialog>
											</CardContent>
										</Card>
									</div>
								) : (
									<Card>
										<CardContent className="text-center py-8">
											<Users className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
											<p className="text-muted-foreground">Select a node above to view and manage validators.</p>
											<p className="text-sm text-muted-foreground mt-2">Only running nodes can perform voting operations.</p>
										</CardContent>
									</Card>
								)}
							</div>
						}
					/>
				</Card>
			</div>
		</div>
	)
}

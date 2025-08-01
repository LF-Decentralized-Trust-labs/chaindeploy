import { Activity, Copy, Vote, Users, MoreHorizontal, Check, X, AlertCircle, Server, Key, Plus, Minus } from 'lucide-react'
import { useState, useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import * as z from 'zod'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '../ui/form'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '../ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '../ui/dropdown-menu'
import { toast } from 'sonner'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
	getNodesByIdRpcQbftPendingVotesOptions,
	getNodesByIdRpcQbftValidatorsByBlockNumberOptions,
	postNodesByIdRpcQbftProposeValidatorVoteMutation,
	postNodesByIdRpcQbftDiscardValidatorVoteMutation,
	getKeysFilterOptions,
} from '@/api/client/@tanstack/react-query.gen'
import { useMutation } from '@tanstack/react-query'
import { Input } from '../ui/input'
import { Alert, AlertDescription } from '../ui/alert'

interface BesuValidatorsTabProps {
	nodeId: number
	nodesLoading: boolean
}

export function BesuValidatorsTab({ nodeId, nodesLoading }: BesuValidatorsTabProps) {
	const [voteValidatorDialogOpen, setVoteValidatorDialogOpen] = useState(false)
	const [voteValidatorLoading, setVoteValidatorLoading] = useState(false)
	const [copiedAddresses, setCopiedAddresses] = useState<Set<string>>(new Set())
	const [showVoteKeySelector, setShowVoteKeySelector] = useState(false)

	const queryClient = useQueryClient()

	// QBFT queries for the selected node
	const { data: qbftPendingVotes, isLoading: qbftPendingVotesLoading } = useQuery({
		...getNodesByIdRpcQbftPendingVotesOptions({
			path: { id: nodeId },
		}),
		enabled: !!nodeId,
	})

	const { data: qbftValidatorsByBlockNumber, isLoading: qbftValidatorsByBlockNumberLoading } = useQuery({
		...getNodesByIdRpcQbftValidatorsByBlockNumberOptions({
			path: { id: nodeId },
			query: { blockNumber: 'latest' },
		}),
		enabled: !!nodeId,
	})

	// Query for available EC secp256k1 keys
	const { data: availableKeys, isLoading: keysLoading } = useQuery({
		...getKeysFilterOptions({
			query: {
				algorithm: 'EC',
				curve: 'secp256k1',
			},
		}),
	})

	// Filter keys to only show EC/secp256k1 keys
	const validKeys = availableKeys?.items?.filter((key) => key.algorithm === 'EC' && key.curve === 'secp256k1') || []

	console.log('qbftValidatorsByBlockNumber', qbftValidatorsByBlockNumber)
	console.log('nodeId', nodeId)
	const handleVote = async (validatorAddress: string, vote: boolean) => {
		if (!nodeId) {
			toast.error('Please select a node first')
			return
		}

		try {
			await proposeVoteMutation.mutateAsync({
				path: { id: nodeId },
				body: {
					validatorAddress,
					vote,
				},
			})
		} catch (error) {
			// Error is handled by the mutation's onError
			console.error('Error in handleVote:', error)
		}
	}

	const handleDiscardVote = async (validatorAddress: string) => {
		if (!nodeId) {
			toast.error('Please select a node first')
			return
		}

		try {
			await discardVoteMutation.mutateAsync({
				path: { id: nodeId },
				body: {
					validatorAddress,
				},
			})
		} catch (error) {
			// Error is handled by the mutation's onError
			console.error('Error in handleDiscardVote:', error)
		}
	}

	// Form for voting for validator (now handles both adding and voting)
	const voteValidatorForm = useForm<{ validatorAddress: string; vote: boolean }>({
		resolver: zodResolver(
			z.object({
				validatorAddress: z.string().regex(/^0x[a-fA-F0-9]{40}$/, 'Must be a valid Ethereum address (0x followed by 40 hex characters)'),
				vote: z.boolean(),
			})
		),
		defaultValues: {
			validatorAddress: '',
			vote: true, // Default to true (add validator)
		},
	})

	// Vote for validator mutation (now handles both adding and voting)
	const voteValidatorMutation = useMutation({
		...postNodesByIdRpcQbftProposeValidatorVoteMutation({
			path: { id: nodeId || 0 },
			body: {
				validatorAddress: '',
				vote: true,
			},
		}),
		onSuccess: () => {
			toast.success('Validator vote proposed successfully')
			setVoteValidatorDialogOpen(false)
			voteValidatorForm.reset()
			queryClient.invalidateQueries({
				queryKey: getNodesByIdRpcQbftPendingVotesOptions({
					path: { id: nodeId || 0 },
				}).queryKey,
			})
			queryClient.invalidateQueries({
				queryKey: getNodesByIdRpcQbftValidatorsByBlockNumberOptions({
					path: { id: nodeId || 0 },
					query: { blockNumber: 'latest' },
				}).queryKey,
			})
		},
		onError: (error) => {
			console.error('Error proposing vote:', error)
			toast.error('Failed to propose validator vote')
		},
	})

	const discardVoteMutation = useMutation({
		...postNodesByIdRpcQbftDiscardValidatorVoteMutation({
			path: { id: nodeId || 0 },
			body: {
				validatorAddress: '',
			},
		}),
		onSuccess: () => {
			toast.success('Vote discarded successfully')
			queryClient.invalidateQueries({
				queryKey: getNodesByIdRpcQbftPendingVotesOptions({
					path: { id: nodeId || 0 },
				}).queryKey,
			})
		},
		onError: (error) => {
			console.error('Error discarding vote:', error)
			toast.error('Failed to discard vote')
		},
	})

	const proposeVoteMutation = useMutation({
		...postNodesByIdRpcQbftProposeValidatorVoteMutation({
			path: { id: nodeId || 0 },
			body: {
				validatorAddress: '',
				vote: true,
			},
		}),
		onSuccess: () => {
			toast.success('Vote proposed successfully')
			queryClient.invalidateQueries({
				queryKey: getNodesByIdRpcQbftPendingVotesOptions({
					path: { id: nodeId || 0 },
				}).queryKey,
			})
		},
		onError: (error) => {
			console.error('Error proposing vote:', error)
			toast.error('Failed to propose vote')
		},
	})

	const handleVoteForValidator = async (data: { validatorAddress: string; vote: boolean }) => {
		if (!nodeId) {
			toast.error('Please select a node first')
			return
		}

		setVoteValidatorLoading(true)
		try {
			await voteValidatorMutation.mutateAsync({
				path: { id: nodeId },
				body: {
					validatorAddress: data.validatorAddress,
					vote: data.vote,
				},
			})
		} catch (error) {
			// Error is handled by the mutation's onError
			console.error('Error in handleVoteForValidator:', error)
		} finally {
			setVoteValidatorLoading(false)
		}
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

	// Format validator address for better readability
	const formatValidatorAddress = (address: string) => {
		if (address.length <= 10) return address
		return `${address.slice(0, 6)}...${address.slice(-4)}`
	}

	// Show loading state while nodes are being fetched
	if (nodesLoading) {
		return (
			<div className="space-y-6">
				<div className="flex items-center gap-4">
					<div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center">
						<Activity className="h-6 w-6 text-primary" />
					</div>
					<div>
						<h2 className="text-lg font-semibold">Validator Management</h2>
						<p className="text-sm text-muted-foreground">Manage QBFT validators and voting</p>
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
			{nodeId ? (
				<div className="space-y-6">
					{/* Current Validators - Full Width */}
					<Card>
						<CardHeader>
							<div className="flex items-center justify-between">
								<div>
									<CardTitle className="text-base">Current Validators</CardTitle>
									<CardDescription>Validators for the latest block and pending votes</CardDescription>
								</div>
								<Dialog open={voteValidatorDialogOpen} onOpenChange={setVoteValidatorDialogOpen}>
									<DialogTrigger asChild>
										<Button variant="outline" size="sm" className="gap-2">
											<Vote className="h-4 w-4" />
											Manage Validators
										</Button>
									</DialogTrigger>
									<DialogContent className="max-w-md">
										<DialogHeader>
											<DialogTitle>Manage Validators</DialogTitle>
											<DialogDescription>Add new validators or vote for existing ones. You can approve or reject validators.</DialogDescription>
										</DialogHeader>
										<Form {...voteValidatorForm}>
											<form onSubmit={voteValidatorForm.handleSubmit(handleVoteForValidator)} className="space-y-4">
												<FormField
													control={voteValidatorForm.control}
													name="validatorAddress"
													render={({ field }) => (
														<FormItem>
															<FormLabel>Validator Address</FormLabel>
															<div className="space-y-2">
																<FormControl>
																	<Input placeholder="0x1234567890abcdef1234567890abcdef12345678" {...field} />
																</FormControl>
																<div className="flex items-center gap-2">
																	<Button
																		type="button"
																		variant="outline"
																		size="sm"
																		onClick={() => setShowVoteKeySelector(!showVoteKeySelector)}
																		className="gap-2"
																	>
																		<Key className="h-4 w-4" />
																		Select from Keys
																	</Button>
																</div>
																{showVoteKeySelector && validKeys && (
																	<div className="border rounded-lg p-3 space-y-2 max-h-40 overflow-y-auto">
																		<div className="text-xs font-medium text-muted-foreground mb-2">
																			Available Keys (EC secp256k1):
																		</div>
																		{validKeys.map((key) => (
																			<div
																				key={key.id}
																				className="flex items-center justify-between p-2 border rounded hover:bg-accent/50 cursor-pointer"
																				onClick={() => {
																					field.onChange(key.ethereumAddress!)
																					setShowVoteKeySelector(false)
																				}}
																			>
																				<div className="flex-1">
																					<div className="text-sm font-medium">{key.name}</div>
																					<div className="text-xs text-muted-foreground font-mono">
																						{key.ethereumAddress}
																					</div>
																				</div>
																				<Button
																					type="button"
																					variant="ghost"
																					size="sm"
																					onClick={(e) => {
																						e.stopPropagation()
																						navigator.clipboard.writeText(key.ethereumAddress!)
																						toast.success('Address copied to clipboard')
																					}}
																				>
																					<Copy className="h-3 w-3" />
																				</Button>
																			</div>
																		))}
																		{validKeys.length === 0 && (
																			<div className="text-xs text-muted-foreground text-center py-2">
																				No EC secp256k1 keys available
																			</div>
																		)}
																	</div>
																)}
															</div>
															<FormMessage />
														</FormItem>
													)}
												/>
												<FormField
													control={voteValidatorForm.control}
													name="vote"
													render={({ field }) => (
														<FormItem>
															<FormLabel>Action Type</FormLabel>
															<FormControl>
																<Select onValueChange={(value) => field.onChange(value === 'true')} defaultValue={field.value ? 'true' : 'false'}>
																	<SelectTrigger>
																		<SelectValue placeholder="Select action type" />
																	</SelectTrigger>
																	<SelectContent>
																		<SelectItem value="true">Add Validator</SelectItem>
																		<SelectItem value="false">Remove Validator</SelectItem>
																	</SelectContent>
																</Select>
															</FormControl>
															<FormMessage />
														</FormItem>
													)}
												/>
												<DialogFooter>
													<Button type="button" variant="outline" onClick={() => setVoteValidatorDialogOpen(false)} disabled={voteValidatorLoading}>
														Cancel
													</Button>
													<Button type="submit" disabled={voteValidatorLoading}>
														{voteValidatorLoading ? 'Processing...' : 'Submit'}
													</Button>
												</DialogFooter>
											</form>
										</Form>
									</DialogContent>
								</Dialog>
							</div>
						</CardHeader>
						<CardContent>
							{qbftValidatorsByBlockNumberLoading ? (
								<div className="text-center py-8">
									<div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary mx-auto mb-3"></div>
									<p className="text-sm text-muted-foreground">Loading validators...</p>
								</div>
							) : (
								<div className="space-y-4">
									{/* Current Validators */}
									{qbftValidatorsByBlockNumber && qbftValidatorsByBlockNumber.length > 0 ? (
										<div className="space-y-3">
											<div className="text-sm font-medium text-muted-foreground mb-2">Active Validators</div>
											{qbftValidatorsByBlockNumber.map((validator, index) => (
												<div key={index} className="flex items-center justify-between p-3 rounded-lg border bg-muted/30 hover:bg-muted/50 transition-colors">
													<div className="flex items-center gap-3">
														<div className="w-2 h-2 rounded-full bg-green-500"></div>
														<div>
															<p className="font-mono text-sm font-medium">{formatValidatorAddress(validator)}</p>
															<p className="text-xs text-muted-foreground">{validator}</p>
														</div>
													</div>
													<Button variant="ghost" size="sm" onClick={() => navigator.clipboard.writeText(validator)} className="h-8 w-8 p-0">
														<Copy className="h-4 w-4" />
													</Button>
												</div>
											))}
										</div>
									) : (
										<div className="text-center py-8">
											<Users className="h-8 w-8 text-muted-foreground mx-auto mb-3" />
											<p className="text-sm text-muted-foreground">No active validators found</p>
										</div>
									)}

									{/* Pending Votes */}
									{qbftPendingVotesLoading ? (
										<div className="text-center py-4">
											<div className="animate-spin rounded-full h-4 w-4 border-b-2 border-primary mx-auto mb-2"></div>
											<p className="text-xs text-muted-foreground">Loading pending votes...</p>
										</div>
									) : qbftPendingVotes && Object.keys(qbftPendingVotes).length > 0 ? (
										<div className="space-y-3">
											<div className="text-sm font-medium text-muted-foreground mb-2">Pending Votes</div>
											{Object.entries(qbftPendingVotes || {}).map(([validator, willAdd]) => (
												<div key={validator} className="flex items-center justify-between p-3 rounded-lg border bg-yellow-50 dark:bg-yellow-950/20">
													<div className="flex items-center gap-3 flex-1 min-w-0">
														<div className="flex items-center gap-2">
															<div className="w-2 h-2 rounded-full bg-yellow-500"></div>
															{willAdd ? (
																<Plus className="h-4 w-4 text-green-600" />
															) : (
																<Minus className="h-4 w-4 text-red-600" />
															)}
														</div>
														<div className="flex-1">
															<div className="flex items-center gap-2">
																<p className="font-mono text-sm font-medium">{formatValidatorAddress(validator)}</p>
																<Badge variant={willAdd ? "default" : "destructive"} className="text-xs">
																	{willAdd ? "Will Add" : "Will Remove"}
																</Badge>
															</div>
															<p className="text-xs text-muted-foreground">{validator}</p>
														</div>
														<Button variant="ghost" size="sm" onClick={() => copyToClipboard(validator)} className="h-6 w-6 p-0 flex-shrink-0">
															{copiedAddresses.has(validator) ? <Check className="h-3 w-3 text-green-500" /> : <Copy className="h-3 w-3" />}
														</Button>
													</div>
													<Button
														variant="ghost"
														size="sm"
														onClick={() => handleDiscardVote(validator)}
														disabled={discardVoteMutation.isPending}
														className="h-8 w-8 p-0 text-destructive hover:text-destructive"
														title="Discard vote"
													>
														<X className="h-4 w-4" />
													</Button>
												</div>
											))}
										</div>
									) : null}
								</div>
							)}
						</CardContent>
					</Card>
				</div>
			) : null}
		</div>
	)
}

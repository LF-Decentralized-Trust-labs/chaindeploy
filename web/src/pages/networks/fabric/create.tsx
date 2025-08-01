import { HandlerOrganizationResponse, HttpFabricNetworkConfig, HttpFabricPolicy, HttpNodeResponse, HttpSmartBftConsenter } from '@/api/client'
import { getNodesOptions, getOrganizationsOptions, postNetworksFabricMutation } from '@/api/client/@tanstack/react-query.gen'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { Toggle } from '@/components/ui/toggle'
import { cn } from '@/lib/utils'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { AlertCircle, Network, Settings, TriangleAlert, Users } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import * as z from 'zod'

interface OrganizationWithNodes extends HandlerOrganizationResponse {
	orderers: HttpNodeResponse[]
}

const channelFormSchema = z
	.object({
		name: z.string().min(1, 'Channel name is required'),
		organizations: z.array(
			z.object({
				id: z.number(),
				enabled: z.boolean().default(false),
				isPeer: z.boolean().default(false),
				isOrderer: z.boolean().default(false),
				consenters: z.array(z.number()),
			})
		),
		// Consensus configuration
		consensusType: z.enum(['etcdraft', 'smartbft']).default('etcdraft'),
		// Capabilities configuration
		channelCapabilities: z.array(z.string()).optional(),
		applicationCapabilities: z.array(z.string()).optional(),
		ordererCapabilities: z.array(z.string()).optional(),
		// Batch configuration
		batchSize: z
			.object({
				maxMessageCount: z.number().min(1, 'Max message count must be at least 1').default(500),
				absoluteMaxBytes: z.number().min(1, 'Absolute max bytes must be at least 1').default(103809024),
				preferredMaxBytes: z.number().min(1, 'Preferred max bytes must be at least 1').default(524288),
			})
			.optional(),
		batchTimeout: z.string().default('2s'),
		// etcdraft options
		etcdRaftOptions: z
			.object({
				tickInterval: z.string().default('500ms'),
				electionTick: z.number().min(1).default(10),
				heartbeatTick: z.number().min(1).default(1),
				maxInflightBlocks: z.number().min(1).default(5),
				snapshotIntervalSize: z.number().min(1).default(20971520),
			})
			.optional(),
		// SmartBFT options
		smartBFTOptions: z
			.object({
				requestBatchMaxCount: z.number().min(1).default(500),
				requestBatchMaxBytes: z.number().min(1).default(1048576),
				requestBatchMaxInterval: z.string().default('50ms'),
				requestMaxBytes: z.number().min(1).default(1048576),
				incomingMessageBufferSize: z.number().min(1).default(200),
				requestPoolSize: z.number().min(1).default(500),
				viewChangeResendInterval: z.string().default('5s'),
				viewChangeTimeout: z.string().default('20s'),
				leaderHeartbeatCount: z.number().min(1).default(10),
				leaderHeartbeatTimeout: z.string().default('2s'),
				collectTimeout: z.string().default('1s'),
				syncOnStart: z.boolean().default(true),
				speedUpViewChange: z.boolean().default(false),
				leaderRotation: z.string().default('round_robin'),
				decisionsPerLeader: z.number().min(1).default(1000),
				requestComplainTimeout: z.string().default('20s'),
				requestAutoRemoveTimeout: z.string().default('3m'),
				requestForwardTimeout: z.string().default('2s'),
			})
			.optional(),
		configurePolicies: z.boolean().default(false),
		applicationPolicies: z
			.record(
				z.object({
					type: z.enum(['ImplicitMeta', 'Signature']),
					rule: z.string(),
					organizations: z.array(z.string()).optional(),
					signatureOperator: z.enum(['OR', 'AND', 'OUTOF']).optional(),
					signatureN: z.number().optional(),
				})
			)
			.optional(),
		ordererPolicies: z
			.record(
				z.object({
					type: z.enum(['ImplicitMeta', 'Signature']),
					rule: z.string(),
					organizations: z.array(z.string()).optional(),
					signatureOperator: z.enum(['OR', 'AND', 'OUTOF']).optional(),
					signatureN: z.number().optional(),
				})
			)
			.optional(),
	})
	.refine(
		(data) => {
			const enabledOrgs = [...data.organizations.filter((org) => org.enabled)]

			// At least one peer organization must be enabled
			return enabledOrgs.some((org) => org.isPeer)
		},
		{ message: 'At least one peer organization must be enabled' }
	)
	.refine(
		(data) => {
			// For SmartBFT, we need at least 4 consenters
			if (data.consensusType === 'smartbft') {
				const totalConsenters = [...data.organizations.filter((org) => org.enabled && org.isOrderer).flatMap((org) => org.consenters)].length
				return totalConsenters >= 4
			}
			return true
		},
		{ message: 'SmartBFT requires at least 4 consenters' }
	)
	.refine(
		(data) => {
			// For SmartBFT, we need at least 4 consenters
			if (data.consensusType === 'smartbft') {
				const totalConsenters = [...data.organizations.filter((org) => org.enabled && org.isOrderer).flatMap((org) => org.consenters)].length
				return totalConsenters >= 4
			}
			return true
		},
		{ message: 'SmartBFT requires at least 4 consenters' }
	)

type ChannelFormValues = z.infer<typeof channelFormSchema>

export default function FabricCreateChannel() {
	const { data: organizations, isLoading: isLoadingOrgs } = useQuery({
		...getOrganizationsOptions({
			query: {
				limit: 1000,
			},
		}),
	})
	const { data: nodes, isLoading: isLoadingNodes } = useQuery({
		...getNodesOptions({
			query: {
				limit: 1000,
				page: 1,
				platform: 'FABRIC',
			},
		}),
	})

	const navigate = useNavigate()
	const createNetwork = useMutation({
		...postNetworksFabricMutation(),
		onSuccess: (network) => {
			toast.success('Network created successfully')
			navigate(`/networks/${network.id}/fabric`)
		},
		onError: (error: any) => {
			toast.error(`Failed to create network: ${error.message}`)
		},
	})
	const form = useForm<ChannelFormValues>({
		resolver: zodResolver(channelFormSchema),
		defaultValues: {
			name: '',
			organizations: [],
			consensusType: 'etcdraft',
			channelCapabilities: ['V2_0', 'V3_0'],
			applicationCapabilities: ['V2_0', 'V2_5'],
			ordererCapabilities: ['V2_0'],
			batchSize: {
				maxMessageCount: 500,
				absoluteMaxBytes: 103809024,
				preferredMaxBytes: 524288,
			},
			batchTimeout: '2s',
			etcdRaftOptions: {
				tickInterval: '500ms',
				electionTick: 10,
				heartbeatTick: 1,
				maxInflightBlocks: 5,
				snapshotIntervalSize: 20971520,
			},
			smartBFTOptions: {
				requestBatchMaxCount: 500,
				requestBatchMaxBytes: 1048576,
				requestBatchMaxInterval: '50ms',
				requestMaxBytes: 1048576,
				incomingMessageBufferSize: 200,
				requestPoolSize: 500,
				viewChangeResendInterval: '5s',
				viewChangeTimeout: '20s',
				leaderHeartbeatCount: 10,
				leaderHeartbeatTimeout: '2s',
				collectTimeout: '1s',
				syncOnStart: true,
				speedUpViewChange: false,
				leaderRotation: 'round_robin',
				decisionsPerLeader: 1000,
				requestComplainTimeout: '20s',
				requestAutoRemoveTimeout: '3m',
				requestForwardTimeout: '2s',
			},
		},
	})

	const [formError, setFormError] = useState<string | null>(null)

	// Update form values when queries complete for local organizations
	useEffect(() => {
		if (organizations && nodes?.items) {
			const defaultOrgs = organizations.items?.map((org) => {
				const orderers = nodes.items?.filter((node) => node.platform === 'FABRIC' && node.nodeType === 'FABRIC_ORDERER' && node.fabricOrderer?.mspId === org.mspId)
				const peers = nodes.items?.filter((node) => node.platform === 'FABRIC' && node.nodeType === 'FABRIC_PEER' && node.fabricPeer?.mspId === org.mspId)
				const isPeer = peers && peers.length > 0
				const isOrderer = orderers && orderers.length > 0
				return {
					id: org.id!,
					enabled: isPeer || isOrderer,
					isPeer,
					isOrderer,
					consenters: orderers?.map((orderer) => orderer.id!) || [],
				}
			})
			form.setValue('organizations', defaultOrgs, { shouldDirty: true })
		}
	}, [organizations, nodes, form])

	// Process external organizations and orderers

	const organizationsWithNodes = useMemo(
		() =>
			organizations?.items?.map((org) => ({
				...org,
				orderers: nodes?.items?.filter((node) => node.platform === 'FABRIC' && node.nodeType === 'FABRIC_ORDERER' && node.fabricOrderer?.mspId === org.mspId) || [],
			})) as OrganizationWithNodes[],
		[organizations, nodes]
	)

	const onSubmit = async (data: ChannelFormValues) => {
		try {
			const enabledLocalOrgs = data.organizations
				.filter((org) => org.enabled)
				.map((org) => ({
					id: org.id,
					nodeIds: org.isOrderer ? org.consenters : [],
					isPeer: org.isPeer,
					isOrderer: org.isOrderer,
				}))

			const config: HttpFabricNetworkConfig = {
				// Consensus configuration
				consensusType: data.consensusType,

				// Capabilities configuration
				channelCapabilities: data.channelCapabilities,
				applicationCapabilities: data.applicationCapabilities,
				ordererCapabilities: data.ordererCapabilities,

				// Batch configuration
				batchSize: data.batchSize,
				batchTimeout: data.batchTimeout,

				// etcdraft options (only if consensus type is etcdraft)
				...(data.consensusType === 'etcdraft' && {
					etcdRaftOptions: data.etcdRaftOptions,
				}),

				// SmartBFT options (only if consensus type is smartbft)
				...(data.consensusType === 'smartbft' && {
					smartBFTOptions: data.smartBFTOptions,
					smartBFTConsenters: (() => {
						// Collect all selected consenters from local organizations
						const localConsenters: HttpSmartBftConsenter[] = []
						data.organizations
							.filter((org) => org.enabled && org.isOrderer)
							.forEach((org) => {
								org.consenters.forEach((consenterId) => {
									const orderer = nodes?.items?.find((node) => node.id === consenterId)
									if (orderer) {
										localConsenters.push({
											id: orderer.id,
											address: {
												host: orderer.fabricOrderer?.externalEndpoint?.split(':')[0] || 'localhost',
												port: parseInt(orderer.fabricOrderer?.externalEndpoint?.split(':')[1] || '7050'),
											},
											clientTLSCert: orderer.fabricOrderer?.tlsCert || '',
											serverTLSCert: orderer.fabricOrderer?.tlsCert || '',
											identity: orderer.fabricOrderer?.signCert || '',
											mspId: orderer.fabricOrderer?.mspId || '',
										})
									}
								})
							})

						return localConsenters
					})(),
				}),

				// Organization configuration
				peerOrganizations: enabledLocalOrgs
					.filter((org) => org.isPeer)
					.map((org) => ({
						id: org.id,
						nodeIds: [],
					})),
				ordererOrganizations: enabledLocalOrgs
					.filter((org) => org.isOrderer)
					.map((org) => ({
						id: org.id,
						nodeIds: org.nodeIds,
					})),
			}

			// Add policy configuration if enabled
			if (data.configurePolicies) {
				config.applicationPolicies = Object.entries(data.applicationPolicies || {}).reduce(
					(acc, [name, policy]) => {
						if (policy.type === 'ImplicitMeta') {
							acc[name] = {
								type: policy.type,
								rule: policy.rule,
							}
						} else if (policy.type === 'Signature') {
							// Get the MSP IDs for the selected organizations
							const selectedOrgs =
								policy.organizations
									?.map((orgId) => {
										// Check if it's a local organization
										const localOrg = organizations?.items?.find((o) => o.id?.toString() === orgId)
										if (localOrg) {
											return localOrg.mspId
										}
										return null
									})
									.filter(Boolean) || []

							// Build the policy string based on the operator
							let policyString = ''
							// Map policy names to their correct role identifiers
							const roleMap: Record<string, string> = {
								Admins: 'admin',
								Writers: 'member',
								Readers: 'member',
								LifecycleEndorsement: 'member',
								Endorsement: 'member',
								BlockValidation: 'member',
							}
							const role = roleMap[name] || 'member'

							if (policy.signatureOperator === 'OR') {
								policyString = selectedOrgs.map((mspId) => `'${mspId}.${role}'`).join(',')
								policyString = `OR(${policyString})`
							} else if (policy.signatureOperator === 'AND') {
								policyString = selectedOrgs.map((mspId) => `'${mspId}.${role}'`).join(',')
								policyString = `AND(${policyString})`
							} else if (policy.signatureOperator === 'OUTOF' && policy.signatureN) {
								policyString = selectedOrgs.map((mspId) => `'${mspId}.${role}'`).join(',')
								policyString = `OutOf(${policy.signatureN},${policyString})`
							}
							const backendPolicy: HttpFabricPolicy = {
								type: policy.type,
								rule: policyString,
							}
							acc[name] = backendPolicy
						}
						return acc
					},
					{} as Record<string, HttpFabricPolicy>
				)

				config.ordererPolicies = Object.entries(data.ordererPolicies || {}).reduce(
					(acc, [name, policy]) => {
						if (policy.type === 'ImplicitMeta') {
							acc[name] = {
								type: policy.type,
								rule: policy.rule,
							}
						} else if (policy.type === 'Signature') {
							// Get the MSP IDs for the selected organizations
							const selectedOrgs =
								policy.organizations
									?.map((orgId) => {
										// Check if it's a local organization
										const localOrg = organizations?.items?.find((o) => o.id?.toString() === orgId)
										if (localOrg) {
											return localOrg.mspId
										}
										return null
									})
									.filter(Boolean) || []

							// Build the policy string based on the operator
							let policyString = ''
							// Map policy names to their correct role identifiers
							const roleMap: Record<string, string> = {
								Admins: 'admin',
								Writers: 'member',
								Readers: 'member',
								LifecycleEndorsement: 'member',
								Endorsement: 'member',
								BlockValidation: 'member',
							}
							const role = roleMap[name] || 'member'

							if (policy.signatureOperator === 'OR') {
								policyString = selectedOrgs.map((mspId) => `'${mspId}.${role}'`).join(',')
								policyString = `OR(${policyString})`
							} else if (policy.signatureOperator === 'AND') {
								policyString = selectedOrgs.map((mspId) => `'${mspId}.${role}'`).join(',')
								policyString = `AND(${policyString})`
							} else if (policy.signatureOperator === 'OUTOF' && policy.signatureN) {
								policyString = selectedOrgs.map((mspId) => `'${mspId}.${role}'`).join(',')
								policyString = `OutOf(${policy.signatureN},${policyString})`
							}

							acc[name] = {
								type: policy.type,
								rule: policyString,
							}
						}
						return acc
					},
					{} as Record<string, any>
				)
			}

			await createNetwork.mutate({
				body: {
					name: data.name,
					config,
					description: '',
				},
			})
		} catch (error: any) {
			const errorMessage = error.message || 'An unexpected error occurred. Please try again later.'
			setFormError(errorMessage)
			toast.error('Network creation failed', {
				description: errorMessage,
			})
		}
	}

	const isLoading = isLoadingOrgs || isLoadingNodes

	// Add after the form initialization
	const defaultApplicationPolicies = {
		Readers: { type: 'ImplicitMeta' as const, rule: 'ANY Readers' },
		Writers: { type: 'ImplicitMeta' as const, rule: 'ANY Writers' },
		Admins: { type: 'ImplicitMeta' as const, rule: 'MAJORITY Admins' },
		LifecycleEndorsement: { type: 'ImplicitMeta' as const, rule: 'MAJORITY Endorsement' },
		Endorsement: { type: 'ImplicitMeta' as const, rule: 'MAJORITY Endorsement' },
	}

	const defaultOrdererPolicies = {
		Readers: { type: 'ImplicitMeta' as const, rule: 'ANY Readers' },
		Writers: { type: 'ImplicitMeta' as const, rule: 'ANY Writers' },
		Admins: { type: 'ImplicitMeta' as const, rule: 'MAJORITY Admins' },
		BlockValidation: { type: 'ImplicitMeta' as const, rule: 'ANY Writers' },
	}

	// Add after the form initialization
	useEffect(() => {
		if (form.watch('configurePolicies')) {
			form.setValue('applicationPolicies', defaultApplicationPolicies)
			form.setValue('ordererPolicies', defaultOrdererPolicies)
		} else {
			form.setValue('applicationPolicies', undefined)
			form.setValue('ordererPolicies', undefined)
		}
	}, [form.watch('configurePolicies')])

	return (
		<div className="flex-1 p-8">
			<div className="max-w-4xl mx-auto">
				<div className="mb-8">
					<h1 className="text-2xl font-semibold">Configure Channel</h1>
					<p className="text-muted-foreground">Create a new Fabric channel</p>
				</div>

				{formError && (
					<Alert variant="destructive" className="mb-6">
						<AlertCircle className="h-4 w-4" />
						<AlertTitle>Validation Error</AlertTitle>
						<AlertDescription>
							<div className="space-y-2">
								{formError.split('\n').map(
									(line, index) =>
										line.trim() && (
											<div key={index} className="flex items-start gap-2">
												<span>•</span>
												<span>{line.trim()}</span>
											</div>
										)
								)}
							</div>
						</AlertDescription>
					</Alert>
				)}

				<Form {...form}>
					<form
						onSubmit={form.handleSubmit(onSubmit, (errors) => {
							// Create a more specific error message based on the actual validation errors
							const errorMessages: string[] = []
							Object.entries(errors).forEach(([key, value]) => {
								errorMessages.push(`${key ? `${key}: ${value.message || 'Unknown error'}` : value.message || 'Unknown error'}`)
							})

							// Set the specific error message
							setFormError(errorMessages.join('\n'))

							toast.error('Please fix the validation errors', {
								description: 'There are errors in the form that need to be fixed before you can create a network.',
							})
						})}
						className="space-y-8"
					>
						<Card className={cn(form.formState.errors.name && 'border-destructive')}>
							<CardHeader>
								<div className="flex items-center justify-between">
									<div className="flex items-center gap-2">
										<Network className="h-5 w-5 text-muted-foreground" />
										<div>
											<CardTitle>Channel Information</CardTitle>
											<CardDescription>Basic channel configuration</CardDescription>
										</div>
									</div>
									{form.formState.errors.name && <AlertCircle className="h-5 w-5 text-destructive" />}
								</div>
							</CardHeader>
							<CardContent>
								<FormField
									control={form.control}
									name="name"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Channel Name</FormLabel>
											<FormControl>
												<Input placeholder="mychannel" {...field} />
											</FormControl>
											<FormDescription>The name of the channel to be created</FormDescription>
											<FormMessage />
										</FormItem>
									)}
								/>
							</CardContent>
						</Card>

						<Card>
							<CardHeader>
								<div className="flex items-center gap-2">
									<Settings className="h-5 w-5 text-muted-foreground" />
									<div>
										<CardTitle>Capabilities Configuration</CardTitle>
										<CardDescription>Configure channel, application, and orderer capabilities</CardDescription>
									</div>
								</div>
							</CardHeader>
							<CardContent className="space-y-6">
								{/* Channel Capabilities */}
								<FormField
									control={form.control}
									name="channelCapabilities"
									render={({ field }) => {
										const consensusType = form.watch('consensusType')
										const isSmartBFT = consensusType === 'smartbft'

										const capabilities = ['V2_0', 'V3_0']

										return (
											<FormItem>
												<FormLabel>Channel Capabilities</FormLabel>
												<FormDescription>
													{isSmartBFT
														? 'SmartBFT requires V3_0 channel capabilities. V3_0 is automatically selected and cannot be removed.'
														: 'Select channel capabilities. V2_0 and V3_0 are available.'}
												</FormDescription>
												<div className="flex flex-wrap gap-2 mt-2">
													{capabilities.map((capability) => (
														<Toggle
															key={capability}
															pressed={field.value?.includes(capability)}
															onPressedChange={(pressed) => {
																if (pressed) {
																	field.onChange([...(field.value || []), capability])
																} else if (!(isSmartBFT && capability === 'V3_0')) {
																	field.onChange((field.value || []).filter((c) => c !== capability))
																}
															}}
															disabled={isSmartBFT && capability === 'V3_0'}
															variant="outline"
															className="data-[state=on]:bg-primary data-[state=on]:text-primary-foreground"
														>
															{capability}
														</Toggle>
													))}
												</div>
												<FormMessage />
											</FormItem>
										)
									}}
								/>

								{/* Application Capabilities */}
								<FormField
									control={form.control}
									name="applicationCapabilities"
									render={({ field }) => {
										const capabilities = ['V2_0', 'V2_5']

										return (
											<FormItem>
												<FormLabel>Application Capabilities</FormLabel>
												<FormDescription>Select application capabilities for the channel. By default, both V2_0 and V2_5 are enabled.</FormDescription>
												<div className="flex flex-wrap gap-2 mt-2">
													{capabilities.map((capability) => (
														<Toggle
															key={capability}
															pressed={field.value?.includes(capability)}
															onPressedChange={(pressed) => {
																if (pressed) {
																	field.onChange([...(field.value || []), capability])
																} else {
																	field.onChange((field.value || []).filter((c) => c !== capability))
																}
															}}
															variant="outline"
															className="data-[state=on]:bg-primary data-[state=on]:text-primary-foreground"
														>
															{capability}
														</Toggle>
													))}
												</div>
												<FormMessage />
											</FormItem>
										)
									}}
								/>

								{/* Orderer Capabilities */}
								<FormField
									control={form.control}
									name="ordererCapabilities"
									render={({ field }) => {
										const capabilities = ['V2_0']

										return (
											<FormItem>
												<FormLabel>Orderer Capabilities</FormLabel>
												<FormDescription>Select orderer capabilities for the channel. By default, V2_0 is enabled.</FormDescription>
												<div className="flex flex-wrap gap-2 mt-2">
													{capabilities.map((capability) => (
														<Toggle
															key={capability}
															pressed={field.value?.includes(capability)}
															onPressedChange={(pressed) => {
																if (pressed) {
																	field.onChange([...(field.value || []), capability])
																} else {
																	field.onChange((field.value || []).filter((c) => c !== capability))
																}
															}}
															variant="outline"
															className="data-[state=on]:bg-primary data-[state=on]:text-primary-foreground"
														>
															{capability}
														</Toggle>
													))}
												</div>
												<FormMessage />
											</FormItem>
										)
									}}
								/>
							</CardContent>
						</Card>

						<Card>
							<CardHeader>
								<div className="flex items-center gap-2">
									<Settings className="h-5 w-5 text-muted-foreground" />
									<div>
										<CardTitle>Consensus & Batch Configuration</CardTitle>
										<CardDescription>Configure consensus type and batch processing settings</CardDescription>
									</div>
								</div>
							</CardHeader>
							<CardContent className="space-y-6">
								{/* Consensus Type Selection */}
								<FormField
									control={form.control}
									name="consensusType"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Consensus Type</FormLabel>
											<Select onValueChange={field.onChange} defaultValue={field.value}>
												<FormControl>
													<SelectTrigger>
														<SelectValue placeholder="Select consensus type" />
													</SelectTrigger>
												</FormControl>
												<SelectContent>
													<SelectItem value="etcdraft">etcdraft (Raft-based consensus)</SelectItem>
													<SelectItem value="smartbft">SmartBFT (Byzantine Fault Tolerant)</SelectItem>
												</SelectContent>
											</Select>
											<FormDescription>Choose the consensus mechanism for the channel. etcdraft is recommended for most use cases.</FormDescription>
											<FormMessage />
										</FormItem>
									)}
								/>

								{/* Batch Configuration */}
								<div className="space-y-4">
									<h4 className="text-sm font-medium">Batch Configuration</h4>
									<div className="grid grid-cols-1 md:grid-cols-3 gap-4">
										<FormField
											control={form.control}
											name="batchSize.maxMessageCount"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Max Message Count</FormLabel>
													<FormControl>
														<Input type="number" {...field} onChange={(e) => field.onChange(parseInt(e.target.value))} />
													</FormControl>
													<FormDescription>Maximum number of messages in a batch</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>
										<FormField
											control={form.control}
											name="batchSize.absoluteMaxBytes"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Absolute Max Bytes</FormLabel>
													<FormControl>
														<Input type="number" {...field} onChange={(e) => field.onChange(parseInt(e.target.value))} />
													</FormControl>
													<FormDescription>Maximum batch size in bytes</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>
										<FormField
											control={form.control}
											name="batchSize.preferredMaxBytes"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Preferred Max Bytes</FormLabel>
													<FormControl>
														<Input type="number" {...field} onChange={(e) => field.onChange(parseInt(e.target.value))} />
													</FormControl>
													<FormDescription>Preferred batch size in bytes</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>
									</div>
									<FormField
										control={form.control}
										name="batchTimeout"
										render={({ field }) => (
											<FormItem>
												<FormLabel>Batch Timeout</FormLabel>
												<FormControl>
													<Input placeholder="2s" {...field} />
												</FormControl>
												<FormDescription>Time to wait before creating a new batch (e.g., "2s", "500ms")</FormDescription>
												<FormMessage />
											</FormItem>
										)}
									/>
								</div>

								{/* etcdraft Options */}
								{form.watch('consensusType') === 'etcdraft' && (
									<div className="space-y-4">
										<h4 className="text-sm font-medium">etcdraft Options</h4>
										<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
											<FormField
												control={form.control}
												name="etcdRaftOptions.tickInterval"
												render={({ field }) => (
													<FormItem>
														<FormLabel>Tick Interval</FormLabel>
														<FormControl>
															<Input placeholder="500ms" {...field} />
														</FormControl>
														<FormDescription>Time interval between ticks</FormDescription>
														<FormMessage />
													</FormItem>
												)}
											/>
											<FormField
												control={form.control}
												name="etcdRaftOptions.electionTick"
												render={({ field }) => (
													<FormItem>
														<FormLabel>Election Tick</FormLabel>
														<FormControl>
															<Input type="number" {...field} onChange={(e) => field.onChange(parseInt(e.target.value))} />
														</FormControl>
														<FormDescription>Number of ticks before election timeout</FormDescription>
														<FormMessage />
													</FormItem>
												)}
											/>
											<FormField
												control={form.control}
												name="etcdRaftOptions.heartbeatTick"
												render={({ field }) => (
													<FormItem>
														<FormLabel>Heartbeat Tick</FormLabel>
														<FormControl>
															<Input type="number" {...field} onChange={(e) => field.onChange(parseInt(e.target.value))} />
														</FormControl>
														<FormDescription>Number of ticks between heartbeats</FormDescription>
														<FormMessage />
													</FormItem>
												)}
											/>
											<FormField
												control={form.control}
												name="etcdRaftOptions.maxInflightBlocks"
												render={({ field }) => (
													<FormItem>
														<FormLabel>Max Inflight Blocks</FormLabel>
														<FormControl>
															<Input type="number" {...field} onChange={(e) => field.onChange(parseInt(e.target.value))} />
														</FormControl>
														<FormDescription>Maximum number of inflight blocks</FormDescription>
														<FormMessage />
													</FormItem>
												)}
											/>
											<FormField
												control={form.control}
												name="etcdRaftOptions.snapshotIntervalSize"
												render={({ field }) => (
													<FormItem>
														<FormLabel>Snapshot Interval Size</FormLabel>
														<FormControl>
															<Input type="number" {...field} onChange={(e) => field.onChange(parseInt(e.target.value))} />
														</FormControl>
														<FormDescription>Number of blocks between snapshots</FormDescription>
														<FormMessage />
													</FormItem>
												)}
											/>
										</div>
									</div>
								)}

								{/* SmartBFT Options */}
								{form.watch('consensusType') === 'smartbft' && (
									<div className="space-y-4">
										<h4 className="text-sm font-medium">SmartBFT Options</h4>
										<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
											<FormField
												control={form.control}
												name="smartBFTOptions.requestBatchMaxCount"
												render={({ field }) => (
													<FormItem>
														<FormLabel>Request Batch Max Count</FormLabel>
														<FormControl>
															<Input type="number" {...field} onChange={(e) => field.onChange(parseInt(e.target.value))} />
														</FormControl>
														<FormDescription>Maximum number of requests in a batch</FormDescription>
														<FormMessage />
													</FormItem>
												)}
											/>
											<FormField
												control={form.control}
												name="smartBFTOptions.requestBatchMaxBytes"
												render={({ field }) => (
													<FormItem>
														<FormLabel>Request Batch Max Bytes</FormLabel>
														<FormControl>
															<Input type="number" {...field} onChange={(e) => field.onChange(parseInt(e.target.value))} />
														</FormControl>
														<FormDescription>Maximum batch size in bytes</FormDescription>
														<FormMessage />
													</FormItem>
												)}
											/>
											<FormField
												control={form.control}
												name="smartBFTOptions.requestBatchMaxInterval"
												render={({ field }) => (
													<FormItem>
														<FormLabel>Request Batch Max Interval</FormLabel>
														<FormControl>
															<Input placeholder="50ms" {...field} />
														</FormControl>
														<FormDescription>Maximum time to wait for batch completion</FormDescription>
														<FormMessage />
													</FormItem>
												)}
											/>
											<FormField
												control={form.control}
												name="smartBFTOptions.viewChangeTimeout"
												render={({ field }) => (
													<FormItem>
														<FormLabel>View Change Timeout</FormLabel>
														<FormControl>
															<Input placeholder="20s" {...field} />
														</FormControl>
														<FormDescription>Timeout for view change operations</FormDescription>
														<FormMessage />
													</FormItem>
												)}
											/>
										</div>
									</div>
								)}
							</CardContent>
						</Card>

						{form.formState.errors.organizations?.root?.message && (
							<div className="flex flex-col items-center justify-center p-8 border-2 border-dashed rounded-lg border-destructive/20 bg-destructive/5">
								<div className="h-12 w-12 text-destructive mb-4 flex items-center justify-center">
									<TriangleAlert className="h-8 w-8" />
								</div>
								<div className="space-y-2 text-center">
									<FormMessage>{form.formState.errors.organizations?.root?.message}</FormMessage>
									<p className="text-sm text-muted-foreground">
										Requirements:
										<ul className="list-disc list-inside mt-1">
											<li>At least one organization must be enabled</li>
											<li>At least one organization must have consenters</li>
											<li>Total consenters across all organizations must be at least 3</li>
											{form.watch('consensusType') === 'smartbft' && <li>SmartBFT requires at least 4 consenters for Byzantine fault tolerance</li>}
										</ul>
									</p>
								</div>
							</div>
						)}

						<Card className={cn(form.formState.errors.organizations && 'border-destructive')}>
							<CardHeader>
								<div className="flex items-center justify-between">
									<div className="flex items-center gap-2">
										<Users className="h-5 w-5 text-muted-foreground" />
										<div>
											<CardTitle>Local Organizations</CardTitle>
											<CardDescription>Configure organizations from your local network</CardDescription>
										</div>
									</div>
									{form.formState.errors.organizations && <AlertCircle className="h-5 w-5 text-destructive" />}
								</div>
							</CardHeader>
							<CardContent className="space-y-6">
								{isLoadingOrgs || isLoadingNodes ? (
									Array.from({ length: 3 }).map((_, i) => (
										<div key={i} className="space-y-4 rounded-lg border p-4">
											<div className="flex items-center justify-between">
												<div className="flex items-center gap-4">
													<Skeleton className="h-4 w-[40px]" />
													<div>
														<Skeleton className="h-5 w-[150px] mb-2" />
														<Skeleton className="h-4 w-[200px]" />
													</div>
												</div>
												<Skeleton className="h-5 w-[100px]" />
											</div>
										</div>
									))
								) : organizationsWithNodes?.length === 0 ? (
									<div className="text-center p-4 border rounded-lg">
										<p className="text-muted-foreground">No local organizations available</p>
									</div>
								) : (
									organizationsWithNodes?.map((org, index) => (
										<div key={org.id} className={cn('space-y-4 rounded-lg border p-4', form.formState.errors.organizations?.[index] && 'border-destructive')}>
											<div className="flex items-center justify-between">
												<div className="flex items-center gap-4">
													<FormField
														control={form.control}
														name={`organizations.${index}.enabled`}
														render={({ field }) => (
															<FormItem className="flex items-center gap-2 space-y-0">
																<FormControl>
																	<Switch checked={field.value} onCheckedChange={field.onChange} />
																</FormControl>
																<div>
																	<h3 className="font-medium">{org.mspId}</h3>
																	<p className="text-sm text-muted-foreground">{org.description}</p>
																</div>
															</FormItem>
														)}
													/>
												</div>
												<Badge variant="outline">{org.orderers.length} Orderers</Badge>
											</div>

											{form.watch(`organizations.${index}.enabled`) && (
												<>
													<div className="flex gap-6">
														<FormField
															control={form.control}
															name={`organizations.${index}.isPeer`}
															render={({ field }) => (
																<FormItem className="flex items-center gap-2">
																	<FormControl>
																		<Checkbox checked={field.value} onCheckedChange={field.onChange} />
																	</FormControl>
																	<FormLabel className="!mt-0">Peer Organization</FormLabel>
																</FormItem>
															)}
														/>
														<FormField
															control={form.control}
															name={`organizations.${index}.isOrderer`}
															render={({ field }) => (
																<FormItem className="flex items-center gap-2">
																	<FormControl>
																		<Checkbox checked={field.value} onCheckedChange={field.onChange} />
																	</FormControl>
																	<FormLabel className="!mt-0">Orderer Organization</FormLabel>
																</FormItem>
															)}
														/>
													</div>

													{form.watch(`organizations.${index}.isOrderer`) && org.orderers.length > 0 && (
														<>
															<Separator />
															<FormField
																control={form.control}
																name={`organizations.${index}.consenters`}
																render={({ field }) => (
																	<FormItem>
																		<div className="flex items-center justify-between">
																			<FormLabel>Consenters</FormLabel>
																			{form.formState.errors.organizations?.[index]?.consenters && (
																				<div className="text-sm text-destructive">
																					{Array.isArray(form.formState.errors.organizations[index].consenters)
																						? form.formState.errors.organizations[index].consenters.map((err: any, i: number) => (
																								<span key={i}>{err?.message || JSON.stringify(err)}</span>
																							))
																						: form.formState.errors.organizations[index].consenters?.message ||
																							JSON.stringify(form.formState.errors.organizations[index].consenters)}
																				</div>
																			)}
																		</div>
																		<div className="grid gap-2">
																			{org.orderers.map((orderer) => (
																				<FormItem key={orderer.id} className="flex items-center gap-2">
																					<FormControl>
																						<Checkbox
																							checked={field.value?.includes(orderer.id!)}
																							onCheckedChange={(checked) => {
																								const value = checked
																									? [...(field.value || []), orderer.id]
																									: (field.value || []).filter((id) => id !== orderer.id)
																								field.onChange(value)
																							}}
																						/>
																					</FormControl>
																					<FormLabel className="!mt-0">{orderer.name}</FormLabel>
																				</FormItem>
																			))}
																		</div>
																		{org.orderers.length > 0 && field.value?.length === 0 && form.watch(`organizations.${index}.isOrderer`) && (
																			<p className="text-sm text-amber-500 mt-2">
																				<AlertCircle className="h-3 w-3 inline-block mr-1" />
																				You should select at least one consenter for this orderer organization
																			</p>
																		)}
																		<FormMessage />
																	</FormItem>
																)}
															/>
														</>
													)}
												</>
											)}
										</div>
									))
								)}
							</CardContent>
						</Card>

						<Card>
							<CardHeader>
								<div className="flex items-center gap-2">
									<Settings className="h-5 w-5 text-muted-foreground" />
									<div>
										<CardTitle>Channel Policies</CardTitle>
										<CardDescription>Configure application and orderer policies</CardDescription>
									</div>
								</div>
							</CardHeader>
							<CardContent>
								<FormField
									control={form.control}
									name="configurePolicies"
									render={({ field }) => (
										<FormItem className="flex items-center gap-2">
											<FormControl>
												<Switch checked={field.value} onCheckedChange={field.onChange} />
											</FormControl>
											<FormLabel>Configure Policies</FormLabel>
										</FormItem>
									)}
								/>

								{form.watch('configurePolicies') && (
									<div className="space-y-6 mt-6">
										<div>
											<h3 className="text-lg font-medium mb-4">Application Policies</h3>
											<div className="space-y-4">
												{Object.entries(form.watch('applicationPolicies') || {}).map(([policyName, policy]) => (
													<div key={policyName} className="space-y-4 p-4 border rounded-lg">
														<div className="flex items-center justify-between">
															<h4 className="font-medium">{policyName}</h4>
														</div>
														<div className="space-y-4">
															<FormField
																control={form.control}
																name={`applicationPolicies.${policyName}.type`}
																render={({ field }) => (
																	<FormItem>
																		<FormLabel>Policy Type</FormLabel>
																		<Select onValueChange={field.onChange} defaultValue={field.value}>
																			<FormControl>
																				<SelectTrigger>
																					<SelectValue placeholder="Select policy type" />
																				</SelectTrigger>
																			</FormControl>
																			<SelectContent>
																				<SelectItem value="ImplicitMeta">ImplicitMeta</SelectItem>
																				<SelectItem value="Signature">Signature</SelectItem>
																			</SelectContent>
																		</Select>
																		<FormMessage />
																	</FormItem>
																)}
															/>
															{policy.type === 'ImplicitMeta' && (
																<FormField
																	control={form.control}
																	name={`applicationPolicies.${policyName}.rule`}
																	render={({ field }) => (
																		<FormItem>
																			<FormLabel>Rule</FormLabel>
																			<Select onValueChange={field.onChange} defaultValue={field.value}>
																				<FormControl>
																					<SelectTrigger>
																						<SelectValue placeholder="Select rule" />
																					</SelectTrigger>
																				</FormControl>
																				<SelectContent>
																					<SelectItem value="ANY Readers">ANY Readers</SelectItem>
																					<SelectItem value="ANY Writers">ANY Writers</SelectItem>
																					<SelectItem value="ANY Admins">ANY Admins</SelectItem>
																					<SelectItem value="MAJORITY Readers">MAJORITY Readers</SelectItem>
																					<SelectItem value="MAJORITY Writers">MAJORITY Writers</SelectItem>
																					<SelectItem value="MAJORITY Admins">MAJORITY Admins</SelectItem>
																					<SelectItem value="MAJORITY Endorsement">MAJORITY Endorsement</SelectItem>
																				</SelectContent>
																			</Select>
																			<FormMessage />
																		</FormItem>
																	)}
																/>
															)}
															{policy.type === 'Signature' && (
																<>
																	<FormField
																		control={form.control}
																		name={`applicationPolicies.${policyName}.signatureOperator`}
																		render={({ field }) => (
																			<FormItem>
																				<FormLabel>Signature Operator</FormLabel>
																				<Select onValueChange={field.onChange} defaultValue={field.value}>
																					<FormControl>
																						<SelectTrigger>
																							<SelectValue placeholder="Select operator" />
																						</SelectTrigger>
																					</FormControl>
																					<SelectContent>
																						<SelectItem value="OR">OR</SelectItem>
																						<SelectItem value="AND">AND</SelectItem>
																						<SelectItem value="OUTOF">OUTOF</SelectItem>
																					</SelectContent>
																				</Select>
																				<FormMessage />
																			</FormItem>
																		)}
																	/>
																	{policy.signatureOperator === 'OUTOF' && (
																		<FormField
																			control={form.control}
																			name={`applicationPolicies.${policyName}.signatureN`}
																			render={({ field }) => (
																				<FormItem>
																					<FormLabel>N (Number of required signatures)</FormLabel>
																					<FormControl>
																						<Input type="number" {...field} onChange={(e) => field.onChange(parseInt(e.target.value))} />
																					</FormControl>
																					<FormMessage />
																				</FormItem>
																			)}
																		/>
																	)}
																	<FormField
																		control={form.control}
																		name={`applicationPolicies.${policyName}.organizations`}
																		render={({ field }) => (
																			<FormItem>
																				<FormLabel>Organizations</FormLabel>
																				<div className="space-y-2">
																					{organizations?.items?.map((org) => (
																						<div key={org.id} className="flex items-center gap-2">
																							<Checkbox
																								checked={field.value?.includes(org.id?.toString() || '')}
																								onCheckedChange={(checked) => {
																									const value = checked
																										? [...(field.value || []), org.id?.toString() || '']
																										: (field.value || []).filter((id) => id !== org.id?.toString())
																									field.onChange(value)
																								}}
																							/>
																							<span className="text-sm">{org.mspId}</span>
																						</div>
																					))}
																				</div>
																				<FormMessage />
																			</FormItem>
																		)}
																	/>
																</>
															)}
														</div>
													</div>
												))}
											</div>
										</div>

										<div>
											<h3 className="text-lg font-medium mb-4">Orderer Policies</h3>
											<div className="space-y-4">
												{Object.entries(form.watch('ordererPolicies') || {}).map(([policyName, policy]) => (
													<div key={policyName} className="space-y-4 p-4 border rounded-lg">
														<div className="flex items-center justify-between">
															<h4 className="font-medium">{policyName}</h4>
														</div>
														<div className="space-y-4">
															<FormField
																control={form.control}
																name={`ordererPolicies.${policyName}.type`}
																render={({ field }) => (
																	<FormItem>
																		<FormLabel>Policy Type</FormLabel>
																		<Select onValueChange={field.onChange} defaultValue={field.value}>
																			<FormControl>
																				<SelectTrigger>
																					<SelectValue placeholder="Select policy type" />
																				</SelectTrigger>
																			</FormControl>
																			<SelectContent>
																				<SelectItem value="ImplicitMeta">ImplicitMeta</SelectItem>
																				<SelectItem value="Signature">Signature</SelectItem>
																			</SelectContent>
																		</Select>
																		<FormMessage />
																	</FormItem>
																)}
															/>
															{policy.type === 'ImplicitMeta' && (
																<FormField
																	control={form.control}
																	name={`ordererPolicies.${policyName}.rule`}
																	render={({ field }) => (
																		<FormItem>
																			<FormLabel>Rule</FormLabel>
																			<Select onValueChange={field.onChange} defaultValue={field.value}>
																				<FormControl>
																					<SelectTrigger>
																						<SelectValue placeholder="Select rule" />
																					</SelectTrigger>
																				</FormControl>
																				<SelectContent>
																					<SelectItem value="ANY Readers">ANY Readers</SelectItem>
																					<SelectItem value="ANY Writers">ANY Writers</SelectItem>
																					<SelectItem value="ANY Admins">ANY Admins</SelectItem>
																					<SelectItem value="MAJORITY Readers">MAJORITY Readers</SelectItem>
																					<SelectItem value="MAJORITY Writers">MAJORITY Writers</SelectItem>
																					<SelectItem value="MAJORITY Admins">MAJORITY Admins</SelectItem>
																					<SelectItem value="MAJORITY Endorsement">MAJORITY Endorsement</SelectItem>
																				</SelectContent>
																			</Select>
																			<FormMessage />
																		</FormItem>
																	)}
																/>
															)}
															{policy.type === 'Signature' && (
																<>
																	<FormField
																		control={form.control}
																		name={`ordererPolicies.${policyName}.signatureOperator`}
																		render={({ field }) => (
																			<FormItem>
																				<FormLabel>Signature Operator</FormLabel>
																				<Select onValueChange={field.onChange} defaultValue={field.value}>
																					<FormControl>
																						<SelectTrigger>
																							<SelectValue placeholder="Select operator" />
																						</SelectTrigger>
																					</FormControl>
																					<SelectContent>
																						<SelectItem value="OR">OR</SelectItem>
																						<SelectItem value="AND">AND</SelectItem>
																						<SelectItem value="OUTOF">OUTOF</SelectItem>
																					</SelectContent>
																				</Select>
																				<FormMessage />
																			</FormItem>
																		)}
																	/>
																	{policy.signatureOperator === 'OUTOF' && (
																		<FormField
																			control={form.control}
																			name={`ordererPolicies.${policyName}.signatureN`}
																			render={({ field }) => (
																				<FormItem>
																					<FormLabel>N (Number of required signatures)</FormLabel>
																					<FormControl>
																						<Input type="number" {...field} onChange={(e) => field.onChange(parseInt(e.target.value))} />
																					</FormControl>
																					<FormMessage />
																				</FormItem>
																			)}
																		/>
																	)}
																	<FormField
																		control={form.control}
																		name={`ordererPolicies.${policyName}.organizations`}
																		render={({ field }) => (
																			<FormItem>
																				<FormLabel>Organizations</FormLabel>
																				<div className="space-y-2">
																					{organizations?.items?.map((org) => (
																						<div key={org.id} className="flex items-center gap-2">
																							<Checkbox
																								checked={field.value?.includes(org.id?.toString() || '')}
																								onCheckedChange={(checked) => {
																									const value = checked
																										? [...(field.value || []), org.id?.toString() || '']
																										: (field.value || []).filter((id) => id !== org.id?.toString())
																									field.onChange(value)
																								}}
																							/>
																							<span className="text-sm">{org.mspId}</span>
																						</div>
																					))}
																				</div>
																				<FormMessage />
																			</FormItem>
																		)}
																	/>
																</>
															)}
														</div>
													</div>
												))}
											</div>
										</div>
									</div>
								)}
							</CardContent>
						</Card>

						<div className="flex justify-end">
							<Button type="submit" disabled={form.formState.isSubmitting || isLoading}>
								{form.formState.isSubmitting ? 'Creating...' : 'Create Channel'}
							</Button>
						</div>
					</form>
				</Form>
			</div>
		</div>
	)
}

import { getNodesDefaultsFabric } from '@/api/client'
import { getNodesDefaultsFabricOptions, getNodesOptions, getOrganizationsOptions, postNodesMutation } from '@/api/client/@tanstack/react-query.gen'
import { HttpCreateNodeRequest, HttpNodeResponse, TypesFabricOrdererConfig, TypesFabricPeerConfig } from '@/api/client/types.gen'
import { FabricNodeForm, FabricNodeFormValues } from '@/components/nodes/fabric-node-form'
import { NodeListItem } from '@/components/nodes/node-list-item'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Steps } from '@/components/ui/steps'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { slugify } from '@/utils'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { ArrowLeft, ArrowRight, CheckCircle2, Server, AlertCircle, XCircle } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { z } from 'zod'

interface NodeConfig extends FabricNodeFormValues {
	name: string
}

interface CreationError {
	nodeName: string
	error: string
}

interface CreationResult {
	success: HttpNodeResponse[]
	errors: CreationError[]
}

interface CreationSummaryProps {
	creationCompleted: boolean
	creationResults: CreationResult
	nodeConfigs: NodeConfig[]
	onResetForm: () => void
	onRetryFailedNodes: () => void
}

function CreationSummary({ 
	creationCompleted, 
	creationResults, 
	nodeConfigs, 
	onResetForm, 
	onRetryFailedNodes 
}: CreationSummaryProps) {
	if (!creationCompleted) return null

	const failedNodes = nodeConfigs.filter(config => 
		creationResults.errors.some(error => error.nodeName === config.name)
	)

	return (
		<div className="space-y-6">
			<div className="border rounded-lg p-6 bg-muted/50">
				<h3 className="text-lg font-semibold mb-4">Creation Summary</h3>
				
				{creationResults.success.length > 0 && (
					<div className="mb-6">
						<h4 className="font-medium text-green-600 mb-3 flex items-center gap-2">
							<CheckCircle2 className="h-4 w-4" />
							Successfully Created ({creationResults.success.length})
						</h4>
						<div className="space-y-2">
							{creationResults.success.map((node) => (
								<NodeListItem 
									key={node.id?.toString() || node.name || `node-${Math.random()}`} 
									node={node}
									isSelected={false}
									onSelectionChange={() => {}}
									disabled={false}
									showCheckbox={false}
								/>
							))}
						</div>
					</div>
				)}

				{creationResults.errors.length > 0 && (
					<div className="mb-6">
						<h4 className="font-medium text-red-600 mb-3 flex items-center gap-2">
							<XCircle className="h-4 w-4" />
							Failed to Create ({creationResults.errors.length})
						</h4>
						<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
							{failedNodes.map((config) => {
								const error = creationResults.errors.find(e => e.nodeName === config.name)
								return (
									<Card key={config.name} className="p-4 border-red-200 bg-red-50/50">
										<div className="flex items-center justify-between">
											<div className="flex items-center gap-4">
												<div className="h-10 w-10 rounded-lg bg-red-100 flex items-center justify-center">
													<Server className="h-5 w-5 text-red-600" />
												</div>
												<div>
													<h3 className="font-medium">{config.name}</h3>
													<p className="text-sm text-muted-foreground">
														FABRIC • {config.fabricProperties.nodeType === 'FABRIC_PEER' ? 'Peer' : 'Orderer'}
													</p>
												</div>
											</div>
											<Badge variant="secondary" className="bg-red-100 text-red-700">
												Failed
											</Badge>
										</div>
										<div className="mt-4 space-y-2">
											<div className="flex gap-2">
												<div className="text-xs px-2 py-1 rounded-md bg-red-100 text-red-700">
													Type: {config.fabricProperties.nodeType === 'FABRIC_PEER' ? 'Peer' : 'Orderer'}
												</div>
												{config.fabricProperties.listenAddress && (
													<div className="text-xs px-2 py-1 rounded-md bg-muted">
														Listen: {config.fabricProperties.listenAddress}
													</div>
												)}
											</div>
											<div className="text-sm text-red-600 bg-red-50 p-2 rounded-md">
												<strong>Error:</strong> {error?.error}
											</div>
										</div>
									</Card>
								)
							})}
						</div>
					</div>
				)}

				{creationCompleted && (
					<div className="mt-6 pt-4 border-t">
						<div className="flex gap-3 flex-wrap">
							<Button asChild>
								<Link to="/nodes">View All Nodes</Link>
							</Button>
							<Button variant="outline" onClick={onResetForm}>
								Create More Nodes
							</Button>
							{creationResults.errors.length > 0 && (
								<Button variant="outline" onClick={onRetryFailedNodes}>
									Try Again ({creationResults.errors.length} failed)
								</Button>
							)}
						</div>
					</div>
				)}
			</div>
		</div>
	)
}

const steps = [
	{ id: 'basic', title: 'Basic Information' },
	{ id: 'configure', title: 'Configure Nodes' },
	{ id: 'review', title: 'Review & Create' },
]

const bulkCreateSchema = z.object({
	organization: z.string().min(1, 'Organization is required'),
	peerCount: z.number().min(0).max(10),
	ordererCount: z.number().min(0).max(5),
	nodes: z
		.array(
			z.object({
				name: z.string(),
				fabricProperties: z.object({
					nodeType: z.enum(['FABRIC_PEER', 'FABRIC_ORDERER']),
					mode: z.enum(['PRODUCTION', 'DEVELOPMENT']),
					organizationId: z.string(),
					listenAddress: z.string(),
					operationsListenAddress: z.string(),
					externalEndpoint: z.string().optional(),
					domains: z.array(z.string()).optional(),
					chaincodeAddress: z.string().optional(),
					eventsAddress: z.string().optional(),
					adminAddress: z.string().optional(),
				}),
			})
		)
		.optional(),
})

type BulkCreateValues = z.infer<typeof bulkCreateSchema>

export default function BulkCreateNodesPage() {
	const navigate = useNavigate()
	const [currentStep, setCurrentStep] = useState('basic')
	const [nodeConfigs, setNodeConfigs] = useState<NodeConfig[]>([])
	const [creationProgress, setCreationProgress] = useState<{
		current: number
		total: number
		currentNode: string | null
	}>({ current: 0, total: 0, currentNode: null })
	const [creationResults, setCreationResults] = useState<CreationResult>({ success: [], errors: [] })
	const [isCreating, setIsCreating] = useState(false)
	const [creationCompleted, setCreationCompleted] = useState(false)

	const { data: organizations, isLoading: isLoadingOrgs } = useQuery({
		...getOrganizationsOptions({query: {limit:1000}}),
	})

	const form = useForm<BulkCreateValues>({
		resolver: zodResolver(bulkCreateSchema),
		defaultValues: {
			peerCount: 0,
			ordererCount: 0,
		},
	})
	const { data: defaults } = useQuery({
		...getNodesDefaultsFabricOptions({
			query: {
				ordererCount: form.watch('ordererCount'),
				peerCount: form.watch('peerCount'),
			},
		}),
	})

	const { data: existingNodes } = useQuery({
		...getNodesOptions(),
	})

	const selectedOrg = organizations?.items?.find((org) => org.id?.toString() === form.watch('organization'))
	const peerCount = form.watch('peerCount')
	const ordererCount = form.watch('ordererCount')

	const getUniqueNodeName = useCallback(
		(basePrefix: string, baseName: string, index: number, currentConfigs: NodeConfig[]): string => {
			const isNameTaken = (name: string) => {
				// Check existing nodes in the system
				const existingNodeHasName = existingNodes?.items?.some((node) => node.name === name)
				// Check nodes being created in this batch
				const configHasName = currentConfigs.some((config) => config.name === name)
				return existingNodeHasName || configHasName
			}

			const candidateName = `${basePrefix}${index}-${baseName}`
			if (!isNameTaken(candidateName)) {
				return candidateName
			}

			// If name exists, try next index
			let counter = index + 1
			while (isNameTaken(`${basePrefix}${counter}-${baseName}`)) {
				counter++
			}
			return `${basePrefix}${counter}-${baseName}`
		},
		[existingNodes]
	)

	const loadDefaults = useCallback(async () => {
		const r = await getNodesDefaultsFabric({
			query: {
				ordererCount: form.watch('ordererCount'),
				peerCount: form.watch('peerCount'),
			},
		})

		const newConfigs: NodeConfig[] = []
		const sluggedMspId = slugify(selectedOrg?.mspId || '')

		// Add peer configs
		let peerIndex = 0
		for (const peer of r.data?.peers || []) {
			const name = getUniqueNodeName('peer', sluggedMspId, peerIndex, newConfigs)
			newConfigs.push({
				name,
				fabricProperties: {
					nodeType: 'FABRIC_PEER',
					version: '3.1.0',
					mode: 'service',
					organizationId: selectedOrg?.id!,
					listenAddress: peer.listenAddress || '',
					operationsListenAddress: peer.operationsListenAddress || '',
					...peer,
				},
			})
			peerIndex++
		}

		// Add orderer configs
		let ordererIndex = 0
		for (const orderer of r.data?.orderers || []) {
			const name = getUniqueNodeName('orderer', sluggedMspId, ordererIndex, newConfigs)
			newConfigs.push({
				name,
				fabricProperties: {
					nodeType: 'FABRIC_ORDERER',
					mode: 'service',
					version: '3.1.0',
					organizationId: selectedOrg?.id!,
					listenAddress: orderer.listenAddress || '',
					operationsListenAddress: orderer.operationsListenAddress || '',
					...orderer,
				},
			})
			ordererIndex++
		}

		setNodeConfigs(newConfigs)
	}, [selectedOrg, peerCount, ordererCount, defaults, existingNodes, getUniqueNodeName])

	useEffect(() => {
		if (selectedOrg && (peerCount || ordererCount)) {
			loadDefaults()
		}
	}, [selectedOrg, peerCount, ordererCount, loadDefaults])
	const createNode = useMutation({
		...postNodesMutation(),
	})
	const onSubmit = async (_: BulkCreateValues) => {
		if (currentStep !== 'review') {
			if (currentStep === 'basic') {
				setCurrentStep('configure')
			} else if (currentStep === 'configure') {
				setCurrentStep('review')
			}
			return
		}

		try {
			setIsCreating(true)
			setCreationProgress({ current: 0, total: nodeConfigs.length, currentNode: null })
			setCreationResults({ success: [], errors: [] })
			setCreationCompleted(false)

			// Create nodes sequentially to show progress
			for (let i = 0; i < nodeConfigs.length; i++) {
				const config = nodeConfigs[i]
				setCreationProgress({
					current: i,
					total: nodeConfigs.length,
					currentNode: config.name,
				})

				try {
					let fabricPeer: TypesFabricPeerConfig | undefined
					let fabricOrderer: TypesFabricOrdererConfig | undefined

					if (config.fabricProperties.nodeType === 'FABRIC_PEER') {
						fabricPeer = {
							nodeType: 'FABRIC_PEER',
							mode: config.fabricProperties.mode,
							organizationId: config.fabricProperties.organizationId,
							listenAddress: config.fabricProperties.listenAddress,
							operationsListenAddress: config.fabricProperties.operationsListenAddress,
							externalEndpoint: config.fabricProperties.externalEndpoint,
							domainNames: config.fabricProperties.domains || [],
							name: config.name,
							chaincodeAddress: config.fabricProperties.chaincodeAddress || '',
							eventsAddress: config.fabricProperties.eventsAddress || '',
							mspId: selectedOrg?.mspId!,
							version: config.fabricProperties.version,
							addressOverrides: config.fabricProperties.addressOverrides,
						} as TypesFabricPeerConfig
					} else {
						fabricOrderer = {
							nodeType: 'FABRIC_ORDERER',
							mode: config.fabricProperties.mode,
							organizationId: config.fabricProperties.organizationId,
							listenAddress: config.fabricProperties.listenAddress,
							operationsListenAddress: config.fabricProperties.operationsListenAddress,
							externalEndpoint: config.fabricProperties.externalEndpoint,
							domainNames: config.fabricProperties.domains || [],
							name: config.name,
							adminAddress: config.fabricProperties.adminAddress || '',
							mspId: selectedOrg?.mspId!,
							version: config.fabricProperties.version,
						} as TypesFabricOrdererConfig
					}

					const createNodeDto: HttpCreateNodeRequest = {
						name: config.name,
						blockchainPlatform: 'FABRIC',
						fabricPeer,
						fabricOrderer,
					}

					const response = await createNode.mutateAsync({
						body: createNodeDto,
					})

					// Track successful creation with the actual node response
					setCreationResults(prev => ({
						...prev,
						success: [...prev.success, response]
					}))

				} catch (error: any) {
					// Track failed creation
					const errorMessage = error?.error?.message || error?.message || 'Unknown error occurred'
					setCreationResults(prev => ({
						...prev,
						errors: [...prev.errors, {
							nodeName: config.name,
							error: errorMessage
						}]
					}))
				}
			}

			setCreationProgress({
				current: nodeConfigs.length,
				total: nodeConfigs.length,
				currentNode: null,
			})

			setCreationCompleted(true)

			// Show appropriate toast based on results
			if (creationResults.errors.length === 0) {
				toast.success('All nodes created successfully')
			} else if (creationResults.success.length === 0) {
				toast.error('Failed to create any nodes')
			} else {
				toast.success(`Created ${creationResults.success.length} nodes`, {
					description: `${creationResults.errors.length} nodes failed to create`
				})
			}

		} catch (error: any) {
			toast.error('Failed to create nodes', {
				description: error.error?.message || error.message,
			})
		} finally {
			setIsCreating(false)
		}
	}

	const canProceed = () => {
		if (currentStep === 'basic') {
			return form.watch('organization') && (form.watch('peerCount') > 0 || form.watch('ordererCount') > 0)
		}
		if (currentStep === 'configure') {
			return nodeConfigs.length > 0
		}
		return true
	}

	const getCreationStatus = () => {
		if (creationProgress.total === 0) return null
		
		if (creationProgress.current < creationProgress.total) {
			return 'creating'
		}
		
		if (creationResults.errors.length === 0) {
			return 'success'
		} else if (creationResults.success.length === 0) {
			return 'failed'
		} else {
			return 'partial'
		}
	}

	const creationStatus = getCreationStatus()

	const resetForm = () => {
		setCreationCompleted(false)
		setCreationResults({ success: [], errors: [] })
		setCreationProgress({ current: 0, total: 0, currentNode: null })
		setCurrentStep('basic')
		form.reset({
			organization: '',
			peerCount: 0,
			ordererCount: 0,
		})
		setNodeConfigs([])
	}

	const retryFailedNodes = () => {
		// Filter out only the failed nodes and retry them
		const failedNodes = nodeConfigs.filter(config => 
			creationResults.errors.some(error => error.nodeName === config.name)
		)
		setNodeConfigs(failedNodes)
		setCreationCompleted(false)
		setCreationResults({ success: [], errors: [] })
		setCreationProgress({ current: 0, total: 0, currentNode: null })
		setCurrentStep('review')
	}

	return (
		<div className="flex-1 p-8">
			<div className="max-w-3xl mx-auto">
				<div className="flex items-center gap-2 text-muted-foreground mb-8">
					<Button variant="ghost" size="sm" asChild>
						<Link to="/nodes">
							<ArrowLeft className="mr-2 h-4 w-4" />
							Nodes
						</Link>
					</Button>
				</div>

				<div className="flex items-center gap-4 mb-8">
					<Server className="h-8 w-8" />
					<div>
						<h1 className="text-2xl font-semibold">Bulk Create Nodes</h1>
						<p className="text-muted-foreground">Create multiple peers and orderers at once</p>
					</div>
				</div>

				<Steps steps={steps} currentStep={currentStep} className="mb-8" />

				<Form {...form}>
					<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-8">
						{currentStep === 'basic' && (
							<Card className="p-6">
								<div className="space-y-6">
									<FormField
										control={form.control}
										name="organization"
										render={({ field }) => (
											<FormItem>
												<FormLabel>Organization</FormLabel>
												<Select disabled={isLoadingOrgs} onValueChange={field.onChange} defaultValue={field.value}>
													<FormControl>
														<SelectTrigger>
															<SelectValue placeholder="Select an organization" />
														</SelectTrigger>
													</FormControl>
													<SelectContent>
														{organizations?.items?.map((org) => (
															<SelectItem key={org.id} value={org.id?.toString() || ''}>
																{org.mspId}
															</SelectItem>
														))}
													</SelectContent>
												</Select>
												<FormDescription>Select the organization for the nodes</FormDescription>
												<FormMessage />
											</FormItem>
										)}
									/>

									<div className="grid grid-cols-2 gap-4">
										<FormField
											control={form.control}
											name="peerCount"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Number of Peers</FormLabel>
													<FormControl>
														<Input type="number" min={0} max={10} {...field} onChange={(e) => field.onChange(parseInt(e.target.value))} />
													</FormControl>
													<FormDescription>Create up to 10 peers</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>

										<FormField
											control={form.control}
											name="ordererCount"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Number of Orderers</FormLabel>
													<FormControl>
														<Input type="number" min={0} max={5} {...field} onChange={(e) => field.onChange(parseInt(e.target.value))} />
													</FormControl>
													<FormDescription>Create up to 5 orderers</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>
									</div>
								</div>
							</Card>
						)}

						{currentStep === 'configure' && (
							<div className="space-y-8">
								{nodeConfigs.map((config, index) => {
									// Calculate type-specific index
									const typeConfigs = nodeConfigs.filter((c) => c.fabricProperties.nodeType === config.fabricProperties.nodeType)
									const typeIndex = typeConfigs.findIndex((c) => c.name === config.name) + 1

									return (
										<Card key={index} className="p-6">
											<div className="mb-6">
												<h3 className="text-lg font-semibold">
													{config.fabricProperties.nodeType === 'FABRIC_PEER' ? 'Peer' : 'Orderer'} {typeIndex}
												</h3>
												<p className="text-sm text-muted-foreground">Configure {config.name}</p>
											</div>

											<FabricNodeForm
												defaultValues={config}
												onSubmit={(values) => {
													const newConfigs = [...nodeConfigs]
													newConfigs[index] = { ...values, name: config.name }
													setNodeConfigs(newConfigs)
												}}
												onChange={(values) => {
													const newConfigs = [...nodeConfigs]
													newConfigs[index] = { ...values, name: config.name }
													setNodeConfigs(newConfigs)
												}}
												organizations={organizations?.items?.map((org) => ({ id: org.id!, name: org.mspId! })) || []}
												hideSubmit
												hideOrganization
												hideNodeType
											/>
										</Card>
									)
								})}
							</div>
						)}

						{currentStep === 'review' && (
							<Card className="p-6">
								<div className="space-y-6">
									{!creationCompleted && (
										<>
											<div>
												<h3 className="text-lg font-semibold mb-4">Summary</h3>
												<dl className="space-y-4">
													<div>
														<dt className="text-sm font-medium text-muted-foreground">Organization</dt>
														<dd className="mt-1">{organizations?.items?.find((org) => org.id?.toString() === form.watch('organization'))?.mspId}</dd>
													</div>
													<div>
														<dt className="text-sm font-medium text-muted-foreground">Nodes to Create</dt>
														<dd className="mt-1">
															{nodeConfigs.filter((c) => c.fabricProperties.nodeType === 'FABRIC_PEER').length} Peers,{' '}
															{nodeConfigs.filter((c) => c.fabricProperties.nodeType === 'FABRIC_ORDERER').length} Orderers
														</dd>
													</div>
												</dl>
											</div>

											{creationProgress.total > 0 && (
												<div className="space-y-4">
													<div className="flex justify-between text-sm">
														<span>
															{creationStatus === 'creating' && 'Creating nodes...'}
															{creationStatus === 'success' && 'All nodes created successfully'}
															{creationStatus === 'failed' && 'All nodes failed to create'}
															{creationStatus === 'partial' && 'Some nodes created successfully'}
														</span>
														<span>
															{creationProgress.current} of {creationProgress.total}
														</span>
													</div>
													<Progress value={(creationProgress.current / creationProgress.total) * 100} />
													{creationProgress.currentNode && creationStatus === 'creating' && (
														<p className="text-sm text-muted-foreground">Creating {creationProgress.currentNode}...</p>
													)}
												</div>
											)}

											{creationStatus && creationStatus !== 'creating' && (
												<div className="space-y-4">
													{creationResults.success.length > 0 && (
														<Alert>
															<CheckCircle2 className="h-4 w-4" />
															<AlertDescription>
																<div className="flex items-center gap-2">
																	<span>Successfully created {creationResults.success.length} nodes:</span>
																	<div className="flex flex-wrap gap-1">
																		{creationResults.success.map((node) => (
																			<Badge key={node.id?.toString() || node.name || `node-${Math.random()}`} variant="secondary" className="text-xs">
																				{node.name}
																			</Badge>
																		))}
																	</div>
																</div>
															</AlertDescription>
														</Alert>
													)}

													{creationResults.errors.length > 0 && (
														<Alert variant="destructive">
															<XCircle className="h-4 w-4" />
															<AlertDescription>
																<div className="space-y-2">
																	<span>Failed to create {creationResults.errors.length} nodes:</span>
																	<div className="space-y-1">
																		{creationResults.errors.map((error, index) => (
																			<div key={index} className="text-sm">
																				<strong>{error.nodeName}:</strong> {error.error}
																			</div>
																		))}
																	</div>
																</div>
															</AlertDescription>
														</Alert>
													)}
												</div>
											)}
										</>
									)}

									<CreationSummary
										creationCompleted={creationCompleted}
										creationResults={creationResults}
										nodeConfigs={nodeConfigs}
										onResetForm={resetForm}
										onRetryFailedNodes={retryFailedNodes}
									/>
								</div>
							</Card>
						)}

						<div className="flex justify-between">
							{currentStep !== 'basic' && !creationCompleted && (
								<Button type="button" variant="outline" onClick={() => setCurrentStep(currentStep === 'review' ? 'configure' : 'basic')}>
									Previous
								</Button>
							)}
							<div className="flex gap-4 ml-auto">
								<Button variant="outline" asChild>
									<Link to="/nodes">Cancel</Link>
								</Button>
								{!creationCompleted && (
									<Button type="submit" disabled={!canProceed() || isCreating}>
										{currentStep === 'review' ? (
											<>
												{isCreating ? (
													<>
														<AlertCircle className="mr-2 h-4 w-4" />
														Creating...
													</>
												) : (
													<>
														<CheckCircle2 className="mr-2 h-4 w-4" />
														Create Nodes
													</>
												)}
											</>
										) : (
											<>
												Next
												<ArrowRight className="ml-2 h-4 w-4" />
											</>
										)}
									</Button>
								)}
							</div>
						</div>
					</form>
				</Form>
			</div>
		</div>
	)
}

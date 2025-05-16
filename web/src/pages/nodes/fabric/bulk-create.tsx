import { getNodesDefaultsFabric } from '@/api/client'
import { getNodesDefaultsFabricOptions, getNodesOptions, getOrganizationsOptions, postNodesMutation } from '@/api/client/@tanstack/react-query.gen'
import { HttpCreateNodeRequest, TypesFabricOrdererConfig, TypesFabricPeerConfig } from '@/api/client/types.gen'
import { FabricNodeForm, FabricNodeFormValues } from '@/components/nodes/fabric-node-form'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Steps } from '@/components/ui/steps'
import { slugify } from '@/utils'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { ArrowLeft, ArrowRight, CheckCircle2, Server } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { z } from 'zod'

interface NodeConfig extends FabricNodeFormValues {
	name: string
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

	const { data: organizations, isLoading: isLoadingOrgs } = useQuery({
		...getOrganizationsOptions(),
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

	const selectedOrg = organizations?.find((org) => org.id?.toString() === form.watch('organization'))
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
			setCreationProgress({ current: 0, total: nodeConfigs.length, currentNode: null })

			// Create nodes sequentially to show progress
			for (let i = 0; i < nodeConfigs.length; i++) {
				const config = nodeConfigs[i]
				setCreationProgress({
					current: i,
					total: nodeConfigs.length,
					currentNode: config.name,
				})

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

				await createNode.mutateAsync({
					body: createNodeDto,
				})
			}

			setCreationProgress({
				current: nodeConfigs.length,
				total: nodeConfigs.length,
				currentNode: null,
			})

			toast.success('All nodes created successfully')
			navigate('/nodes')
		} catch (error: any) {
			toast.error('Failed to create nodes', {
				description: error.error.message,
			})
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
														{organizations?.map((org) => (
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
													console.log('values', values, 'onChange')
													setNodeConfigs(newConfigs)
												}}
												organizations={organizations?.map((org) => ({ id: org.id!, name: org.mspId! })) || []}
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
									<div>
										<h3 className="text-lg font-semibold mb-4">Summary</h3>
										<dl className="space-y-4">
											<div>
												<dt className="text-sm font-medium text-muted-foreground">Organization</dt>
												<dd className="mt-1">{organizations?.find((org) => org.id?.toString() === form.watch('organization'))?.mspId}</dd>
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
										<div className="space-y-2">
											<div className="flex justify-between text-sm">
												<span>Creating nodes...</span>
												<span>
													{creationProgress.current} of {creationProgress.total}
												</span>
											</div>
											<Progress value={(creationProgress.current / creationProgress.total) * 100} />
											{creationProgress.currentNode && <p className="text-sm text-muted-foreground">Creating {creationProgress.currentNode}...</p>}
										</div>
									)}
								</div>
							</Card>
						)}

						<div className="flex justify-between">
							{currentStep !== 'basic' && (
								<Button type="button" variant="outline" onClick={() => setCurrentStep(currentStep === 'review' ? 'configure' : 'basic')}>
									Previous
								</Button>
							)}
							<div className="flex gap-4 ml-auto">
								<Button variant="outline" asChild>
									<Link to="/nodes">Cancel</Link>
								</Button>
								<Button type="submit" disabled={!canProceed() || creationProgress.total > 0}>
									{currentStep === 'review' ? (
										<>
											<CheckCircle2 className="mr-2 h-4 w-4" />
											Create Nodes
										</>
									) : (
										<>
											Next
											<ArrowRight className="ml-2 h-4 w-4" />
										</>
									)}
								</Button>
							</div>
						</div>
					</form>
				</Form>
			</div>
		</div>
	)
}

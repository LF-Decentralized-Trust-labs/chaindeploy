import { getNodesDefaultsFabric, postNodes, postOrganizations } from '@/api/client'
import {
	getKeyProvidersOptions,
	getOrganizationsOptions,
	postNetworksFabricMutation,
} from '@/api/client/@tanstack/react-query.gen'
import {
	HttpCreateNodeRequest,
	HttpFabricNetworkConfig,
	TypesFabricOrdererConfig,
	TypesFabricPeerConfig,
} from '@/api/client/types.gen'
import { NetworkCreatedDialog } from '@/components/dashboard/NetworkCreatedDialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Steps } from '@/components/ui/steps'
import { slugify } from '@/utils'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { AlertCircle, ArrowLeft, ArrowRight, Building2, CheckCircle2, ChevronDown, HelpCircle, Loader2, Network, Plus, Server, Settings2, Trash2 } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import * as z from 'zod'

// --- Types ---

interface OrgConfig {
	mspId: string
	description: string
	isPeer: boolean
	isOrderer: boolean
	providerId?: number
	// When using an existing org
	existingOrgId?: number
	// After creation
	created?: boolean
	createdId?: number
}

interface NodeConfig {
	name: string
	nodeType: 'FABRIC_PEER' | 'FABRIC_ORDERER'
	orgIndex: number // reference to OrgConfig index
	mode: string
	version: string
	listenAddress: string
	operationsListenAddress: string
	externalEndpoint: string
	chaincodeAddress?: string
	eventsAddress?: string
	adminAddress?: string
	// After creation
	created?: boolean
	createdId?: number
}

// --- Steps ---

const wizardSteps = [
	{ id: 'basics', title: 'Network Basics' },
	{ id: 'organizations', title: 'Organizations' },
	{ id: 'nodes', title: 'Nodes' },
	{ id: 'review', title: 'Review & Create' },
]

// --- Schema ---

const basicsSchema = z.object({
	networkName: z.string().min(1, 'Network name is required'),
	consensusType: z.enum(['etcdraft', 'smartbft']).default('etcdraft'),
})

type BasicsValues = z.infer<typeof basicsSchema>

// --- Session Storage ---

const STORAGE_KEY = 'fabric-network-wizard'

interface WizardState {
	currentStep: string
	orgs: OrgConfig[]
	nodes: NodeConfig[]
	nodesGenerated: boolean
	peerCount: number
	ordererCount: number
	basics: BasicsValues
}

function loadWizardState(): Partial<WizardState> | null {
	try {
		const raw = sessionStorage.getItem(STORAGE_KEY)
		if (!raw) return null
		return JSON.parse(raw)
	} catch {
		return null
	}
}

function saveWizardState(state: WizardState) {
	try {
		sessionStorage.setItem(STORAGE_KEY, JSON.stringify(state))
	} catch {
		// ignore storage errors
	}
}

function clearWizardState() {
	sessionStorage.removeItem(STORAGE_KEY)
}

// --- Component ---

export default function FabricNetworkWizard() {
	const navigate = useNavigate()
	const queryClient = useQueryClient()

	const saved = useMemo(() => loadWizardState(), [])

	const [currentStep, setCurrentStep] = useState(saved?.currentStep || 'basics')
	const [orgs, setOrgs] = useState<OrgConfig[]>(
		saved?.orgs || [
			{ mspId: 'Org1MSP', description: '', isPeer: true, isOrderer: false },
			{ mspId: 'OrdererMSP', description: '', isPeer: false, isOrderer: true },
		]
	)
	const [nodes, setNodes] = useState<NodeConfig[]>(saved?.nodes || [])
	const [nodesGenerated, setNodesGenerated] = useState(saved?.nodesGenerated || false)
	const [peerCount, setPeerCount] = useState(saved?.peerCount || 1)
	const [ordererCount, setOrdererCount] = useState(saved?.ordererCount || 3)
	const [creationProgress, setCreationProgress] = useState<{
		phase: string
		current: number
		total: number
		detail: string
	} | null>(null)
	const [createdNetwork, setCreatedNetwork] = useState<{ id?: number; name: string } | null>(null)

	const basicsForm = useForm<BasicsValues>({
		resolver: zodResolver(basicsSchema),
		defaultValues: saved?.basics || {
			networkName: '',
			consensusType: 'etcdraft',
		},
	})

	// Persist state to sessionStorage on changes
	useEffect(() => {
		saveWizardState({
			currentStep,
			orgs,
			nodes,
			nodesGenerated,
			peerCount,
			ordererCount,
			basics: basicsForm.getValues(),
		})
	}, [currentStep, orgs, nodes, nodesGenerated, peerCount, ordererCount])

	// Also persist when form values change
	useEffect(() => {
		const subscription = basicsForm.watch(() => {
			saveWizardState({
				currentStep,
				orgs,
				nodes,
				nodesGenerated,
				peerCount,
				ordererCount,
				basics: basicsForm.getValues(),
			})
		})
		return () => subscription.unsubscribe()
	}, [basicsForm, currentStep, orgs, nodes, nodesGenerated, peerCount, ordererCount])

	const consensusType = basicsForm.watch('consensusType')
	const minOrderers = consensusType === 'smartbft' ? 4 : 3

	// Ensure orderer count meets minimum when consensus changes
	useEffect(() => {
		if (ordererCount < minOrderers) {
			setOrdererCount(minOrderers)
			setNodesGenerated(false)
		}
	}, [minOrderers])

	// Queries
	const { data: providers } = useQuery({
		...getKeyProvidersOptions(),
	})

	const { data: existingOrgsData } = useQuery({
		...getOrganizationsOptions({ query: { limit: 1000 } }),
	})

	const existingOrgsList = existingOrgsData?.items || []

	const createNetworkMutation = useMutation({
		...postNetworksFabricMutation(),
		onSuccess: (network) => {
			clearWizardState()
			setCreatedNetwork({ id: network.id, name: network.name || basicsForm.getValues().networkName })
		},
		onError: (error: any) => {
			toast.error(`Failed to create network: ${error.message || 'Unknown error'}`)
			setCreationProgress(null)
		},
	})

	// --- Organization step logic ---

	const updateOrg = (index: number, updates: Partial<OrgConfig>) => {
		setOrgs((prev) => prev.map((org, i) => (i === index ? { ...org, ...updates } : org)))
	}

	const orgValidation = useMemo(() => {
		const hasPeerOrg = orgs.some((o) => o.isPeer && o.mspId.trim())
		const hasOrdererOrg = orgs.some((o) => o.isOrderer && o.mspId.trim())
		const allHaveMspId = orgs.every((o) => o.mspId.trim())
		const filledMspIds = orgs.map((o) => o.mspId.trim()).filter(Boolean)
		const noDuplicates = new Set(filledMspIds).size === filledMspIds.length
		return { hasPeerOrg, hasOrdererOrg, allHaveMspId, noDuplicates, valid: hasPeerOrg && hasOrdererOrg && allHaveMspId && noDuplicates }
	}, [orgs])

	// --- Node step logic ---

	const generateNodes = useCallback(async () => {
		// Calculate total peers and orderers across all orgs
		const totalPeers = orgs.filter((o) => o.isPeer).length * peerCount
		const totalOrderers = orgs.filter((o) => o.isOrderer).length * ordererCount

		if (totalPeers === 0 && totalOrderers === 0) {
			setNodes([])
			setNodesGenerated(true)
			return
		}

		try {
			// Single API call with total counts to get non-overlapping ports
			const r = await getNodesDefaultsFabric({
				query: { peerCount: totalPeers, ordererCount: totalOrderers },
			})

			const newNodes: NodeConfig[] = []
			let peerIdx = 0
			let ordererIdx = 0

			for (let orgIdx = 0; orgIdx < orgs.length; orgIdx++) {
				const org = orgs[orgIdx]
				const sluggedMspId = slugify(org.mspId)

				if (org.isPeer) {
					for (let i = 0; i < peerCount; i++) {
						const peer = r.data?.peers?.[peerIdx]
						if (!peer) break
						newNodes.push({
							name: `peer${i}-${sluggedMspId}`,
							nodeType: 'FABRIC_PEER',
							orgIndex: orgIdx,
							mode: 'service',
							version: '3.1.3',
							listenAddress: peer.listenAddress || '',
							operationsListenAddress: peer.operationsListenAddress || '',
							externalEndpoint: peer.externalEndpoint || '',
							chaincodeAddress: peer.chaincodeAddress || '',
							eventsAddress: peer.eventsAddress || '',
						})
						peerIdx++
					}
				}

				if (org.isOrderer) {
					for (let i = 0; i < ordererCount; i++) {
						const orderer = r.data?.orderers?.[ordererIdx]
						if (!orderer) break
						newNodes.push({
							name: `orderer${i}-${sluggedMspId}`,
							nodeType: 'FABRIC_ORDERER',
							orgIndex: orgIdx,
							mode: 'service',
							version: '3.1.3',
							listenAddress: orderer.listenAddress || '',
							operationsListenAddress: orderer.operationsListenAddress || '',
							externalEndpoint: orderer.externalEndpoint || '',
							adminAddress: orderer.adminAddress || '',
						})
						ordererIdx++
					}
				}
			}

			setNodes(newNodes)
			setNodesGenerated(true)
		} catch (err) {
			console.error('Failed to load node defaults', err)
		}
	}, [orgs, peerCount, ordererCount])

	useEffect(() => {
		if (currentStep === 'nodes' && !nodesGenerated) {
			generateNodes()
		}
	}, [currentStep, nodesGenerated, generateNodes])

	const updateNode = (index: number, updates: Partial<NodeConfig>) => {
		setNodes((prev) => prev.map((n, i) => (i === index ? { ...n, ...updates } : n)))
	}

	const nodesValidation = useMemo(() => {
		const peerNodes = nodes.filter((n) => n.nodeType === 'FABRIC_PEER')
		const ordererNodes = nodes.filter((n) => n.nodeType === 'FABRIC_ORDERER')
		const hasPeers = peerNodes.length >= 1
		const hasOrderers = ordererNodes.length >= minOrderers
		const allHaveNames = nodes.every((n) => n.name.trim())
		const uniqueNames = new Set(nodes.map((n) => n.name.trim()))
		const noDuplicates = uniqueNames.size === nodes.length
		return { hasPeers, hasOrderers, allHaveNames, noDuplicates, valid: hasPeers && hasOrderers && allHaveNames && noDuplicates }
	}, [nodes, minOrderers])

	// --- Creation logic ---

	const handleCreate = async () => {
		const networkName = basicsForm.getValues().networkName
		const consensus = basicsForm.getValues().consensusType
		const newOrgs = orgs.filter((o) => !o.existingOrgId)
		const totalSteps = newOrgs.length + nodes.length + 1 // new orgs + nodes + network
		let step = 0

		try {
			// Phase 1: Create organizations (skip existing ones)
			setCreationProgress({ phase: 'Creating organizations...', current: 0, total: totalSteps, detail: '' })

			const createdOrgs: { orgIndex: number; id: number; mspId: string }[] = []

			for (let i = 0; i < orgs.length; i++) {
				const org = orgs[i]

				if (org.existingOrgId) {
					// Use existing org
					createdOrgs.push({ orgIndex: i, id: org.existingOrgId, mspId: org.mspId })
					updateOrg(i, { created: true, createdId: org.existingOrgId })
					continue
				}

				step++
				setCreationProgress({
					phase: 'Creating organizations...',
					current: step,
					total: totalSteps,
					detail: `Creating ${org.mspId}...`,
				})

				const result = await postOrganizations({
					body: {
						mspId: org.mspId,
						name: org.mspId,
						description: org.description,
						providerId: org.providerId || providers?.[0]?.id,
					},
				})

				if (!result.data?.id) {
					throw new Error(`Failed to create organization ${org.mspId}: ${(result as any).error?.message || 'unexpected response'}`)
				}
				createdOrgs.push({ orgIndex: i, id: result.data.id, mspId: org.mspId })
				updateOrg(i, { created: true, createdId: result.data.id })
			}

			// Phase 2: Create nodes
			setCreationProgress({
				phase: 'Creating nodes...',
				current: step,
				total: totalSteps,
				detail: '',
			})

			const createdNodes: { nodeIndex: number; id: number; nodeType: string; orgIndex: number }[] = []

			for (let i = 0; i < nodes.length; i++) {
				const node = nodes[i]
				step++
				setCreationProgress({
					phase: 'Creating nodes...',
					current: step,
					total: totalSteps,
					detail: `Creating ${node.name}...`,
				})

				const orgInfo = createdOrgs.find((o) => o.orgIndex === node.orgIndex)!

				let fabricPeer: TypesFabricPeerConfig | undefined
				let fabricOrderer: TypesFabricOrdererConfig | undefined

				if (node.nodeType === 'FABRIC_PEER') {
					fabricPeer = {
						nodeType: 'FABRIC_PEER',
						mode: node.mode,
						organizationId: orgInfo.id,
						listenAddress: node.listenAddress,
						operationsListenAddress: node.operationsListenAddress,
						externalEndpoint: node.externalEndpoint,
						domainNames: [],
						name: node.name,
						chaincodeAddress: node.chaincodeAddress || '',
						eventsAddress: node.eventsAddress || '',
						mspId: orgInfo.mspId,
						version: node.version,
					} as TypesFabricPeerConfig
				} else {
					fabricOrderer = {
						nodeType: 'FABRIC_ORDERER',
						mode: node.mode,
						organizationId: orgInfo.id,
						listenAddress: node.listenAddress,
						operationsListenAddress: node.operationsListenAddress,
						externalEndpoint: node.externalEndpoint,
						domainNames: [],
						name: node.name,
						adminAddress: node.adminAddress || '',
						mspId: orgInfo.mspId,
						version: node.version,
					} as TypesFabricOrdererConfig
				}

				const createNodeDto: HttpCreateNodeRequest = {
					name: node.name,
					blockchainPlatform: 'FABRIC',
					fabricPeer,
					fabricOrderer,
				}

				const result = await postNodes({ body: createNodeDto })
				if (!result.data?.id) {
					throw new Error(`Failed to create node ${node.name}: ${(result as any).error?.message || 'unexpected response'}`)
				}
				createdNodes.push({
					nodeIndex: i,
					id: result.data.id,
					nodeType: node.nodeType,
					orgIndex: node.orgIndex,
				})
				updateNode(i, { created: true, createdId: result.data.id })
			}

			// Phase 3: Create network
			step++
			setCreationProgress({
				phase: 'Creating network...',
				current: step,
				total: totalSteps,
				detail: `Creating channel ${networkName}...`,
			})

			// Build the network config
			const peerOrganizations = createdOrgs
				.filter((o) => orgs[o.orgIndex].isPeer)
				.map((o) => ({ id: o.id, nodeIds: [] as number[] }))

			const ordererOrganizations = createdOrgs
				.filter((o) => orgs[o.orgIndex].isOrderer)
				.map((o) => ({
					id: o.id,
					nodeIds: createdNodes.filter((n) => n.orgIndex === o.orgIndex && n.nodeType === 'FABRIC_ORDERER').map((n) => n.id),
				}))

			const config: HttpFabricNetworkConfig = {
				consensusType: consensus,
				channelCapabilities: consensus === 'smartbft' ? ['V3_0'] : ['V2_0', 'V3_0'],
				applicationCapabilities: ['V2_0', 'V2_5'],
				ordererCapabilities: ['V2_0'],
				batchSize: {
					maxMessageCount: 500,
					absoluteMaxBytes: 103809024,
					preferredMaxBytes: 524288,
				},
				batchTimeout: '2s',
				...(consensus === 'etcdraft' && {
					etcdRaftOptions: {
						tickInterval: '500ms',
						electionTick: 10,
						heartbeatTick: 1,
						maxInflightBlocks: 5,
						snapshotIntervalSize: 20971520,
					},
				}),
				peerOrganizations,
				ordererOrganizations,
			}

			await createNetworkMutation.mutateAsync({
				body: {
					name: networkName,
					config,
					description: '',
				},
			})

			// Invalidate queries so lists refresh
			queryClient.invalidateQueries({ queryKey: ['getOrganizations'] })
			queryClient.invalidateQueries({ queryKey: ['getNodes'] })
			queryClient.invalidateQueries({ queryKey: ['getNetworks'] })
		} catch (error: any) {
			const msg = error?.error?.message || error?.message || 'An error occurred during creation'
			toast.error('Creation failed', { description: msg })
			setCreationProgress(null)
		}
	}

	// --- Navigation ---

	const canProceedFromBasics = basicsForm.formState.isValid || (basicsForm.getValues().networkName.trim() !== '')
	const canProceedFromOrgs = orgValidation.valid
	const canProceedFromNodes = nodesValidation.valid

	const goNext = () => {
		const stepOrder = wizardSteps.map((s) => s.id)
		const idx = stepOrder.indexOf(currentStep)
		if (idx < stepOrder.length - 1) {
			setCurrentStep(stepOrder[idx + 1])
		}
	}

	const goBack = () => {
		const stepOrder = wizardSteps.map((s) => s.id)
		const idx = stepOrder.indexOf(currentStep)
		if (idx > 0) {
			setCurrentStep(stepOrder[idx - 1])
		}
	}

	// --- Render ---

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

				<div className="flex items-center gap-4 mb-8">
					<Network className="h-8 w-8" />
					<div>
						<h1 className="text-2xl font-semibold">Create Fabric Network</h1>
						<p className="text-muted-foreground">Set up a complete Fabric network with organizations, nodes, and channel</p>
					</div>
				</div>

				<Steps steps={wizardSteps} currentStep={currentStep} className="mb-8" />

				{/* Step 1: Network Basics */}
				{currentStep === 'basics' && (
					<Card className="p-6">
						<Form {...basicsForm}>
							<form
								onSubmit={(e) => {
									e.preventDefault()
									goNext()
								}}
								className="space-y-6"
							>
								<FormField
									control={basicsForm.control}
									name="networkName"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Network Name</FormLabel>
											<FormControl>
												<Input placeholder="e.g., my-network" {...field} />
											</FormControl>
											<FormDescription>The name for your Fabric channel</FormDescription>
											<FormMessage />
										</FormItem>
									)}
								/>

								<FormField
									control={basicsForm.control}
									name="consensusType"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Consensus Type</FormLabel>
											<FormControl>
												<RadioGroup onValueChange={field.onChange} value={field.value} className="grid grid-cols-2 gap-4">
													<button
														type="button"
														onClick={() => field.onChange('etcdraft')}
														className={`flex items-start w-full p-4 gap-4 border rounded-lg cursor-pointer hover:border-primary transition-colors text-left ${
															field.value === 'etcdraft' ? 'border-primary bg-primary/5' : ''
														}`}
													>
														<RadioGroupItem value="etcdraft" className="mt-1" />
														<div>
															<h3 className="font-medium">Raft (etcdraft)</h3>
															<p className="text-sm text-muted-foreground">
																Crash fault tolerant. Requires minimum 3 orderers.
															</p>
														</div>
													</button>
													<button
														type="button"
														onClick={() => field.onChange('smartbft')}
														className={`flex items-start w-full p-4 gap-4 border rounded-lg cursor-pointer hover:border-primary transition-colors text-left ${
															field.value === 'smartbft' ? 'border-primary bg-primary/5' : ''
														}`}
													>
														<RadioGroupItem value="smartbft" className="mt-1" />
														<div>
															<h3 className="font-medium">SmartBFT</h3>
															<p className="text-sm text-muted-foreground">
																Byzantine fault tolerant. Requires minimum 4 orderers. Needs V3_0 capabilities.
															</p>
														</div>
													</button>
												</RadioGroup>
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>

								<div className="flex justify-end">
									<Button type="submit" disabled={!canProceedFromBasics}>
										Next
										<ArrowRight className="ml-2 h-4 w-4" />
									</Button>
								</div>
							</form>
						</Form>
					</Card>
				)}

				{/* Step 2: Organizations */}
				{currentStep === 'organizations' && (
					<TooltipProvider>
						<div className="space-y-6">
							{/* Explainer for beginners */}
							<Card className="bg-muted/50 border-dashed">
								<CardContent className="pt-6">
									<div className="flex gap-3">
										<HelpCircle className="h-5 w-5 text-muted-foreground shrink-0 mt-0.5" />
										<div className="space-y-2 text-sm text-muted-foreground">
											<p>
												A Fabric network needs two types of organizations:
											</p>
											<ul className="list-disc list-inside space-y-1 ml-1">
												<li><strong className="text-foreground">Peer organizations</strong> — run peer nodes that store the ledger (blockchain data) and execute smart contracts (chaincode)</li>
												<li><strong className="text-foreground">Orderer organizations</strong> — run orderer nodes that sequence transactions and create blocks. You need at least {minOrderers} orderers for {consensusType === 'etcdraft' ? 'Raft' : 'SmartBFT'} consensus.</li>
											</ul>
											<p>You need at least <strong className="text-foreground">1 peer org</strong> and <strong className="text-foreground">1 orderer org</strong>. A single org can serve both roles.</p>
										</div>
									</div>
								</CardContent>
							</Card>

							{/* Organization cards */}
							{orgs.map((org, index) => {
								const hasExistingOrgs = existingOrgsList.length > 0
								const isExisting = !!org.existingOrgId
								const roleLabel = org.isPeer && org.isOrderer
									? 'Peer + Orderer'
									: org.isPeer
									? 'Peer'
									: org.isOrderer
									? 'Orderer'
									: null

								return (
									<Card key={org.existingOrgId ? `existing-${org.existingOrgId}` : `new-${index}`}>
										<CardHeader className="pb-4">
											<div className="flex items-center justify-between">
												<div className="flex items-center gap-3">
													<div className="flex items-center justify-center h-8 w-8 rounded-full bg-primary/10 text-primary text-sm font-medium">
														{index + 1}
													</div>
													<div>
														<CardTitle className="text-base flex items-center gap-2">
															{org.mspId || `Organization ${index + 1}`}
															{roleLabel && <Badge variant="outline">{roleLabel}</Badge>}
															{isExisting && <Badge variant="secondary">Existing</Badge>}
														</CardTitle>
														<CardDescription>
															{org.isPeer && org.isOrderer
																? 'Hosts ledger data and orders transactions'
																: org.isPeer
																? 'Hosts ledger data and runs smart contracts'
																: org.isOrderer
																? 'Orders transactions and creates blocks'
																: 'Choose a role for this organization'}
														</CardDescription>
													</div>
												</div>
												{orgs.length > 2 && (
													<Button
														variant="ghost"
														size="icon"
														className="h-8 w-8 text-muted-foreground hover:text-destructive"
														onClick={() => {
															setOrgs((prev) => prev.filter((_, i) => i !== index))
															setNodesGenerated(false)
														}}
													>
														<Trash2 className="h-4 w-4" />
													</Button>
												)}
											</div>
										</CardHeader>
										<CardContent className="space-y-4">
											{/* Source toggle: existing vs new */}
											{hasExistingOrgs && (
												<div>
													<label className="text-sm font-medium flex items-center gap-1.5 mb-1.5">
														Organization source
														<Tooltip>
															<TooltipTrigger asChild>
																<HelpCircle className="h-3.5 w-3.5 text-muted-foreground" />
															</TooltipTrigger>
															<TooltipContent side="right" className="max-w-xs">
																<p>Choose an organization you already created, or create a new one for this network.</p>
															</TooltipContent>
														</Tooltip>
													</label>
													<Select
														value={isExisting ? `existing-${org.existingOrgId}` : 'new'}
														onValueChange={(val) => {
															if (val === 'new') {
																updateOrg(index, {
																	existingOrgId: undefined,
																	mspId: '',
																	description: '',
																})
															} else {
																const existingId = Number(val.replace('existing-', ''))
																const existing = existingOrgsList.find((o) => o.id === existingId)
																if (existing) {
																	updateOrg(index, {
																		existingOrgId: existing.id,
																		mspId: existing.mspId || '',
																		description: existing.description || '',
																	})
																}
															}
															setNodesGenerated(false)
														}}
													>
														<SelectTrigger>
															<SelectValue />
														</SelectTrigger>
														<SelectContent>
															<SelectItem value="new">
																<span className="flex items-center gap-2">
																	<Plus className="h-3.5 w-3.5" />
																	Create new organization
																</span>
															</SelectItem>
															{existingOrgsList.map((eo) => {
																const usedByOther = orgs.some(
																	(o, i) => i !== index && o.existingOrgId === eo.id
																)
																return (
																	<SelectItem key={eo.id} value={`existing-${eo.id}`} disabled={usedByOther}>
																		<span className="flex items-center gap-2">
																			<Building2 className="h-3.5 w-3.5" />
																			{eo.mspId}
																			{usedByOther && <span className="text-xs text-muted-foreground">(already used)</span>}
																		</span>
																	</SelectItem>
																)
															})}
														</SelectContent>
													</Select>
												</div>
											)}

											{/* New org fields */}
											{!isExisting && (
												<div className="grid grid-cols-2 gap-4">
													<div>
														<label className="text-sm font-medium flex items-center gap-1.5 mb-1.5">
															MSP ID
															<Tooltip>
																<TooltipTrigger asChild>
																	<HelpCircle className="h-3.5 w-3.5 text-muted-foreground" />
																</TooltipTrigger>
																<TooltipContent side="right" className="max-w-xs">
																	<p>The Membership Service Provider ID uniquely identifies this organization in the network. Convention: OrgNameMSP (e.g., Org1MSP, AcmeMSP).</p>
																</TooltipContent>
															</Tooltip>
														</label>
														<Input
															placeholder={org.isPeer ? 'e.g., PeerOrg1MSP' : org.isOrderer ? 'e.g., OrdererOrgMSP' : 'e.g., Org1MSP'}
															value={org.mspId}
															onChange={(e) => {
																updateOrg(index, { mspId: e.target.value })
																setNodesGenerated(false)
															}}
														/>
													</div>
													<div>
														<label className="text-sm font-medium mb-1.5 block">Description</label>
														<Input
															placeholder="Optional description"
															value={org.description}
															onChange={(e) => updateOrg(index, { description: e.target.value })}
														/>
													</div>
												</div>
											)}

											{/* Key provider (only for new orgs) */}
											{!isExisting && providers && providers.length > 1 && (
												<div>
													<label className="text-sm font-medium flex items-center gap-1.5 mb-1.5">
														Key Provider
														<Tooltip>
															<TooltipTrigger asChild>
																<HelpCircle className="h-3.5 w-3.5 text-muted-foreground" />
															</TooltipTrigger>
															<TooltipContent side="right" className="max-w-xs">
																<p>Where cryptographic keys for this organization are stored. The default database provider works for most setups.</p>
															</TooltipContent>
														</Tooltip>
													</label>
													<Select
														value={org.providerId?.toString() || providers[0]?.id?.toString() || ''}
														onValueChange={(val) => updateOrg(index, { providerId: Number(val) })}
													>
														<SelectTrigger>
															<SelectValue placeholder="Select key provider" />
														</SelectTrigger>
														<SelectContent>
															{providers.map((p) => (
																<SelectItem key={p.id} value={p.id!.toString()}>
																	{p.name}
																</SelectItem>
															))}
														</SelectContent>
													</Select>
												</div>
											)}

											{/* Role selection */}
											<div>
												<label className="text-sm font-medium mb-2 block">Role in the network</label>
												<div className="grid grid-cols-2 gap-3">
													<div
														role="button"
														tabIndex={0}
														onClick={() => {
															updateOrg(index, { isPeer: !org.isPeer })
															setNodesGenerated(false)
														}}
														onKeyDown={(e) => {
															if (e.key === 'Enter' || e.key === ' ') {
																e.preventDefault()
																updateOrg(index, { isPeer: !org.isPeer })
																setNodesGenerated(false)
															}
														}}
														className={`flex items-start gap-3 p-3 rounded-lg border text-left transition-colors cursor-pointer ${
															org.isPeer ? 'border-primary bg-primary/5' : 'hover:border-muted-foreground/50'
														}`}
													>
														<Checkbox
															checked={org.isPeer}
															tabIndex={-1}
															className="mt-0.5"
														/>
														<div>
															<p className="text-sm font-medium">Peer organization</p>
															<p className="text-xs text-muted-foreground mt-0.5">
																Stores the ledger and executes smart contracts
															</p>
														</div>
													</div>
													<div
														role="button"
														tabIndex={0}
														onClick={() => {
															updateOrg(index, { isOrderer: !org.isOrderer })
															setNodesGenerated(false)
														}}
														onKeyDown={(e) => {
															if (e.key === 'Enter' || e.key === ' ') {
																e.preventDefault()
																updateOrg(index, { isOrderer: !org.isOrderer })
																setNodesGenerated(false)
															}
														}}
														className={`flex items-start gap-3 p-3 rounded-lg border text-left transition-colors cursor-pointer ${
															org.isOrderer ? 'border-primary bg-primary/5' : 'hover:border-muted-foreground/50'
														}`}
													>
														<Checkbox
															checked={org.isOrderer}
															tabIndex={-1}
															className="mt-0.5"
														/>
														<div>
															<p className="text-sm font-medium">Orderer organization</p>
															<p className="text-xs text-muted-foreground mt-0.5">
																Sequences transactions and creates blocks
															</p>
														</div>
													</div>
												</div>
											</div>
										</CardContent>
									</Card>
								)
							})}

							<Button
								variant="outline"
								className="w-full"
								onClick={() => {
									setOrgs((prev) => [...prev, { mspId: '', description: '', isPeer: false, isOrderer: false }])
									setNodesGenerated(false)
								}}
							>
								<Plus className="mr-2 h-4 w-4" />
								Add another organization
							</Button>

							{!orgValidation.valid && (
								<Alert variant="destructive">
									<AlertCircle className="h-4 w-4" />
									<AlertDescription>
										<ul className="list-disc list-inside space-y-0.5">
											{!orgValidation.allHaveMspId && (
												<li>All organizations must have an MSP ID</li>
											)}
											{!orgValidation.noDuplicates && (
												<li>MSP IDs must be unique across organizations</li>
											)}
											{!orgValidation.hasPeerOrg && (
												<li>At least one organization must have the peer role</li>
											)}
											{!orgValidation.hasOrdererOrg && (
												<li>At least one organization must have the orderer role</li>
											)}
										</ul>
									</AlertDescription>
								</Alert>
							)}

							<div className="flex justify-between">
								<Button variant="outline" onClick={goBack}>
									<ArrowLeft className="mr-2 h-4 w-4" />
									Back
								</Button>
								<Button onClick={goNext} disabled={!canProceedFromOrgs}>
									Next
									<ArrowRight className="ml-2 h-4 w-4" />
								</Button>
							</div>
						</div>
					</TooltipProvider>
				)}

				{/* Step 3: Nodes */}
				{currentStep === 'nodes' && (
					<TooltipProvider>
						<div className="space-y-6">
							{/* Explainer */}
							<Card className="bg-muted/50 border-dashed">
								<CardContent className="pt-6">
									<div className="flex gap-3">
										<HelpCircle className="h-5 w-5 text-muted-foreground shrink-0 mt-0.5" />
										<div className="space-y-2 text-sm text-muted-foreground">
											<p>
												Each organization needs nodes to participate in the network. The wizard auto-generates node names
												and port assignments — you only need to adjust the count.
											</p>
											<ul className="list-disc list-inside space-y-0.5 ml-1">
												<li>Peer orgs need at least <strong className="text-foreground">1 peer</strong></li>
												<li>Orderer orgs need at least <strong className="text-foreground">{minOrderers} orderers</strong> for {consensusType === 'smartbft' ? 'SmartBFT' : 'Raft'} consensus</li>
											</ul>
										</div>
									</div>
								</CardContent>
							</Card>

							{/* Node count & version controls */}
							<Card>
								<CardHeader className="pb-4">
									<CardTitle className="text-base">Node Configuration</CardTitle>
									<CardDescription>Set the number of nodes and Fabric version</CardDescription>
								</CardHeader>
								<CardContent>
									<div className="grid grid-cols-3 gap-4">
										<div>
											<label className="text-sm font-medium mb-1.5 block">Peers per peer org</label>
											<Input
												type="number"
												min={1}
												max={10}
												value={peerCount}
												onChange={(e) => {
													setPeerCount(Math.max(1, parseInt(e.target.value) || 1))
													setNodesGenerated(false)
												}}
											/>
										</div>
										<div>
											<label className="text-sm font-medium mb-1.5 block">Orderers per orderer org</label>
											<Input
												type="number"
												min={minOrderers}
												max={10}
												value={ordererCount}
												onChange={(e) => {
													setOrdererCount(Math.max(minOrderers, parseInt(e.target.value) || minOrderers))
													setNodesGenerated(false)
												}}
											/>
										</div>
										<div>
											<label className="text-sm font-medium mb-1.5 block">Fabric version</label>
											<Select
												value={nodes[0]?.version || '3.1.3'}
												onValueChange={(val) => {
													setNodes((prev) => prev.map((n) => ({ ...n, version: val })))
												}}
											>
												<SelectTrigger>
													<SelectValue />
												</SelectTrigger>
												<SelectContent>
													<SelectItem value="3.1.3">3.1.3 (latest)</SelectItem>
													<SelectItem value="3.1.2">3.1.2</SelectItem>
													<SelectItem value="3.1.0">3.1.0</SelectItem>
													<SelectItem value="3.0.0">3.0.0</SelectItem>
													<SelectItem value="2.5.12">2.5.12</SelectItem>
												</SelectContent>
											</Select>
										</div>
									</div>

									{!nodesGenerated && (
										<Button onClick={generateNodes} variant="outline" className="w-full mt-4">
											<Server className="mr-2 h-4 w-4" />
											Generate Node Configurations
										</Button>
									)}
								</CardContent>
							</Card>

							{/* Generated nodes per org */}
							{nodesGenerated && nodes.length > 0 && (
								<>
									{orgs.map((org, orgIdx) => {
										const orgNodes = nodes.filter((n) => n.orgIndex === orgIdx)
										if (orgNodes.length === 0) return null
										return (
											<Card key={orgIdx}>
												<CardHeader className="pb-3">
													<div className="flex items-center gap-2">
														<Building2 className="h-4 w-4 text-muted-foreground" />
														<CardTitle className="text-base">{org.mspId || `Organization ${orgIdx + 1}`}</CardTitle>
														<Badge variant="secondary">
															{orgNodes.length} {orgNodes.length === 1 ? 'node' : 'nodes'}
														</Badge>
													</div>
												</CardHeader>
												<CardContent className="space-y-3">
													{orgNodes.map((node) => {
														const nodeIdx = nodes.indexOf(node)
														return (
															<Collapsible key={nodeIdx}>
																<div className="rounded-lg border">
																	{/* Node header — always visible */}
																	<div className="flex items-center gap-3 p-3">
																		<Server className="h-4 w-4 text-muted-foreground shrink-0" />
																		<Badge variant={node.nodeType === 'FABRIC_PEER' ? 'default' : 'secondary'} className="shrink-0">
																			{node.nodeType === 'FABRIC_PEER' ? 'Peer' : 'Orderer'}
																		</Badge>
																		<Input
																			className="max-w-xs h-8 text-sm"
																			value={node.name}
																			onChange={(e) => updateNode(nodeIdx, { name: e.target.value })}
																		/>
																		<div className="ml-auto flex items-center gap-2 text-xs text-muted-foreground shrink-0">
																			<span>{node.listenAddress}</span>
																			<CollapsibleTrigger asChild>
																				<Button variant="ghost" size="sm" className="h-7 px-2">
																					<Settings2 className="h-3.5 w-3.5 mr-1" />
																					Ports
																					<ChevronDown className="h-3.5 w-3.5 ml-1" />
																				</Button>
																			</CollapsibleTrigger>
																		</div>
																	</div>

																	{/* Collapsible port details */}
																	<CollapsibleContent>
																		<div className="border-t px-3 py-3 bg-muted/30">
																			<div className="grid grid-cols-2 gap-3 text-sm">
																				<div>
																					<label className="text-xs text-muted-foreground flex items-center gap-1">
																						Listen Address
																						<Tooltip>
																							<TooltipTrigger asChild>
																								<HelpCircle className="h-3 w-3" />
																							</TooltipTrigger>
																							<TooltipContent side="top" className="max-w-xs">
																								<p>The address and port this node listens on for incoming connections (gRPC).</p>
																							</TooltipContent>
																						</Tooltip>
																					</label>
																					<Input
																						value={node.listenAddress}
																						onChange={(e) => updateNode(nodeIdx, { listenAddress: e.target.value })}
																						className="h-8 text-sm"
																					/>
																				</div>
																				<div>
																					<label className="text-xs text-muted-foreground flex items-center gap-1">
																						Operations Address
																						<Tooltip>
																							<TooltipTrigger asChild>
																								<HelpCircle className="h-3 w-3" />
																							</TooltipTrigger>
																							<TooltipContent side="top" className="max-w-xs">
																								<p>Exposes health checks and metrics (Prometheus). Used for monitoring.</p>
																							</TooltipContent>
																						</Tooltip>
																					</label>
																					<Input
																						value={node.operationsListenAddress}
																						onChange={(e) => updateNode(nodeIdx, { operationsListenAddress: e.target.value })}
																						className="h-8 text-sm"
																					/>
																				</div>
																				<div>
																					<label className="text-xs text-muted-foreground flex items-center gap-1">
																						External Endpoint
																						<Tooltip>
																							<TooltipTrigger asChild>
																								<HelpCircle className="h-3 w-3" />
																							</TooltipTrigger>
																							<TooltipContent side="top" className="max-w-xs">
																								<p>The address other nodes use to connect to this node. Use your server's public IP in production.</p>
																							</TooltipContent>
																						</Tooltip>
																					</label>
																					<Input
																						value={node.externalEndpoint}
																						onChange={(e) => updateNode(nodeIdx, { externalEndpoint: e.target.value })}
																						className="h-8 text-sm"
																					/>
																				</div>
																				{node.nodeType === 'FABRIC_PEER' && (
																					<div>
																						<label className="text-xs text-muted-foreground flex items-center gap-1">
																							Chaincode Address
																							<Tooltip>
																								<TooltipTrigger asChild>
																									<HelpCircle className="h-3 w-3" />
																								</TooltipTrigger>
																								<TooltipContent side="top" className="max-w-xs">
																									<p>The address where chaincode (smart contracts) connect back to the peer.</p>
																								</TooltipContent>
																							</Tooltip>
																						</label>
																						<Input
																							value={node.chaincodeAddress || ''}
																							onChange={(e) => updateNode(nodeIdx, { chaincodeAddress: e.target.value })}
																							className="h-8 text-sm"
																						/>
																					</div>
																				)}
																				{node.nodeType === 'FABRIC_ORDERER' && (
																					<div>
																						<label className="text-xs text-muted-foreground flex items-center gap-1">
																							Admin Address
																							<Tooltip>
																								<TooltipTrigger asChild>
																									<HelpCircle className="h-3 w-3" />
																								</TooltipTrigger>
																								<TooltipContent side="top" className="max-w-xs">
																									<p>The address for orderer admin operations (channel management).</p>
																								</TooltipContent>
																							</Tooltip>
																						</label>
																						<Input
																							value={node.adminAddress || ''}
																							onChange={(e) => updateNode(nodeIdx, { adminAddress: e.target.value })}
																							className="h-8 text-sm"
																						/>
																					</div>
																				)}
																			</div>
																		</div>
																	</CollapsibleContent>
																</div>
															</Collapsible>
														)
													})}
												</CardContent>
											</Card>
										)
									})}
								</>
							)}

							{!nodesValidation.valid && nodesGenerated && (
								<Alert variant="destructive">
									<AlertCircle className="h-4 w-4" />
									<AlertDescription>
										<ul className="list-disc list-inside space-y-0.5">
											{!nodesValidation.hasPeers && (
												<li>At least 1 peer node is required</li>
											)}
											{!nodesValidation.hasOrderers && (
												<li>At least {minOrderers} orderer nodes are required for {consensusType === 'smartbft' ? 'SmartBFT' : 'Raft'}</li>
											)}
											{!nodesValidation.noDuplicates && (
												<li>Node names must be unique</li>
											)}
										</ul>
									</AlertDescription>
								</Alert>
							)}

							<div className="flex justify-between">
								<Button variant="outline" onClick={goBack}>
									<ArrowLeft className="mr-2 h-4 w-4" />
									Back
								</Button>
								<Button onClick={goNext} disabled={!canProceedFromNodes}>
									Next
									<ArrowRight className="ml-2 h-4 w-4" />
								</Button>
							</div>
						</div>
					</TooltipProvider>
				)}

				{/* Step 4: Review & Create */}
				{currentStep === 'review' && (
					<div className="space-y-6">
						{/* Network basics */}
						<Card>
							<CardHeader className="pb-3">
								<div className="flex items-center justify-between">
									<CardTitle className="text-base flex items-center gap-2">
										<Network className="h-4 w-4 text-muted-foreground" />
										Network Configuration
									</CardTitle>
									<Button variant="ghost" size="sm" className="text-xs h-7" onClick={() => setCurrentStep('basics')} disabled={!!creationProgress}>
										Edit
									</Button>
								</div>
							</CardHeader>
							<CardContent>
								<div className="grid grid-cols-2 gap-x-8 gap-y-2 text-sm">
									<div className="flex justify-between py-1.5 border-b">
										<span className="text-muted-foreground">Network name</span>
										<span className="font-medium">{basicsForm.getValues().networkName}</span>
									</div>
									<div className="flex justify-between py-1.5 border-b">
										<span className="text-muted-foreground">Consensus</span>
										<span className="font-medium">{basicsForm.getValues().consensusType === 'etcdraft' ? 'Raft (etcdraft)' : 'SmartBFT'}</span>
									</div>
									<div className="flex justify-between py-1.5 border-b">
										<span className="text-muted-foreground">Fabric version</span>
										<span className="font-medium">{nodes[0]?.version || '3.1.3'}</span>
									</div>
									<div className="flex justify-between py-1.5 border-b">
										<span className="text-muted-foreground">Batch timeout</span>
										<span className="font-medium">2s</span>
									</div>
								</div>
							</CardContent>
						</Card>

						{/* Organizations */}
						<Card>
							<CardHeader className="pb-3">
								<div className="flex items-center justify-between">
									<CardTitle className="text-base flex items-center gap-2">
										<Building2 className="h-4 w-4 text-muted-foreground" />
										Organizations
										<Badge variant="secondary">{orgs.length}</Badge>
									</CardTitle>
									<Button variant="ghost" size="sm" className="text-xs h-7" onClick={() => setCurrentStep('organizations')} disabled={!!creationProgress}>
										Edit
									</Button>
								</div>
							</CardHeader>
							<CardContent className="space-y-2">
								{orgs.map((org, i) => (
									<div key={i} className="flex items-center justify-between p-3 rounded-lg border">
										<div className="flex items-center gap-3">
											<span className="flex items-center justify-center h-5 w-5 rounded-full bg-muted text-[10px] font-bold">{i + 1}</span>
											<span className="font-medium">{org.mspId}</span>
											{org.existingOrgId ? <Badge variant="secondary" className="text-xs">Existing</Badge> : <Badge variant="default" className="text-xs">New</Badge>}
										</div>
										<div className="flex gap-2">
											{org.isPeer && <Badge variant="outline">Peer</Badge>}
											{org.isOrderer && <Badge variant="outline">Orderer</Badge>}
										</div>
									</div>
								))}
							</CardContent>
						</Card>

						{/* Nodes grouped by org */}
						<Card>
							<CardHeader className="pb-3">
								<div className="flex items-center justify-between">
									<CardTitle className="text-base flex items-center gap-2">
										<Server className="h-4 w-4 text-muted-foreground" />
										Nodes
										<Badge variant="secondary">{nodes.length}</Badge>
									</CardTitle>
									<Button variant="ghost" size="sm" className="text-xs h-7" onClick={() => setCurrentStep('nodes')} disabled={!!creationProgress}>
										Edit
									</Button>
								</div>
							</CardHeader>
							<CardContent className="space-y-4">
								{orgs.map((org, orgIdx) => {
									const orgNodes = nodes.filter((n) => n.orgIndex === orgIdx)
									if (orgNodes.length === 0) return null
									return (
										<div key={orgIdx}>
											<p className="text-xs font-medium text-muted-foreground mb-2">{org.mspId}</p>
											<div className="space-y-1.5">
												{orgNodes.map((node, ni) => (
													<div key={ni} className="flex items-center justify-between py-2 px-3 rounded-md border text-sm">
														<div className="flex items-center gap-2">
															<Badge variant={node.nodeType === 'FABRIC_PEER' ? 'default' : 'secondary'} className="text-[10px] px-1.5 py-0">
																{node.nodeType === 'FABRIC_PEER' ? 'Peer' : 'Orderer'}
															</Badge>
															<span className="font-medium">{node.name}</span>
														</div>
														<span className="text-xs text-muted-foreground">{node.listenAddress}</span>
													</div>
												))}
											</div>
										</div>
									)
								})}
							</CardContent>
						</Card>

						{creationProgress && (
							<Card>
								<CardContent className="pt-6">
									<div className="space-y-3">
										<div className="flex justify-between text-sm font-medium">
											<span>{creationProgress.phase}</span>
											<span>
												{creationProgress.current} / {creationProgress.total}
											</span>
										</div>
										<Progress value={(creationProgress.current / creationProgress.total) * 100} />
										{creationProgress.detail && (
											<p className="text-sm text-muted-foreground flex items-center gap-2">
												<Loader2 className="h-3 w-3 animate-spin" />
												{creationProgress.detail}
											</p>
										)}
									</div>
								</CardContent>
							</Card>
						)}

						<div className="flex justify-between">
							<Button variant="outline" onClick={goBack} disabled={!!creationProgress}>
								<ArrowLeft className="mr-2 h-4 w-4" />
								Back
							</Button>
							<Button onClick={handleCreate} disabled={!!creationProgress}>
								<CheckCircle2 className="mr-2 h-4 w-4" />
								{creationProgress ? 'Creating...' : 'Create Network'}
							</Button>
						</div>
					</div>
				)}
			</div>

			<NetworkCreatedDialog
				open={!!createdNetwork}
				onOpenChange={(open) => {
					if (!open) {
						clearWizardState()
						setCreatedNetwork(null)
						navigate('/networks')
					}
				}}
				networkName={createdNetwork?.name || ''}
				networkId={createdNetwork?.id}
				platform="fabric"
			/>
		</div>
	)
}

import { getNodesDefaultsBesuNode, postKeys } from '@/api/client'
import { getKeyProvidersOptions, postNetworksBesuMutation, postNodesMutation } from '@/api/client/@tanstack/react-query.gen'
import { BesuNodeForm, BesuNodeFormValues } from '@/components/nodes/besu-node-form'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Steps } from '@/components/ui/steps'
import { hexToNumber, isValidHex, numberToHex } from '@/utils'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { ArrowLeft, ArrowRight, CheckCircle2, Server } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import * as z from 'zod'

const steps = [
	{ id: 'nodes', title: 'Number of Nodes' },
	{ id: 'network', title: 'Network Configuration' },
	{ id: 'nodes-config', title: 'Nodes Configuration' },
	{ id: 'review', title: 'Review & Create' },
]

const nodesStepSchema = z.object({
	numberOfNodes: z.number().min(1).max(10),
})

const networkStepSchema = z.object({
	networkName: z.string().min(1, 'Network name is required'),
	networkDescription: z.string().optional(),
	blockPeriod: z.number().min(1),
	chainId: z.number(),
	coinbase: z.string(),
	consensus: z.enum(['qbft']).default('qbft'),
	difficulty: z.string().refine((val) => isValidHex(val), { message: 'Must be a valid hex value starting with 0x' }),
	epochLength: z.number(),
	gasLimit: z.string().refine((val) => isValidHex(val), { message: 'Must be a valid hex value starting with 0x' }),
	mixHash: z.string().refine((val) => isValidHex(val), { message: 'Must be a valid hex value starting with 0x' }),
	nonce: z.string().refine((val) => isValidHex(val), { message: 'Must be a valid hex value starting with 0x' }),
	requestTimeout: z.number(),
	timestamp: z.string().refine((val) => isValidHex(val), { message: 'Must be a valid hex value starting with 0x' }),
	selectedValidatorKeys: z.array(z.number()).min(1, 'At least one validator key must be selected'),
	alloc: z
		.array(
			z.object({
				account: z.string().refine((val) => isValidHex(val), { message: 'Must be a valid hex value starting with 0x' }),
				balance: z.string().refine((val) => isValidHex(val), { message: 'Must be a valid hex value starting with 0x' }),
			})
		)
		.default([]),
})

type NodesStepValues = z.infer<typeof nodesStepSchema>
type NetworkStepValues = z.infer<typeof networkStepSchema>

const defaultNetworkValues: Partial<NetworkStepValues> = {
	blockPeriod: 5,
	chainId: 1337,
	coinbase: '0x0000000000000000000000000000000000000000',
	consensus: 'qbft',
	difficulty: numberToHex(1),
	epochLength: 30000,
	gasLimit: numberToHex(700000000),
	mixHash: '0x63746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365',
	nonce: numberToHex(0),
	requestTimeout: 10,
	timestamp: numberToHex(new Date().getUTCSeconds()),
	alloc: [],
}

type Step = 'nodes' | 'network' | 'nodes-config' | 'review'

export default function BulkCreateBesuNetworkPage() {
	const navigate = useNavigate()
	const [currentStep, setCurrentStep] = useState<Step>(() => {
		if (typeof window !== 'undefined') {
			const savedStep = localStorage.getItem('besuBulkCreateStep')
			return (savedStep as Step) || 'nodes'
		}
		return 'nodes'
	})
	const [validatorKeys, setValidatorKeys] = useState<{ id: number; name: string; publicKey: string; ethereumAddress: string }[]>(() => {
		if (typeof window !== 'undefined') {
			const savedKeys = localStorage.getItem('besuBulkCreateKeys')
			if (savedKeys) {
				const parsedKeys = JSON.parse(savedKeys)
				// If the saved keys don't have publicKey, we'll need to fetch them again
				if (parsedKeys.length > 0 && !parsedKeys[0].publicKey) {
					return []
				}
				return parsedKeys
			}
		}
		return []
	})

	const [creationProgress, setCreationProgress] = useState<{
		current: number
		total: number
		currentNode: string | null
	}>({ current: 0, total: 0, currentNode: null })
	// const [nodeConfigs, setNodeConfigs] = useState<BesuNodeFormValues[]>(() => {
	// 	const savedConfigs = localStorage.getItem('besuBulkCreateNodeConfigs')
	// 	return savedConfigs ? JSON.parse(savedConfigs) : []
	// })
	const [nodeConfigs, setNodeConfigs] = useState<BesuNodeFormValues[]>([])
	console.log('nodeConfigs', nodeConfigs)
	const { data: providersData } = useQuery({
		...getKeyProvidersOptions({}),
	})

	const nodesForm = useForm<NodesStepValues>({
		resolver: zodResolver(nodesStepSchema),
		defaultValues: (() => {
			const savedData = localStorage.getItem('besuBulkCreateNodesForm')
			return savedData ? JSON.parse(savedData) : { numberOfNodes: 4 }
		})(),
	})

	const networkForm = useForm<NetworkStepValues>({
		resolver: zodResolver(networkStepSchema),
		defaultValues: (() => {
			const savedData = localStorage.getItem('besuBulkCreateNetworkForm')
			if (savedData) {
				const parsedData = JSON.parse(savedData)
				return {
					...defaultNetworkValues,
					...parsedData,
				}
			}
			// Get current time in seconds (not milliseconds)
			const currentTimeInSeconds = Math.floor(new Date().getTime() / 1000)
			return {
				...defaultNetworkValues,
				timestamp: numberToHex(currentTimeInSeconds),
				selectedValidatorKeys: [],
			}
		})(),
	})

	const createNode = useMutation(postNodesMutation())

	// Save form data to localStorage whenever it changes
	useEffect(() => {
		const subscription = nodesForm.watch((value) => {
			localStorage.setItem('besuBulkCreateNodesForm', JSON.stringify(value))
		})
		return () => subscription.unsubscribe()
	}, [nodesForm])

	useEffect(() => {
		const subscription = networkForm.watch((value) => {
			localStorage.setItem('besuBulkCreateNetworkForm', JSON.stringify(value))
		})
		return () => subscription.unsubscribe()
	}, [networkForm])

	// Save current step to localStorage whenever it changes
	useEffect(() => {
		localStorage.setItem('besuBulkCreateStep', currentStep)
	}, [currentStep])

	// Save validator keys to localStorage whenever they change
	useEffect(() => {
		if (validatorKeys.length > 0) {
			localStorage.setItem('besuBulkCreateKeys', JSON.stringify(validatorKeys))
		}
	}, [validatorKeys])

	// Save node configs to localStorage whenever they change
	useEffect(() => {
		if (nodeConfigs.length > 0) {
			localStorage.setItem('besuBulkCreateNodeConfigs', JSON.stringify(nodeConfigs))
		}
	}, [nodeConfigs])

	// Add a useEffect to update the form when validatorKeys change
	useEffect(() => {
		if (validatorKeys.length > 0) {
			networkForm.setValue(
				'selectedValidatorKeys',
				validatorKeys.map((key) => key.id)
			)
		}
	}, [validatorKeys, networkForm])

	// Add this effect after the other useEffect hooks
	useEffect(() => {
		const initializeNodeConfigs = async () => {
			// Only run if we're on nodes-config step and have no existing configs
			if (currentStep === 'nodes-config') {
				try {
					const networkId = localStorage.getItem('besuBulkCreateNetworkId')
					const networkName = networkForm.getValues('networkName')
					const numberOfNodes = nodesForm.getValues('numberOfNodes')

					// Fetch default Besu node configuration
					const besuDefaultNodes = await getNodesDefaultsBesuNode({
						query: {
							besuNodes: numberOfNodes,
						},
					})

					if (!besuDefaultNodes.data) {
						throw new Error('No default nodes found')
					}

					// Create node configs using the default nodes array
					const newNodeConfigs = Array.from({ length: numberOfNodes }).map((_, index) => {
						const defaultNode = besuDefaultNodes.data.defaults![index]!

						const { p2pHost, p2pPort, rpcHost, rpcPort, externalIp, internalIp } = defaultNode
						let bootNodes = ''
						if (index > 0 && validatorKeys[0]?.publicKey) {
							// For all nodes after the first one, use the first node as bootnode
							const firstNodeExternalIp = besuDefaultNodes.data.defaults![0]?.externalIp || '127.0.0.1'
							const firstNodeP2pPort = besuDefaultNodes.data.defaults![0]?.p2pPort || '30303'
							bootNodes = `enode://${validatorKeys[0].publicKey.substring(2)}@${firstNodeExternalIp}:${firstNodeP2pPort}`
						}

						return {
							name: `besu-${networkName}-${index + 1}`,
							blockchainPlatform: 'BESU',
							type: 'besu',
							mode: 'service',
							externalIp: externalIp,
							internalIp: internalIp,
							keyId: validatorKeys[index]?.id || 0,
							networkId: networkId ? parseInt(networkId) : 0,
							p2pHost: p2pHost,
							p2pPort: Number(p2pPort),
							rpcHost: rpcHost,
							rpcPort: Number(rpcPort),
							metricsHost: '127.0.0.1',
							metricsPort: 9545 + index,
							bootNodes: bootNodes,
							requestTimeout: 30,
						} as BesuNodeFormValues
					})
					setNodeConfigs(newNodeConfigs)
				} catch (error: any) {
					toast.error('Failed to initialize node configurations', {
						description: error.message,
					})
				}
			}
		}

		initializeNodeConfigs()
	}, [currentStep, nodeConfigs.length, networkForm, nodesForm, validatorKeys])

	const createNetwork = useMutation({
		...postNetworksBesuMutation(),
		onSuccess: () => {
			toast.success('Network created successfully')
		},
		onError: (error: any) => {
			toast.error('Failed to create network', {
				description: error.message,
			})
		},
	})

	const createValidatorKeys = async (numberOfKeys: number, networkName: string) => {
		try {
			setCreationProgress({ current: 0, total: numberOfKeys, currentNode: null })

			const keyPromises = Array.from({ length: numberOfKeys }, (_, i) => {
				return postKeys({
					body: {
						name: `Besu Validator Key ${i + 1}`,
						providerId: providersData?.[0]?.id,
						algorithm: 'EC',
						curve: 'secp256k1',
						description: `Validator Key ${i + 1} for ${networkName}`,
					},
				}).then((key) => {
					setCreationProgress((prev) => ({
						...prev,
						current: prev.current + 1,
						currentNode: `Creating validator key ${i + 1}`,
					}))
					return key
				})
			})

			const createdKeys = await Promise.all(keyPromises)
			const newValidatorKeys = createdKeys.map((key) => ({
				id: key.data!.id!,
				name: key.data!.name!,
				publicKey: key.data!.publicKey!,
				ethereumAddress: key.data!.ethereumAddress!,
			}))
			console.log('newValidatorKeys', newValidatorKeys)
			setValidatorKeys(newValidatorKeys)
			setCreationProgress({ current: 0, total: 0, currentNode: null })
			return newValidatorKeys
		} catch (error: any) {
			toast.error('Failed to create validator keys', {
				description: error.message,
			})
			throw error
		}
	}

	const onNodesStepSubmit = async (data: NodesStepValues) => {
		try {
			const newValidatorKeys = await createValidatorKeys(data.numberOfNodes, networkForm.getValues('networkName'))
			networkForm.setValue(
				'selectedValidatorKeys',
				newValidatorKeys.map((key) => key.id)
			)
			setCurrentStep('network')
		} catch (error) {
			// Error is already handled in createValidatorKeys
		}
	}

	const onNetworkStepSubmit = async (data: NetworkStepValues) => {
		try {
			setCreationProgress({ current: 0, total: 1, currentNode: 'Creating network' })

			const validatorKeyIds = networkForm.getValues('selectedValidatorKeys')
			if (validatorKeyIds.length === 0) {
				toast.error('Please select at least one validator key')
				return
			}

			// Create network
			const networkData = {
				name: data.networkName,
				description: data.networkDescription,
				config: {
					blockPeriod: data.blockPeriod,
					chainId: data.chainId,
					coinbase: data.coinbase,
					consensus: data.consensus,
					difficulty: data.difficulty,
					epochLength: data.epochLength,
					gasLimit: data.gasLimit,
					initialValidatorsKeyIds: validatorKeyIds,
					mixHash: data.mixHash,
					nonce: data.nonce,
					requestTimeout: data.requestTimeout,
					timestamp: data.timestamp,
					alloc: data.alloc.reduce((acc, item) => {
						acc[item.account!] = { balance: item.balance }
						return acc
					}, {} as Record<string, { balance: string }>),
				},
			}

			const network = await createNetwork.mutateAsync({
				body: networkData,
			})

			if (!network.id) {
				throw new Error('Network ID not returned from creation')
			}

			// Store the network ID for use in step 3
			localStorage.setItem('besuBulkCreateNetworkId', network.id.toString())

			// Initialize node configs before moving to step 3
			const numberOfNodes = nodesForm.getValues('numberOfNodes')
			const networkName = data.networkName

			// Fetch default Besu node configuration
			const besuDefaultNodes = await getNodesDefaultsBesuNode({
				query: {
					besuNodes: numberOfNodes,
				},
			})
			if (!besuDefaultNodes.data) {
				throw new Error('No default nodes found')
			}
			// Create node configs using the default nodes array
			const newNodeConfigs = Array.from({ length: numberOfNodes }).map((_, index) => {
				// Get the default node config for this index, or use empty object if not available
				const defaultNode = besuDefaultNodes.data.defaults![index]!

				// Parse default addresses for this node
				const { p2pHost, p2pPort, rpcHost, rpcPort, externalIp, internalIp } = defaultNode

				let bootNodes = ''
				if (index > 0 && validatorKeys[0]?.publicKey) {
					// For all nodes after the first one, use the first node as bootnode
					// Use the first node's external IP and p2p port
					const firstNodeExternalIp = besuDefaultNodes.data.defaults![0]?.externalIp || '127.0.0.1'
					const firstNodeP2pPort = besuDefaultNodes.data.defaults![0]?.p2pPort || '30303'
					bootNodes = `enode://${validatorKeys[0].publicKey.substring(2)}@${firstNodeExternalIp}:${Number(firstNodeP2pPort)}`
				}

				return {
					name: `besu-${networkName}-${index + 1}`,
					blockchainPlatform: 'BESU',
					type: 'besu',
					mode: 'service',
					externalIp: externalIp,
					internalIp: internalIp,
					keyId: validatorKeys[index]?.id || 0,
					networkId: network.id,
					p2pHost: p2pHost,
					p2pPort: Number(p2pPort),
					rpcHost: rpcHost,
					rpcPort: Number(rpcPort),
					metricsHost: '127.0.0.1',
					metricsPort: 9545 + index,
					bootNodes: bootNodes,
					requestTimeout: 30,
				} as BesuNodeFormValues
			})

			setNodeConfigs(newNodeConfigs)
			setCreationProgress({ current: 1, total: 1, currentNode: null })

			setCurrentStep('nodes-config')
		} catch (error: any) {
			toast.error('Failed to create network', {
				description: error.message,
			})
		}
	}

	const onNodesConfigStepSubmit = async () => {
		setCurrentStep('review')
	}

	const onReviewStepSubmit = async () => {
		try {
			setCreationProgress({ current: 0, total: nodeConfigs.length, currentNode: null })

			const networkId = localStorage.getItem('besuBulkCreateNetworkId')
			if (!networkId) {
				throw new Error('Network ID not found')
			}

			// Create nodes
			for (let i = 0; i < nodeConfigs.length; i++) {
				const nodeConfig = nodeConfigs[i]
				setCreationProgress((prev) => ({
					...prev,
					current: prev.current + 1,
					currentNode: `Creating node ${i + 1} of ${nodeConfigs.length}`,
				}))

				await createNode.mutateAsync({
					body: {
						name: nodeConfig.name,
						blockchainPlatform: nodeConfig.blockchainPlatform,
						besuNode: {
							type: nodeConfig.type,
							mode: nodeConfig.mode,
							networkId: parseInt(networkId),
							externalIp: nodeConfig.externalIp,
							internalIp: nodeConfig.internalIp,
							keyId: nodeConfig.keyId,
							p2pHost: '127.0.0.1',
							p2pPort: nodeConfig.p2pPort,
							rpcHost: '127.0.0.1',
							rpcPort: nodeConfig.rpcPort,
							metricsPort: nodeConfig.metricsPort,
							bootNodes: nodeConfig.bootNodes
								?.split(',')
								.map((node) => node.trim())
								.filter(Boolean),
							metricsEnabled: true,
						},
					},
				})
			}

			setCreationProgress((prev) => ({
				...prev,
				current: prev.total,
				currentNode: null,
			}))

			// Clear localStorage only after successful submission
			localStorage.removeItem('besuBulkCreateNodesForm')
			localStorage.removeItem('besuBulkCreateNetworkForm')
			localStorage.removeItem('besuBulkCreateStep')
			localStorage.removeItem('besuBulkCreateKeys')
			localStorage.removeItem('besuBulkCreateNodeConfigs')
			localStorage.removeItem('besuBulkCreateNetworkId')

			toast.success('Nodes created successfully')
			navigate('/networks')
		} catch (error: any) {
			toast.error('Failed to create nodes', {
				description: error.message,
			})
		}
	}

	return (
		<div className="flex-1 p-8">
			<div className="max-w-3xl mx-auto">
				<div className="flex items-center gap-2 text-muted-foreground mb-8">
					<Button variant="ghost" size="sm" asChild>
						<Link to="/networks">
							<ArrowLeft className="mr-2 h-4 w-4" />
							Networks
						</Link>
					</Button>
				</div>

				<div className="flex items-center gap-4 mb-8">
					<Server className="h-8 w-8" />
					<div>
						<h1 className="text-2xl font-semibold">Create Besu Network</h1>
						<p className="text-muted-foreground">Create a new Besu network with multiple nodes</p>
					</div>
				</div>

				<Steps steps={steps} currentStep={currentStep} className="mb-8" />

				{currentStep === 'nodes' && (
					<Form {...nodesForm}>
						<form onSubmit={nodesForm.handleSubmit(onNodesStepSubmit)} className="space-y-8">
							<Card className="p-6">
								<div className="space-y-6">
									<FormField
										control={nodesForm.control}
										name="numberOfNodes"
										render={({ field }) => (
											<FormItem>
												<FormLabel>Number of Nodes</FormLabel>
												<FormControl>
													<Input type="number" min={1} max={10} {...field} onChange={(e) => field.onChange(parseInt(e.target.value))} />
												</FormControl>
												<FormDescription>This will create {field.value} validator keys and nodes</FormDescription>
												<FormMessage />
											</FormItem>
										)}
									/>
								</div>
							</Card>

							<div className="flex justify-between">
								<Button variant="outline" asChild>
									<Link to="/networks">Cancel</Link>
								</Button>
								<Button type="submit">
									Next
									<ArrowRight className="ml-2 h-4 w-4" />
								</Button>
							</div>
						</form>
					</Form>
				)}

				{currentStep === 'network' && (
					<Form {...networkForm}>
						<form onSubmit={networkForm.handleSubmit(onNetworkStepSubmit)} className="space-y-8">
							<Card className="p-6">
								<div className="space-y-6">
									<FormField
										control={networkForm.control}
										name="consensus"
										render={({ field }) => (
											<FormItem>
												<FormLabel>Consensus Algorithm</FormLabel>
												<Select onValueChange={field.onChange} defaultValue={field.value}>
													<FormControl>
														<SelectTrigger>
															<SelectValue placeholder="Select consensus algorithm" />
														</SelectTrigger>
													</FormControl>
													<SelectContent>
														<SelectItem value="qbft">QBFT (Quorum Byzantine Fault Tolerance)</SelectItem>
													</SelectContent>
												</Select>
												<FormDescription>Choose the consensus mechanism for your network</FormDescription>
												<FormMessage />
											</FormItem>
										)}
									/>

									<FormField
										control={networkForm.control}
										name="selectedValidatorKeys"
										render={({}) => (
											<FormItem>
												<FormLabel>Validator Keys</FormLabel>
												<FormDescription>Validator keys generated for your network</FormDescription>
												<div className="space-y-4 mt-2">
													{validatorKeys.map((key) => (
														<div key={key.id} className="p-3 border rounded-lg hover:bg-accent/50">
															<div className="flex-1 space-y-1">
																<div className="flex items-center gap-2">
																	<label className="font-medium">{key.name}</label>
																	<span className="px-2 py-0.5 text-xs rounded-full bg-primary/10 text-primary">Generated in step 1</span>
																</div>
																<div className="text-sm space-y-1">
																	<div className="flex items-center gap-2">
																		<span className="text-muted-foreground">Address:</span>
																		<code className="text-xs bg-muted px-2 py-1 rounded">{key.ethereumAddress}</code>
																	</div>
																	<div className="flex items-center gap-2">
																		<span className="text-muted-foreground">Public Key:</span>
																		<code className="text-xs bg-muted px-2 py-1 rounded">{key.publicKey}</code>
																	</div>
																</div>
															</div>
														</div>
													))}
												</div>
												<FormMessage />
											</FormItem>
										)}
									/>

									<FormField
										control={networkForm.control}
										name="networkName"
										render={({ field }) => (
											<FormItem>
												<FormLabel>Network Name</FormLabel>
												<FormControl>
													<Input placeholder="mybesunetwork" {...field} />
												</FormControl>
												<FormDescription>A unique name for your network</FormDescription>
												<FormMessage />
											</FormItem>
										)}
									/>

									<FormField
										control={networkForm.control}
										name="networkDescription"
										render={({ field }) => (
											<FormItem>
												<FormLabel>Description</FormLabel>
												<FormControl>
													<Input placeholder="My Besu Network" {...field} />
												</FormControl>
												<FormDescription>A brief description of your network</FormDescription>
												<FormMessage />
											</FormItem>
										)}
									/>

									<div className="grid grid-cols-2 gap-4">
										<FormField
											control={networkForm.control}
											name="blockPeriod"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Block Period (seconds)</FormLabel>
													<FormControl>
														<Input type="number" {...field} onChange={(e) => field.onChange(Number(e.target.value))} />
													</FormControl>
													<FormDescription>Time between blocks</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>

										<FormField
											control={networkForm.control}
											name="chainId"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Chain ID</FormLabel>
													<FormControl>
														<Input type="number" {...field} onChange={(e) => field.onChange(Number(e.target.value))} />
													</FormControl>
													<FormDescription>Network identifier</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>
									</div>

									<div className="grid grid-cols-2 gap-4">
										<FormField
											control={networkForm.control}
											name="coinbase"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Coinbase Address</FormLabel>
													<FormControl>
														<Input {...field} placeholder="0x0000000000000000000000000000000000000000" />
													</FormControl>
													<FormDescription>Mining rewards address</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>

										<FormField
											control={networkForm.control}
											name="difficulty"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Difficulty</FormLabel>
													<FormControl>
														<Input
															type="number"
															value={field.value === '0x0' ? 0 : hexToNumber(field.value)}
															onChange={(e) => field.onChange(numberToHex(Number(e.target.value)))}
															min={0}
														/>
													</FormControl>
													<FormDescription>Initial mining difficulty (will be converted to hex)</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>
									</div>

									<div className="grid grid-cols-2 gap-4">
										<FormField
											control={networkForm.control}
											name="epochLength"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Epoch Length</FormLabel>
													<FormControl>
														<Input type="number" {...field} onChange={(e) => field.onChange(Number(e.target.value))} />
													</FormControl>
													<FormDescription>Number of blocks per epoch</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>

										<FormField
											control={networkForm.control}
											name="gasLimit"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Gas Limit</FormLabel>
													<FormControl>
														<Input
															type="number"
															value={field.value === '0x0' ? 0 : hexToNumber(field.value)}
															onChange={(e) => field.onChange(numberToHex(Number(e.target.value)))}
															min={0}
														/>
													</FormControl>
													<FormDescription>Maximum gas per block (will be converted to hex)</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>
									</div>

									<div className="grid grid-cols-2 gap-4">
										<FormField
											control={networkForm.control}
											name="requestTimeout"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Request Timeout (seconds)</FormLabel>
													<FormControl>
														<Input type="number" {...field} onChange={(e) => field.onChange(Number(e.target.value))} />
													</FormControl>
													<FormDescription>Network request timeout</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>

										<FormField
											control={networkForm.control}
											name="mixHash"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Mix Hash</FormLabel>
													<FormControl>
														<Input {...field} />
													</FormControl>
													<FormDescription>Consensus-specific hash</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>
									</div>

									<div className="grid grid-cols-2 gap-4">
										<FormField
											control={networkForm.control}
											name="nonce"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Nonce</FormLabel>
													<FormControl>
														<Input
															type="number"
															value={field.value === '0x0' ? 0 : hexToNumber(field.value)}
															onChange={(e) => field.onChange(numberToHex(Number(e.target.value)))}
															min={0}
														/>
													</FormControl>
													<FormDescription>Genesis block nonce (will be converted to hex)</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>

										<FormField
											control={networkForm.control}
											name="timestamp"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Timestamp</FormLabel>
													<FormControl>
														<Input
															type="number"
															value={field.value === '0x0' ? 0 : hexToNumber(field.value)}
															onChange={(e) => field.onChange(numberToHex(Number(e.target.value)))}
															min={0}
														/>
													</FormControl>
													<FormDescription>Genesis block timestamp (will be converted to hex)</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>
									</div>

									<FormField
										control={networkForm.control}
										name="alloc"
										render={({ field }) => (
											<FormItem>
												<FormLabel>Initial Allocations</FormLabel>
												<FormDescription>Add initial account balances for the genesis block (in hex format)</FormDescription>
												<div className="space-y-4">
													{field.value.map((allocation, index) => (
														<div key={index} className="flex gap-4 items-start">
															<div className="flex-1">
																<Input
																	placeholder="0x..."
																	value={allocation.account}
																	onChange={(e) => {
																		const value = e.target.value
																		// Ensure the value starts with 0x
																		const hexValue = value.startsWith('0x') ? value : `0x${value}`
																		const newAlloc = [...field.value]
																		newAlloc[index] = { ...newAlloc[index], account: hexValue }
																		field.onChange(newAlloc)
																	}}
																/>
															</div>
															<div className="flex-1">
																<Input
																	placeholder="0x..."
																	value={allocation.balance}
																	onChange={(e) => {
																		const value = e.target.value
																		// Ensure the value starts with 0x
																		const hexValue = value.startsWith('0x') ? value : `0x${value}`
																		const newAlloc = [...field.value]
																		newAlloc[index] = { ...newAlloc[index], balance: hexValue }
																		field.onChange(newAlloc)
																	}}
																/>
															</div>
															<Button
																type="button"
																variant="outline"
																size="icon"
																onClick={() => {
																	const newAlloc = field.value.filter((_, i) => i !== index)
																	field.onChange(newAlloc)
																}}
															>
																×
															</Button>
														</div>
													))}
													<Button
														type="button"
														variant="outline"
														onClick={() => {
															field.onChange([...field.value, { account: '0x', balance: '0x0' }])
														}}
													>
														Add Allocation
													</Button>
												</div>
												<FormMessage />
											</FormItem>
										)}
									/>
								</div>
							</Card>

							<div className="flex justify-between">
								<Button type="button" variant="outline" onClick={() => setCurrentStep('nodes')}>
									Previous
								</Button>
								<div className="flex gap-4">
									<Button variant="outline" asChild>
										<Link to="/networks">Cancel</Link>
									</Button>
									<Button type="submit">
										Next
										<ArrowRight className="ml-2 h-4 w-4" />
									</Button>
								</div>
							</div>
						</form>
					</Form>
				)}

				{currentStep === 'nodes-config' && (
					<div className="space-y-8">
						<Card className="p-6">
							<div className="space-y-6">
								<h3 className="text-lg font-semibold">Configure Nodes</h3>
								<p className="text-muted-foreground">Configure {nodesForm.getValues('numberOfNodes')} nodes for your network</p>

								{Array.from({ length: nodesForm.getValues('numberOfNodes') }).map((_, index) => {
									const networkId = localStorage.getItem('besuBulkCreateNetworkId')
									const networkName = networkForm.getValues('networkName')

									// Calculate bootnodes based on node position
									let bootNodes = ''
									if (index > 0) {
										// For all nodes after the first one, use only the first node as bootnode
										const firstNodeP2PPort = 30303
										bootNodes = `enode://${validatorKeys[0]?.publicKey.substring(2)}@127.0.0.1:${firstNodeP2PPort}`
									}

									const defaultNodeConfig = {
										name: `besu-${networkName}-${index + 1}`,
										blockchainPlatform: 'BESU',
										type: 'besu',
										mode: 'service',
										externalIp: '127.0.0.1',
										internalIp: '127.0.0.1',
										keyId: validatorKeys[index]?.id || 0,
										networkId: networkId ? parseInt(networkId) : 0,
										p2pHost: '127.0.0.1',
										p2pPort: 30303 + index,
										rpcHost: '127.0.0.1',
										rpcPort: 8545 + index,
										metricsHost: '127.0.0.1',
										metricsPort: 9545 + index,
										bootNodes: bootNodes,
										requestTimeout: 30,
									} as BesuNodeFormValues

									return (
										<div key={index} className="space-y-4">
											<h4 className="font-medium">
												Node {index + 1} {index < 2 ? '(Bootnode + Validator)' : '(Validator)'}
											</h4>
											<BesuNodeForm
												defaultValues={nodeConfigs[index] || defaultNodeConfig}
												onChange={(values) => {
													const newConfigs = [...nodeConfigs]
													newConfigs[index] = values
													setNodeConfigs(newConfigs)
												}}
												hideSubmit
												onSubmit={() => {}}
											/>
											<hr className="my-6" />
										</div>
									)
								})}
							</div>
						</Card>

						<div className="flex justify-between">
							<Button type="button" variant="outline" onClick={() => setCurrentStep('network')}>
								Previous
							</Button>
							<div className="flex gap-4">
								<Button variant="outline" asChild>
									<Link to="/networks">Cancel</Link>
								</Button>
								<Button type="button" onClick={onNodesConfigStepSubmit} disabled={nodeConfigs.length !== nodesForm.getValues('numberOfNodes')}>
									Next
									<ArrowRight className="ml-2 h-4 w-4" />
								</Button>
							</div>
						</div>
					</div>
				)}

				{currentStep === 'review' && (
					<div className="space-y-8">
						<Card className="p-6">
							<div className="space-y-6">
								<div>
									<h3 className="text-lg font-semibold mb-4">Summary</h3>
									<dl className="space-y-4">
										<div>
											<dt className="text-sm font-medium text-muted-foreground">Network Name</dt>
											<dd className="mt-1">{networkForm.getValues('networkName')}</dd>
										</div>
										<div>
											<dt className="text-sm font-medium text-muted-foreground">Number of Nodes</dt>
											<dd className="mt-1">{nodesForm.getValues('numberOfNodes')}</dd>
										</div>
										<div>
											<dt className="text-sm font-medium text-muted-foreground">Chain ID</dt>
											<dd className="mt-1">{networkForm.getValues('chainId')}</dd>
										</div>
										<div>
											<dt className="text-sm font-medium text-muted-foreground">Nodes</dt>
											<dd className="mt-1">
												<ul className="list-disc list-inside">
													{nodeConfigs?.map((config, index) => (
														<li key={index}>{config?.name}</li>
													))}
												</ul>
											</dd>
										</div>
									</dl>
								</div>

								{creationProgress.total > 0 && (
									<div className="space-y-2">
										<div className="flex justify-between text-sm">
											<span>Creating network and nodes...</span>
											<span>
												{creationProgress.current} of {creationProgress.total}
											</span>
										</div>
										<Progress value={(creationProgress.current / creationProgress.total) * 100} />
										{creationProgress.currentNode && <p className="text-sm text-muted-foreground">{creationProgress.currentNode}</p>}
									</div>
								)}
							</div>
						</Card>

						<div className="flex justify-between">
							<Button type="button" variant="outline" onClick={() => setCurrentStep('nodes-config')}>
								Previous
							</Button>
							<div className="flex gap-4">
								<Button variant="outline" asChild>
									<Link to="/networks">Cancel</Link>
								</Button>
								<Button type="button" onClick={onReviewStepSubmit}>
									<CheckCircle2 className="mr-2 h-4 w-4" />
									Create Network
								</Button>
							</div>
						</div>
					</div>
				)}
			</div>
		</div>
	)
}

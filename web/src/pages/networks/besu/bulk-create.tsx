import { getNodesDefaultsBesuNode, postKeys, HttpCreateBesuNetworkRequest } from '@/api/client'
import { getKeyProvidersOptions, getKeysOptions, postNetworksBesuMutation, postNodesMutation, deleteNodesByIdMutation } from '@/api/client/@tanstack/react-query.gen'
import { BesuNodeForm, BesuNodeFormValues } from '@/components/nodes/besu-node-form'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Steps } from '@/components/ui/steps'
import { hexToNumber, isValidHex, numberToHex, numberToNonceHex } from '@/utils'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { ArrowLeft, ArrowRight, CheckCircle2, Copy, Server, RefreshCw, Trash2, ChevronDown, ChevronRight, AlertCircle } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import * as z from 'zod'
import { usePageTitle } from '@/hooks/use-page-title'

// Helper function to convert ETH to WEI
const ethToWei = (eth: number): string => {
	return (eth * 10 ** 18).toString()
}

// Helper function to convert WEI to ETH
const weiToEth = (wei: string): number => {
	const eth = Number(wei) / 10 ** 18
	// Handle floating-point precision issues for very small numbers
	return Math.abs(eth) < 1e-10 ? 0 : eth
}

// Helper function to convert ETH to HEX (via WEI)
const ethToHex = (eth: number): string => {
	const wei = ethToWei(eth)
	return numberToHex(Number(wei))
}

const steps = [
	{ id: 'nodes', title: 'Number of Nodes' },
	{ id: 'network', title: 'Network Configuration' },
	{ id: 'nodes-config', title: 'Nodes Configuration' },
	{ id: 'review', title: 'Review & Create' },
]

// Besu versions from https://github.com/hyperledger/besu/releases
const besuVersions = [
	'25.7.0',
	'25.6.0',
	'25.5.0',
	'25.4.0',
	'25.3.0',
	'25.2.0',
	'25.1.0',
	'24.12.2',
]

const nodesStepSchema = z.object({
	numberOfNodes: z.number().min(1).max(10),
	networkName: z.string().min(1, 'Network name is required'),
	version: z.string().min(1, 'Version is required'),
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
	externalValidatorKeys: z.array(z.object({
		ethereumAddress: z.string(),
		publicKey: z.string(),
	})).optional(),
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
	externalValidatorKeys: [],
	mixHash: '0x63746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365',
	nonce: numberToNonceHex(0),
	requestTimeout: 10,
	timestamp: numberToHex(new Date().getUTCSeconds()),
	alloc: [],
}

type Step = 'nodes' | 'network' | 'nodes-config' | 'review'

export default function BulkCreateBesuNetworkPage() {
	usePageTitle('Bulk Besu')
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

	const [nodeCreationResults, setNodeCreationResults] = useState<Record<string, {
		status: 'pending' | 'success' | 'error'
		error?: string
		nodeId?: number
		nodeStatus?: 'RUNNING' | 'STOPPED' | 'ERROR' | string
	}>>({})

	const [failedNodes, setFailedNodes] = useState<string[]>([])
	const [, setCreatedNodeIds] = useState<number[]>([])
	const [hasAttemptedCreation, setHasAttemptedCreation] = useState(false)
	const [expandedNodes, setExpandedNodes] = useState<Record<number, boolean>>(() => {
		// Expand first node by default
		return { 0: true }
	})

	const createNode = useMutation(postNodesMutation())
	const deleteNode = useMutation(deleteNodesByIdMutation())

	const [nodeConfigs, setNodeConfigs] = useState<BesuNodeFormValues[]>([])
	const [besuVerificationError, setBesuVerificationError] = useState<string | null>(null)
	const { data: providersData } = useQuery({
		...getKeyProvidersOptions({}),
	})

	const { data: availableKeys } = useQuery({
		...getKeysOptions(),
	})

	const [showKeySelector, setShowKeySelector] = useState<number | null>(null)
	const [creatingKey, setCreatingKey] = useState<number | null>(null)

	// Function to ensure validator keys are in allocations
	const ensureValidatorAllocations = () => {
		if (validatorKeys.length > 0) {
			const currentAlloc = networkForm.getValues('alloc')
			const existingValidatorAddresses = currentAlloc.map(alloc => alloc.account)

			// Find validator keys that aren't in allocations yet
			const newValidatorAllocations = validatorKeys
				.filter(key => !existingValidatorAddresses.includes(key.ethereumAddress))
				.map(key => ({
					account: key.ethereumAddress,
					balance: ethToHex(1000000), // 1 million ETH
				}))

			if (newValidatorAllocations.length > 0) {
				networkForm.setValue('alloc', [...currentAlloc, ...newValidatorAllocations])
			}
		}
	}

	const nodesForm = useForm<NodesStepValues>({
		resolver: zodResolver(nodesStepSchema),
		defaultValues: (() => {
			const savedData = localStorage.getItem('besuBulkCreateNodesForm')
			return savedData ? JSON.parse(savedData) : { numberOfNodes: 4, networkName: '', version: '25.7.0' }
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

	const clearLocalStorage = () => {
		localStorage.removeItem('besuBulkCreateNodesForm')
		localStorage.removeItem('besuBulkCreateNetworkForm')
		localStorage.removeItem('besuBulkCreateStep')
		localStorage.removeItem('besuBulkCreateKeys')
		localStorage.removeItem('besuBulkCreateNodeConfigs')
		localStorage.removeItem('besuBulkCreateNetworkId')
		setHasAttemptedCreation(false)
		setBesuVerificationError(null)
	}

	const retryAllFailedNodes = async () => {
		const networkId = localStorage.getItem('besuBulkCreateNetworkId')
		if (!networkId) {
			toast.error('Network ID not found')
			return
		}

		const failedNodeConfigs = nodeConfigs.filter(config => failedNodes.includes(config.name))

		for (let i = 0; i < failedNodeConfigs.length; i++) {
			const nodeConfig = failedNodeConfigs[i]
			const nodeIndex = nodeConfigs.findIndex(config => config.name === nodeConfig.name)
			await createSingleNode(nodeConfig, networkId, nodeIndex)
		}
	}

	const continueWithSuccessful = () => {
		clearLocalStorage()
		toast.success('Continuing with successfully created nodes')
		navigate('/networks')
	}

	const toggleNodeExpansion = (nodeIndex: number) => {
		setExpandedNodes(prev => ({
			...prev,
			[nodeIndex]: !prev[nodeIndex]
		}))
	}

	const expandAllNodes = () => {
		const allExpanded = Array.from({ length: nodesForm.getValues('numberOfNodes') }, (_, i) => i)
			.reduce((acc, i) => ({ ...acc, [i]: true }), {})
		setExpandedNodes(allExpanded)
	}

	const collapseAllNodes = () => {
		setExpandedNodes({})
	}

	const deleteAndRecreateNode = async (nodeName: string) => {
		const result = nodeCreationResults[nodeName]
		if (!result?.nodeId) {
			toast.error('Node ID not found')
			return
		}

		try {
			// Delete the node first
			await deleteNode.mutateAsync({ path: { id: result.nodeId } })

			// Remove from created nodes list
			setCreatedNodeIds(prev => prev.filter(id => id !== result.nodeId))

			// Reset the node result
			setNodeCreationResults(prev => {
				const newResults = { ...prev }
				delete newResults[nodeName]
				return newResults
			})

			setFailedNodes(prev => prev.filter(name => name !== nodeName))

			toast.success('Node deleted successfully. You can now retry creation.')

			// Retry creation immediately
			await retryNodeCreation(nodeName)

		} catch (error: any) {
			toast.error('Failed to delete node', {
				description: error.message,
			})
		}
	}

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

	// Clear verification error when version changes
	useEffect(() => {
		setBesuVerificationError(null)
	}, [nodesForm.watch('version')])

	// Add a useEffect to update the form when validatorKeys change
	useEffect(() => {
		if (validatorKeys.length > 0) {
			networkForm.setValue(
				'selectedValidatorKeys',
				validatorKeys.map((key) => key.id)
			)

			// Add validator keys to initial allocations if they're not already there
			const currentAlloc = networkForm.getValues('alloc')
			const existingValidatorAddresses = currentAlloc.map(alloc => alloc.account)

			// Find validator keys that aren't in allocations yet
			const newValidatorAllocations = validatorKeys
				.filter(key => !existingValidatorAddresses.includes(key.ethereumAddress))
				.map(key => ({
					account: key.ethereumAddress,
					balance: ethToHex(1000000), // 1 million ETH
				}))

			if (newValidatorAllocations.length > 0) {
				networkForm.setValue('alloc', [...currentAlloc, ...newValidatorAllocations])
			}
		}
	}, [validatorKeys, networkForm])

	// Ensure validator allocations are added when component loads or step changes
	useEffect(() => {
		if (currentStep === 'network' && validatorKeys.length > 0) {
			ensureValidatorAllocations()
		}
	}, [currentStep, validatorKeys])

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

						const { p2pHost, p2pPort, rpcHost, rpcPort, metricsPort, externalIp, internalIp } = defaultNode
						let bootNodes: string[] = []
						if (index > 0 && validatorKeys[0]?.publicKey) {
							// For all nodes after the first one, use the first node as bootnode
							const firstNodeExternalIp = besuDefaultNodes.data.defaults![0]?.externalIp || '127.0.0.1'
							const firstNodeP2pPort = besuDefaultNodes.data.defaults![0]?.p2pPort || '30303'
							bootNodes = [`enode://${validatorKeys[0].publicKey.substring(2)}@${firstNodeExternalIp}:${firstNodeP2pPort}`]
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
							metricsEnabled: true,
							metricsHost: '127.0.0.1',
							metricsPort,
							metricsProtocol: 'PROMETHEUS',
							bootNodes: bootNodes,
							requestTimeout: 30,
							version: nodesForm.getValues('version') || '25.7.0',
							// Gas and access control configuration
							minGasPrice: 0,
							hostAllowList: '',
							// Permissions configuration
							accountsAllowList: [],
							nodesAllowList: [],
							// JWT Authentication configuration
							jwtEnabled: false,
							jwtPublicKeyContent: '',
							jwtAuthenticationAlgorithm: '',
							// Other configuration
							environmentVariables: [],
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
			setValidatorKeys(newValidatorKeys)

			// Add validator keys to initial allocations
			const validatorAllocations = newValidatorKeys.map((key) => ({
				account: key.ethereumAddress,
				balance: ethToHex(1000000), // Start with 1 million ETH balance for validators
			}))

			networkForm.setValue('alloc', validatorAllocations)

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
			// Set the network name in the network form from nodes form
			networkForm.setValue('networkName', data.networkName)

			const newValidatorKeys = await createValidatorKeys(data.numberOfNodes, data.networkName)
			networkForm.setValue(
				'selectedValidatorKeys',
				newValidatorKeys.map((key) => key.id)
			)

			// Add validator keys to initial allocations
			const validatorAllocations = newValidatorKeys.map((key) => ({
				account: key.ethereumAddress,
				balance: ethToHex(1000000), // Start with 1 million ETH balance for validators
			}))

			networkForm.setValue('alloc', validatorAllocations)

			setCurrentStep('network')
		} catch (error: any) {
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
			const networkData: HttpCreateBesuNetworkRequest = {
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
				body: networkData as HttpCreateBesuNetworkRequest,
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

				let bootNodes: string[] = []
				if (index > 0 && validatorKeys[0]?.publicKey) {
					// For all nodes after the first one, use the first node as bootnode
					// Use the first node's external IP and p2p port
					const firstNodeExternalIp = besuDefaultNodes.data.defaults![0]?.externalIp || '127.0.0.1'
					const firstNodeP2pPort = besuDefaultNodes.data.defaults![0]?.p2pPort || '30303'
					bootNodes = [`enode://${validatorKeys[0].publicKey.substring(2)}@${firstNodeExternalIp}:${Number(firstNodeP2pPort)}`]
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
					version: nodesForm.getValues('version') || '25.7.0',
					// Gas and access control configuration
					minGasPrice: 0,
					hostAllowList: '',
					// Permissions configuration
					accountsAllowList: [],
					nodesAllowList: [],
					// JWT Authentication configuration
					jwtEnabled: false,
					jwtPublicKeyContent: '',
					jwtAuthenticationAlgorithm: '',
					// Metrics configuration
					metricsEnabled: true,
					metricsProtocol: 'PROMETHEUS',
					environmentVariables: [],
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

	const createSingleNode = async (nodeConfig: BesuNodeFormValues, networkId: string, _index: number): Promise<{ status: 'success' | 'error', nodeId: number, nodeStatus?: string, error?: string }> => {
		try {
			const result = await createNode.mutateAsync({
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
						version: nodeConfig.version || nodesForm.getValues('version') || '25.7.0',
						bootNodes: Array.isArray(nodeConfig.bootNodes)
							? nodeConfig.bootNodes.map((node) => node.trim()).filter(Boolean)
							: typeof nodeConfig.bootNodes === 'string'
								? (nodeConfig.bootNodes as string).split(',').map((node) => node.trim()).filter(Boolean)
								: [],
						metricsEnabled: true,
						metricsProtocol: 'PROMETHEUS',
						minGasPrice: nodeConfig.minGasPrice,
						hostAllowList: nodeConfig.hostAllowList,
						accountsAllowList: nodeConfig.accountsAllowList,
						nodesAllowList: nodeConfig.nodesAllowList,
						jwtEnabled: nodeConfig.jwtEnabled,
						jwtPublicKeyContent: nodeConfig.jwtPublicKeyContent,
						jwtAuthenticationAlgorithm: nodeConfig.jwtAuthenticationAlgorithm,
					},
				},
			})

			// Verify the response has a valid node ID
			if (!result.id) {
				throw new Error('Node created but no ID returned')
			}

			// Check if node status indicates immediate failure
			if (result.status === 'ERROR' || result.status === 'FAILED') {
				throw new Error(`Node created with error status: ${result.status}`)
			}

			return { status: 'success', nodeId: result.id, nodeStatus: result.status }

		} catch (error: any) {
			const errorMessage = error.response?.data?.message || error.message || 'Unknown error'
			return { status: 'error', nodeId: 0, error: errorMessage }
		}
	}

	const retryNodeCreation = async (nodeName: string) => {
		const networkId = localStorage.getItem('besuBulkCreateNetworkId')
		if (!networkId) {
			toast.error('Network ID not found')
			return
		}
		const result = nodeCreationResults[nodeName]
		if (result?.nodeId) {
			try {
				await deleteNode.mutateAsync({ path: { id: result.nodeId } })
				// Remove from created nodes list
				setCreatedNodeIds(prev => prev.filter(id => id !== result.nodeId))
				// Reset the node result
				setNodeCreationResults(prev => {
					const newResults = { ...prev }
					delete newResults[nodeName]
					return newResults
				})
				setFailedNodes(prev => prev.filter(name => name !== nodeName))
			} catch (error: any) {
				// Silently handle deletion errors - node might not exist
				console.warn(`Failed to delete node ${result.nodeId}:`, error.message)
			}
		}

		const nodeIndex = nodeConfigs.findIndex(config => config.name === nodeName)
		if (nodeIndex === -1) {
			toast.error('Node configuration not found. Please refresh the page and try again.')
			return
		}

		const nodeConfig = nodeConfigs[nodeIndex]
		if (!nodeConfig) {
			toast.error('Invalid node configuration')
			return
		}

		// Update the node configuration to ensure it has the latest network ID
		const updatedNodeConfig = {
			...nodeConfig,
			networkId: parseInt(networkId)
		}

		// Update the nodeConfigs array with the updated configuration
		setNodeConfigs(prev => {
			const newConfigs = [...prev]
			newConfigs[nodeIndex] = updatedNodeConfig
			return newConfigs
		})

		return await createSingleNode(updatedNodeConfig, networkId, nodeIndex)
	}

	const removeFailedNode = (nodeName: string) => {
		setNodeConfigs(prev => prev.filter(config => config.name !== nodeName))
		setNodeCreationResults(prev => {
			const newResults = { ...prev }
			delete newResults[nodeName]
			return newResults
		})
		setFailedNodes(prev => prev.filter(name => name !== nodeName))
		toast.success('Node removed from creation list')
	}

	const onReviewStepSubmit = async () => {
		try {
			setCreationProgress({ current: 0, total: nodeConfigs.length, currentNode: null })
			setHasAttemptedCreation(true)
			// Reset state for new creation attempt
			setNodeCreationResults({})
			setFailedNodes([])

			const networkId = localStorage.getItem('besuBulkCreateNetworkId')
			if (!networkId) {
				throw new Error('Network ID not found')
			}

			// Start creating nodes immediately
			setCreationProgress(prev => ({
				...prev,
				currentNode: `Creating ${nodeConfigs.length} nodes in parallel...`,
			}))

			const nodeCreationPromises = nodeConfigs.map((nodeConfig, index) =>
				createSingleNode(nodeConfig, networkId, index).then(result => {
					setCreationProgress(prev => ({
						...prev,
						current: prev.current + 1,
						currentNode: `${prev.current + 1} of ${nodeConfigs.length} nodes processed`,
					}))
					return result
				})
			)

			const nodesCreated = await Promise.allSettled(nodeCreationPromises)

			const failedNodesList: string[] = []
			nodesCreated.forEach((result, index) => {
				const nodeName = nodeConfigs[index].name
				if (result.status === 'fulfilled' && result.value.status === 'success') {
					setNodeCreationResults(prev => ({
						...prev,
						[nodeName]: {
							status: 'success',
							nodeId: result.value.nodeId,
							nodeStatus: result.value.nodeStatus as any,
						},
					}))
					setCreatedNodeIds(prev => [...prev, result.value.nodeId])
				} else {
					failedNodesList.push(nodeName)
					const errorMessage = result.status === 'rejected' ? (result.reason?.message || 'Unknown error') : (result.value?.error || 'Failed to create node')
					setNodeCreationResults(prev => ({
						...prev,
						[nodeName]: { status: 'error', error: errorMessage },
					}))
				}
			})

			setFailedNodes(failedNodesList)
			setCreationProgress(prev => ({ ...prev, currentNode: null }))

			// Navigate to the Besu network detail page when creation finishes
			if (failedNodesList.length === 0) {
				try {
					const finalNetworkId = localStorage.getItem('besuBulkCreateNetworkId') || networkId
					if (finalNetworkId) {
						navigate(`/networks/${finalNetworkId}/besu?congrats=true`)
					}
				} catch (_) {
					// ignore navigation errors
				}
			}
		} catch (error: any) {
			toast.error('Failed to create nodes', { description: error.message })
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

				<div className="flex items-center justify-between mb-8">
					<div className="flex items-center gap-4">
						<Server className="h-8 w-8" />
						<div>
							<h1 className="text-2xl font-semibold">Create Besu Network</h1>
							<p className="text-muted-foreground">Create a new Besu network with multiple nodes</p>
						</div>
					</div>
					<Button
						variant="outline"
						onClick={() => {
							clearLocalStorage()
							// Reset all form states
							nodesForm.reset({ numberOfNodes: 4, networkName: '', version: '25.7.0' })
							networkForm.reset({
								...defaultNetworkValues,
								timestamp: numberToHex(Math.floor(new Date().getTime() / 1000)),
								selectedValidatorKeys: [],
							})
							setValidatorKeys([])
							setNodeConfigs([])
							setCurrentStep('nodes')
							setNodeCreationResults({})
							setFailedNodes([])
							setCreatedNodeIds([])
							setHasAttemptedCreation(false)
							setCreationProgress({ current: 0, total: 0, currentNode: null })
							setExpandedNodes({ 0: true })
							toast.success('Form reset successfully')
						}}
					>
						<RefreshCw className="mr-2 h-4 w-4" />
						Reset Form
					</Button>
				</div>

				<Steps steps={steps} currentStep={currentStep} className="mb-8" />

				{besuVerificationError && (
					<Alert className="mb-6" variant="destructive">
						<AlertCircle className="h-4 w-4" />
						<AlertTitle>Besu version verification failed</AlertTitle>
						<AlertDescription className="whitespace-pre-line">
							{besuVerificationError}
						</AlertDescription>
					</Alert>
				)}

				{currentStep === 'nodes' && (
					<Form {...nodesForm}>
						<form onSubmit={nodesForm.handleSubmit(onNodesStepSubmit)} className="space-y-8">
							<Card className="p-6">
								<div className="space-y-6">
									<FormField
										control={nodesForm.control}
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

									<FormField
										control={nodesForm.control}
										name="version"
										render={({ field }) => (
											<FormItem>
												<FormLabel>Besu Version</FormLabel>
												<Select onValueChange={field.onChange} defaultValue={field.value}>
													<FormControl>
														<SelectTrigger>
															<SelectValue placeholder="Select Besu version" />
														</SelectTrigger>
													</FormControl>
													<SelectContent>
														{besuVersions.map((version) => (
															<SelectItem key={version} value={version}>
																{version}
															</SelectItem>
														))}
													</SelectContent>
												</Select>
												<FormDescription>Version of Besu to use for all nodes</FormDescription>
												<FormMessage />
											</FormItem>
										)}
									/>
								</div>
							</Card>

							<div className="flex justify-between">
								<Button variant="outline" asChild>
									<Link to="/networks" onClick={clearLocalStorage}>Cancel</Link>
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
										render={({ }) => (
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
																		<div className="flex items-center gap-2">
																			<code className="text-xs bg-muted px-2 py-1 rounded">{key.ethereumAddress}</code>
																			<Button
																				type="button"
																				variant="ghost"
																				size="sm"
																				className="h-6 w-6 p-0"
																				onClick={() => {
																					navigator.clipboard.writeText(key.ethereumAddress)
																					toast.success('Address copied to clipboard')
																				}}
																			>
																				<Copy className="h-3 w-3" />
																			</Button>
																		</div>
																	</div>
																	<div className="flex items-center gap-2">
																		<span className="text-muted-foreground">Public Key:</span>
																		<div className="flex items-center gap-2">
																			<code className="text-xs bg-muted px-2 py-1 rounded">
																				{key.publicKey.length > 20
																					? `${key.publicKey.slice(0, 10)}...${key.publicKey.slice(-10)}`
																					: key.publicKey
																				}
																			</code>
																			<Button
																				type="button"
																				variant="ghost"
																				size="sm"
																				className="h-6 w-6 p-0"
																				onClick={() => {
																					navigator.clipboard.writeText(key.publicKey)
																					toast.success('Public key copied to clipboard')
																				}}
																			>
																				<Copy className="h-3 w-3" />
																			</Button>
																		</div>
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
															value={hexToNumber(field.value)}
															onChange={(e) => field.onChange(numberToNonceHex(Number(e.target.value)))}
															min={0}
														/>
													</FormControl>
													<FormDescription>Genesis block nonce (will be converted to hex, 8 bytes)</FormDescription>
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
												<FormDescription>
													Set initial account balances for the genesis block. Validator keys have been automatically added.
												</FormDescription>
												<div className="space-y-6">
													{field.value.map((allocation, index) => {
														// Check if this allocation corresponds to a validator key
														const validatorKey = validatorKeys.find(key => key.ethereumAddress === allocation.account)
														const isValidatorKey = !!validatorKey

														return (
															<div key={index} className="border rounded-lg p-4 space-y-3">
																{isValidatorKey ? (
																	<div className="space-y-3">
																		<div className="flex items-center gap-2">
																			<span className="px-2 py-1 text-xs rounded-full bg-primary/10 text-primary font-medium">
																				Validator {validatorKey?.name}
																			</span>
																		</div>
																		<div className="grid grid-cols-1 md:grid-cols-2 gap-3">
																			<div>
																				<label className="text-sm font-medium text-muted-foreground mb-1 block">
																					Ethereum Address
																				</label>
																				<div className="flex items-center gap-2">
																					<Input
																						value={allocation.account}
																						readOnly
																						className="font-mono text-xs"
																					/>
																					<Button
																						type="button"
																						variant="ghost"
																						size="sm"
																						onClick={() => {
																							navigator.clipboard.writeText(allocation.account)
																							toast.success('Address copied to clipboard')
																						}}
																					>
																						<Copy className="h-3 w-3" />
																					</Button>
																				</div>
																			</div>
																			<div>
																				<label className="text-sm font-medium text-muted-foreground mb-1 block">
																					Initial Balance (ETH)
																				</label>
																				<Input
																					type="number"
																					placeholder="0"
																					value={(() => {
																						if (allocation.balance === '0x0') return 0
																						const weiValue = hexToNumber(allocation.balance)
																						const ethValue = weiToEth(weiValue.toString())
																						// Round to 10 decimal places to avoid floating point issues
																						return Math.round(ethValue * 1e10) / 1e10
																					})()}
																					onChange={(e) => {
																						const value = Number(e.target.value)
																						const hexValue = ethToHex(value)
																						const newAlloc = [...field.value]
																						newAlloc[index] = { ...newAlloc[index], balance: hexValue }
																						field.onChange(newAlloc)
																					}}
																					min={0}
																					step={0.1}
																				/>
																			</div>
																		</div>
																	</div>
																) : (
																	<div className="space-y-3">
																		<div className="flex items-center gap-2">
																			<span className="px-2 py-1 text-xs rounded-full bg-muted text-muted-foreground">
																				Additional Account
																			</span>
																		</div>
																		<div className="grid grid-cols-1 md:grid-cols-2 gap-3">
																			<div>
																				<label className="text-sm font-medium text-muted-foreground mb-1 block">
																					Ethereum Address
																				</label>
																				<div className="space-y-2">
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
																						className="font-mono text-xs"
																					/>
																					<div className="flex gap-2">
																						<Button
																							type="button"
																							variant="outline"
																							size="sm"
																							onClick={() => setShowKeySelector(showKeySelector === index ? null : index)}
																						>
																							{showKeySelector === index ? 'Hide Keys' : 'Select from Keys'}
																						</Button>
																						<Button
																							type="button"
																							variant="outline"
																							size="sm"
																							disabled={creatingKey === index}
																							onClick={async () => {
																								setCreatingKey(index)
																								try {
																									const newKey = await postKeys({
																										body: {
																											name: `Additional Key ${Date.now()}`,
																											providerId: providersData?.[0]?.id,
																											algorithm: 'EC',
																											curve: 'secp256k1',
																											description: `Additional key for network allocation`,
																										},
																									})

																									if (newKey.data?.ethereumAddress) {
																										const newAlloc = [...field.value]
																										newAlloc[index] = { ...newAlloc[index], account: newKey.data.ethereumAddress }
																										field.onChange(newAlloc)
																										toast.success('New key created and address added')
																									}
																								} catch (error: any) {
																									toast.error('Failed to create new key', {
																										description: error.message,
																									})
																								} finally {
																									setCreatingKey(null)
																								}
																							}}
																						>
																							{creatingKey === index ? 'Creating...' : 'Create New Key'}
																						</Button>
																					</div>
																					{showKeySelector === index && availableKeys && (
																						<div className="border rounded-lg p-3 space-y-2 max-h-40 overflow-y-auto">
																							<div className="text-xs font-medium text-muted-foreground mb-2">
																								Available Keys (EC secp256k1):
																							</div>
																							{availableKeys?.items
																								?.filter(key => key.algorithm === 'EC' && key.curve === 'secp256k1')
																								.map((key) => (
																									<div
																										key={key.id}
																										className="flex items-center justify-between p-2 border rounded hover:bg-accent/50 cursor-pointer"
																										onClick={() => {
																											const newAlloc = [...field.value]
																											newAlloc[index] = { ...newAlloc[index], account: key.ethereumAddress! }
																											field.onChange(newAlloc)
																											setShowKeySelector(null)
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
																							{availableKeys?.items?.filter(key => key.algorithm === 'EC' && key.curve === 'secp256k1').length === 0 && (
																								<div className="text-xs text-muted-foreground text-center py-2">
																									No EC secp256k1 keys available
																								</div>
																							)}
																						</div>
																					)}
																				</div>
																			</div>
																			<div>
																				<label className="text-sm font-medium text-muted-foreground mb-1 block">
																					Initial Balance (ETH)
																				</label>
																				<Input
																					type="number"
																					placeholder="0"
																					value={(() => {
																						if (allocation.balance === '0x0') return 0
																						const weiValue = hexToNumber(allocation.balance)
																						const ethValue = weiToEth(weiValue.toString())
																						// Round to 10 decimal places to avoid floating point issues
																						return Math.round(ethValue * 1e10) / 1e10
																					})()}
																					onChange={(e) => {
																						const value = Number(e.target.value)
																						const hexValue = ethToHex(value)
																						const newAlloc = [...field.value]
																						newAlloc[index] = { ...newAlloc[index], balance: hexValue }
																						field.onChange(newAlloc)
																					}}
																					min={0}
																					step={0.1}
																				/>
																			</div>
																		</div>
																		<div className="flex justify-end">
																			<Button
																				type="button"
																				variant="outline"
																				size="sm"
																				onClick={() => {
																					const newAlloc = field.value.filter((_, i) => i !== index)
																					field.onChange(newAlloc)
																				}}
																			>
																				Remove
																			</Button>
																		</div>
																	</div>
																)}
															</div>
														)
													})}
													<Button
														type="button"
														variant="outline"
														onClick={() => {
															field.onChange([...field.value, { account: '0x', balance: ethToHex(1000000) }])
														}}
													>
														Add Additional Account
													</Button>
												</div>
												<FormMessage />
											</FormItem>
										)}
									/>

									{/* External Validator Keys Section */}
									<FormField
										control={networkForm.control}
										name="externalValidatorKeys"
										render={({ field }) => (
											<FormItem>
												<FormLabel>External Validator Keys (Optional)</FormLabel>
												<FormDescription className="mb-3">
													Add external validator keys by providing their Ethereum address and public key
												</FormDescription>
												<div className="space-y-3">
													{(field.value || []).map((extKey, index) => (
														<div key={index} className="p-3 border rounded-lg space-y-2">
															<div className="grid grid-cols-1 gap-2">
																<div>
																	<label className="text-sm font-medium text-muted-foreground mb-1 block">
																		Ethereum Address
																	</label>
																	<Input
																		placeholder="0x..."
																		value={extKey.ethereumAddress}
																		onChange={(e) => {
																			const currentKeys = [...(field.value || [])]
																			currentKeys[index] = { ...currentKeys[index], ethereumAddress: e.target.value }
																			field.onChange(currentKeys)
																		}}
																		className="font-mono text-xs"
																	/>
																</div>
																<div>
																	<label className="text-sm font-medium text-muted-foreground mb-1 block">
																		Public Key
																	</label>
																	<Input
																		placeholder="0x04..."
																		value={extKey.publicKey}
																		onChange={(e) => {
																			const currentKeys = [...(field.value || [])]
																			currentKeys[index] = { ...currentKeys[index], publicKey: e.target.value }
																			field.onChange(currentKeys)
																		}}
																		className="font-mono text-xs"
																	/>
																</div>
															</div>
															<div className="flex justify-end">
																<Button
																	type="button"
																	variant="outline"
																	size="sm"
																	onClick={() => {
																		const currentKeys = (field.value || []).filter((_, i) => i !== index)
																		field.onChange(currentKeys)
																	}}
																>
																	Remove
																</Button>
															</div>
														</div>
													))}
													<Button
														type="button"
														variant="outline"
														size="sm"
														onClick={() => {
															field.onChange([...(field.value || []), { ethereumAddress: '', publicKey: '' }])
														}}
													>
														Add External Validator Key
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
										<Link to="/networks" onClick={clearLocalStorage}>Cancel</Link>
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
								<div className="flex items-center justify-between">
									<div>
										<h3 className="text-lg font-semibold">Configure Nodes</h3>
										<p className="text-muted-foreground">Configure {nodesForm.getValues('numberOfNodes')} nodes for your network</p>
									</div>
									<div className="flex items-center gap-2">
										<Button
											type="button"
											variant="outline"
											size="sm"
											onClick={expandAllNodes}
										>
											Expand All
										</Button>
										<Button
											type="button"
											variant="outline"
											size="sm"
											onClick={collapseAllNodes}
										>
											Collapse All
										</Button>
									</div>
								</div>

								{Array.from({ length: nodesForm.getValues('numberOfNodes') }).map((_, index) => {
									const networkId = localStorage.getItem('besuBulkCreateNetworkId')
									const networkName = networkForm.getValues('networkName')

									// Calculate bootnodes based on node position
									let bootNodes: string[] = []
									if (index > 0) {
										// For all nodes after the first one, use only the first node as bootnode
										const firstNodeP2PPort = 30303 + index
										bootNodes = [`enode://${validatorKeys[0]?.publicKey.substring(2)}@127.0.0.1:${firstNodeP2PPort}`]
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
										version: nodesForm.getValues('version') || '25.7.0',
										// Gas and access control configuration
										minGasPrice: 0,
										hostAllowList: '',
										// Permissions configuration
										accountsAllowList: [],
										nodesAllowList: [],
										// JWT Authentication configuration
										jwtEnabled: false,
										jwtPublicKeyContent: '',
										jwtAuthenticationAlgorithm: '',
										// Metrics configuration
										metricsEnabled: true,
										metricsProtocol: 'PROMETHEUS',
										environmentVariables: [],
									} as BesuNodeFormValues

									// Get the validator key and its allocation for this node
									const validatorKey = validatorKeys[index]
									const allocation = networkForm.getValues('alloc').find(alloc => alloc.account === validatorKey?.ethereumAddress)

									const isExpanded = expandedNodes[index]

									return (
										<div key={index} className="border rounded-lg">
											<div
												className="p-4 cursor-pointer hover:bg-muted/50 transition-colors"
												onClick={() => toggleNodeExpansion(index)}
											>
												<div className="flex items-center justify-between">
													<div className="flex items-center gap-2">
														{isExpanded ? (
															<ChevronDown className="h-4 w-4 text-muted-foreground" />
														) : (
															<ChevronRight className="h-4 w-4 text-muted-foreground" />
														)}
														<h4 className="font-medium">
															Node {index + 1} {index < 2 ? '(Bootnode + Validator)' : '(Validator)'}
														</h4>
													</div>
													<div className="flex items-center gap-2">
														<span className="text-sm text-muted-foreground">Initial Balance:</span>
														<span className="font-mono text-sm bg-muted px-2 py-1 rounded">
															{(() => {
																if (allocation?.balance === '0x0') return '0 ETH'
																const weiValue = hexToNumber(allocation?.balance || '0x0')
																const ethValue = weiToEth(weiValue.toString())
																return Math.round(ethValue * 1e10) / 1e10 + ' ETH'
															})()}
														</span>
													</div>
												</div>
											</div>

											{isExpanded && (
												<div className="px-4 pb-4 space-y-4">
													{validatorKey && (
														<div className="text-sm text-muted-foreground">
															<div className="flex items-center gap-2">
																<span>Validator Address:</span>
																<code className="text-xs bg-muted px-2 py-1 rounded">
																	{validatorKey.ethereumAddress}
																</code>
																<Button
																	type="button"
																	variant="ghost"
																	size="sm"
																	className="h-6 w-6 p-0"
																	onClick={(e) => {
																		e.stopPropagation()
																		navigator.clipboard.writeText(validatorKey.ethereumAddress)
																		toast.success('Address copied to clipboard')
																	}}
																>
																	<Copy className="h-3 w-3" />
																</Button>
															</div>
														</div>
													)}
													<BesuNodeForm
														defaultValues={nodeConfigs[index] || defaultNodeConfig}
														onChange={(values) => {
															const newConfigs = [...nodeConfigs]
															newConfigs[index] = values
															setNodeConfigs(newConfigs)
														}}
														hideSubmit
														onSubmit={() => { }}
													/>
												</div>
											)}
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
									<Link to="/networks" onClick={clearLocalStorage}>Cancel</Link>
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
											<dt className="text-sm font-medium text-muted-foreground">Besu Version</dt>
											<dd className="mt-1">{nodesForm.getValues('version')}</dd>
										</div>
										<div>
											<dt className="text-sm font-medium text-muted-foreground">Chain ID</dt>
											<dd className="mt-1">{networkForm.getValues('chainId')}</dd>
										</div>
										<div>
											<dt className="text-sm font-medium text-muted-foreground">Nodes</dt>
											<dd className="mt-1">
												<div className="space-y-2">
													{nodeConfigs?.map((config, index) => {
														const nodeName = config?.name
														const result = nodeCreationResults[nodeName]
														// Show as created if we have attempted creation or have results
														const isCreated = hasAttemptedCreation || !!result

														return (
															<div key={index} className="flex items-center justify-between p-3 border rounded-lg">
																<div className="flex items-center gap-3">
																	<div className={`h-3 w-3 rounded-full shrink-0 ${!isCreated
																		? 'bg-muted'
																		: result?.status === 'success'
																			? result?.nodeStatus === 'ERROR'
																				? 'bg-red-500'
																				: result?.nodeStatus === 'RUNNING'
																					? 'bg-green-500'
																					: result?.nodeStatus === 'STOPPED'
																						? 'bg-yellow-500'
																						: 'bg-blue-500'
																			: result?.status === 'error'
																				? 'bg-red-500'
																				: 'bg-yellow-500 animate-pulse'
																		}`} />
																	<div>
																		<p className="font-medium">{nodeName}</p>
																		{result?.status === 'error' && (
																			<p className="text-sm text-red-600">{result.error}</p>
																		)}
																		{result?.status === 'success' && (
																			<>
																				{result?.nodeStatus === 'ERROR' && (
																					<p className="text-sm text-red-600">Node created but status is ERROR</p>
																				)}
																				{result?.nodeStatus === 'RUNNING' && (
																					<p className="text-sm text-green-600">Running successfully</p>
																				)}
																				{result?.nodeStatus === 'STOPPED' && (
																					<p className="text-sm text-yellow-600">Created successfully (Stopped)</p>
																				)}
																				{!result?.nodeStatus && (
																					<p className="text-sm text-blue-600">Created successfully (Checking status...)</p>
																				)}
																			</>
																		)}
																		{result?.status === 'pending' && (
																			<p className="text-sm text-yellow-600">Creating...</p>
																		)}
																	</div>
																</div>

																{isCreated && (result?.status === 'error' || result?.nodeStatus === 'ERROR') && (
																	<div className="flex items-center gap-2">
																		{result?.nodeStatus === 'ERROR' ? (
																			<Button
																				type="button"
																				size="sm"
																				variant="outline"
																				className="border-red-200 hover:bg-red-50"
																				onClick={() => deleteAndRecreateNode(nodeName)}
																				disabled={result?.status === 'pending'}
																			>
																				{result?.status === 'pending' ? (
																					<div className="h-3 w-3 rounded-full bg-current animate-pulse" />
																				) : (
																					<Trash2 className="h-3 w-3" />
																				)}
																				{result?.status === 'pending' ? 'Recreating...' : 'Delete & Recreate'}
																			</Button>
																		) : (
																			<Button
																				type="button"
																				size="sm"
																				variant="outline"
																				className="border-red-200 hover:bg-red-50"
																				onClick={() => retryNodeCreation(nodeName)}
																				disabled={result?.status === 'pending'}
																			>
																				{result?.status === 'pending' ? (
																					<div className="h-3 w-3 rounded-full bg-current animate-pulse" />
																				) : (
																					<RefreshCw className="h-3 w-3" />
																				)}
																				{result?.status === 'pending' ? 'Retrying...' : 'Retry'}
																			</Button>
																		)}
																		<Button
																			type="button"
																			size="sm"
																			variant="outline"
																			className="border-red-200 hover:bg-red-50 text-red-600"
																			onClick={() => removeFailedNode(nodeName)}
																			disabled={result?.status === 'pending'}
																		>
																			<Trash2 className="h-3 w-3" />
																			Remove
																		</Button>
																	</div>
																)}
															</div>
														)
													})}
												</div>
											</dd>
										</div>
									</dl>
								</div>

								{/* Error summary for failed nodes */}
								{failedNodes.length > 0 && (
									<Alert variant="destructive">
										<AlertCircle className="h-4 w-4" />
										<AlertTitle>Some nodes failed to create</AlertTitle>
										<AlertDescription>
											{failedNodes.length} of {nodeConfigs.length} nodes failed. You can retry individual nodes or retry all failed nodes using the buttons below.
										</AlertDescription>
									</Alert>
								)}

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
									<Link to="/networks" onClick={clearLocalStorage}>Cancel</Link>
								</Button>

								{/* Show additional buttons when there are failed nodes */}
								{failedNodes.length > 0 && creationProgress.total > 0 && (
									<>
										<Button type="button" variant="outline" onClick={retryAllFailedNodes}>
											<RefreshCw className="mr-2 h-4 w-4" />
											Retry Failed ({failedNodes.length})
										</Button>
										<Button type="button" onClick={continueWithSuccessful}>
											<CheckCircle2 className="mr-2 h-4 w-4" />
											Continue with Successful
										</Button>
									</>
								)}

								{/* Show create button only if not started or no failures */}
								{(creationProgress.total === 0 || failedNodes.length === 0) && (
									<Button type="button" onClick={onReviewStepSubmit}>
										<CheckCircle2 className="mr-2 h-4 w-4" />
										Create Network
									</Button>
								)}
							</div>
						</div>
					</div>
				)}
			</div>
		</div>
	)
}

import {
	getNodesDefaultsBesuNodeOptions,
	getNodesDefaultsFabricOrdererOptions,
	getNodesDefaultsFabricPeerOptions,
	getOrganizationsOptions,
	postNodesMutation,
} from '@/api/client/@tanstack/react-query.gen'
import { HttpCreateNodeRequest, TypesBlockchainPlatform, TypesFabricOrdererConfig, TypesFabricPeerConfig } from '@/api/client/types.gen'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { FormControl, FormField, FormItem, FormLabel } from '@/components/ui/form'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { useMutation, useQuery } from '@tanstack/react-query'
import { ChevronLeft, ChevronRight, Server } from 'lucide-react'
import { useState } from 'react'
import { FormProvider, useForm } from 'react-hook-form'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { BesuNodeForm } from './nodes/besu-node-form'
import { FabricNodeForm } from './nodes/fabric-node-form'
import { ProtocolSelector } from './protocol-selector'

interface NodeCreationForm {
	protocol: string
	nodeType?: string
	name: string
	configuration: {
		[key: string]: any
	}
}

interface StepProps {
	form: ReturnType<typeof useForm<NodeCreationForm>>
	onNext: () => void
	onBack?: () => void
}

function ProtocolStep({ form, onNext }: StepProps) {
	return (
		<div className="space-y-4">
			<div className="text-center mb-6">
				<h2 className="text-lg font-semibold">Select Protocol</h2>
				<p className="text-sm text-muted-foreground">Choose the blockchain protocol for your node and give it a name</p>
			</div>

			<FormField
				control={form.control}
				name="name"
				render={({ field }) => (
					<FormItem>
						<FormLabel>Node Name</FormLabel>
						<FormControl>
							<input
								{...field}
								className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
								placeholder="Enter node name"
							/>
						</FormControl>
					</FormItem>
				)}
			/>

			<ProtocolSelector control={form.control} name="protocol" />
			<div className="flex justify-end mt-6">
				<Button onClick={onNext} disabled={!form.getValues().protocol || !form.getValues().name}>
					Next
					<ChevronRight className="ml-2 h-4 w-4" />
				</Button>
			</div>
		</div>
	)
}

function NodeTypeStep({ form, onNext, onBack }: StepProps) {
	const protocol = form.getValues().protocol
	const nodeName = form.getValues().name

	const nodeTypes =
		protocol === 'fabric'
			? [
					{ id: 'peer', name: 'Peer Node', description: 'Maintains the ledger and state, and runs smart contracts' },
					{ id: 'orderer', name: 'Orderer Node', description: 'Orders transactions and creates blocks' },
			  ]
			: [{ id: 'full', name: 'Full Node', description: 'Maintains a complete copy of the blockchain' }]

	return (
		<div className="space-y-4">
			<div className="text-center mb-6">
				<h2 className="text-lg font-semibold">Select Node Type</h2>
				<p className="text-sm text-muted-foreground">Choose the type of node you want to create</p>
			</div>

			<div className="rounded-lg border p-4 mb-6">
				<dl className="space-y-2">
					<div className="grid grid-cols-3 gap-4">
						<dt className="text-sm font-medium text-muted-foreground">Blockchain</dt>
						<dd className="col-span-2 text-sm capitalize">{protocol}</dd>
					</div>
					<div className="grid grid-cols-3 gap-4">
						<dt className="text-sm font-medium text-muted-foreground">Node Name</dt>
						<dd className="col-span-2 text-sm">{nodeName}</dd>
					</div>
				</dl>
			</div>

			<FormField
				control={form.control}
				name="nodeType"
				render={({ field }) => (
					<FormItem>
						<FormControl>
							<RadioGroup onValueChange={field.onChange} value={field.value} className="grid gap-4">
								{nodeTypes.map((type) => (
									<label
										key={type.id}
										className={`flex items-start p-4 gap-4 border rounded-lg cursor-pointer hover:border-primary transition-colors ${
											field.value === type.id ? 'border-primary bg-primary/5' : ''
										}`}
									>
										<RadioGroupItem value={type.id} id={type.id} className="mt-1" />
										<div>
											<h3 className="font-medium">{type.name}</h3>
											<p className="text-sm text-muted-foreground">{type.description}</p>
										</div>
									</label>
								))}
							</RadioGroup>
						</FormControl>
					</FormItem>
				)}
			/>
			<div className="flex justify-between mt-6">
				<Button variant="outline" onClick={onBack}>
					<ChevronLeft className="mr-2 h-4 w-4" />
					Back
				</Button>
				<Button onClick={onNext} disabled={!form.getValues().nodeType}>
					Next
					<ChevronRight className="ml-2 h-4 w-4" />
				</Button>
			</div>
		</div>
	)
}

function ConfigurationStep({ form, onNext, onBack }: StepProps) {
	const protocol = form.getValues().protocol
	const nodeType = form.getValues().nodeType
	const nodeName = form.getValues().name

	// Fabric queries
	const { data: organizations } = useQuery({
		...getOrganizationsOptions(),
		enabled: protocol === 'fabric',
	})

	const { data: peerDefaults } = useQuery({
		...getNodesDefaultsFabricPeerOptions(),
		enabled: protocol === 'fabric' && nodeType === 'peer',
	})

	const { data: ordererDefaults } = useQuery({
		...getNodesDefaultsFabricOrdererOptions(),
		enabled: protocol === 'fabric' && nodeType === 'orderer',
	})
	console.log('protocol', protocol)
	// Besu queries
	const { data: besuDefaults } = useQuery({
		...getNodesDefaultsBesuNodeOptions({
			query: { besuNodes: 1 },
		}),
		enabled: protocol === 'besu',
	})
	console.log('besuDefaults', besuDefaults)

	const handleFabricSubmit = (data: any) => {
		const organization = organizations?.find((org) => org.id === data.fabricProperties.organizationId)

		if (data.fabricProperties.nodeType === 'FABRIC_PEER') {
			form.setValue('configuration', {
				nodeType: 'FABRIC_PEER',
				mode: data.fabricProperties.mode,
				organizationId: data.fabricProperties.organizationId,
				listenAddress: data.fabricProperties.listenAddress,
				operationsListenAddress: data.fabricProperties.operationsListenAddress,
				externalEndpoint: data.fabricProperties.externalEndpoint,
				domainNames: data.fabricProperties.domains || [],
				name: data.name,
				chaincodeAddress: data.fabricProperties.chaincodeAddress || '',
				eventsAddress: data.fabricProperties.eventsAddress || '',
				mspId: organization?.mspId || '',
				version: data.fabricProperties.version || '2.5.12',
			})
		} else {
			form.setValue('configuration', {
				nodeType: 'FABRIC_ORDERER',
				mode: data.fabricProperties.mode,
				organizationId: data.fabricProperties.organizationId,
				listenAddress: data.fabricProperties.listenAddress,
				operationsListenAddress: data.fabricProperties.operationsListenAddress,
				externalEndpoint: data.fabricProperties.externalEndpoint,
				domainNames: data.fabricProperties.domains || [],
				name: data.name,
				adminAddress: data.fabricProperties.adminAddress || '',
				mspId: organization?.mspId || '',
				version: data.fabricProperties.version || '2.5.12',
			})
		}

		form.setValue('name', data.name)
		onNext()
	}

	const handleBesuSubmit = (data: any) => {
		form.setValue('name', data.name)
		form.setValue('configuration', {
			...data,
			bootNodes: data.bootNodes
				?.split('\n')
				.map((node: string) => node.trim())
				.filter(Boolean),
			env: data.environmentVariables?.reduce(
				(acc: any, { key, value }: any) => ({
					...acc,
					[key]: value,
				}),
				{}
			),
			blockchainPlatform: 'BESU',
			type: 'besu',
		})
		onNext()
	}

	return (
		<div className="space-y-4">
			<div className="text-center mb-6">
				<h2 className="text-lg font-semibold">Configure Node</h2>
				<p className="text-sm text-muted-foreground">Set up the configuration for your node</p>
			</div>

			<div className="rounded-lg border p-4 mb-6">
				<dl className="space-y-2">
					<div className="grid grid-cols-3 gap-4">
						<dt className="text-sm font-medium text-muted-foreground">Blockchain</dt>
						<dd className="col-span-2 text-sm capitalize">{protocol}</dd>
					</div>
					<div className="grid grid-cols-3 gap-4">
						<dt className="text-sm font-medium text-muted-foreground">Node Type</dt>
						<dd className="col-span-2 text-sm capitalize">{nodeType}</dd>
					</div>
					<div className="grid grid-cols-3 gap-4">
						<dt className="text-sm font-medium text-muted-foreground">Node Name</dt>
						<dd className="col-span-2 text-sm">{nodeName}</dd>
					</div>
				</dl>
			</div>

			{protocol === 'fabric' && (
				<FabricNodeForm
					onSubmit={handleFabricSubmit}
					isSubmitting={false}
					hideSubmit={false}
					hideNodeType={true}
					organizations={organizations?.map((org) => ({ id: org.id!, name: org.mspId! })) || []}
					defaults={nodeType === 'peer' ? peerDefaults : ordererDefaults}
					defaultValues={
						form.getValues().configuration && Object.keys(form.getValues().configuration).length > 0
							? {
									name: form.getValues().name,
									fabricProperties: {
										...form.getValues().configuration,
									},
								}
							: {
									name: form.getValues().name,
									fabricProperties: {
										nodeType: nodeType === 'peer' ? 'FABRIC_PEER' : 'FABRIC_ORDERER',
										mode: 'service',
										version: '3.1.0',
										organizationId: organizations?.[0]?.id || 0,
										listenAddress: '',
										operationsListenAddress: '',
										externalEndpoint: '',
										domains: [],
										addressOverrides: [],
									},
								}
					}
					submitText="Next"
				/>
			)}
			{protocol === 'besu' && besuDefaults && besuDefaults.defaults && besuDefaults.defaults[0] && (
				<BesuNodeForm
					onSubmit={handleBesuSubmit}
					isSubmitting={false}
					hideSubmit={false}
					defaultValues={
						form.getValues().configuration && Object.keys(form.getValues().configuration).length > 0
							? {
									...form.getValues().configuration,
									name: form.getValues().name,
								}
							: {
									name: form.getValues().name,
									blockchainPlatform: 'BESU',
									type: 'besu',
									mode: 'service',
									rpcHost: besuDefaults?.defaults?.[0]?.rpcHost?.split(':')[0] || '0.0.0.0',
									rpcPort: besuDefaults?.defaults?.[0]?.rpcPort || 8545,
									p2pHost: besuDefaults?.defaults?.[0]?.p2pHost?.split(':')[0] || '0.0.0.0',
									p2pPort: besuDefaults?.defaults?.[0]?.p2pPort || 30303,
									externalIp: besuDefaults?.defaults?.[0]?.externalIp || '0.0.0.0',
									internalIp: besuDefaults?.defaults?.[0]?.internalIp || '0.0.0.0',
									keyId: 0,
									networkId: 1,
									requestTimeout: 30,
									environmentVariables: [],
								}
					}
					submitButtonText="Next"
				/>
			)}

			<div className="flex justify-between mt-6">
				<Button variant="outline" onClick={onBack}>
					<ChevronLeft className="mr-2 h-4 w-4" />
					Back
				</Button>
			</div>
		</div>
	)
}

function ReviewStep({ form, onBack }: StepProps) {
	const navigate = useNavigate()
	const formData = form.getValues()

	const createNode = useMutation({
		...postNodesMutation(),
		onSuccess: (response) => {
			toast.success('Node created successfully')
			navigate(`/nodes/${response.id}`)
		},
		onError: (error: any) => {
			toast.error('Failed to create node', {
				description: error.error?.message || error.message || 'An unknown error occurred',
			})
		},
	})

	const handleCreate = () => {
		const protocol = formData.protocol.toUpperCase() as TypesBlockchainPlatform
		let createNodeDto: HttpCreateNodeRequest

		if (protocol === 'FABRIC') {
			// Get organization from configuration
			const config = formData.configuration
			let fabricPeer: TypesFabricPeerConfig | undefined
			let fabricOrderer: TypesFabricOrdererConfig | undefined

			if (config.nodeType === 'FABRIC_PEER') {
				fabricPeer = {
					nodeType: 'FABRIC_PEER',
					mode: config.mode,
					organizationId: config.organizationId,
					listenAddress: config.listenAddress,
					operationsListenAddress: config.operationsListenAddress,
					externalEndpoint: config.externalEndpoint,
					domainNames: config.domainNames || [],
					name: formData.name,
					chaincodeAddress: config.chaincodeAddress || '',
					eventsAddress: config.eventsAddress || '',
					mspId: config.mspId || '',
					version: config.version || '2.5.12',
				} as TypesFabricPeerConfig
			} else {
				fabricOrderer = {
					nodeType: 'FABRIC_ORDERER',
					mode: config.mode,
					organizationId: config.organizationId,
					listenAddress: config.listenAddress,
					operationsListenAddress: config.operationsListenAddress,
					externalEndpoint: config.externalEndpoint,
					domainNames: config.domainNames || [],
					name: formData.name,
					adminAddress: config.adminAddress || '',
					mspId: config.mspId || '',
					version: config.version || '2.5.12',
				} as TypesFabricOrdererConfig
			}

			createNodeDto = {
				name: formData.name,
				blockchainPlatform: protocol,
				fabricPeer,
				fabricOrderer,
			}
		} else if (protocol === 'BESU') {
			const config = formData.configuration
			createNodeDto = {
				name: formData.name,
				blockchainPlatform: protocol,
				besuNode: {
					...config,
					externalIp: config.externalIp || '0.0.0.0',
					internalIp: config.internalIp || '0.0.0.0',

					bootNodes: config.bootNodes
						?.split('\n')
						.map((node: string) => node.trim())
						.filter(Boolean),
					env: config.environmentVariables?.reduce(
						(acc: any, { key, value }: any) => ({
							...acc,
							[key]: value,
						}),
						{}
					),
					keyId: config.keyId || '',
					networkId: config.networkId || '',
					p2pHost: config.p2pHost || '',
					p2pPort: config.p2pPort || 0,
					type: 'besu',
					rpcHost: config.rpcHost || '',
					rpcPort: config.rpcPort || 0,
				},
			}
		} else {
			throw new Error(`Unsupported blockchain platform: ${protocol}`)
		}
		createNode.mutate({
			body: createNodeDto,
		})
	}

	return (
		<div className="space-y-4">
			<div className="text-center mb-6">
				<h2 className="text-lg font-semibold">Review Configuration</h2>
				<p className="text-sm text-muted-foreground">Review your node configuration before creation</p>
			</div>
			<div className="space-y-4">
				<div className="rounded-lg border p-4">
					<dl className="space-y-4">
						<div className="grid grid-cols-3 gap-4">
							<dt className="text-sm font-medium text-muted-foreground">Protocol</dt>
							<dd className="col-span-2 text-sm capitalize">{formData.protocol}</dd>
						</div>
						<div className="grid grid-cols-3 gap-4">
							<dt className="text-sm font-medium text-muted-foreground">Node Type</dt>
							<dd className="col-span-2 text-sm capitalize">{formData.nodeType}</dd>
						</div>
						<div className="grid grid-cols-3 gap-4">
							<dt className="text-sm font-medium text-muted-foreground">Name</dt>
							<dd className="col-span-2 text-sm">{formData.name}</dd>
						</div>
						{Object.entries(formData.configuration || {}).map(([key, value]) => {
							// Skip rendering objects or arrays directly to prevent recursion
							// and potential rendering issues
							let displayValue

							if (value === undefined || value === null || value === '') {
								displayValue = <span className="text-muted-foreground italic">No value provided</span>
							} else if (typeof value === 'object') {
								displayValue = JSON.stringify(value)
							} else {
								displayValue = String(value)
							}

							return (
								<div key={key} className="grid grid-cols-3 gap-4">
									<dt className="text-sm font-medium text-muted-foreground">{key}</dt>
									<dd className="col-span-2 text-sm">{displayValue}</dd>
								</div>
							)
						})}
					</dl>
				</div>
			</div>
			<div className="flex justify-between mt-6">
				<Button variant="outline" onClick={onBack}>
					<ChevronLeft className="mr-2 h-4 w-4" />
					Back
				</Button>
				<Button onClick={handleCreate} disabled={createNode.isPending}>
					<Server className="mr-2 h-4 w-4" />
					{createNode.isPending ? 'Creating...' : 'Create Node'}
				</Button>
			</div>
		</div>
	)
}

export function NodeCreationWizard() {
	const [step, setStep] = useState(0)
	const form = useForm<NodeCreationForm>({
		defaultValues: {
			protocol: '',
			nodeType: '',
			name: '',
			configuration: {},
		},
	})

	const steps = [
		{
			component: ProtocolStep,
			title: 'Select Protocol',
		},
		{
			component: NodeTypeStep,
			title: 'Node Type',
		},
		{
			component: ConfigurationStep,
			title: 'Configuration',
		},
		{
			component: ReviewStep,
			title: 'Review',
		},
	]

	const CurrentStep = steps[step].component

	return (
		<div className="max-w-4xl mx-auto">
			<div className="mb-8">
				<div className="flex items-center justify-center space-x-12">
					{steps.map((_, i) => (
						<div key={i} className={`flex items-center ${i < steps.length - 1 ? 'after:content-[""] after:block after:w-24 after:h-px after:bg-border after:ml-12' : ''}`}>
							<div
								className={`flex items-center justify-center w-8 h-8 rounded-full border-2 ${
									i === step ? 'border-primary bg-primary text-primary-foreground' : i < step ? 'border-primary text-primary' : 'border-muted-foreground text-muted-foreground'
								}`}
							>
								{i + 1}
							</div>
						</div>
					))}
				</div>
			</div>

			<Card className="p-6">
				<FormProvider {...form}>
					<CurrentStep form={form} onNext={() => setStep(step + 1)} onBack={() => setStep(step - 1)} />
				</FormProvider>
			</Card>
		</div>
	)
}

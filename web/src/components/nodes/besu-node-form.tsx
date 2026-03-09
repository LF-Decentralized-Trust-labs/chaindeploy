import { getKeysOptions, getNetworksBesuOptions, getNodesDefaultsBesuNodeOptions } from '@/api/client/@tanstack/react-query.gen'
import { SingleKeySelect } from '@/components/networks/single-key-select'
import { Button } from '@/components/ui/button'
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'

// Besu versions from https://github.com/hyperledger/besu/releases
const BESU_VERSIONS = [
	'25.7.0',
	'25.6.0',
	'25.5.0',
	'25.4.0',
	'25.3.0',
	'25.2.0',
	'25.1.0',
	'24.12.2',
]

const besuNodeFormSchema = z.object({
	name: z.string().min(3, 'Node name must be at least 3 characters').max(50, 'Node name must be less than 50 characters'),
	blockchainPlatform: z.literal('BESU'),
	externalIp: z.string().ip('Please enter a valid IP address'),
	internalIp: z.string().ip('Please enter a valid IP address'),
	keyId: z.number().positive('Please select a valid key'),
	networkId: z.number().positive('Please select a valid network'),
	mode: z.enum(['service', 'docker']).default('service'),
	p2pHost: z.string().min(1, 'P2P host is required'),
	p2pPort: z.number().min(1024, 'Port must be between 1024 and 65535').max(65535, 'Port must be between 1024 and 65535'),
	rpcHost: z.string().min(1, 'RPC host is required'),
	rpcPort: z.number().min(1024, 'Port must be between 1024 and 65535').max(65535, 'Port must be between 1024 and 65535'),
	metricsEnabled: z.boolean().default(true),
	metricsHost: z.string().default('127.0.0.1'),
	metricsPort: z.number().min(1024, 'Port must be between 1024 and 65535').max(65535, 'Port must be between 1024 and 65535').optional(),
	type: z.literal('besu'),
	bootNodes: z.union([
		z.array(z.string()),
		z.string()
	]).optional().transform((val) => {
		if (typeof val === 'string') {
			return val.split(/[,\s]+/).map(node => node.trim()).filter(Boolean)
		}
		return val || []
	}),
	requestTimeout: z.number().positive('Request timeout must be a positive number'),
	version: z.string().min(1, 'Version is required').default('25.7.0'),
	environmentVariables: z
		.array(
			z.object({
				key: z.string().min(1, 'Environment variable key is required'),
				value: z.string(),
			})
		)
		.optional(),
	// Gas and access control configuration
	minGasPrice: z.number().min(0, 'Minimum gas price must be 0 or greater').optional(),
	hostAllowList: z.string().optional(),
	// Permissions configuration
	accountsAllowList: z.union([
		z.array(z.string()),
		z.string()
	]).optional().transform((val) => {
		if (typeof val === 'string') {
			return val.split(/[,\s]+/).map(acc => acc.trim()).filter(Boolean)
		}
		return val || []
	}),
	nodesAllowList: z.union([
		z.array(z.string()),
		z.string()
	]).optional().transform((val) => {
		if (typeof val === 'string') {
			return val.split(/[,\s]+/).map(node => node.trim()).filter(Boolean)
		}
		return val || []
	}),
	// JWT Authentication configuration
	jwtEnabled: z.boolean().default(false),
	jwtPublicKeyContent: z.string().optional(),
	jwtAuthenticationAlgorithm: z.string().optional(),
}).refine((data) => {
	// If JWT is enabled, require JWT-related fields
	if (data.jwtEnabled) {
		if (!data.jwtPublicKeyContent || !data.jwtAuthenticationAlgorithm) {
			return false
		}
	}
	return true
}, {
	message: "JWT public key content and algorithm are required when JWT is enabled",
	path: ["jwtPublicKeyContent"]
})

export type BesuNodeFormValues = z.infer<typeof besuNodeFormSchema>

interface BesuNodeFormProps {
	onSubmit: (data: BesuNodeFormValues) => void
	isSubmitting?: boolean
	hideSubmit?: boolean
	defaultValues?: BesuNodeFormValues
	onChange?: (values: BesuNodeFormValues) => void
	networkId?: number
	submitButtonText?: string
	submitButtonLoadingText?: string
}

export function BesuNodeForm({ onSubmit, isSubmitting, hideSubmit, defaultValues, onChange, networkId, submitButtonText = 'Create Node', submitButtonLoadingText = 'Creating...' }: BesuNodeFormProps) {
	const form = useForm<BesuNodeFormValues>({
		resolver: zodResolver(besuNodeFormSchema),
		defaultValues: {
			blockchainPlatform: 'BESU',
			type: 'besu',
			mode: 'service',
			rpcHost: '127.0.0.1',
			rpcPort: 8545,
			p2pPort: 30303,
			metricsEnabled: true,
			metricsHost: '127.0.0.1',
			metricsPort: 9545,
			requestTimeout: 30,
			version: '25.7.0',
			environmentVariables: [],
			networkId: networkId,
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
			...defaultValues,
		},
		mode: 'onChange',
	})

	const { data: besuDefaultConfig } = useQuery(getNodesDefaultsBesuNodeOptions())
	const { data: networks } = useQuery(getNetworksBesuOptions({}))
	const { data: keys } = useQuery(getKeysOptions({}))

	const { errors } = form.formState

	useEffect(() => {
		// Set form values from defaultValues if they exist
		if (defaultValues) {
			Object.entries(defaultValues).forEach(([key, value]) => {
				if (value !== undefined) {
					form.setValue(key as keyof BesuNodeFormValues, value)
				}
			})
		}
	}, [defaultValues, form.setValue])

	useEffect(() => {
		if (besuDefaultConfig && !defaultValues) {
			const { p2pHost, p2pPort, rpcHost, rpcPort, externalIp, internalIp } = besuDefaultConfig.defaults![0]
			form.setValue('p2pHost', p2pHost || '127.0.0.1')
			form.setValue('p2pPort', Number(p2pPort) || 30303)
			form.setValue('externalIp', externalIp || '127.0.0.1')
			form.setValue('internalIp', internalIp || '127.0.0.1')
			form.setValue('rpcHost', rpcHost || '127.0.0.1')
			form.setValue('rpcPort', Number(rpcPort) || 8545)
		}
	}, [besuDefaultConfig, defaultValues, form.setValue])

	useEffect(() => {
		if (networkId) {
			form.setValue('networkId', networkId)
		}
	}, [networkId, form.setValue])

	// Debounce the onChange callback to prevent too many updates
	useEffect(() => {
		const subscription = form.watch((value) => {
			const timeoutId = setTimeout(() => {
				onChange?.(value as BesuNodeFormValues)
			}, 100)
			return () => clearTimeout(timeoutId)
		})
		return () => subscription.unsubscribe()
	}, [form.watch, onChange])

	// Enhanced form submission handler with validation
	const handleFormSubmit = async (data: BesuNodeFormValues) => {
		if (typeof onSubmit !== 'function') {
			return
		}

		const isValid = await form.trigger()
		if (!isValid) {
			return
		}

		try {
			await onSubmit(data)
		} catch (error) {
			console.error('Form submission failed:', error)
		}
	}

	return (
		<Form {...form}>
			<form onSubmit={form.handleSubmit(handleFormSubmit)} className="space-y-6">
				<FormField
					control={form.control}
					name="name"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Node Name</FormLabel>
							<FormControl>
								<Input placeholder="Enter node name" {...field} />
							</FormControl>
							<FormDescription>A unique identifier for your node</FormDescription>
							<FormMessage />
						</FormItem>
					)}
				/>

				{!networkId && (
					<FormField
						control={form.control}
						name="networkId"
						render={({ field }) => (
							<FormItem>
								<FormLabel>Network</FormLabel>
								<Select onValueChange={(value) => field.onChange(Number(value))} value={field.value?.toString()}>
									<FormControl>
										<SelectTrigger>
											<SelectValue placeholder="Select network" />
										</SelectTrigger>
									</FormControl>
									<SelectContent>
										{networks?.networks?.map((network) => (
											<SelectItem key={network.id} value={network.id!.toString()}>
												{network.name}
											</SelectItem>
										))}
									</SelectContent>
								</Select>
								<FormMessage />
							</FormItem>
						)}
					/>
				)}

				<FormField
					control={form.control}
					name="keyId"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Key</FormLabel>
							<FormControl>
								<SingleKeySelect keys={keys?.items ?? []} value={field.value} onChange={field.onChange} />
							</FormControl>
							<FormMessage />
						</FormItem>
					)}
				/>

				<div className="grid grid-cols-2 gap-4">
					<FormField
						control={form.control}
						name="externalIp"
						render={({ field }) => (
							<FormItem>
								<FormLabel>External IP</FormLabel>
								<FormControl>
									<Input {...field} placeholder="0.0.0.0" />
								</FormControl>
								<FormMessage />
							</FormItem>
						)}
					/>

					<FormField
						control={form.control}
						name="internalIp"
						render={({ field }) => (
							<FormItem>
								<FormLabel>Internal IP</FormLabel>
								<FormControl>
									<Input {...field} placeholder="0.0.0.0" />
								</FormControl>
								<FormMessage />
							</FormItem>
						)}
					/>
				</div>

				<FormField
					control={form.control}
					name="mode"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Mode</FormLabel>
							<Select onValueChange={field.onChange} defaultValue={field.value}>
								<FormControl>
									<SelectTrigger>
										<SelectValue placeholder="Select mode" />
									</SelectTrigger>
								</FormControl>
								<SelectContent>
									<SelectItem value="docker">Docker</SelectItem>
									<SelectItem value="service">Service</SelectItem>
								</SelectContent>
							</Select>
							<FormMessage />
						</FormItem>
					)}
				/>

				<FormField
					control={form.control}
					name="version"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Version</FormLabel>
							<Select onValueChange={field.onChange} value={field.value}>
								<FormControl>
									<SelectTrigger>
										<SelectValue placeholder="Select Besu version" />
									</SelectTrigger>
								</FormControl>
								<SelectContent>
									{BESU_VERSIONS.map((version) => (
										<SelectItem key={version} value={version}>
											{version}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
							<FormDescription>Besu version to use for this node</FormDescription>
							<FormMessage />
						</FormItem>
					)}
				/>

				<div className="grid grid-cols-2 gap-4">
					<FormField
						control={form.control}
						name="p2pHost"
						render={({ field }) => (
							<FormItem>
								<FormLabel>P2P Host</FormLabel>
								<FormControl>
									<Input {...field} />
								</FormControl>
								<FormMessage />
							</FormItem>
						)}
					/>

					<FormField
						control={form.control}
						name="p2pPort"
						render={({ field }) => (
							<FormItem>
								<FormLabel>P2P Port</FormLabel>
								<FormControl>
									<Input type="number" {...field} onChange={(e) => field.onChange(Number(e.target.value))} />
								</FormControl>
								<FormMessage />
							</FormItem>
						)}
					/>
				</div>

				<div className="grid grid-cols-2 gap-4">
					<FormField
						control={form.control}
						name="rpcHost"
						render={({ field }) => (
							<FormItem>
								<FormLabel>RPC Host</FormLabel>
								<FormControl>
									<Input {...field} />
								</FormControl>
								<FormMessage />
							</FormItem>
						)}
					/>

					<FormField
						control={form.control}
						name="rpcPort"
						render={({ field }) => (
							<FormItem>
								<FormLabel>RPC Port</FormLabel>
								<FormControl>
									<Input type="number" {...field} onChange={(e) => field.onChange(Number(e.target.value))} />
								</FormControl>
								<FormMessage />
							</FormItem>
						)}
					/>
				</div>

				<FormField
					control={form.control}
					name="metricsEnabled"
					render={({ field }) => (
						<FormItem className="flex flex-row items-center justify-between rounded-lg border p-4">
							<div className="space-y-0.5">
								<FormLabel className="text-base">Enable Metrics</FormLabel>
								<FormDescription>
									Enable Prometheus metrics collection for this node
								</FormDescription>
							</div>
							<FormControl>
								<Switch
									checked={field.value}
									onCheckedChange={field.onChange}
								/>
							</FormControl>
						</FormItem>
					)}
				/>

				{form.watch('metricsEnabled') && (
					<div className="grid grid-cols-2 gap-4">
						<FormField
							control={form.control}
							name="metricsHost"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Metrics Host</FormLabel>
									<FormControl>
										<Input {...field} placeholder="127.0.0.1" />
									</FormControl>
									<FormDescription>Host for Prometheus metrics endpoint</FormDescription>
									<FormMessage />
								</FormItem>
							)}
						/>

						<FormField
							control={form.control}
							name="metricsPort"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Metrics Port</FormLabel>
									<FormControl>
										<Input type="number" {...field} onChange={(e) => field.onChange(Number(e.target.value))} />
									</FormControl>
									<FormDescription>Port for Prometheus metrics endpoint</FormDescription>
									<FormMessage />
								</FormItem>
							)}
						/>
					</div>
				)}

				<FormField
					control={form.control}
					name="bootNodes"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Boot Nodes</FormLabel>
							<FormControl>
								<Input 
									{...field} 
									value={Array.isArray(field.value) ? field.value.join(', ') : field.value || ''}
									onChange={(e) => {
										const inputValue = e.target.value
										field.onChange(inputValue)
									}}
									onBlur={(e) => {
										const inputValue = e.target.value
										const bootNodes = inputValue.split(/[,\s]+/).map(node => node.trim()).filter(Boolean)
										field.onChange(bootNodes)
									}}
									placeholder="Enter boot nodes (comma-separated)" 
								/>
							</FormControl>
							<FormDescription>Comma-separated list of boot node URLs</FormDescription>
							<FormMessage />
						</FormItem>
					)}
				/>

				<FormField
					control={form.control}
					name="minGasPrice"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Minimum Gas Price</FormLabel>
							<FormControl>
								<Input 
									type="number" 
									{...field} 
									onChange={(e) => field.onChange(Number(e.target.value))}
									placeholder="0" 
								/>
							</FormControl>
							<FormDescription>Minimum gas price in wei for transactions</FormDescription>
							<FormMessage />
						</FormItem>
					)}
				/>

				<FormField
					control={form.control}
					name="hostAllowList"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Host Allow List</FormLabel>
							<FormControl>
								<Input {...field} placeholder="Enter allowed hosts (comma-separated)" />
							</FormControl>
							<FormDescription>Comma-separated list of allowed hostnames or IP addresses</FormDescription>
							<FormMessage />
						</FormItem>
					)}
				/>

				<FormField
					control={form.control}
					name="accountsAllowList"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Accounts Allow List</FormLabel>
							<FormControl>
								<Input 
									{...field} 
									value={Array.isArray(field.value) ? field.value.join(', ') : field.value || ''}
									onChange={(e) => {
										const inputValue = e.target.value
										field.onChange(inputValue)
									}}
									onBlur={(e) => {
										const inputValue = e.target.value
										const accounts = inputValue.split(/[,\s]+/).map(acc => acc.trim()).filter(Boolean)
										field.onChange(accounts)
									}}
									placeholder="Enter allowed account addresses (comma-separated)" 
								/>
							</FormControl>
							<FormDescription>Comma-separated list of allowed account addresses</FormDescription>
							<FormMessage />
						</FormItem>
					)}
				/>

				<FormField
					control={form.control}
					name="nodesAllowList"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Nodes Allow List</FormLabel>
							<FormControl>
								<Input 
									{...field} 
									value={Array.isArray(field.value) ? field.value.join(', ') : field.value || ''}
									onChange={(e) => {
										const inputValue = e.target.value
										field.onChange(inputValue)
									}}
									onBlur={(e) => {
										const inputValue = e.target.value
										const nodes = inputValue.split(/[,\s]+/).map(node => node.trim()).filter(Boolean)
										field.onChange(nodes)
									}}
									placeholder="Enter allowed node IDs (comma-separated)" 
								/>
							</FormControl>
							<FormDescription>Comma-separated list of allowed node IDs</FormDescription>
							<FormMessage />
						</FormItem>
					)}
				/>

				<FormField
					control={form.control}
					name="jwtEnabled"
					render={({ field }) => (
						<FormItem className="flex flex-row items-center justify-between rounded-lg border p-4">
							<div className="space-y-0.5">
								<FormLabel className="text-base">Enable JWT Authentication</FormLabel>
								<FormDescription>
									Enable JWT-based authentication for this node
								</FormDescription>
							</div>
							<FormControl>
								<Switch
									checked={field.value}
									onCheckedChange={field.onChange}
								/>
							</FormControl>
						</FormItem>
					)}
				/>

				{form.watch('jwtEnabled') && (
					<>
						<FormField
							control={form.control}
							name="jwtPublicKeyContent"
							render={({ field }) => (
								<FormItem>
									<FormLabel>JWT Public Key Content</FormLabel>
									<FormControl>
										<Input {...field} placeholder="Enter JWT public key content" />
									</FormControl>
									<FormDescription>Public key content for JWT verification</FormDescription>
									<FormMessage />
								</FormItem>
							)}
						/>

						<FormField
							control={form.control}
							name="jwtAuthenticationAlgorithm"
							render={({ field }) => (
								<FormItem>
									<FormLabel>JWT Authentication Algorithm</FormLabel>
									<FormControl>
										<Input {...field} placeholder="e.g., RS256, ES256" />
									</FormControl>
									<FormDescription>Algorithm used for JWT authentication</FormDescription>
									<FormMessage />
								</FormItem>
							)}
						/>
					</>
				)}

				{!hideSubmit && (
					<div className="space-y-4">
						{Object.keys(errors).length > 0 && (
							<div className="p-4 bg-red-50 border border-red-200 rounded-lg dark:bg-red-950 dark:border-red-800">
								<h4 className="text-sm font-medium text-red-800 dark:text-red-200 mb-2">Please fix the following errors:</h4>
								<ul className="text-sm text-red-700 dark:text-red-300 space-y-1">
									{Object.entries(errors).map(([fieldName, error]) => (
										<li key={fieldName}>
											<strong className="capitalize">{fieldName.replace(/([A-Z])/g, ' $1').trim()}:</strong> {error?.message || 'Invalid value'}
										</li>
									))}
								</ul>
							</div>
						)}

						<Button
							type="submit"
							disabled={isSubmitting}
							onClick={async () => {
								await form.trigger()
							}}
						>
							{isSubmitting ? submitButtonLoadingText : submitButtonText}
						</Button>
					</div>
				)}
			</form>
		</Form>
	)
}

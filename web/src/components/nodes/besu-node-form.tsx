import { getKeysOptions, getNetworksBesuOptions, getNodesDefaultsBesuNodeOptions } from '@/api/client/@tanstack/react-query.gen'
import { SingleKeySelect } from '@/components/networks/single-key-select'
import { Button } from '@/components/ui/button'
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'

const besuNodeFormSchema = z.object({
	name: z.string().min(3).max(50),
	blockchainPlatform: z.literal('BESU'),
	externalIp: z.string().ip(),
	internalIp: z.string().ip(),
	keyId: z.number(),
	networkId: z.number().positive(),
	mode: z.enum(['service', 'docker']).default('service'),
	p2pHost: z.string(),
	p2pPort: z.number().min(1024).max(65535),
	rpcHost: z.string(),
	rpcPort: z.number().min(1024).max(65535),
	metricsHost: z.string().default('127.0.0.1'),
	metricsPort: z.number().min(1024).max(65535).optional(),
	type: z.literal('besu'),
	bootNodes: z.string().optional(),
	requestTimeout: z.number().positive(),
	environmentVariables: z
		.array(
			z.object({
				key: z.string(),
				value: z.string(),
			})
		)
		.optional(),
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
}

export function BesuNodeForm({ onSubmit, isSubmitting, hideSubmit, defaultValues, onChange, networkId, submitButtonText = 'Create Node' }: BesuNodeFormProps) {
	const form = useForm<BesuNodeFormValues>({
		resolver: zodResolver(besuNodeFormSchema),
		defaultValues: {
			blockchainPlatform: 'BESU',
			type: 'besu',
			mode: 'service',
			rpcHost: '127.0.0.1',
			rpcPort: 8545,
			p2pPort: 30303,
			metricsHost: '127.0.0.1',
			metricsPort: 9545,
			requestTimeout: 30,
			environmentVariables: [],
			networkId: networkId,
			...defaultValues,
		},
	})

	const { data: besuDefaultConfig } = useQuery(getNodesDefaultsBesuNodeOptions())
	const { data: networks } = useQuery(getNetworksBesuOptions({}))
	const { data: keys } = useQuery(getKeysOptions({}))
	useEffect(() => {
		// Set form values from defaultValues if they exist
		if (defaultValues) {
			// Use Object.entries to iterate through all properties of defaultValues
			Object.entries(defaultValues).forEach(([key, value]) => {
				// Only set the value if it's defined
				if (value !== undefined) {
					form.setValue(key as keyof BesuNodeFormValues, value)
				}
			})
		}
	}, [defaultValues])
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

	return (
		<Form {...form}>
			<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
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

				<FormField
					control={form.control}
					name="bootNodes"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Boot Nodes</FormLabel>
							<FormControl>
								<Input {...field} placeholder="Enter boot nodes (comma-separated)" />
							</FormControl>
							<FormDescription>Comma-separated list of boot node URLs</FormDescription>
							<FormMessage />
						</FormItem>
					)}
				/>

				{!hideSubmit && (
					<Button type="submit" disabled={isSubmitting}>
						{isSubmitting ? 'Creating...' : submitButtonText}
					</Button>
				)}
			</form>
		</Form>
	)
}

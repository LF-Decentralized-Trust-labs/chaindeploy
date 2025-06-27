import { GetNodesDefaultsFabricOrdererResponse, GetNodesDefaultsFabricPeerResponse } from '@/api/client'
import { isValidPEMCertificate } from '@/components/../lib/utils'
import { Button } from '@/components/ui/button'
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { zodResolver } from '@hookform/resolvers/zod'
import { Trash2 } from 'lucide-react'
import { useEffect } from 'react'
import { useForm, useWatch } from 'react-hook-form'
import { toast } from 'sonner'
import * as z from 'zod'

const addressPortRegex = /^[^:]+:\d+$/
const addressPortMessage = 'Must be in the format address:port (e.g., 0.0.0.0:7051)'

const fabricNodeFormSchema = z.object({
	name: z.string().min(1, 'Name is required'),
	fabricProperties: z
		.object({
			nodeType: z.enum(['FABRIC_PEER', 'FABRIC_ORDERER']),
			mode: z.enum(['docker', 'service']),
			version: z.string(),
			organizationId: z.number(),
			listenAddress: z.string().regex(addressPortRegex, { message: addressPortMessage }),
			operationsListenAddress: z.string().regex(addressPortRegex, { message: addressPortMessage }),
			externalEndpoint: z.string().regex(addressPortRegex, { message: addressPortMessage }),
			domains: z.array(z.string()).optional(),
			chaincodeAddress: z.string().regex(addressPortRegex, { message: addressPortMessage }).optional(),
			eventsAddress: z.string().regex(addressPortRegex, { message: addressPortMessage }).optional(),
			adminAddress: z.string().regex(addressPortRegex, { message: addressPortMessage }).optional(),
			addressOverrides: z
				.array(
					z.object({
						from: z.string().regex(addressPortRegex, { message: addressPortMessage }),
						to: z.string().regex(addressPortRegex, { message: addressPortMessage }),
						tlsCACert: z.string().refine(
							(value) => {
								if (!value) return true // allow empty, handled elsewhere if required
								return isValidPEMCertificate(value)
							},
							{ message: 'Must be a valid PEM-encoded certificate' }
						),
					})
				)
				.optional(),
		})
		.refine(
			(data) => {
				if (data.nodeType === 'FABRIC_PEER') {
					return !!(data.chaincodeAddress && data.eventsAddress)
				}
				return true
			},
			{
				message: 'Chaincode address and events address are required for peer nodes',
				path: ['chaincodeAddress'],
			}
		)
		.refine(
			(data) => {
				if (data.nodeType === 'FABRIC_ORDERER') {
					return !!data.adminAddress
				}
				return true
			},
			{
				message: 'Admin address is required for orderer nodes',
				path: ['adminAddress'],
			}
		),
})

export type FabricNodeFormValues = z.infer<typeof fabricNodeFormSchema>

interface FabricNodeFormProps {
	onSubmit: (data: FabricNodeFormValues) => void
	isSubmitting?: boolean
	organizations?: { id: number; name: string }[]
	defaults?: GetNodesDefaultsFabricPeerResponse | GetNodesDefaultsFabricOrdererResponse
	onNodeTypeChange?: (type: 'FABRIC_PEER' | 'FABRIC_ORDERER') => void
	hideSubmit?: boolean
	hideOrganization?: boolean
	hideNodeType?: boolean
	defaultValues?: FabricNodeFormValues
	onChange?: (values: FabricNodeFormValues) => void
	submitText?: string
	submitButtonLoadingText?: string
}

export function FabricNodeForm({
	onSubmit,
	isSubmitting,
	organizations,
	defaults,
	onNodeTypeChange,
	hideSubmit,
	hideOrganization,
	hideNodeType,
	defaultValues,
	onChange,
	submitText = 'Create Node',
	submitButtonLoadingText = 'Creating...',
	}: FabricNodeFormProps) {
	const form = useForm<FabricNodeFormValues>({
		resolver: zodResolver(fabricNodeFormSchema),
		defaultValues: defaultValues || {
			name: '',
			fabricProperties: {
				nodeType: 'FABRIC_PEER',
				mode: 'service',
				version: '3.1.0',
				organizationId: undefined,
				listenAddress: defaults?.listenAddress || '',
				operationsListenAddress: defaults?.operationsListenAddress || '',
				externalEndpoint: defaults?.externalEndpoint || '',
				domains: [],
				addressOverrides: [],
			},
		},
	})
	const values = useWatch({ control: form.control })

	useEffect(() => {
		onChange?.(values as FabricNodeFormValues)
	}, [values])

	const nodeType = form.watch('fabricProperties.nodeType')

	useEffect(() => {
		if (defaults) {
			form.setValue('fabricProperties.listenAddress', defaults.listenAddress || '')
			form.setValue('fabricProperties.operationsListenAddress', defaults.operationsListenAddress || '')
			form.setValue('fabricProperties.externalEndpoint', defaults.externalEndpoint || '')

			if (nodeType === 'FABRIC_PEER' && 'chaincodeAddress' in defaults) {
				form.setValue('fabricProperties.chaincodeAddress', defaults.chaincodeAddress || '')
				form.setValue('fabricProperties.eventsAddress', defaults.eventsAddress || '')
			} else if (nodeType === 'FABRIC_ORDERER' && 'adminAddress' in defaults) {
				form.setValue('fabricProperties.adminAddress', defaults.adminAddress || '')
			}
		}
	}, [defaults, form, nodeType])

	const handleNodeTypeChange = (value: 'FABRIC_PEER' | 'FABRIC_ORDERER') => {
		form.setValue('fabricProperties.nodeType', value)
		// Clear type-specific fields when switching
		if (value === 'FABRIC_PEER') {
			form.setValue('fabricProperties.adminAddress', undefined)
		} else {
			form.setValue('fabricProperties.chaincodeAddress', undefined)
			form.setValue('fabricProperties.eventsAddress', undefined)
		}
		onNodeTypeChange?.(value)
	}

	return (
		<Form {...form}>
			<form
				onSubmit={form.handleSubmit(onSubmit, (errors) => {
					// Function to recursively extract error messages
					const extractErrorMessages = (obj: any, path = ''): string[] => {
						if (!obj) return []

						if (typeof obj === 'object') {
							if ('message' in obj && 'type' in obj) {
								// This is an error object
								return [`${path}: ${obj.message}`]
							}

							// Recursively process nested objects
							return Object.entries(obj).flatMap(([key, value]) => {
								const newPath = path ? `${path}.${key}` : key
								return extractErrorMessages(value, newPath)
							})
						}

						return []
					}

					const errorMessages = extractErrorMessages(errors)

					if (errorMessages.length > 0) {
						toast.error(`Errors in the form: ${errorMessages.join(', ')}`)
					}
				})}
				className="space-y-6"
			>
				<FormField
					control={form.control}
					name="name"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Node Name</FormLabel>
							<FormControl>
								<Input placeholder="Enter node name" {...field} />
							</FormControl>
							<FormMessage />
						</FormItem>
					)}
				/>

				{!hideOrganization && (
					<FormField
						control={form.control}
						name="fabricProperties.organizationId"
						render={({ field }) => (
							<FormItem>
								<FormLabel>Organization</FormLabel>
								<Select name="organization" onValueChange={(value) => field.onChange(parseInt(value))} value={field.value?.toString()}>
									<FormControl>
										<SelectTrigger>
											<SelectValue placeholder="Select organization" />
										</SelectTrigger>
									</FormControl>
									<SelectContent>
										{organizations?.map((org) => (
											<SelectItem key={org.id} value={org.id.toString()}>
												{org.name}
											</SelectItem>
										))}
									</SelectContent>
								</Select>
								<FormMessage />
							</FormItem>
						)}
					/>
				)}

				<div className="grid gap-4 md:grid-cols-2">
					{!hideNodeType && (
						<FormField
							control={form.control}
							name="fabricProperties.nodeType"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Node Type</FormLabel>
									<Select onValueChange={handleNodeTypeChange} value={field.value}>
										<FormControl>
											<SelectTrigger>
												<SelectValue placeholder="Select node type" />
											</SelectTrigger>
										</FormControl>
										<SelectContent>
											<SelectItem value="FABRIC_PEER">Peer</SelectItem>
											<SelectItem value="FABRIC_ORDERER">Orderer</SelectItem>
										</SelectContent>
									</Select>
									<FormMessage />
								</FormItem>
							)}
						/>
					)}

					<FormField
						control={form.control}
						name="fabricProperties.version"
						render={({ field }) => (
							<FormItem>
								<FormLabel>Version</FormLabel>
								<Select onValueChange={field.onChange} value={field.value}>
									<FormControl>
										<SelectTrigger>
											<SelectValue placeholder="Select version" />
										</SelectTrigger>
									</FormControl>
									<SelectContent>
										<SelectItem value="2.5.12">2.5.12</SelectItem>
										<SelectItem value="3.0.0">3.0.0</SelectItem>
										<SelectItem value="3.1.0">3.1.0</SelectItem>
									</SelectContent>
								</Select>
								<FormDescription>Select the Fabric version to use</FormDescription>
								<FormMessage />
							</FormItem>
						)}
					/>

					<FormField
						control={form.control}
						name="fabricProperties.mode"
						render={({ field }) => (
							<FormItem>
								<FormLabel>Mode</FormLabel>
								<Select onValueChange={field.onChange} value={field.value}>
									<FormControl>
										<SelectTrigger>
											<SelectValue placeholder="Select deployment mode" />
										</SelectTrigger>
									</FormControl>
									<SelectContent>
										<SelectItem value="docker">Docker</SelectItem>
										<SelectItem value="service">Service</SelectItem>
									</SelectContent>
								</Select>
								<FormDescription>Choose how the node will be deployed</FormDescription>
								<FormMessage />
							</FormItem>
						)}
					/>
				</div>

				<div className="space-y-4">
					<h3 className="text-lg font-medium">Network Configuration</h3>
					<div className="grid gap-4 md:grid-cols-2">
						<FormField
							control={form.control}
							name="fabricProperties.listenAddress"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Listen Address</FormLabel>
									<FormControl>
										<Input placeholder="e.g., 0.0.0.0:7051" {...field} />
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>

						<FormField
							control={form.control}
							name="fabricProperties.operationsListenAddress"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Operations Address</FormLabel>
									<FormControl>
										<Input placeholder="e.g., 0.0.0.0:9443" {...field} />
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>

						<FormField
							control={form.control}
							name="fabricProperties.externalEndpoint"
							render={({ field }) => (
								<FormItem>
									<FormLabel>External Endpoint</FormLabel>
									<FormControl>
										<Input placeholder="e.g., peer0.org1.example.com:7051" {...field} />
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>
					</div>
				</div>

				<div className="space-y-4">
					<h3 className="text-lg font-medium">Domain Configuration</h3>
					<div className="grid gap-4">
						<FormField
							control={form.control}
							name="fabricProperties.domains"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Domains</FormLabel>
									<FormDescription>Enter additional domains or IP addresses (one per line). Note: localhost and 127.0.0.1 are added by default</FormDescription>
									<FormControl>
										<textarea
											className="min-h-[100px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
											placeholder="example.com&#10;192.168.1.1"
											onChange={(e) => {
												const domains = e.target.value
													.split('\n')
													.map((d) => d.trim())
													.filter(Boolean)
												field.onChange(domains)
											}}
											value={field.value?.join('\n') || ''}
										/>
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>
					</div>
				</div>

				<div className="space-y-4">
					<h3 className="text-lg font-medium">Address Overrides</h3>
					<FormField
						control={form.control}
						name="fabricProperties.addressOverrides"
						render={({ field }) => (
							<FormItem>
								<FormLabel>Address Overrides</FormLabel>
								<FormDescription>Configure address overrides for the node</FormDescription>
								<div className="space-y-4">
									{field.value?.map((override, index) => {
										const errors = form.formState.errors.fabricProperties?.addressOverrides?.[index] as any
										return (
											<div key={index} className="flex gap-4 items-start">
												<div className="flex-1">
													<FormControl>
														<Input
															placeholder="From address"
															value={override.from}
															onChange={(e) => {
																const newOverrides = [...(field.value || [])]
																newOverrides[index] = { ...override, from: e.target.value }
																field.onChange(newOverrides)
															}}
														/>
													</FormControl>
													{errors?.from && <p className="text-destructive text-xs mt-1">{errors.from.message}</p>}
												</div>
												<div className="flex-1">
													<FormControl>
														<Input
															placeholder="To address"
															value={override.to}
															onChange={(e) => {
																const newOverrides = [...(field.value || [])]
																newOverrides[index] = { ...override, to: e.target.value }
																field.onChange(newOverrides)
															}}
														/>
													</FormControl>
													{errors?.to && <p className="text-destructive text-xs mt-1">{errors.to.message}</p>}
												</div>
												<div className="flex-1">
													<FormControl>
														<Textarea
															placeholder="TLS CA Certificate"
															className="font-mono text-xs"
															value={override.tlsCACert}
															onChange={(e) => {
																const newOverrides = [...(field.value || [])]
																newOverrides[index] = { ...override, tlsCACert: e.target.value }
																field.onChange(newOverrides)
															}}
														/>
													</FormControl>
													{errors?.tlsCACert && <p className="text-destructive text-xs mt-1">{errors.tlsCACert.message}</p>}
												</div>
												<Button
													type="button"
													variant="destructive"
													size="icon"
													onClick={() => {
														const newOverrides = [...(field.value || [])]
														newOverrides.splice(index, 1)
														field.onChange(newOverrides)
													}}
												>
													<Trash2 className="h-4 w-4" />
												</Button>
											</div>
										)
									})}
									<Button
										type="button"
										variant="outline"
										onClick={() => {
											const newOverrides = [...(field.value || []), { from: '', to: '', tlsCACert: '' }]
											field.onChange(newOverrides)
										}}
									>
										Add Address Override
									</Button>
								</div>
							</FormItem>
						)}
					/>
				</div>

				{nodeType === 'FABRIC_PEER' && (
					<div className="space-y-4">
						<h3 className="text-lg font-medium">Peer Configuration</h3>
						<div className="grid gap-4 md:grid-cols-2">
							<FormField
								control={form.control}
								name="fabricProperties.chaincodeAddress"
								render={({ field }) => (
									<FormItem>
										<FormLabel>Chaincode Address</FormLabel>
										<FormControl>
											<Input placeholder="e.g., 0.0.0.0:7052" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>

							<FormField
								control={form.control}
								name="fabricProperties.eventsAddress"
								render={({ field }) => (
									<FormItem>
										<FormLabel>Events Address</FormLabel>
										<FormControl>
											<Input placeholder="e.g., 0.0.0.0:7053" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
						</div>
					</div>
				)}

				{nodeType === 'FABRIC_ORDERER' && (
					<div className="space-y-4">
						<h3 className="text-lg font-medium">Orderer Configuration</h3>
						<div className="grid gap-4 md:grid-cols-2">
							<FormField
								control={form.control}
								name="fabricProperties.adminAddress"
								render={({ field }) => (
									<FormItem>
										<FormLabel>Admin Address</FormLabel>
										<FormControl>
											<Input placeholder="e.g., 0.0.0.0:7053" {...field} />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
						</div>
					</div>
				)}

				{!hideSubmit && (
					<Button type="submit" disabled={isSubmitting}>
						{isSubmitting ? submitButtonLoadingText : submitText}
					</Button>
				)}
			</form>
		</Form>
	)
}

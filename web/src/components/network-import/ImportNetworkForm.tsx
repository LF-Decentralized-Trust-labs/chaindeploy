import { getOrganizationsOptions, postNetworksBesuImportMutation, postNetworksFabricImportMutation, postNetworksFabricImportWithOrgMutation } from '@/api/client/@tanstack/react-query.gen'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { AlertCircle, Upload } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { z } from 'zod'

const fabricGenesisSchema = z.object({
	importMethod: z.literal('genesis'),
	genesisBlock: z.instanceof(File).optional(),
	description: z.string().optional(),
})

const fabricOrgSchema = z.object({
	importMethod: z.literal('organization'),
	organizationId: z.number({
		required_error: 'Organization is required',
	}),
	ordererUrl: z.string().url({
		message: 'Please enter a valid URL',
	}),
	ordererTlsCert: z.string().min(1, {
		message: 'TLS certificate is required',
	}),
	channelId: z.string().min(1, {
		message: 'Channel ID is required',
	}),
	description: z.string().optional(),
})

const formSchema = z.discriminatedUnion('networkType', [
	z.object({
		networkType: z.literal('fabric'),
		fabricImport: z.discriminatedUnion('importMethod', [fabricGenesisSchema, fabricOrgSchema]),
	}),
	z.object({
		networkType: z.literal('besu'),
		networkName: z.string().min(1, 'Network name is required'),
		chainId: z.number({
			required_error: 'Chain ID is required',
			invalid_type_error: 'Chain ID must be a number',
		}),
		genesisBlock: z.instanceof(File).optional(),
	}),
])

type FormValues = z.infer<typeof formSchema>

export function ImportNetworkForm() {
	const [error, setError] = useState<string | null>(null)
	const navigate = useNavigate()
	const [fabricImportMethod, setFabricImportMethod] = useState<'genesis' | 'organization'>('organization')

	const { data: organizations } = useQuery({
		...getOrganizationsOptions({query: {limit:1000}}),
	})

	const importFabricNetwork = useMutation({
		...postNetworksFabricImportMutation(),
		onSuccess: () => {
			toast.success('Network imported successfully')
			navigate('/networks')
		},
		onError: (error: Error) => {
			const errorMessage = error.message || 'Failed to import Fabric network'
			setError(errorMessage)
			toast.error('Failed to import network', {
				description: errorMessage,
			})
		},
	})

	const importFabricNetworkByOrg = useMutation({
		...postNetworksFabricImportWithOrgMutation(),
		onSuccess: () => {
			toast.success('Network imported successfully')
			navigate('/networks')
		},
		onError: (error: Error) => {
			const errorMessage = error.message || 'Failed to import Fabric network'
			setError(errorMessage)
			toast.error('Failed to import network', {
				description: errorMessage,
			})
		},
	})

	const importBesuNetwork = useMutation({
		...postNetworksBesuImportMutation(),
		onSuccess: () => {
			toast.success('Network imported successfully')
			navigate('/networks')
		},
		onError: (error: Error) => {
			const errorMessage = error.message || 'Failed to import Besu network'
			setError(errorMessage)
			toast.error('Failed to import network', {
				description: errorMessage,
			})
		},
	})

	const form = useForm<FormValues>({
		resolver: zodResolver(formSchema),
		defaultValues: {
			networkType: 'fabric',
			fabricImport: {
				importMethod: 'organization',
			},
		},
	})

	const networkType = form.watch('networkType')

	// Update form values when import method changes
	const handleImportMethodChange = (value: 'genesis' | 'organization') => {
		setFabricImportMethod(value)
		form.setValue('fabricImport.importMethod', value)
	}

	const onSubmit = async (data: FormValues) => {
		setError(null)

		if (data.networkType === 'fabric') {
			if (data.fabricImport.importMethod === 'genesis') {
				if (!data.fabricImport.genesisBlock) {
					setError('Genesis block is required')
					return
				}

				const reader = new FileReader()
				reader.readAsArrayBuffer(data.fabricImport.genesisBlock)

				reader.onload = () => {
					const arrayBuffer = reader.result as ArrayBuffer
					const uint8Array = new Uint8Array(arrayBuffer)
					const base64String = btoa(String.fromCharCode.apply(null, Array.from(uint8Array)))

					importFabricNetwork.mutate({
						body: {
							genesisFile: base64String,
							description: data.fabricImport.description,
						},
					})
				}

				reader.onerror = () => {
					setError('Error reading genesis block file')
				}
			} else {
				const tlsCertUint8Array = new TextEncoder().encode(data.fabricImport.ordererTlsCert)
				const tlsCertBase64 = btoa(String.fromCharCode.apply(null, Array.from(tlsCertUint8Array)))
				// Organization-based import
				importFabricNetworkByOrg.mutate({
					body: {
						organizationId: data.fabricImport.organizationId,
						ordererUrl: data.fabricImport.ordererUrl,
						ordererTlsCert: tlsCertBase64,
						channelId: data.fabricImport.channelId,
						description: data.fabricImport.description,
					},
				})
			}
		} else if (data.networkType === 'besu') {
			if (!data.genesisBlock) {
				setError('Genesis block is required')
				return
			}

			const reader = new FileReader()
			reader.readAsArrayBuffer(data.genesisBlock)

			reader.onload = () => {
				const arrayBuffer = reader.result as ArrayBuffer
				const uint8Array = new Uint8Array(arrayBuffer)
				const base64String = btoa(String.fromCharCode.apply(null, Array.from(uint8Array)))

				importBesuNetwork.mutate({
					body: {
						chainId: data.chainId,
						genesisFile: base64String,
						name: data.networkName,
					},
				})
			}

			reader.onerror = () => {
				setError('Error reading genesis block file')
			}
		}
	}

	const isLoading = importFabricNetwork.isPending || importBesuNetwork.isPending || importFabricNetworkByOrg.isPending

	return (
		<Card className="w-full max-w-2xl">
			<CardHeader>
				<CardTitle>Import Network</CardTitle>
				<CardDescription>Import an existing network</CardDescription>
			</CardHeader>
			<CardContent>
				{error && (
					<Alert variant="destructive" className="mb-6">
						<AlertCircle className="h-4 w-4" />
						<AlertTitle>Error</AlertTitle>
						<AlertDescription>{error}</AlertDescription>
					</Alert>
				)}

				<Form {...form}>
					<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
						<FormField
							control={form.control}
							name="networkType"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Network Type</FormLabel>
									<Select onValueChange={field.onChange} defaultValue={field.value}>
										<FormControl>
											<SelectTrigger>
												<SelectValue placeholder="Select network type" />
											</SelectTrigger>
										</FormControl>
										<SelectContent>
											<SelectItem value="fabric">Hyperledger Fabric</SelectItem>
											<SelectItem value="besu">Hyperledger Besu</SelectItem>
										</SelectContent>
									</Select>
									<FormDescription>Select the type of network you want to import</FormDescription>
									<FormMessage />
								</FormItem>
							)}
						/>

						{networkType === 'fabric' && (
							<div className="space-y-6">
								<div className="space-y-2">
									<FormLabel>Import Method</FormLabel>
									<RadioGroup
										defaultValue="organization"
										value={fabricImportMethod}
										onValueChange={(value) => handleImportMethodChange(value as 'genesis' | 'organization')}
										className="flex flex-col space-y-1"
									>
										<FormItem className="flex items-center space-x-3 space-y-0">
											<FormControl>
												<RadioGroupItem value="organization" />
											</FormControl>
											<FormLabel className="font-normal">Import using organization, orderer URL and TLS certificate</FormLabel>
										</FormItem>
										<FormItem className="flex items-center space-x-3 space-y-0">
											<FormControl>
												<RadioGroupItem value="genesis" />
											</FormControl>
											<FormLabel className="font-normal">Import using genesis block</FormLabel>
										</FormItem>
									</RadioGroup>
								</div>

								{fabricImportMethod === 'genesis' ? (
									<FormField
										control={form.control}
										name="fabricImport.genesisBlock"
										render={({ field: { onChange, value, ...field } }) => (
											<FormItem>
												<FormLabel>Genesis Block</FormLabel>
												<FormControl>
													<div className="flex items-center gap-4">
														<Input
															type="file"
															accept=".block,.json"
															onChange={(e) => {
																const file = e.target.files?.[0]
																if (file) {
																	onChange(file)
																}
															}}
															{...field}
														/>
														<Button
															type="button"
															variant="outline"
															size="icon"
															onClick={() => {
																const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
																fileInput?.click()
															}}
														>
															<Upload className="h-4 w-4" />
														</Button>
													</div>
												</FormControl>
												<FormDescription>Upload a genesis block file for your Fabric network</FormDescription>
												<FormMessage />
											</FormItem>
										)}
									/>
								) : null}

								{fabricImportMethod === 'organization' && (
									<>
										<FormField
											control={form.control}
											name="fabricImport.channelId"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Channel Name</FormLabel>
													<FormControl>
														<Input placeholder="e.g., mychannel" {...field} />
													</FormControl>
													<FormDescription>The name of the channel to join</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>
										<FormField
											control={form.control}
											name="fabricImport.organizationId"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Organization</FormLabel>
													<Select onValueChange={(value) => field.onChange(parseInt(value))} value={field.value?.toString()}>
														<FormControl>
															<SelectTrigger>
																<SelectValue placeholder="Select organization" />
															</SelectTrigger>
														</FormControl>
														<SelectContent>
															{organizations?.items?.map((org) => (
																<SelectItem key={org.id} value={org.id!.toString()}>
																	{org.mspId}
																</SelectItem>
															))}
														</SelectContent>
													</Select>
													<FormDescription>Select the organization to use for importing</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>

										<FormField
											control={form.control}
											name="fabricImport.ordererUrl"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Orderer URL</FormLabel>
													<FormControl>
														<Input placeholder="e.g., grpcs://orderer.example.com:7050" {...field} />
													</FormControl>
													<FormDescription>The URL of the orderer node</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>

										<FormField
											control={form.control}
											name="fabricImport.ordererTlsCert"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Orderer TLS Certificate</FormLabel>
													<FormControl>
														<Textarea placeholder="Paste the PEM-encoded TLS certificate here" className="font-mono text-xs h-32" {...field} />
													</FormControl>
													<FormDescription>The TLS certificate of the orderer node</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>

										<FormField
											control={form.control}
											name="fabricImport.description"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Description (Optional)</FormLabel>
													<FormControl>
														<Input placeholder="Enter network description" {...field} />
													</FormControl>
													<FormDescription>A brief description of the network</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>
									</>
								)}
							</div>
						)}

						{networkType === 'besu' && (
							<>
								<FormField
									control={form.control}
									name="networkName"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Network Name</FormLabel>
											<FormControl>
												<Input placeholder="Enter network name" {...field} />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>

								<FormField
									control={form.control}
									name="chainId"
									render={({ field: { value, onChange, ...field } }) => (
										<FormItem>
											<FormLabel>Chain ID</FormLabel>
											<FormControl>
												<Input type="number" placeholder="Enter chain ID" {...field} value={value || ''} onChange={(e) => onChange(Number(e.target.value))} />
											</FormControl>
											<FormDescription>The chain ID for your Besu network</FormDescription>
											<FormMessage />
										</FormItem>
									)}
								/>

								<FormField
									control={form.control}
									name="genesisBlock"
									render={({ field: { onChange, value, ...field } }) => (
										<FormItem>
											<FormLabel>Genesis Block</FormLabel>
											<FormControl>
												<div className="flex items-center gap-4">
													<Input
														type="file"
														accept=".block,.json"
														onChange={(e) => {
															const file = e.target.files?.[0]
															if (file) {
																onChange(file)
															}
														}}
														{...field}
													/>
													<Button
														type="button"
														variant="outline"
														size="icon"
														onClick={() => {
															const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
															fileInput?.click()
														}}
													>
														<Upload className="h-4 w-4" />
													</Button>
												</div>
											</FormControl>
											<FormDescription>Upload a genesis block file for your Besu network</FormDescription>
											<FormMessage />
										</FormItem>
									)}
								/>
							</>
						)}

						<Button type="submit" disabled={isLoading} className="w-full">
							{isLoading ? 'Importing...' : 'Import Network'}
						</Button>
					</form>
				</Form>
			</CardContent>
		</Card>
	)
}

import { getNodesOptions, getOrganizationsOptions } from '@/api/client/@tanstack/react-query.gen'
import { getKeysById } from '@/api/client/sdk.gen'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'

interface DeploymentModalProps {
	isOpen: boolean
	onClose: () => void
	onDeploy: (params: Record<string, unknown>) => void
	parameters?: Record<string, any> // JSON Schema
}

type FormValues = {
	[key: string]: number | number[] | string | boolean | Record<string, any>
}

interface FabricKeySelectValue {
	keyId: number
	orgId: number
}

interface FabricKeySelectProps {
	value?: FabricKeySelectValue
	onChange: (value: FabricKeySelectValue) => void
	disabled?: boolean
}

const FabricKeySelect = ({ value, onChange, disabled }: FabricKeySelectProps) => {
	const [selectedOrgId, setSelectedOrgId] = useState<number | null>(value?.orgId ?? null)
	const [selectedKeys, setSelectedKeys] = useState<Array<{ id: number; name: string; description?: string; algorithm?: string; keySize?: number; curve?: string }>>([])
	const [isLoading, setIsLoading] = useState(false)

	const { data: organizations } = useQuery({
		...getOrganizationsOptions({query: {limit:1000}}),
	})
	// Get the selected organization
	const selectedOrg = useMemo(() => organizations?.items?.find((org) => org.id === selectedOrgId), [organizations, selectedOrgId])
	// Get key IDs for the selected organization
	const keyIds = useMemo(() => {
		if (!selectedOrg) return []
		const ids: number[] = []
		if (selectedOrg.adminSignKeyId) ids.push(selectedOrg.adminSignKeyId)
		if (selectedOrg.adminTlsKeyId) ids.push(selectedOrg.adminTlsKeyId)
		if (selectedOrg.clientSignKeyId) ids.push(selectedOrg.clientSignKeyId)
		return ids
	}, [selectedOrg])
	const selectedKey = useMemo(() => selectedKeys.find((key) => key.id === value?.keyId), [selectedKeys, value])
	// Fetch key details when organization changes
	const fetchKeyDetails = useCallback(async () => {
		if (!keyIds.length) {
			setSelectedKeys([])
			return
		}
		setIsLoading(true)
		try {
			const keyDetails = await Promise.all(
				keyIds.map(async (keyId) => {
					const { data } = await getKeysById({ path: { id: keyId } })
					return {
						id: keyId,
						name: data.name || `Key ${keyId}`,
						description: data.description,
						algorithm: data.algorithm,
						keySize: data.keySize,
						curve: data.curve,
					}
				})
			)
			setSelectedKeys(keyDetails)
		} catch (error) {
			setSelectedKeys([])
		} finally {
			setIsLoading(false)
		}
	}, [keyIds])

	useEffect(() => {
		if (selectedOrgId && keyIds.length > 0) {
			fetchKeyDetails()
		}
	}, [fetchKeyDetails, selectedOrgId, keyIds])

	// When org changes, reset key selection
	useEffect(() => {
		if (selectedOrgId !== value?.orgId) {
			onChange({ orgId: selectedOrgId ?? 0, keyId: 0 })
		}
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [selectedOrgId])

	return (
		<div className="space-y-4">
			<Select
				value={selectedOrgId?.toString()}
				onValueChange={(val) => {
					setSelectedOrgId(Number(val))
				}}
				disabled={disabled}
			>
				<SelectTrigger>
					<SelectValue placeholder="Select an organization" />
				</SelectTrigger>
				<SelectContent>
					<ScrollArea className="h-[200px]">
						{organizations?.items?.map((org) => (
							<SelectItem key={org.id} value={org.id?.toString()}>
								{org.mspId}
							</SelectItem>
						))}
					</ScrollArea>
				</SelectContent>
			</Select>

			<Select
				value={value?.keyId ? value.keyId.toString() : undefined}
				onValueChange={(val) => {
					if (selectedOrgId) {
						onChange({ orgId: selectedOrgId, keyId: Number(val) })
					}
				}}
				disabled={disabled || !selectedOrgId || isLoading}
			>
				<SelectTrigger>
					{selectedKey ? (
						<div className="text-left">
							<div className="font-medium">{selectedKey.name}</div>
							<div className="text-xs text-muted-foreground">
								{selectedKey.description} • {selectedKey.algorithm}
							</div>
						</div>
					) : (
						'Select a key'
					)}
				</SelectTrigger>
				<SelectContent>
					<ScrollArea className="h-[200px]">
						{selectedKeys.map((key) => (
							<SelectItem key={key.id} value={key.id.toString()}>
								<div className="flex flex-col">
									<span>{key.name}</span>
									{key.description && <span className="text-xs text-muted-foreground">{key.description}</span>}
									{key.algorithm && (
										<span className="text-xs text-muted-foreground">
											Algorithm: {key.algorithm}
											{key.keySize && ` (${key.keySize} bits)`}
											{key.curve && ` - ${key.curve}`}
										</span>
									)}
								</div>
							</SelectItem>
						))}
					</ScrollArea>
				</SelectContent>
			</Select>
		</div>
	)
}

const FileInput = ({ value, onChange, disabled }: { value?: string; onChange: (value: string) => void; disabled?: boolean }) => {
	const [isDragging, setIsDragging] = useState(false)

	const handleFileDrop = (e: React.DragEvent<HTMLDivElement>) => {
		e.preventDefault()
		setIsDragging(false)

		const file = e.dataTransfer.files[0]
		if (file) {
			const reader = new FileReader()
			reader.onload = (event) => {
				if (event.target?.result) {
					onChange(event.target.result as string)
				}
			}
			reader.readAsText(file)
		}
	}

	const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
		const file = e.target.files?.[0]
		if (file) {
			const reader = new FileReader()
			reader.onload = (event) => {
				if (event.target?.result) {
					onChange(event.target.result as string)
				}
			}
			reader.readAsText(file)
		}
	}

	return (
		<div className="space-y-4">
			<div
				className={`border-2 border-dashed rounded-lg p-4 text-center ${
					isDragging ? 'border-primary bg-primary/5' : 'border-muted-foreground/25'
				}`}
				onDragOver={(e) => {
					e.preventDefault()
					setIsDragging(true)
				}}
				onDragLeave={() => setIsDragging(false)}
				onDrop={handleFileDrop}
			>
				<input
					type="file"
					className="hidden"
					id="file-input"
					onChange={handleFileSelect}
					disabled={disabled}
				/>
				<label
					htmlFor="file-input"
					className="cursor-pointer text-sm text-muted-foreground hover:text-foreground"
				>
					Drop a file here or click to select
				</label>
			</div>
			<Textarea
				value={value}
				onChange={(e) => onChange(e.target.value)}
				placeholder="Or paste your content here..."
				className="min-h-[200px]"
				disabled={disabled}
			/>
		</div>
	)
}

const DeploymentModal = ({ isOpen, onClose, onDeploy, parameters }: DeploymentModalProps) => {
	// Fetch data for x-source fields
	const { data: organizations } = useQuery({
		...getOrganizationsOptions({query: {limit:1000}}),
	})

	const { data: nodes } = useQuery({
		...getNodesOptions(),
	})

	// Dynamically create Zod schema from JSON Schema
	const createZodSchema = (jsonSchema: Record<string, any>) => {
		const schema: Record<string, any> = {}

		Object.entries(jsonSchema.properties || {}).forEach(([key, value]: [string, any]) => {
			// Handle x-source special cases
			if (value['x-source']) {
				switch (value['x-source']) {
					case 'fabric-org':
						schema[key] = z.number()
						break
					case 'key':
						schema[key] = z.number()
						break
					case 'fabric-key':
						schema[key] = z.object({ keyId: z.number(), orgId: z.number() })
						break
					case 'fabric-peer':
						if (value.type === 'array') {
							schema[key] = z.array(z.number())
						} else {
							schema[key] = z.number()
						}
						break
					case 'file':
						schema[key] = z.string()
						break
					default:
						// Fall back to normal type handling
						break
				}
			} else {
				switch (value.type) {
					case 'string':
						schema[key] = z.string()
						if (value.minLength) schema[key] = schema[key].min(value.minLength)
						if (value.maxLength) schema[key] = schema[key].max(value.maxLength)
						if (value.pattern) schema[key] = schema[key].regex(new RegExp(value.pattern))
						break
					case 'number':
						schema[key] = z.number()
						if (value.minimum) schema[key] = schema[key].min(value.minimum)
						if (value.maximum) schema[key] = schema[key].max(value.maximum)
						break
					case 'boolean':
						schema[key] = z.boolean()
						break
					case 'object':
						if (value.properties) {
							schema[key] = createZodSchema(value)
						} else {
							schema[key] = z.record(z.any())
						}
						break
					case 'array':
						if (value.items) {
							let itemSchema
							if (value.items.type === 'object' && value.items.properties) {
								itemSchema = createZodSchema(value.items)
							} else {
								switch (value.items.type) {
									case 'string':
										itemSchema = z.string()
										break
									case 'number':
										itemSchema = z.number()
										break
									case 'boolean':
										itemSchema = z.boolean()
										break
									default:
										itemSchema = z.any()
								}
							}
							schema[key] = z.array(itemSchema)
							if (value.minItems) schema[key] = schema[key].min(value.minItems)
							if (value.maxItems) schema[key] = schema[key].max(value.maxItems)
						} else {
							schema[key] = z.array(z.any())
						}
						break
				}
			}

			if (!jsonSchema.required?.includes(key)) {
				schema[key] = schema[key].optional()
			}
		})

		return z.object(schema)
	}

	const formSchema = parameters ? createZodSchema(parameters) : z.object({})

	const form = useForm<FormValues>({
		resolver: zodResolver(formSchema),
		defaultValues: parameters
			? Object.fromEntries(
					Object.entries(parameters.properties || {}).map(([key, value]: [string, any]) => {
						if (value['x-source'] === 'fabric-key') {
							return [key, { keyId: 0, orgId: 0 }]
						}
						// You can add more default value logic for other types if needed
						return [key, undefined]
					})
			  )
			: {},
	})

	const onSubmit = (values: FormValues) => {
		onDeploy(values)
		onClose()
	}

	// Create form fields dynamically based on JSON Schema
	const renderFormFields = () => {
		if (!parameters?.properties) return null

		return Object.entries(parameters.properties).map(([key, value]: [string, any]) => {
			if (value['x-source']) {
				switch (value['x-source']) {
					case 'fabric-org':
						return (
							<FormField
								key={key}
								control={form.control}
								name={key}
								render={({ field }) => (
									<FormItem>
										<FormLabel className="capitalize">
											{value.title || key}
											{parameters.required?.includes(key) && <span className="text-red-500 ml-1">*</span>}
										</FormLabel>
										<Select value={field.value?.toString()} onValueChange={(val) => field.onChange(Number(val))}>
											<FormControl>
												<SelectTrigger>
													<SelectValue placeholder="Select an organization" />
												</SelectTrigger>
											</FormControl>
											<SelectContent>
												<ScrollArea className="h-[200px]">
													{organizations?.items?.map((org) => (
														<SelectItem key={org.id} value={org.id?.toString()}>
															{org.mspId}
														</SelectItem>
													))}
												</ScrollArea>
											</SelectContent>
										</Select>
										{value.description && <p className="text-sm text-muted-foreground">{value.description}</p>}
										<FormMessage />
									</FormItem>
								)}
							/>
						)
					case 'fabric-key':
						return (
							<FormField
								key={key}
								control={form.control}
								name={key}
								render={({ field }) => (
									<FormItem>
										<FormLabel className="capitalize">
											{value.title || key}
											{parameters.required?.includes(key) && <span className="text-red-500 ml-1">*</span>}
										</FormLabel>
										<FormControl>
											<FabricKeySelect value={field.value as { keyId: number; orgId: number }} onChange={field.onChange} />
										</FormControl>
										{value.description && <p className="text-sm text-muted-foreground">{value.description}</p>}
										<FormMessage />
									</FormItem>
								)}
							/>
						)
					case 'fabric-peer':
						if (value.type === 'array') {
							return (
								<FormField
									key={key}
									control={form.control}
									name={key}
									render={({ field }) => (
										<FormItem className="space-y-4">
											<FormLabel className="capitalize">
												{value.title || key}
												{parameters.required?.includes(key) && <span className="text-red-500 ml-1">*</span>}
											</FormLabel>
											<div className="border rounded-md">
												<ScrollArea className="h-[200px] overflow-y-auto">
													<div className="p-4 space-y-3">
														{nodes?.items
															?.filter((node) => node.nodeType === 'FABRIC_PEER')
															.map((peer) => (
																<div key={peer.id} className="flex items-center space-x-3 hover:bg-muted/50 rounded-md p-2">
																	<Checkbox
																		id={`peer-${peer.id}`}
																		checked={(field.value as number[])?.includes(peer.id)}
																		onCheckedChange={(checked) => {
																			const currentValue = (field.value as number[]) || []
																			if (checked) {
																				field.onChange([...currentValue, peer.id])
																			} else {
																				field.onChange(currentValue.filter((id) => id !== peer.id))
																			}
																		}}
																		className="h-5 w-5"
																	/>
																	<label
																		htmlFor={`peer-${peer.id}`}
																		className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70 cursor-pointer flex-1"
																	>
																		<div className="font-semibold">{peer.name}</div>
																		<div className="text-muted-foreground text-xs mt-1">ID: {peer.id}</div>
																		<div className="text-muted-foreground text-xs mt-1">MSPID: {peer.fabricPeer?.mspId}</div>
																	</label>
																</div>
															))}
													</div>
												</ScrollArea>
											</div>
											{value.description && <p className="text-sm text-muted-foreground">{value.description}</p>}
											<FormMessage />
										</FormItem>
									)}
								/>
							)
						} else {
							return (
								<FormField
									key={key}
									control={form.control}
									name={key}
									render={({ field }) => (
										<FormItem>
											<FormLabel className="capitalize">
												{value.title || key}
												{parameters.required?.includes(key) && <span className="text-red-500 ml-1">*</span>}
											</FormLabel>
											<Select value={field.value?.toString()} onValueChange={(val) => field.onChange(Number(val))}>
												<FormControl>
													<SelectTrigger>
														<SelectValue placeholder="Select a peer" />
													</SelectTrigger>
												</FormControl>
												<SelectContent>
													<ScrollArea className="h-[200px]">
														{nodes?.items
															?.filter((node) => node.nodeType === 'FABRIC_PEER')
															.map((peer) => (
																<SelectItem key={peer.id} value={peer.id?.toString()}>
																	{peer.name}
																</SelectItem>
															))}
													</ScrollArea>
												</SelectContent>
											</Select>
											{value.description && <p className="text-sm text-muted-foreground">{value.description}</p>}
											<FormMessage />
										</FormItem>
									)}
								/>
							)
						}
					case 'file':
						return (
							<FormField
								key={key}
								control={form.control}
								name={key}
								render={({ field }) => (
									<FormItem>
										<FormLabel className="capitalize">
											{value.title || key}
											{parameters.required?.includes(key) && <span className="text-red-500 ml-1">*</span>}
										</FormLabel>
										<FormControl>
											<FileInput
												value={field.value as string}
												onChange={field.onChange}
												disabled={false}
											/>
										</FormControl>
										{value.description && <p className="text-sm text-muted-foreground">{value.description}</p>}
										<FormMessage />
									</FormItem>
								)}
							/>
						)
				}
			}
			// Default rendering for normal fields
			return (
				<FormField
					key={key}
					control={form.control}
					name={key}
					render={({ field }) => (
						<FormItem>
							<FormLabel className="capitalize">
								{value.title || key}
								{parameters.required?.includes(key) && <span className="text-red-500 ml-1">*</span>}
							</FormLabel>
							<FormControl>
								{value.type === 'boolean' ? (
									<Checkbox checked={field.value as boolean} onCheckedChange={field.onChange} />
								) : (
									<Input
										type={value.type === 'number' ? 'number' : 'text'}
										placeholder={value.description}
										value={field.value as string | number}
										onChange={(e) => {
											const val = value.type === 'number' ? Number(e.target.value) : e.target.value
											field.onChange(val)
										}}
									/>
								)}
							</FormControl>
							{value.description && <p className="text-sm text-muted-foreground">{value.description}</p>}
							<FormMessage />
						</FormItem>
					)}
				/>
			)
		})
	}

	return (
		<Dialog open={isOpen} onOpenChange={onClose}>
			<DialogContent className="sm:max-w-[425px] max-h-screen overflow-y-auto">
				<DialogHeader>
					<DialogTitle>Deployment</DialogTitle>
				</DialogHeader>
				<div className="grid gap-4 py-4">
					<Form {...form}>{renderFormFields()}</Form>
				</div>
				<DialogFooter>
					<Button type="submit" onClick={() => onSubmit(form.getValues())}>
						Deploy
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	)
}

export default DeploymentModal

import { postKeys } from '@/api/client'
import { getKeyProvidersOptions, getKeysFilterOptions, postKeysMutation, postNetworksBesuMutation } from '@/api/client/@tanstack/react-query.gen'
import { KeySelect } from '@/components/networks/key-select'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { hexToNumber, isValidHex, numberToHex } from '@/utils'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate } from 'react-router-dom'
import { NetworkCreatedDialog } from '@/components/dashboard/NetworkCreatedDialog'
import { toast } from 'sonner'
import * as z from 'zod'

const besuFormSchema = z
	.object({
		name: z.string().min(1, 'Network name is required'),
		description: z.string().optional(),
		keySelection: z.enum(['existing', 'generate']),
		numberOfKeys: z.number().min(1).max(10).optional(),
		providerId: z.number().optional(),
		blockPeriod: z.number().min(1),
		chainId: z.number(),
		coinbase: z.string(),
		consensus: z.enum(['qbft']).default('qbft'),
		difficulty: z.string().refine((val) => isValidHex(val), { message: 'Must be a valid hex value starting with 0x' }),
		epochLength: z.number(),
		gasLimit: z.string().refine((val) => isValidHex(val), { message: 'Must be a valid hex value starting with 0x' }),
		initialValidatorsKeyIds: z.array(z.number()),
		mixHash: z.string().refine((val) => isValidHex(val), { message: 'Must be a valid hex value starting with 0x' }),
		nonce: z.string().refine((val) => isValidHex(val), { message: 'Must be a valid hex value starting with 0x' }),
		requestTimeout: z.number(),
		timestamp: z.string().refine((val) => isValidHex(val), { message: 'Must be a valid hex value starting with 0x' }),
	})
	.refine(
		(data) => {
			// Only require initialValidatorsKeyIds when using existing keys
			if (data.keySelection === 'existing') {
				return data.initialValidatorsKeyIds.length >= 1
			}
			// When generating keys, require both numberOfKeys and providerId
			return typeof data.numberOfKeys === 'number' && data.numberOfKeys >= 1 && typeof data.providerId === 'number'
		},
		{
			message: 'Please either select existing validator keys or specify the number of keys and provider to generate',
			path: ['initialValidatorsKeyIds'], // This will show the error on the key selection field
		}
	)

type BesuFormValues = z.infer<typeof besuFormSchema>

const defaultValues: Partial<BesuFormValues> = {
	keySelection: 'existing',
	numberOfKeys: 4,
	blockPeriod: 5,
	chainId: 1337,
	coinbase: '0x0000000000000000000000000000000000000000',
	consensus: 'qbft',
	difficulty: numberToHex(1),
	epochLength: 30000,
	gasLimit: numberToHex(700000000),
	initialValidatorsKeyIds: [],
	mixHash: '0x63746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365',
	nonce: numberToHex(0),
	requestTimeout: 10,
	timestamp: numberToHex(1740000392),
}

export default function CreateBesuNetworkPage() {
	const navigate = useNavigate()
	const { data: keys } = useQuery({
		...getKeysFilterOptions({
			query: {
				algorithm: 'EC',
				curve: 'secp256k1',
			},
		}),
	})

	// Filter keys to only show EC/secp256k1 keys
	const validKeys = keys?.items?.filter((key) => key.algorithm === 'EC' && key.curve === 'secp256k1') || []

	const form = useForm<BesuFormValues>({
		resolver: zodResolver(besuFormSchema),
		defaultValues: {
			...defaultValues,
			timestamp: numberToHex(new Date().getTime()),
		},
	})
	const { data: providersData } = useQuery({
		...getKeyProvidersOptions({}),
	})
	const createKey = useMutation({
		...postKeysMutation(),
	})

	const [createdNetwork, setCreatedNetwork] = useState<{ id?: number; name: string } | null>(null)

	const createNetwork = useMutation({
		...postNetworksBesuMutation(),
		onSuccess: (data) => {
			setCreatedNetwork({ id: data.id, name: data.name || 'Besu Network' })
		},
		onError: (error: any) => {
			toast.error('Failed to create network', {
				description: error.message,
			})
		},
	})

	// Add state for progress
	const [progress, setProgress] = useState(0)
	const [progressSteps, setProgressSteps] = useState<string[]>([])

	const onSubmit = async (data: BesuFormValues) => {
		try {
			setProgress(0)
			let validatorKeyIds = data.initialValidatorsKeyIds

			if (data.keySelection === 'generate' && data.numberOfKeys) {
				setProgressSteps([`Generating ${data.numberOfKeys} validator keys...`])

				const keyPromises = Array.from({ length: data.numberOfKeys }, (_, i) => {
					return postKeys({
						body: {
							name: `Besu Validator Key ${i + 1}`,
							providerId: data.providerId,
							algorithm: 'EC',
							curve: 'secp256k1',
							description: `Validator Key ${i + 1} for ${data.name}`,
						},
					}).then((key) => {
						setProgress((prev) => prev + 50 / data.numberOfKeys!)
						return key
					})
				})

				const createdKeys = await Promise.all(keyPromises)
				validatorKeyIds = createdKeys.map((key) => key.data!.id!)
			}

			setProgressSteps((prev) => [...prev, 'Creating Besu network...'])
			setProgress(50)

			// Restructure the data for the API
			const networkData = {
				name: data.name,
				description: data.description,
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
				},
			}

			await createNetwork.mutateAsync({
				body: networkData,
			})

			setProgress(100)
			setProgressSteps((prev) => [...prev, 'Network created successfully!'])
		} catch (error: any) {
			toast.error('Failed to create network', {
				description: error.message,
			})
		} finally {
			setTimeout(() => {
				setProgress(0)
				setProgressSteps([])
			}, 2000)
		}
	}

	const keySelection = form.watch('keySelection')

	return (
		<div className="flex-1 p-8">
			<div className="max-w-4xl mx-auto">
				<div className="mb-8">
					<h1 className="text-2xl font-semibold">Create Besu Network</h1>
					<p className="text-muted-foreground">Configure a new Hyperledger Besu network</p>
				</div>

				<Form {...form}>
					<form
						onSubmit={form.handleSubmit(onSubmit, (error) => {
							const errorFields = Object.entries(error).map(([key, value]) => ({
								field: key,
								message: value?.message || 'Invalid value',
							}))

							toast.error('Failed to create network', {
								description: (
									<div className="space-y-2">
										<p>Please fix the following errors:</p>
										<ul className="list-disc pl-4">
											{errorFields.map(({ field, message }) => (
												<li key={field}>
													{field === 'initialValidatorsKeyIds' && message === 'Please either select existing validator keys or specify the number of keys to generate'
														? message
														: `${field}: ${message}`}
												</li>
											))}
										</ul>
									</div>
								),
							})
						})}
						className="space-y-8"
					>
						<Card>
							<CardHeader>
								<CardTitle>Basic Information</CardTitle>
								<CardDescription>Enter the basic details for your Besu network</CardDescription>
							</CardHeader>
							<CardContent className="space-y-4">
								<FormField
									control={form.control}
									name="name"
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
									control={form.control}
									name="description"
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
							</CardContent>
						</Card>

						<Card>
							<CardHeader>
								<CardTitle>Network Configuration</CardTitle>
								<CardDescription>Configure the technical parameters of your Besu network</CardDescription>
							</CardHeader>
							<CardContent className="space-y-6">
								<div className="grid grid-cols-2 gap-4">
									<FormField
										control={form.control}
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
										control={form.control}
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
										control={form.control}
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
										control={form.control}
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
										control={form.control}
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
										control={form.control}
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
										control={form.control}
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
										control={form.control}
										name="mixHash"
										render={({ field }) => (
											<FormItem>
												<FormLabel>Mix Hash</FormLabel>
												<FormControl>
													<Input {...field} />
												</FormControl>
												<FormDescription>Consensus-specific hash (Only used in PoW networks)</FormDescription>
												<FormMessage />
											</FormItem>
										)}
									/>
								</div>

								<div className="grid grid-cols-2 gap-4">
									<FormField
										control={form.control}
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
										control={form.control}
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

								<div className="grid grid-2 gap-4">
									<FormField
										control={form.control}
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
														<SelectItem value="ibft2">IBFT 2.0 (Istanbul Byzantine Fault Tolerance)</SelectItem>
														<SelectItem value="clique">Clique (Proof of Authority)</SelectItem>
														<SelectItem value="ethash">Ethash (Proof of Work)</SelectItem>
													</SelectContent>
												</Select>
												<FormDescription>Choose the consensus mechanism for your network</FormDescription>
												<FormMessage />
											</FormItem>
										)}
									/>
								</div>

								<div className="grid grid-1 gap-4">
									<FormField
										control={form.control}
										name="keySelection"
										render={({ field }) => (
											<FormItem className="space-y-3">
												<FormLabel>Key Selection Method</FormLabel>
												<FormControl>
													<RadioGroup onValueChange={field.onChange} defaultValue={field.value} className="flex flex-col space-y-1">
														<FormItem className="flex items-center space-x-3 space-y-0">
															<FormControl>
																<RadioGroupItem value="existing" />
															</FormControl>
															<FormLabel className="font-normal">Use existing keys</FormLabel>
														</FormItem>
														<FormItem className="flex items-center space-x-3 space-y-0">
															<FormControl>
																<RadioGroupItem value="generate" />
															</FormControl>
															<FormLabel className="font-normal">Generate new keys</FormLabel>
														</FormItem>
													</RadioGroup>
												</FormControl>
											</FormItem>
										)}
									/>

									{keySelection === 'existing' ? (
										<FormField
											control={form.control}
											name="initialValidatorsKeyIds"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Validator Keys</FormLabel>
													<FormControl>
														<KeySelect keys={validKeys} value={field.value} onChange={field.onChange} />
													</FormControl>
													<FormDescription>Select the validator keys for your network (EC/secp256k1 only)</FormDescription>
													<FormMessage />
												</FormItem>
											)}
										/>
									) : (
										<div className="space-y-4">
											<FormField
												control={form.control}
												name="numberOfKeys"
												render={({ field }) => (
													<FormItem>
														<FormLabel>Number of Keys to Generate</FormLabel>
														<FormControl>
															<Input type="number" min={1} max={10} {...field} onChange={(e) => field.onChange(Number(e.target.value))} />
														</FormControl>
														<FormDescription>Specify how many validator keys to generate (1-10)</FormDescription>
														<FormMessage />
													</FormItem>
												)}
											/>

											<FormField
												control={form.control}
												name="providerId"
												render={({ field }) => (
													<FormItem>
														<FormLabel>Key Provider</FormLabel>
														<Select onValueChange={(value) => field.onChange(Number(value))} defaultValue={field.value?.toString()}>
															<FormControl>
																<SelectTrigger>
																	<SelectValue placeholder="Select a key provider" />
																</SelectTrigger>
															</FormControl>
															<SelectContent>
																{providersData?.map((provider) => (
																	<SelectItem key={provider.id} value={provider.id!.toString()}>
																		{provider.name}
																	</SelectItem>
																))}
															</SelectContent>
														</Select>
														<FormDescription>Choose the provider for generating keys</FormDescription>
														<FormMessage />
													</FormItem>
												)}
											/>
										</div>
									)}
								</div>
							</CardContent>
						</Card>

						{(createNetwork.isPending || createKey.isPending) && (
							<Card>
								<CardContent className="pt-6">
									<div className="space-y-4">
										<Progress value={progress} className="h-2" />
										<div className="space-y-2">
											{progressSteps.map((step, index) => (
												<div key={index} className="text-sm text-muted-foreground flex items-center gap-2">
													{index === progressSteps.length - 1 ? (
														<div className="h-2 w-2 rounded-full bg-primary animate-pulse" />
													) : (
														<div className="h-2 w-2 rounded-full bg-primary" />
													)}
													{step}
												</div>
											))}
										</div>
									</div>
								</CardContent>
							</Card>
						)}

						<div className="flex justify-end">
							<Button type="submit" disabled={createNetwork.isPending || createKey.isPending}>
								{createNetwork.isPending || createKey.isPending ? 'Creating...' : 'Create Network'}
							</Button>
						</div>
					</form>
				</Form>
			</div>

			<NetworkCreatedDialog
				open={!!createdNetwork}
				onOpenChange={(open) => {
					if (!open) {
						setCreatedNetwork(null)
						navigate('/')
					}
				}}
				networkName={createdNetwork?.name || ''}
				networkId={createdNetwork?.id}
				platform="besu"
			/>
		</div>
	)
}

import { getKeysOptions, getNetworksBesuOptions, getNodesDefaultsBesuNodeOptions, getNodesPlatformByPlatformOptions, postNodesMutation } from '@/api/client/@tanstack/react-query.gen'
import { SingleKeySelect } from '@/components/networks/single-key-select'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { ArrowLeft } from 'lucide-react'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { z } from 'zod'

// Form validation schema
const formSchema = z.object({
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
	type: z.literal('besu'),
	bootNodes: z.string().optional(),
	requestTimeout: z.number().positive(),
	metricsPort: z.number().min(1024).max(65535),
	environmentVariables: z
		.array(
			z.object({
				key: z.string(),
				value: z.string(),
			})
		)
		.optional(),
})

type FormValues = z.infer<typeof formSchema>

export default function CreateBesuNodePage() {
	const navigate = useNavigate()
	const createNode = useMutation(postNodesMutation())

	const form = useForm<FormValues>({
		resolver: zodResolver(formSchema),
		defaultValues: {
			blockchainPlatform: 'BESU',
			type: 'besu',
			mode: 'service',
			rpcHost: '0.0.0.0',
			rpcPort: 8545,
			p2pPort: 30303,
			requestTimeout: 30,
			environmentVariables: [],
		},
	})

	const { data: besuDefaultConfig } = useQuery(getNodesDefaultsBesuNodeOptions())
	useEffect(() => {
		if (besuDefaultConfig) {
			const { p2pHost, p2pPort, rpcHost, rpcPort, externalIp, internalIp } = besuDefaultConfig[0].defaults![0]
			form.setValue('p2pHost', p2pHost!)
			form.setValue('p2pPort', Number(p2pPort))
			form.setValue('rpcHost', rpcHost!)
			form.setValue('rpcPort', Number(rpcPort))

			form.setValue('externalIp', externalIp!)
			form.setValue('internalIp', internalIp!)
		}
	}, [besuDefaultConfig, form.setValue])
	// Add queries for networks and keys
	const { data: networks } = useQuery(getNetworksBesuOptions({}))
	const { data: keys } = useQuery(getKeysOptions({}))
	const { data: besuNodes } = useQuery(
		getNodesPlatformByPlatformOptions({
			path: {
				platform: 'BESU',
			},
		})
	)

	const onSubmit = async (data: FormValues) => {
		try {
			await createNode.mutateAsync({
				body: {
					name: data.name,
					blockchainPlatform: 'BESU',
					besuNode: {
						externalIp: data.externalIp,
						internalIp: data.internalIp,
						keyId: data.keyId,
						networkId: data.networkId,
						mode: data.mode,
						p2pHost: data.p2pHost,
						p2pPort: data.p2pPort,
						rpcHost: data.rpcHost,
						rpcPort: data.rpcPort,
						metricsPort: data.metricsPort,
						type: 'BESU',
						metricsEnabled: true,
						metricsProtocol: 'PROMETHEUS',
						bootNodes: data.bootNodes
							?.split('\n')
							.map((node) => node.trim())
							.filter(Boolean),
						env: data.environmentVariables?.reduce(
							(acc, { key, value }) => ({
								...acc,
								[key]: value,
							}),
							{}
						),
					},
				},
			})
			toast.success('Node created successfully')
			navigate('/nodes')
		} catch (error: any) {
			toast.error('Failed to create node', {
				description: error.error.message,
			})
		}
	}

	return (
		<div className="flex-1 p-8">
			<div className="max-w-2xl mx-auto">
				<div className="flex items-center gap-2 text-muted-foreground mb-8">
					<Button variant="ghost" size="sm" asChild>
						<Link to="/nodes">
							<ArrowLeft className="mr-2 h-4 w-4" />
							Nodes
						</Link>
					</Button>
				</div>

				<div className="mb-8">
					<h1 className="text-2xl font-semibold mb-2">Create Besu Node</h1>
					<p className="text-muted-foreground">Add a new Hyperledger Besu node to your network</p>
				</div>

				<Card className="p-6">
					<Form {...form}>
						<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-8">
							{/* General Information */}
							<div className="space-y-4">
								<h2 className="text-lg font-semibold">General Information</h2>
								<FormField
									control={form.control}
									name="name"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Name</FormLabel>
											<FormControl>
												<Input {...field} />
											</FormControl>
											<FormDescription>A unique identifier for your node</FormDescription>
											<FormMessage />
										</FormItem>
									)}
								/>
							</div>

							{/* Node Configuration */}
							<div className="space-y-4">
								<h2 className="text-lg font-semibold">Node Configuration</h2>

								<div className="grid grid-cols-2 gap-4">
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
								</div>

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

								<div className="space-y-4">
									<FormField
										control={form.control}
										name="bootNodes"
										render={({ field }) => (
											<FormItem>
												<FormLabel>Boot Nodes</FormLabel>
												<div className="space-y-2">
													<Select
														onValueChange={(value) => {
															if (value) {
																const currentValue = field.value || ''
																const newValue = currentValue ? `${currentValue}\n${value}` : value
																field.onChange(newValue)
															}
														}}
													>
														<FormControl>
															<SelectTrigger>
																<SelectValue placeholder="Select existing nodes" />
															</SelectTrigger>
														</FormControl>
														<SelectContent>
															{besuNodes?.items?.map((node) => (
																<SelectItem key={node.id} value={node.besuNode?.enodeUrl ?? ''}>
																	{node.name}
																</SelectItem>
															))}
														</SelectContent>
													</Select>
													<FormControl>
														<Textarea {...field} placeholder="Enter boot node URLs (one per line)" className="min-h-[100px]" />
													</FormControl>
												</div>
												<FormDescription>Select from existing nodes or enter custom enode URLs (one per line)</FormDescription>
												<FormMessage />
											</FormItem>
										)}
									/>
								</div>
							</div>

							<div className="flex justify-end gap-4">
								<Button variant="outline" asChild>
									<Link to="/nodes">Cancel</Link>
								</Button>
								<Button type="submit" disabled={createNode.isPending}>
									Create Node
								</Button>
							</div>
						</form>
					</Form>
				</Card>
			</div>
		</div>
	)
}

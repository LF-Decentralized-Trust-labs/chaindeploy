import { getNetworksFabricOptions, getScFabricChaincodesOptions, getAiBoilerplatesOptions, getChaincodeProjectsOptions } from '@/api/client/@tanstack/react-query.gen'
import { postScFabricChaincodes, postChaincodeProjects } from '@/api/client/sdk.gen'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate } from 'react-router-dom'
import * as z from 'zod'

const chaincodeFormSchema = z.object({
	name: z
		.string()
		.min(1, 'Name is required')
		.regex(/^[a-z0-9_-]+$/, 'Name must be lowercase, no spaces, and can only contain letters, numbers, hyphens, or underscores')
		.transform((val) => val.toLowerCase()),
	networkId: z.string().min(1, 'Network is required'),
})

const developChaincodeFormSchema = z.object({
	name: z
		.string()
		.min(1, 'Name is required')
		.regex(/^[a-z0-9_-]+$/, 'Name must be lowercase, no spaces, and can only contain letters, numbers, hyphens, or underscores')
		.transform((val) => val.toLowerCase()),
	networkId: z.string().min(1, 'Network is required'),
	boilerplate: z.string().min(1, 'Boilerplate is required'),
	description: z.string().min(1, 'Description is required'),
	endorsementPolicy: z.string().min(1, 'Endorsement policy is required'),
})

type ChaincodeFormValues = z.infer<typeof chaincodeFormSchema>
type DevelopChaincodeFormValues = z.infer<typeof developChaincodeFormSchema>

export default function FabricChaincodesPage() {
	const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
	const [isDevelopDialogOpen, setIsDevelopDialogOpen] = useState(false)
	const [createError, setCreateError] = useState<string | null>(null)
	const [developError, setDevelopError] = useState<string | null>(null)
	const form = useForm<ChaincodeFormValues>({
		resolver: zodResolver(chaincodeFormSchema),
		defaultValues: { name: '', networkId: '' },
	})
	const developForm = useForm<DevelopChaincodeFormValues>({
		resolver: zodResolver(developChaincodeFormSchema),
		defaultValues: { 
			name: '', 
			networkId: '', 
			boilerplate: '', 
			description: '',
			endorsementPolicy: ''
		},
	})
	const navigate = useNavigate()

	// Fetch networks
	const { data: networks, refetch } = useQuery({
		...getNetworksFabricOptions({
			query: {
				limit: 10,
				offset: 0,
			},
		}),
	})

	// Fetch chaincodes
	const { data: chaincodesResponse, refetch: refetchChaincodes } = useQuery({
		...getScFabricChaincodesOptions(),
	})

	// Fetch chaincode projects
	const { data: chaincodeProjects, isLoading: isLoadingProjects } = useQuery({
		...getChaincodeProjectsOptions(),
	})

	// Fetch boilerplates
	const { data: boilerplates } = useQuery({
		...getAiBoilerplatesOptions({
			query: {
				network_id: Number(developForm.watch('networkId')) || 0,
			},
		}),
		enabled: !!developForm.watch('networkId'),
	})

	// Create chaincode mutation
	const createChaincodeMutation = useMutation({
		mutationFn: async (data: ChaincodeFormValues) => {
			const response = await postScFabricChaincodes({
				body: {
					name: data.name,
					network_id: Number(data.networkId),
				},
			})
			return response.data
		},
		onSuccess: () => {
			refetchChaincodes()
			setIsCreateDialogOpen(false)
			form.reset()
			setCreateError(null)
		},
		onError: (error: any) => {
			let message = 'Failed to create chaincode.'
			if (error?.response?.data?.message) {
				message = error.response.data.message
			} else if (error?.message) {
				message = error.message
			}
			setCreateError(message)
		},
	})

	// Develop chaincode mutation
	const developChaincodeMutation = useMutation({
		mutationFn: async (data: DevelopChaincodeFormValues) => {
			const response = await postChaincodeProjects({
				body: {
					name: data.name,
					networkId: Number(data.networkId),
					boilerplate: data.boilerplate,
					description: data.description,
					endorsementPolicy: data.endorsementPolicy,
				},
			})
			return response.data
		},
		onSuccess: (data) => {
			setIsDevelopDialogOpen(false)
			developForm.reset()
			setDevelopError(null)
			if (data.id) {
				navigate(`/sc/fabric/projects/chaincodes/${data.id}`)
			}
		},
		onError: (error: any) => {
			let message = 'Failed to create chaincode project.'
			if (error?.response?.data?.message) {
				message = error.response.data.message
			} else if (error?.message) {
				message = error.message
			}
			setDevelopError(message)
		},
	})

	const onSubmit = async (data: ChaincodeFormValues) => {
		setCreateError(null)
		await createChaincodeMutation.mutateAsync(data)
	}

	const onDevelopSubmit = async (data: DevelopChaincodeFormValues) => {
		setDevelopError(null)
		await developChaincodeMutation.mutateAsync(data)
	}

	return (
		<div className="flex-1 p-8">
			<div className="mb-6">
				<div className="flex items-center justify-between">
					<div>
						<h1 className="text-2xl font-semibold">Chaincodes</h1>
						<p className="text-muted-foreground">Manage chaincodes for your Fabric networks</p>
					</div>
					<div className="flex gap-2">
						<Dialog open={isDevelopDialogOpen} onOpenChange={setIsDevelopDialogOpen}>
							<DialogTrigger asChild>
								<Button variant="outline">
									<Plus className="mr-2 h-4 w-4" />
									Develop Chaincode
								</Button>
							</DialogTrigger>
							<DialogContent>
								<DialogHeader>
									<DialogTitle>Develop Chaincode</DialogTitle>
									<DialogDescription>Create a new chaincode project with a boilerplate.</DialogDescription>
								</DialogHeader>
								<Form {...developForm}>
									<form onSubmit={developForm.handleSubmit(onDevelopSubmit)} className="space-y-4">
										<FormField
											control={developForm.control}
											name="name"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Name</FormLabel>
													<FormControl>
														<Input placeholder="Enter chaincode name" {...field} />
													</FormControl>
													<FormMessage />
												</FormItem>
											)}
										/>
										<FormField
											control={developForm.control}
											name="networkId"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Network</FormLabel>
													<Select onValueChange={field.onChange} defaultValue={field.value}>
														<FormControl>
															<SelectTrigger>
																<SelectValue placeholder="Select a network" />
															</SelectTrigger>
														</FormControl>
														<SelectContent>
															{networks?.networks?.map((network) => (
																<SelectItem key={network.id} value={network.id?.toString() || ''}>
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
											control={developForm.control}
											name="boilerplate"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Boilerplate</FormLabel>
													<Select onValueChange={field.onChange} defaultValue={field.value}>
														<FormControl>
															<SelectTrigger>
																<SelectValue placeholder="Select a boilerplate" />
															</SelectTrigger>
														</FormControl>
														<SelectContent>
															{boilerplates?.map((boilerplate) => (
																<SelectItem key={boilerplate.id} value={boilerplate.id?.toString() || ''}>
																	{boilerplate.name}
																</SelectItem>
															))}
														</SelectContent>
													</Select>
													<FormMessage />
												</FormItem>
											)}
										/>
										<FormField
											control={developForm.control}
											name="description"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Description</FormLabel>
													<FormControl>
														<Input placeholder="Enter chaincode description" {...field} />
													</FormControl>
													<FormMessage />
												</FormItem>
											)}
										/>
										<FormField
											control={developForm.control}
											name="endorsementPolicy"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Endorsement Policy</FormLabel>
													<FormControl>
														<Input 
															placeholder="e.g. OR('Org1MSP.member')" 
															{...field} 
														/>
													</FormControl>
													<FormMessage />
													<div className="text-xs text-muted-foreground mt-1">
														Example policies:
														<ul className="list-disc list-inside mt-1">
															<li>OR('Org1MSP.member') - Any member of Org1</li>
															<li>AND('Org1MSP.member', 'Org2MSP.member') - Both Org1 and Org2 members</li>
															<li>OR('Org1MSP.member', 'Org2MSP.member') - Any member of Org1 or Org2</li>
														</ul>
													</div>
												</FormItem>
											)}
										/>
										{developError && <div className="text-sm text-red-500">{developError}</div>}
										<DialogFooter>
											<Button type="submit" disabled={developChaincodeMutation.isPending}>
												{developChaincodeMutation.isPending ? 'Creating...' : 'Create'}
											</Button>
										</DialogFooter>
									</form>
								</Form>
							</DialogContent>
						</Dialog>
						<Dialog open={isCreateDialogOpen} onOpenChange={setIsCreateDialogOpen}>
							<DialogTrigger asChild>
								<Button>
									<Plus className="mr-2 h-4 w-4" />
									Create Chaincode
								</Button>
							</DialogTrigger>
							<DialogContent>
								<DialogHeader>
									<DialogTitle>Create Chaincode</DialogTitle>
									<DialogDescription>Define a new chaincode for your Fabric network.</DialogDescription>
								</DialogHeader>
								<Form {...form}>
									<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
										<FormField
											control={form.control}
											name="name"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Name</FormLabel>
													<FormControl>
														<Input {...field} />
													</FormControl>
													<FormMessage />
												</FormItem>
											)}
										/>
										<FormField
											control={form.control}
											name="networkId"
											render={({ field }) => (
												<FormItem>
													<FormLabel>Network</FormLabel>
													<Select onValueChange={field.onChange} defaultValue={field.value}>
														<FormControl>
															<SelectTrigger>
																<SelectValue placeholder="Select a network" />
															</SelectTrigger>
														</FormControl>
														<SelectContent>
															{networks?.networks?.map((n) => (
																<SelectItem key={n.id} value={n.id.toString()}>
																	{n.name}
																</SelectItem>
															))}
														</SelectContent>
													</Select>
													<FormMessage />
												</FormItem>
											)}
										/>
										{createError && (
											<div className="rounded bg-red-100 border border-red-300 text-red-700 px-3 py-2 text-sm mb-2" role="alert">
												{createError}
											</div>
										)}
										<DialogFooter>
											<Button type="submit" disabled={createChaincodeMutation.isPending}>
												{createChaincodeMutation.isPending ? 'Creating...' : 'Create'}
											</Button>
										</DialogFooter>
									</form>
								</Form>
							</DialogContent>
						</Dialog>
					</div>
				</div>
			</div>

			{/* Chaincode Projects Section */}
			<div className="mb-8">
				<h2 className="text-xl font-semibold mb-4">Chaincode Projects</h2>
				<div className="grid gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
					{isLoadingProjects ? (
						<div className="col-span-full text-center text-muted-foreground">Loading projects...</div>
					) : !chaincodeProjects?.projects?.length ? (
						<Card className="p-6 text-center text-muted-foreground col-span-full">No chaincode projects yet.</Card>
					) : (
						chaincodeProjects.projects.map((project) => (
							<Card key={project.id} className="flex flex-col justify-between h-full">
								<div className="p-6">
									<div className="font-semibold text-lg mb-2">{project.name}</div>
									<div className="text-sm text-muted-foreground mb-4">{project.description}</div>
									<div className="text-xs text-muted-foreground">
										<div>Network: {networks?.networks?.find((n) => n.id === project.networkId)?.name}</div>
										<div>Boilerplate: {project.boilerplate}</div>
									</div>
								</div>
								<div className="p-6 pt-0">
									<Button variant="outline" size="sm" className="w-full" onClick={() => navigate(`/sc/fabric/projects/chaincodes/${project.id}`)}>
										View Details
									</Button>
								</div>
							</Card>
						))
					)}
				</div>
			</div>

			{/* Deployed Chaincodes Section */}
			<div>
				<h2 className="text-xl font-semibold mb-4">Deployed Chaincodes</h2>
				<div className="space-y-6">
					{!chaincodesResponse?.chaincodes?.length ? (
						<Card className="p-6 text-center text-muted-foreground">No chaincodes yet.</Card>
					) : (
						chaincodesResponse.chaincodes.map((detail) => (
							<Card key={detail.id} className="p-6 flex items-center justify-between">
								<div>
									<div className="font-semibold text-lg">{detail.name}</div>
									<div className="text-sm text-muted-foreground">Network: {networks?.networks?.find((n) => n.id === detail.network_id)?.name}</div>
								</div>
								<Button variant="outline" size="sm" onClick={() => navigate(`/sc/fabric/chaincodes/${detail.id}`)}>
									View Details
								</Button>
							</Card>
						))
					)}
				</div>
			</div>
		</div>
	)
}

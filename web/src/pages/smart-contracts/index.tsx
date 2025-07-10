import { getChaincodeProjectsOptions, getNetworksFabricOptions, getScFabricChaincodesOptions } from '@/api/client/@tanstack/react-query.gen'
import { postScFabricChaincodes } from '@/api/client/sdk.gen'
import { DevelopChaincodeDialog } from '@/components/forms/develop-chaincode-dialog'
import { BesuIcon } from '@/components/icons/besu-icon'
import { FabricIcon } from '@/components/icons/fabric-icon'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { TimeAgo } from '@/components/ui/time-ago'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate } from 'react-router-dom'
import * as z from 'zod'

// Chaincode creation schema
const chaincodeFormSchema = z.object({
	name: z
		.string()
		.min(1, 'Name is required')
		.regex(/^[a-z0-9_-]+$/, 'Name must be lowercase, no spaces, and can only contain letters, numbers, hyphens, or underscores')
		.transform((val) => val.toLowerCase()),
	networkId: z.string().min(1, 'Network is required'),
})

type ChaincodeFormValues = z.infer<typeof chaincodeFormSchema>

export default function SmartContractsPage() {
	const navigate = useNavigate()
	const { data, isLoading, error, refetch } = useQuery({
		...getScFabricChaincodesOptions(),
	})

	// Fetch networks for the select
	const { data: networks } = useQuery({
		...getNetworksFabricOptions({ query: { limit: 10, offset: 0 } }),
	})

	// Fetch projects for the section
	const {
		data: projectsData,
		isLoading: isLoadingProjects,
		error: projectsError,
	} = useQuery({
		...getChaincodeProjectsOptions(),
	})

	// Dialog state
	const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
	const [createError, setCreateError] = useState<string | null>(null)

	// Form setup
	const form = useForm<ChaincodeFormValues>({
		resolver: zodResolver(chaincodeFormSchema),
		defaultValues: { name: '', networkId: '' },
	})

	// Mutation for creating chaincode
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
			refetch()
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

	const onSubmit = async (data: ChaincodeFormValues) => {
		setCreateError(null)
		await createChaincodeMutation.mutateAsync(data)
	}

	return (
		<div className="flex-1 p-8">
			<div className="mb-8">
				<div className="flex items-center justify-between mb-4">
					<h2 className="text-xl font-semibold">Fabric Chaincodes</h2>
					<div className="flex gap-2">
						<DevelopChaincodeDialog
							networks={networks?.networks?.filter((n) => n.id !== undefined && n.name !== undefined).map((n) => ({ id: n.id as number, name: n.name as string }))}
							refetch={refetch}
						/>
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
				{isLoading ? (
					<div className="grid gap-6 md:grid-cols-2">
						{Array.from({ length: 4 }).map((_, i) => (
							<Skeleton key={i} className="h-40 w-full" />
						))}
					</div>
				) : error ? (
					<Alert variant="destructive" className="mb-4">
						<AlertTitle>Error loading chaincodes</AlertTitle>
						<AlertDescription>{error instanceof Error ? error.message : 'An unexpected error occurred.'}</AlertDescription>
					</Alert>
				) : !data?.chaincodes?.length ? (
					<Card className="p-6 text-center text-muted-foreground">No chaincodes found.</Card>
				) : (
					<div className="grid gap-6 md:grid-cols-2">
						{data.chaincodes.map((chaincode) => (
							<Card key={chaincode.id} className="cursor-pointer hover:bg-accent/50 transition-colors" onClick={() => navigate(`/sc/fabric/chaincodes/${chaincode.id}`)}>
								<CardHeader>
									<CardTitle className="flex items-center gap-2">
										<FabricIcon className="h-6 w-6" />
										{chaincode.name}
									</CardTitle>
									<CardDescription>
										Network: {chaincode.network_name ?? chaincode.network_id ?? 'N/A'}
										<br />
										Platform: {chaincode.network_platform ?? 'N/A'}
									</CardDescription>
								</CardHeader>
								<CardContent>
									<div className="space-y-2">
										<p className="text-sm text-muted-foreground">
											Created <TimeAgo date={chaincode.created_at} />
										</p>
									</div>
								</CardContent>
							</Card>
						))}
					</div>
				)}
			</div>

			{/* Projects Section */}
			<div className="mb-8">
				<h2 className="text-xl font-semibold mb-4">Chaincode Projects</h2>
				{isLoadingProjects ? (
					<div className="grid gap-6 md:grid-cols-2">
						{Array.from({ length: 3 }).map((_, i) => (
							<Skeleton key={i} className="h-40 w-full" />
						))}
					</div>
				) : projectsError ? (
					typeof projectsError === 'string' && (projectsError as string).includes('404') ? (
						<Alert variant="warning" className="mb-4">
							<AlertTitle>AI is not configured</AlertTitle>
							<AlertDescription>
								Please follow the{' '}
								<a href="https://docs.chainlaunch.dev/ai-setup" target="_blank" rel="noopener noreferrer" className="underline text-primary">
									documentation
								</a>{' '}
								to configure AI features.
							</AlertDescription>
						</Alert>
					) : (
						<Alert variant="destructive" className="mb-4">
							<AlertTitle>Error loading chaincodes</AlertTitle>
							<AlertDescription>{error instanceof Error ? error.message : 'An unexpected error occurred.'}</AlertDescription>
						</Alert>
					)
				) : !projectsData?.projects?.length ? (
					<Card className="p-6 text-center text-muted-foreground">No chaincode projects found.</Card>
				) : (
					<div className="grid gap-6 md:grid-cols-2">
						{projectsData.projects.map((project) => {
							let Icon = null
							if (project.networkPlatform?.toLowerCase() === 'fabric') {
								Icon = FabricIcon
							} else if (project.networkPlatform?.toLowerCase() === 'besu') {
								Icon = BesuIcon
							} // Add more platforms as needed
							return (
								<Card key={project.id} className="cursor-pointer hover:bg-accent/50 transition-colors" onClick={() => navigate(`/sc/fabric/projects/chaincodes/${project.id}`)}>
									<CardHeader>
										<CardTitle className="flex items-center gap-2">
											{Icon && <Icon className="h-6 w-6" />}
											{project.name}
										</CardTitle>
										<CardDescription>
											Network: {project.networkName ?? project.networkId ?? 'N/A'}
											<br />
											Platform: {project.networkPlatform ?? 'N/A'}
										</CardDescription>
									</CardHeader>
									<CardContent>
										<div className="space-y-2">
											{project.description && <p className="text-sm text-muted-foreground line-clamp-2">{project.description}</p>}
											{project.boilerplate && <p className="text-xs text-muted-foreground">Boilerplate: {project.boilerplate}</p>}
										</div>
									</CardContent>
								</Card>
							)
						})}
					</div>
				)}
			</div>
		</div>
	)
}

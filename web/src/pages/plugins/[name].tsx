import {
	getPluginsByNameDeploymentStatusOptions,
	getPluginsByNameOptions,
	getPluginsByNameServicesOptions,
	postPluginsByNameDeployMutation,
	postPluginsByNameResumeMutation,
	postPluginsByNameStopMutation,
} from '@/api/client/@tanstack/react-query.gen'
import { YamlViewer } from '@/components/plugins/YamlViewer'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Play, Square, Pencil } from 'lucide-react'
import { useState } from 'react'
import ReactMarkdown from 'react-markdown'
import { useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import DeploymentModal from './components/DeploymentModal'

const PluginDetailPage = () => {
	const { name } = useParams()
	const navigate = useNavigate()
	const [isDeployModalOpen, setIsDeployModalOpen] = useState(false)
	const [showYaml, setShowYaml] = useState(false)

	// Fetch plugin details
	const {
		data: plugin,
		isLoading,
		refetch,
	} = useQuery(
		getPluginsByNameOptions({
			path: { name: name! },
		})
	)

	// Fetch deployment status
	const { data: status, refetch: refetchStatus } = useQuery({
		...getPluginsByNameDeploymentStatusOptions({
			path: { name: name! },
		}),
		// refetchInterval: (data) => (data?.status === 'running' ? 5000 : false),
	})

	const { data: services, refetch: refetchServices } = useQuery({
		...getPluginsByNameServicesOptions({
			path: { name: name! },
		}),
		// refetchInterval: (data) => (data?.status === 'running' ? 5000 : false),
	})
	// Deploy mutation
	const deployMutation = useMutation({
		...postPluginsByNameDeployMutation(),
		onSuccess: () => {
			refetch()
			refetchStatus()
			refetchServices()
		},
	})

	// Stop mutation
	const stopMutation = useMutation({
		...postPluginsByNameStopMutation(),
		onSuccess: () => {
			refetch()
			refetchStatus()
			refetchServices()
		},
	})
	const resumeMutation = useMutation({
		...postPluginsByNameResumeMutation(),
		onSuccess: () => {
			refetch()
			refetchStatus()
			refetchServices()
		},
	})

	if (isLoading) return <div className="container p-8">Loading...</div>
	if (!plugin) return <div className="container p-8">Plugin not found</div>

	// Update the deploy button click handler
	const handleDeploy = async (params: Record<string, unknown>) => {
		toast.promise(
			deployMutation.mutateAsync({
				path: { name: name! },
				body: params,
			}),
			{
				loading: 'Deploying plugin...',
				success: 'Plugin deployed successfully',
				error: (e) => `Failed to deploy plugin: ${(e as any)?.data?.detail || (e as any)?.message}`,
			}
		)
	}

	return (
		<div className="container p-8">
			<div className="flex justify-between items-center mb-6">
				<div className="flex items-center gap-3">
					<h1 className="text-2xl font-bold">{plugin.metadata?.name}</h1>
					{status?.status && (
						<span
							className={`flex items-center gap-1 text-base font-medium ${
								status.status === 'deployed' ? 'text-green-600' : status.status === 'stopped' ? 'text-yellow-600' : 'text-gray-600'
							}`}
						>
							<span className={`w-2 h-2 rounded-full inline-block ${status.status === 'deployed' ? 'bg-green-500' : status.status === 'stopped' ? 'bg-yellow-500' : 'bg-gray-500'}`} />
							{status.status === 'deployed' ? 'Deployed' : status.status === 'stopped' ? 'Stopped' : 'Not Deployed'}
						</span>
					)}
				</div>
				<div className="flex gap-2">
					<DropdownMenu>
						<DropdownMenuTrigger asChild>
							<Button variant="outline">Actions</Button>
						</DropdownMenuTrigger>
						<DropdownMenuContent align="end">
							<DropdownMenuItem
								onClick={() =>
									toast.promise(resumeMutation.mutateAsync({ path: { name: name! } }), {
										loading: 'Resuming plugin...',
										success: 'Plugin resumed successfully',
										error: (e) => `Failed to resume plugin: ${e.message}`,
									})
								}
								disabled={status?.status !== 'stopped'}
							>
								<Play className="mr-2 h-4 w-4" /> Resume
							</DropdownMenuItem>
							<DropdownMenuItem
								onClick={() =>
									toast.promise(stopMutation.mutateAsync({ path: { name: name! } }), {
										loading: 'Stopping plugin...',
										success: 'Plugin stopped successfully',
										error: 'Failed to stop plugin',
									})
								}
								disabled={status?.status !== 'deployed'}
							>
								<Square className="mr-2 h-4 w-4" /> Stop
							</DropdownMenuItem>
							<DropdownMenuSeparator />
							<DropdownMenuItem onClick={() => setShowYaml(true)}>View YAML</DropdownMenuItem>
							<DropdownMenuItem onClick={() => navigate(`/plugins/${name}/edit`)}>
								<Pencil className="mr-2 h-4 w-4" /> Edit Plugin
							</DropdownMenuItem>
						</DropdownMenuContent>
					</DropdownMenu>
					{status?.status !== 'deployed' && (
						<Button onClick={() => setIsDeployModalOpen(true)}>
							<Play className="mr-2 h-4 w-4" /> Deploy
						</Button>
					)}
				</div>
			</div>

			{/* Plugin Information Tabs */}
			<Tabs defaultValue="documentation" className="w-full">
				<TabsList className="grid w-full grid-cols-2">
					<TabsTrigger value="documentation">Documentation</TabsTrigger>
					<TabsTrigger value="services">Services</TabsTrigger>
				</TabsList>

				{/* Documentation Tab */}
				<TabsContent value="documentation">
					<Card>
						<CardHeader>
							<CardTitle>Plugin Information</CardTitle>
						</CardHeader>
						<CardContent>
							<div className="space-y-6">
								{/* Metadata Section */}
								<div>
									<h3 className="font-semibold mb-4">Metadata</h3>
									<div className="grid grid-cols-2 gap-4">
										{plugin.metadata?.name && (
											<div>
												<span className="font-medium">Name:</span> {plugin.metadata.name}
											</div>
										)}
										{plugin.metadata?.version && (
											<div>
												<span className="font-medium">Version:</span> {plugin.metadata.version}
											</div>
										)}
										{plugin.metadata?.author && (
											<div>
												<span className="font-medium">Author:</span> {plugin.metadata.author}
											</div>
										)}
										{plugin.metadata?.license && (
											<div>
												<span className="font-medium">License:</span> {plugin.metadata.license}
											</div>
										)}
										{plugin.metadata?.repository && (
											<div>
												<span className="font-medium">Repository:</span>{' '}
												<a href={plugin.metadata.repository} target="_blank" rel="noopener noreferrer" className="text-blue-500 hover:underline">
													{plugin.metadata.repository}
												</a>
											</div>
										)}
										{plugin.metadata?.tags && plugin.metadata.tags.length > 0 && (
											<div className="col-span-2">
												<span className="font-medium">Tags:</span>{' '}
												<div className="flex flex-wrap gap-2 mt-1">
													{plugin.metadata.tags.map((tag, index) => (
														<span key={index} className="bg-muted px-2 py-1 rounded-full text-sm">
															{tag}
														</span>
													))}
												</div>
											</div>
										)}
										{plugin.metadata?.description && (
											<div className="col-span-2">
												<span className="font-medium">Description:</span>
												<p className="mt-1 text-muted-foreground">{plugin.metadata.description}</p>
											</div>
										)}
									</div>
								</div>

								{/* Documentation Section */}
								{plugin.spec?.documentation && (
									<div>
										<h3 className="font-semibold mb-4">Documentation</h3>
										<div className="prose prose-sm dark:prose-invert max-w-none prose-headings:font-semibold prose-a:text-blue-500 prose-a:no-underline hover:prose-a:underline">
											<ReactMarkdown>{plugin.spec.documentation.readme}</ReactMarkdown>
										</div>
									</div>
								)}
							</div>
						</CardContent>
					</Card>
				</TabsContent>

				{/* Services Tab */}
				<TabsContent value="services">
					{services && services.length > 0 ? (
						<Card>
							<CardHeader>
								<CardTitle>Services</CardTitle>
							</CardHeader>
							<CardContent>
								<div className="space-y-4">
									{services.map((service, index) => (
										<div key={index} className="border rounded-lg p-4">
											<div className="flex justify-between items-center">
												<h3 className="font-medium">{service.name}</h3>
												<div className="flex items-center gap-2">
													<div
														className={`w-2 h-2 rounded-full ${
															service.state === 'running' ? 'bg-green-500' : service.state === 'stopped' ? 'bg-red-500' : 'bg-yellow-500'
														}`}
													/>
													<span className="text-xs capitalize">{service.state || 'unknown'}</span>
												</div>
											</div>

											{service.image && (
												<div className="mt-2 text-sm">
													<span className="font-semibold">Image:</span> {service.image}
												</div>
											)}

											{service.ports && (
												<div className="mt-2 text-sm">
													<span className="font-semibold">Port:</span> {service.ports}
												</div>
											)}

											{service.config?.command && (
												<div className="mt-2 text-sm">
													<span className="font-semibold">Command:</span> {JSON.stringify(service.config.command)}
												</div>
											)}

											{service.environment && typeof service.environment.length === 'number' && service.environment.length > 0 && (
												<div className="mt-2 text-sm">
													<span className="font-semibold">Environment Variables:</span>
													<ul className="list-disc pl-5 mt-1">
														{Object.entries(service.environment).map(([key, value]) => (
															<li key={key}>{`${key}: ${value}`}</li>
														))}
													</ul>
												</div>
											)}

											{service.volumes && service.volumes.length > 0 && (
												<div className="mt-2 text-sm">
													<span className="font-semibold">Volumes:</span>
													<ul className="list-disc pl-5 mt-1">
														{service.volumes.map((volume, volIndex) => (
															<li key={volIndex}>{volume}</li>
														))}
													</ul>
												</div>
											)}
											{service.config && (
												<div className="mt-2 text-sm">
													<span className="font-semibold">Config:</span>
													<pre className="bg-muted p-2 rounded mt-1 text-xs overflow-x-auto">
														{typeof service.config === 'string' ? service.config : JSON.stringify(service.config, null, 2)}
													</pre>
												</div>
											)}
										</div>
									))}
								</div>
							</CardContent>
						</Card>
					) : (
						<Card>
							<CardContent className="p-6 text-center text-muted-foreground">No services available</CardContent>
						</Card>
					)}
				</TabsContent>
			</Tabs>
			<DeploymentModal isOpen={isDeployModalOpen} onClose={() => setIsDeployModalOpen(false)} onDeploy={handleDeploy} parameters={plugin.spec?.parameters} />
			{showYaml && (
				<div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
					<div className="relative bg-white dark:bg-gray-900 rounded-lg shadow-lg p-6 max-w-2xl w-full">
						<button onClick={() => setShowYaml(false)} className="absolute top-2 right-2 text-gray-500 hover:text-gray-800 dark:hover:text-white" aria-label="Close">
							&#10005;
						</button>
						<YamlViewer yaml={plugin} dialogOpen={showYaml} setDialogOpen={setShowYaml} />
					</div>
				</div>
			)}
		</div>
	)
}

export default PluginDetailPage

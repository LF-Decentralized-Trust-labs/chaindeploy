import {
	getPluginsByNameDeploymentStatusOptions,
	getPluginsByNameOptions,
	getPluginsByNameServicesOptions,
	postPluginsByNameDeployMutation,
	postPluginsByNameStopMutation,
} from '@/api/client/@tanstack/react-query.gen'
import { YamlViewer } from '@/components/plugins/YamlViewer'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useMutation, useQuery } from '@tanstack/react-query'
import { ArrowLeft, Play, Square } from 'lucide-react'
import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import DeploymentModal from './components/DeploymentModal'

const PluginDetailPage = () => {
	const { name } = useParams()
	const navigate = useNavigate()
	const [isDeployModalOpen, setIsDeployModalOpen] = useState(false)

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
				error: (e) => `Failed to deploy plugin: ${(e as any).error.message}`,
			}
		)
	}

	return (
		<div className="container p-8">
			<div className="flex justify-between items-center mb-6">
				<div>
					<h1 className="text-2xl font-bold">{plugin.metadata?.name}</h1>
				</div>
				<div className="flex gap-2">
					<Button variant="outline" onClick={() => navigate('/plugins')}>
						<ArrowLeft className="mr-2 h-4 w-4" />
						Back
					</Button>
					<YamlViewer yaml={plugin} label="View YAML" />
					{status?.status !== 'deployed' ? (
						<Button onClick={() => setIsDeployModalOpen(true)}>
							<Play className="mr-2 h-4 w-4" />
							Deploy
						</Button>
					) : (
						<Button
							variant="destructive"
							onClick={() =>
								toast.promise(
									stopMutation.mutateAsync({
										path: { name: name! },
									}),
									{
										loading: 'Stopping plugin...',
										success: 'Plugin stopped successfully',
										error: 'Failed to stop plugin',
									}
								)
							}
						>
							<Square className="mr-2 h-4 w-4" />
							Stop
						</Button>
					)}
				</div>
			</div>

			{/* Status indicator */}
			<Card className="mb-6">
				<CardHeader>
					<CardTitle>Status</CardTitle>
				</CardHeader>
				<CardContent>
					<div className="flex items-center gap-2">
						<div className={`w-3 h-3 rounded-full ${status?.status === 'deployed' ? 'bg-green-500' : status?.status === 'Stopped' ? 'bg-red-500' : 'bg-yellow-500'}`} />
						<span className="capitalize">{status?.status || 'Unknown'}</span>
					</div>
				</CardContent>
			</Card>

			{/* Plugin Services */}
			{services && services.length > 0 && (
				<Card className="mt-6">
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
											<div className={`w-2 h-2 rounded-full ${service.state === 'running' ? 'bg-green-500' : service.state === 'stopped' ? 'bg-red-500' : 'bg-yellow-500'}`} />
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
			)}

			<DeploymentModal isOpen={isDeployModalOpen} onClose={() => setIsDeployModalOpen(false)} onDeploy={handleDeploy} parameters={plugin.spec?.parameters} />
		</div>
	)
}

export default PluginDetailPage

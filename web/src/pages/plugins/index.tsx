import { deletePluginsByNameMutation, getPluginsOptions, getPluginsAvailableOptions, postPluginsMutation } from '@/api/client/@tanstack/react-query.gen'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Card, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Trash2 } from 'lucide-react'
import { useState, useRef } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import yaml from 'js-yaml'

const PluginsPage = () => {
	const [pluginToDelete, setPluginToDelete] = useState<string | null>(null)
	const [installing, setInstalling] = useState<string | null>(null)
	const navigate = useNavigate()
	const installPluginNameRef = useRef<string | null>(null)

	// Fetch plugins
	const { data: plugins, isLoading, error, refetch } = useQuery(getPluginsOptions())

	// Fetch available plugins
	const { data: availablePluginsData, isLoading: isLoadingAvailable, error: errorAvailable, refetch: refetchAvailable } = useQuery(getPluginsAvailableOptions())
	const availablePlugins = availablePluginsData?.plugins || []

	// Delete mutation
	const deleteMutation = useMutation({
		...deletePluginsByNameMutation(),
		onSuccess: () => {
			toast.success('Plugin deleted successfully')
			refetch()
		},
		onError: (error) => {
			toast.error(`Failed to delete plugin: ${error}`)
		},
	})

	// Install mutation
	const installMutation = useMutation({
		...postPluginsMutation(),
		onSuccess: () => {
			toast.success('Plugin installed successfully')
			refetch()
			refetchAvailable()
			setInstalling(null)
			if (installPluginNameRef.current) {
				navigate(`/plugins/${installPluginNameRef.current}`)
				installPluginNameRef.current = null
			}
		},
		onError: (error) => {
			if (error.message) {
				toast.error(`Failed to install plugin: ${error.message} ${(error.data as any)?.detail}`)
			} else {
				toast.error(`Failed to install plugin: ${error}`)
			}
			setInstalling(null)
		},
	})

	const handleDelete = (name: string) => {
		deleteMutation.mutate({ path: { name } })
		setPluginToDelete(null)
	}

	const handleInstall = (plugin: any) => {
		setInstalling(plugin.name)
		installPluginNameRef.current = plugin.name
		if (plugin.raw_yaml) {
			try {
				const parsed = yaml.load(plugin.raw_yaml) as any
				installMutation.mutate({
					body: {
						apiVersion: parsed.apiVersion,
						kind: parsed.kind,
						metadata: parsed.metadata,
						spec: parsed.spec,
					},
				})
			} catch (err: any) {
				toast.error(`Failed to parse plugin YAML: ${err.message}`)
				setInstalling(null)
			}
		} else {
			installMutation.mutate({
				body: {
					apiVersion: 'chainlaunch/v1',
					kind: 'Plugin',
					metadata: {
						name: plugin.name,
						description: plugin.description,
						author: plugin.author,
						license: plugin.license,
						tags: plugin.tags,
						version: plugin.version,
					},
					spec: {}, // If more info is needed, map here
				},
			})
		}
	}

	if (isLoading) return <div className="container p-8">Loading...</div>
	if (error) return <div className="container p-8 text-red-500">Error loading plugins</div>

	return (
		<div className="container p-8">
			<div className="flex justify-between items-center mb-6">
				<h1 className="text-2xl font-bold">Plugins</h1>
				<Button asChild>
					<Link to="/plugins/new">Create Plugin</Link>
				</Button>
			</div>

			{/* Installed Plugins */}
			{plugins?.length ? (
				<>
					<h2 className="text-xl font-bold mb-4">Installed Plugins</h2>
					<div className="grid gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 mb-8">
						{plugins.map((plugin) => (
							<Card key={plugin.metadata?.name} className="flex flex-col justify-between h-full">
								<CardHeader className="pb-2">
									<div className="flex items-center gap-2 mb-2">
										<CardTitle className="text-lg">
											<Link to={`/plugins/${plugin.metadata?.name}`} className="hover:text-primary transition-colors">
												{plugin.metadata?.name}
											</Link>
										</CardTitle>
										<div
											className={`w-2 h-2 rounded-full ${
												plugin.deploymentStatus?.status === 'deployed'
													? 'bg-green-500'
												: plugin.deploymentStatus?.status === 'stopped'
												? 'bg-red-500'
												: 'bg-yellow-500'
											}`}
										/>
									</div>
									<CardDescription className="line-clamp-2 min-h-[2.5rem]">{(plugin.metadata as any)?.description || 'No description provided.'}</CardDescription>
								</CardHeader>
								<div className="flex justify-end px-6 pb-4">
									<Button variant="destructive" size="icon" onClick={() => setPluginToDelete(plugin.metadata?.name || null)}>
										<Trash2 className="h-4 w-4" />
									</Button>
								</div>
							</Card>
						))}
					</div>
				</>
			) : null}

			{/* Available Plugins (Templates) */}
			<div className="mb-8">
				<h2 className="text-xl font-bold mb-4">Available Plugins</h2>
				{isLoadingAvailable ? (
					<div className="text-muted-foreground">Loading available plugins...</div>
				) : errorAvailable ? (
					<div className="text-red-500">Error loading available plugins</div>
				) : (
					<div className="grid gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
						{availablePlugins.filter((ap) => !plugins?.some((p) => p.metadata?.name === ap.name)).length === 0 ? (
							<Card className="flex flex-col items-center justify-center py-12 col-span-1 sm:col-span-2 lg:col-span-3">
								<p className="text-muted-foreground">No available plugins to install.</p>
							</Card>
						) : (
							availablePlugins
								.filter((ap) => !plugins?.some((p) => p.metadata?.name === ap.name))
								.map((plugin) => (
									<Card key={plugin.name} className="flex flex-col justify-between h-full">
										<CardHeader className="pb-2">
											<CardTitle className="text-lg">{plugin.name}</CardTitle>
											<CardDescription className="line-clamp-2 min-h-[2.5rem]">{plugin.description || 'No description provided.'}</CardDescription>
										</CardHeader>
										<div className="flex flex-col gap-2 px-6 pb-4 space-y-2">
											<div className="text-xs text-muted-foreground">
												By {plugin.author || 'Unknown'}
												{plugin.version && ` • v${plugin.version}`}
											</div>
											<Button disabled={installing === plugin.name} variant="default" onClick={() => handleInstall(plugin)}>
												{installing === plugin.name ? 'Installing...' : 'Install'}
											</Button>
										</div>
									</Card>
								))
						)}
					</div>
				)}
			</div>

			<AlertDialog open={!!pluginToDelete} onOpenChange={(open) => !open && setPluginToDelete(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Are you sure?</AlertDialogTitle>
						<AlertDialogDescription>This will permanently delete the plugin "{pluginToDelete}". This action cannot be undone.</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction className="bg-destructive text-destructive-foreground hover:bg-destructive/90" onClick={() => pluginToDelete && handleDelete(pluginToDelete)}>
							Delete
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	)
}

export default PluginsPage

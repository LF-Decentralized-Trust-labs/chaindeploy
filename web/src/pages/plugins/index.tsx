import { deletePluginsByNameMutation, getPluginsOptions } from '@/api/client/@tanstack/react-query.gen'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Card, CardHeader, CardTitle } from '@/components/ui/card'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Trash2 } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { toast } from 'sonner'

const PluginsPage = () => {
	const [pluginToDelete, setPluginToDelete] = useState<string | null>(null)

	// Fetch plugins
	const { data: plugins, isLoading, error, refetch } = useQuery(getPluginsOptions())

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

	const handleDelete = (name: string) => {
		deleteMutation.mutate({ path: { name } })
		setPluginToDelete(null)
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

			<div className="grid gap-4">
				{!plugins?.length ? (
					<Card className="flex flex-col items-center justify-center py-16">
						<div className="flex flex-col items-center gap-4 text-center">
							<div className="rounded-full bg-muted p-4">
								<svg
									xmlns="http://www.w3.org/2000/svg"
									width="24"
									height="24"
									viewBox="0 0 24 24"
									fill="none"
									stroke="currentColor"
									strokeWidth="2"
									strokeLinecap="round"
									strokeLinejoin="round"
									className="h-8 w-8 text-muted-foreground"
								>
									<path d="m16 6 4 14" />
									<path d="M12 6v14" />
									<path d="M8 8v12" />
									<path d="M4 4v16" />
								</svg>
							</div>
							<div className="space-y-2">
								<h3 className="text-xl font-semibold">No plugins found</h3>
								<p className="text-muted-foreground">Get started by creating your first plugin.</p>
							</div>
							<Button asChild>
								<Link to="/plugins/new">Create Plugin</Link>
							</Button>
						</div>
					</Card>
				) : (
					plugins.map((plugin) => (
						<Card key={plugin.metadata?.name}>
							<CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
								<div className="flex items-center gap-2">
									<CardTitle className="text-lg">
										<Link to={`/plugins/${plugin.metadata?.name}`} className="hover:text-primary transition-colors">
											{plugin.metadata?.name}
										</Link>
									</CardTitle>
									<div
										className={`w-2 h-2 rounded-full ${
											plugin.deploymentStatus?.status === 'deployed' ? 'bg-green-500' : plugin.deploymentStatus?.status === 'stopped' ? 'bg-red-500' : 'bg-yellow-500'
										}`}
									/>
								</div>
								<Button variant="destructive" size="icon" onClick={() => setPluginToDelete(plugin.metadata?.name || null)}>
									<Trash2 className="h-4 w-4" />
								</Button>
							</CardHeader>
						</Card>
					))
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

import { deleteChaincodeProjectsByIdMutation, getChaincodeProjectsByIdOptions, putChaincodeProjectsByIdEndorsementPolicyMutation } from '@/api/client/@tanstack/react-query.gen'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Code, MoreVertical, Trash2, Play, Square, Clock, Settings, Package } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import * as z from 'zod'

const endorsementPolicySchema = z.object({
	endorsementPolicy: z.string().min(1, 'Endorsement policy is required'),
})

type EndorsementPolicyFormValues = z.infer<typeof endorsementPolicySchema>

export default function ChaincodeProjectDetailPage() {
	const { id } = useParams()
	const navigate = useNavigate()
	const projectId = parseInt(id || '0', 10)

	const {
		data: project,
		isLoading,
		error,
		refetch,
	} = useQuery({
		...getChaincodeProjectsByIdOptions({ path: { id: projectId } }),
		enabled: !!projectId,
	})

	const [isDialogOpen, setIsDialogOpen] = useState(false)
	const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
	const form = useForm<EndorsementPolicyFormValues>({
		resolver: zodResolver(endorsementPolicySchema),
		defaultValues: {
			endorsementPolicy: project?.endorsementPolicy || '',
		},
	})

	const [updating, setUpdating] = useState(false)
	const [deleting, setDeleting] = useState(false)
	const queryClient = useQueryClient()
	const updateEndorsementPolicyMutation = useMutation(putChaincodeProjectsByIdEndorsementPolicyMutation())

	const handleUpdate = async (data: EndorsementPolicyFormValues) => {
		setUpdating(true)
		try {
			await updateEndorsementPolicyMutation.mutateAsync({
				path: { id: projectId },
				body: { endorsementPolicy: data.endorsementPolicy },
			})
			toast.success('Endorsement policy updated')
			setIsDialogOpen(false)
			await queryClient.invalidateQueries({ queryKey: ['getChaincodeProjectsById', { path: { id: projectId } }] })
			await refetch()
		} catch (err: any) {
			toast.error('Failed to update endorsement policy', { description: err?.message })
		} finally {
			setUpdating(false)
		}
	}
	const deleteChaincodeProjectMutation = useMutation(deleteChaincodeProjectsByIdMutation())
	const handleDelete = async () => {
		setDeleting(true)
		try {
			await deleteChaincodeProjectMutation.mutateAsync({ path: { id: projectId } })
			toast.success('Chaincode project deleted')
			navigate('/smart-contracts/fabric')
		} catch (err: any) {
			toast.error('Failed to delete chaincode project', { description: err?.message })
		} finally {
			setDeleting(false)
			setIsDeleteDialogOpen(false)
		}
	}

	const getStatusBadge = (status: string) => {
		switch (status?.toLowerCase()) {
			case 'running':
				return <Badge variant="default" className="bg-green-100 text-green-800 hover:bg-green-100"><Play className="w-3 h-3 mr-1" />Running</Badge>
			case 'stopped':
				return <Badge variant="secondary"><Square className="w-3 h-3 mr-1" />Stopped</Badge>
			default:
				return <Badge variant="outline">{status}</Badge>
		}
	}

	if (isLoading) return (
		<div className="container mx-auto p-6 max-w-7xl">
			<div className="animate-pulse">
				<div className="h-8 bg-muted rounded w-1/3 mb-6"></div>
				<div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
					<div className="lg:col-span-2">
						<div className="h-64 bg-muted rounded"></div>
					</div>
					<div className="h-64 bg-muted rounded"></div>
				</div>
			</div>
		</div>
	)
	
	if (error) return (
		<div className="container mx-auto p-6 max-w-7xl">
			<div className="text-center py-12">
				<div className="text-red-500 text-lg font-medium mb-2">Error loading project</div>
				<p className="text-muted-foreground">Unable to load the chaincode project details.</p>
			</div>
		</div>
	)
	
	if (!project) return (
		<div className="container mx-auto p-6 max-w-7xl">
			<div className="text-center py-12">
				<div className="text-lg font-medium mb-2">Project not found</div>
				<p className="text-muted-foreground">The requested chaincode project could not be found.</p>
			</div>
		</div>
	)

	return (
		<div className="container mx-auto p-6 max-w-7xl">
			{/* Header */}
			<div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 mb-8">
				<div className="flex-1">
					<h1 className="text-3xl font-bold tracking-tight">{project.name}</h1>
					{project.description && (
						<p className="text-muted-foreground mt-1">{project.description}</p>
					)}
				</div>
				<div className="flex gap-2">
					<Button onClick={() => navigate(`/sc/fabric/projects/chaincodes/${project.id}/editor`)}>
						<Code className="mr-2 h-4 w-4" />
						Open Editor
					</Button>
					<DropdownMenu>
						<DropdownMenuTrigger asChild>
							<Button variant="outline" size="icon">
								<MoreVertical className="h-4 w-4" />
							</Button>
						</DropdownMenuTrigger>
						<DropdownMenuContent align="end">
							<DropdownMenuItem onClick={() => setIsDialogOpen(true)}>
								<Settings className="mr-2 h-4 w-4" />
								Update Endorsement Policy
							</DropdownMenuItem>
							<DropdownMenuSeparator />
							<DropdownMenuItem className="text-destructive" onClick={() => setIsDeleteDialogOpen(true)}>
								<Trash2 className="mr-2 h-4 w-4" />
								Delete Chaincode Project
							</DropdownMenuItem>
						</DropdownMenuContent>
					</DropdownMenu>
				</div>
			</div>

			{/* Main Content Grid */}
			<div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
				{/* Project Details Card */}
				<div className="lg:col-span-2">
					<Card>
						<CardHeader>
							<CardTitle className="flex items-center gap-2">
								<Package className="h-5 w-5" />
								Project Details
							</CardTitle>
							<CardDescription>Information about this chaincode project</CardDescription>
						</CardHeader>
						<CardContent>
							<div className="grid grid-cols-1 md:grid-cols-2 gap-6">
								<div className="space-y-4">
									<div>
										<label className="text-sm font-medium text-muted-foreground">Status</label>
										<div className="mt-1">
											{getStatusBadge(project.status)}
										</div>
									</div>
									
									<div>
										<label className="text-sm font-medium text-muted-foreground">Boilerplate</label>
										<p className="mt-1 text-sm">{project.boilerplate}</p>
									</div>

									{project.containerPort && (
										<div>
											<label className="text-sm font-medium text-muted-foreground">Container Port</label>
											<p className="mt-1 text-sm">{project.containerPort}</p>
										</div>
									)}
								</div>

								<div className="space-y-4">
									{project.endorsementPolicy && (
										<div>
											<label className="text-sm font-medium text-muted-foreground">Endorsement Policy</label>
											<p className="mt-1 text-sm font-mono bg-muted p-2 rounded text-xs break-all">
												{project.endorsementPolicy}
											</p>
										</div>
									)}
								</div>
							</div>
						</CardContent>
					</Card>
				</div>

				{/* Timeline Card */}
				<div className="lg:col-span-1">
					<Card>
						<CardHeader>
							<CardTitle className="flex items-center gap-2">
								<Clock className="h-5 w-5" />
								Activity Timeline
							</CardTitle>
							<CardDescription>Recent project activity</CardDescription>
						</CardHeader>
						<CardContent>
							<div className="space-y-4">
								{project.lastStartedAt && (
									<div className="flex items-start gap-3">
										<div className="w-2 h-2 bg-green-500 rounded-full mt-2 flex-shrink-0"></div>
										<div className="flex-1 min-w-0">
											<p className="text-sm font-medium">Started</p>
											<p className="text-xs text-muted-foreground">
												{new Date(project.lastStartedAt).toLocaleDateString()} at {new Date(project.lastStartedAt).toLocaleTimeString()}
											</p>
										</div>
									</div>
								)}
								
								{project.lastStoppedAt && (
									<div className="flex items-start gap-3">
										<div className="w-2 h-2 bg-red-500 rounded-full mt-2 flex-shrink-0"></div>
										<div className="flex-1 min-w-0">
											<p className="text-sm font-medium">Stopped</p>
											<p className="text-xs text-muted-foreground">
												{new Date(project.lastStoppedAt).toLocaleDateString()} at {new Date(project.lastStoppedAt).toLocaleTimeString()}
											</p>
										</div>
									</div>
								)}
								
								{!project.lastStartedAt && !project.lastStoppedAt && (
									<div className="text-center py-4">
										<p className="text-sm text-muted-foreground">No activity recorded yet</p>
									</div>
								)}
							</div>
						</CardContent>
					</Card>
				</div>
			</div>

			{/* Delete Confirmation Dialog */}
			<Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>Delete Chaincode Project</DialogTitle>
					</DialogHeader>
					<div className="py-4">
						<p>Are you sure you want to delete "{project.name}"? This action cannot be undone.</p>
					</div>
					<DialogFooter>
						<Button variant="outline" onClick={() => setIsDeleteDialogOpen(false)}>
							Cancel
						</Button>
						<Button variant="destructive" onClick={handleDelete} disabled={deleting}>
							{deleting ? 'Deleting...' : 'Delete'}
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</div>
	)
}

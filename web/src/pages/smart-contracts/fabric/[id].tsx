import { deleteChaincodeProjectsByIdMutation, getChaincodeProjectsByIdOptions, putChaincodeProjectsByIdEndorsementPolicyMutation } from '@/api/client/@tanstack/react-query.gen'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query'
import { useParams, useNavigate } from 'react-router-dom'
import { Code, MoreVertical, Trash2 } from 'lucide-react'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogTrigger } from '@/components/ui/dialog'
import { Form, FormField, FormItem, FormLabel, FormControl, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { useForm } from 'react-hook-form'
import { useState } from 'react'
import { toast } from 'sonner'
import * as z from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'

const endorsementPolicySchema = z.object({
	endorsementPolicy: z.string().min(1, 'Endorsement policy is required'),
})

type EndorsementPolicyFormValues = z.infer<typeof endorsementPolicySchema>

export default function ChaincodeProjectDetailPage() {
	const { id } = useParams()
	const navigate = useNavigate()
	const projectId = parseInt(id || '0', 10)

	const { data: project, isLoading, error, refetch } = useQuery({
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
	const deleteChaincodeProjectMutation = useMutation({
		...deleteChaincodeProjectsByIdMutation(),
	})
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

	if (isLoading) return <div className="container p-8">Loading...</div>
	if (error) return <div className="container p-8 text-red-500">Error loading project</div>
	if (!project) return <div className="container p-8">Project not found</div>

	return (
		<div className="container p-8">
			<div className="flex justify-between items-center mb-6">
				<h1 className="text-2xl font-bold">{project.name}</h1>
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
								Update Endorsement Policy
							</DropdownMenuItem>
							<DropdownMenuSeparator />
							<DropdownMenuItem 
								className="text-destructive" 
								onClick={() => setIsDeleteDialogOpen(true)}
							>
								<Trash2 className="mr-2 h-4 w-4" />
								Delete Chaincode Project
							</DropdownMenuItem>
						</DropdownMenuContent>
					</DropdownMenu>
				</div>
			</div>

			<div className="grid gap-4">
				<Card>
					<CardHeader>
						<CardTitle>Project Details</CardTitle>
						<CardDescription>Information about this chaincode project</CardDescription>
					</CardHeader>
					<CardContent>
						<div className="grid gap-4">
							<div>
								<h3 className="font-semibold mb-1">Description</h3>
								<p className="text-muted-foreground">{project.description || 'No description provided'}</p>
							</div>
							<div>
								<h3 className="font-semibold mb-1">ID</h3>
								<p className="text-muted-foreground">{project.id}</p>
							</div>
							<div>
								<h3 className="font-semibold mb-1">Network ID</h3>
								<p className="text-muted-foreground">{project.networkId}</p>
							</div>
							<div>
								<h3 className="font-semibold mb-1">Boilerplate</h3>
								<p className="text-muted-foreground">{project.boilerplate}</p>
							</div>
							<div>
								<h3 className="font-semibold mb-1">Status</h3>
								<p className="text-muted-foreground">{project.status}</p>
							</div>
							{project.endorsementPolicy && (
								<div>
									<h3 className="font-semibold mb-1">Endorsement Policy</h3>
									<p className="text-muted-foreground">{project.endorsementPolicy}</p>
								</div>
							)}
							{project.containerPort && (
								<div>
									<h3 className="font-semibold mb-1">Container Port</h3>
									<p className="text-muted-foreground">{project.containerPort}</p>
								</div>
							)}
							{project.lastStartedAt && (
								<div>
									<h3 className="font-semibold mb-1">Last Started</h3>
									<p className="text-muted-foreground">{new Date(project.lastStartedAt).toLocaleString()}</p>
								</div>
							)}
							{project.lastStoppedAt && (
								<div>
									<h3 className="font-semibold mb-1">Last Stopped</h3>
									<p className="text-muted-foreground">{new Date(project.lastStoppedAt).toLocaleString()}</p>
								</div>
							)}
						</div>
					</CardContent>
				</Card>
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
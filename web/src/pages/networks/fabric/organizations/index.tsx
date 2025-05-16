import { HandlerOrganizationResponse } from '@/api/client'
import { deleteOrganizationsByIdMutation, getKeyProvidersOptions, getOrganizationsOptions, postOrganizationsMutation } from '@/api/client/@tanstack/react-query.gen'
// createFabricOrganizationMutation, deleteOrganizationMutation, getOrganizationsOptions, listProvidersOptions
import { OrganizationFormValues } from '@/components/forms/organization-form'
import { CreateOrganizationDialog } from '@/components/organizations/create-organization-dialog'
import { OrganizationItem } from '@/components/organizations/organization-item'
import { OrganizationSkeleton } from '@/components/organizations/organization-skeleton'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Building2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'

export default function OrganizationsPage() {
	const [open, setOpen] = useState(false)
	const [orgToDelete, setOrgToDelete] = useState<HandlerOrganizationResponse | null>(null)

	const { data: providers } = useQuery({
		...getKeyProvidersOptions(),
	})

	const {
		data: organizations,
		isLoading,
		refetch,
	} = useQuery({
		...getOrganizationsOptions({}),
	})

	const createOrganization = useMutation({
		...postOrganizationsMutation(),
		onSuccess: () => {
			toast.success('Organization created successfully')
			refetch()
			setOpen(false)
		},
		networkMode: 'always',
		onError: (error) => {
			if (error instanceof Error) {
				toast.error(`An error occurred: ${error.message}`)
			} else {
				toast.error('An unknown error occurred')
			}
		},
	})

	const deleteOrganization = useMutation({
		...deleteOrganizationsByIdMutation(),
		onSuccess: () => {
			toast.success('Organization deleted successfully')
			refetch()
			setOrgToDelete(null)
		},
		onError: (error) => {
			if (error instanceof Error) {
				toast.error(`An error occurred: ${error.message}`)
			} else {
				toast.error('An unknown error occurred')
			}
		},
	})

	const onSubmit = (data: OrganizationFormValues) => {
		createOrganization.mutate({
			body: {
				name: data.mspId,
				mspId: data.mspId,
				description: data.description,
				providerId: data.providerId,
			},
		})
	}

	const handleDelete = (org: HandlerOrganizationResponse) => {
		setOrgToDelete(org)
	}
	if (isLoading) {
		return (
			<div className="flex-1 p-8">
				<div className="max-w-4xl mx-auto">
					<div className="flex items-center justify-between mb-8">
						<div className="space-y-2">
							<Skeleton className="h-8 w-32" />
							<Skeleton className="h-5 w-64" />
						</div>
						<Skeleton className="h-10 w-32" />
					</div>
					<div className="space-y-4">
						{[1, 2, 3].map((i) => (
							<OrganizationSkeleton key={i} />
						))}
					</div>
				</div>
			</div>
		)
	}

	if (!organizations?.length) {
		return (
			<div className="flex-1 p-8">
				<div className="max-w-4xl mx-auto">
					<div className="text-center">
						<div className="flex justify-center mb-4">
							<Building2 className="h-12 w-12 text-muted-foreground" />
						</div>
						<h1 className="text-2xl font-semibold mb-2">No organizations found</h1>
						<p className="text-muted-foreground mb-8">Create your first organization to get started.</p>
						<CreateOrganizationDialog open={open} onOpenChange={setOpen} onSubmit={onSubmit} isSubmitting={createOrganization.isPending} providers={providers} />
					</div>
				</div>
			</div>
		)
	}

	return (
		<div className="flex-1 p-8">
			<div className="max-w-4xl mx-auto">
				<div className="flex items-center justify-between mb-8">
					<div>
						<h1 className="text-2xl font-semibold">Organizations</h1>
						<p className="text-muted-foreground">Manage organizations in your Fabric network</p>
					</div>
					<CreateOrganizationDialog open={open} onOpenChange={setOpen} onSubmit={onSubmit} isSubmitting={createOrganization.isPending} providers={providers} />
				</div>

				<div className="space-y-4">
					{organizations?.map((org) => (
						<OrganizationItem key={org.id} organization={org} onDelete={handleDelete} />
					))}
				</div>
			</div>

			<AlertDialog open={!!orgToDelete} onOpenChange={(open) => !open && setOrgToDelete(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Are you sure?</AlertDialogTitle>
						<AlertDialogDescription>This action cannot be undone. This will permanently delete the organization and all associated keys.</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction disabled={deleteOrganization.isPending} onClick={() => orgToDelete && deleteOrganization.mutate({ path: { id: orgToDelete.id! } })}>
							Delete
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	)
}

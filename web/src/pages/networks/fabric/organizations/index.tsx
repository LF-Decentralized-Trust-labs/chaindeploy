import { HandlerOrganizationResponse } from '@/api/client'
import { deleteOrganizationsByIdMutation, getKeyProvidersOptions, getOrganizationsOptions, postOrganizationsMutation } from '@/api/client/@tanstack/react-query.gen'
import { OrganizationFormValues } from '@/components/forms/organization-form'
import { CreateOrganizationDialog } from '@/components/organizations/create-organization-dialog'
import { OrganizationItem } from '@/components/organizations/organization-item'
import { OrganizationSkeleton } from '@/components/organizations/organization-skeleton'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Pagination } from '@/components/ui/pagination'
import { Skeleton } from '@/components/ui/skeleton'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Building2, ChevronDown } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'

export default function OrganizationsPage() {
	const [open, setOpen] = useState(false)
	const [orgToDelete, setOrgToDelete] = useState<HandlerOrganizationResponse | null>(null)
	// State for pagination
	const [currentPage, setCurrentPage] = useState(1)
	const pageSize = 10
	const [selectedOrganizations, setSelectedOrganizations] = useState<HandlerOrganizationResponse[]>([])
	const [bulkDeleteOpen, setBulkDeleteOpen] = useState(false)

	const { data: providers } = useQuery({
		...getKeyProvidersOptions(),
	})

	// API query with pagination parameters
	const {
		data: organizations,
		isLoading,
		refetch,
	} = useQuery({
		...getOrganizationsOptions({
			query: {
				limit: pageSize,
				offset: (currentPage - 1) * pageSize,
			},
		} as any),
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
				country: Array.isArray(data.caParams?.country) && data.caParams.country.length > 0 ? data.caParams.country : ['US'],
				province: Array.isArray(data.caParams?.province) && data.caParams.province.length > 0 ? data.caParams.province : ['California'],
				locality: Array.isArray(data.caParams?.locality) && data.caParams.locality.length > 0 ? data.caParams.locality : ['San Francisco'],
				streetAddress: Array.isArray(data.caParams?.streetAddress) && data.caParams.streetAddress.length > 0 ? data.caParams.streetAddress : [''],
				postalCode: Array.isArray(data.caParams?.postalCode) && data.caParams.postalCode.length > 0 ? data.caParams.postalCode : [],
			},
		})
	}

	const handleDelete = (org: HandlerOrganizationResponse) => {
		setOrgToDelete(org)
	}

	const handleSelectAll = (checked: boolean) => {
		if (checked) {
			setSelectedOrganizations(organizations?.items || [])
		} else {
			setSelectedOrganizations([])
		}
	}

	const handleSelectOne = (org: HandlerOrganizationResponse, checked: boolean) => {
		if (checked) {
			setSelectedOrganizations((prev) => [...prev, org])
		} else {
			setSelectedOrganizations((prev) => prev.filter((o) => o.id !== org.id))
		}
	}

	const handleBulkDelete = () => {
		setBulkDeleteOpen(true)
	}

	const confirmBulkDelete = async () => {
		try {
			await Promise.all(selectedOrganizations.map((org) => deleteOrganization.mutateAsync({ path: { id: org.id! } })))
			toast.success('Organizations deleted successfully')
			setSelectedOrganizations([])
			refetch()
		} catch (error: any) {
			toast.error('Failed to delete organizations', { description: error.message })
		} finally {
			setBulkDeleteOpen(false)
		}
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

	if (!organizations?.items?.length) {
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

				{organizations?.items?.length > 0 && (
					<div className="flex items-center px-4 py-2 border rounded-lg bg-background mb-4">
						<Checkbox checked={selectedOrganizations.length === organizations.items.length && organizations.items.length > 0} onCheckedChange={handleSelectAll} className="mr-4" />
						<span className="text-sm text-muted-foreground">Select All</span>
						{selectedOrganizations.length > 0 && (
							<DropdownMenu>
								<DropdownMenuTrigger asChild>
									<Button variant="outline" className="ml-4">
										Bulk Actions ({selectedOrganizations.length})
										<ChevronDown className="ml-2 h-4 w-4" />
									</Button>
								</DropdownMenuTrigger>
								<DropdownMenuContent align="end">
									<DropdownMenuItem onClick={handleBulkDelete} className="text-destructive">
										Delete
									</DropdownMenuItem>
								</DropdownMenuContent>
							</DropdownMenu>
						)}
					</div>
				)}

				<div className="space-y-4">
					{organizations?.items?.map((org) => (
						<OrganizationItem
							organization={org}
							onDelete={handleDelete}
							checked={selectedOrganizations.some((o) => o.id === org.id)}
							onCheckedChange={(checked) => handleSelectOne(org, !!checked)}
						/>
					))}
				</div>

				{organizations && typeof organizations.count === 'number' && organizations.count > pageSize && (
					<div className="mt-8 flex justify-center">
						<Pagination
							currentPage={currentPage}
							pageSize={pageSize}
							totalItems={organizations.count}
							onPageChange={setCurrentPage}
						/>
					</div>
				)}
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

			<AlertDialog open={bulkDeleteOpen} onOpenChange={setBulkDeleteOpen}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Confirm Bulk Delete</AlertDialogTitle>
						<AlertDialogDescription>
							Are you sure you want to delete the following organizations?
							<ul className="list-disc pl-4 mt-2 space-y-1">
								{selectedOrganizations.map((org) => (
									<li key={org.id} className="text-sm">
										{org.mspId}
									</li>
								))}
							</ul>
							<p className="text-destructive mt-2">This action cannot be undone. This will permanently delete the selected organizations and all associated keys.</p>
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction onClick={confirmBulkDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
							Delete
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	)
}

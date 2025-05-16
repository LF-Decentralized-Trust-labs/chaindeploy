import { ModelsKeyResponse } from '@/api/client'
import { deleteKeysByIdMutation, getKeyProvidersOptions, getKeysOptions, postKeysMutation } from '@/api/client/@tanstack/react-query.gen'
import { KeyFormValues } from '@/components/forms/key-form'
import { CreateKeyDialog } from '@/components/keys/create-key-dialog'
import { KeyItem } from '@/components/keys/key-item'
import { KeySkeleton } from '@/components/keys/key-skeleton'
import { ProviderFilter } from '@/components/keys/provider-filter'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Key } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { Pagination } from '@/components/ui/pagination'

export default function KeyManagementPage() {
	const [open, setOpen] = useState(false)
	const [keyToDelete, setKeyToDelete] = useState<ModelsKeyResponse | null>(null)
	const [selectedProvider, setSelectedProvider] = useState<number | null>(null)
	const [currentPage, setCurrentPage] = useState(1)
	const pageSize = 10
	const navigate = useNavigate()

	useEffect(() => {
		// Check if user is authenticated
		const isAuthenticated = localStorage.getItem('isAuthenticated')
		if (!isAuthenticated) {
			navigate('/login')
		}
	}, [navigate])

	const { data: providers, isLoading: isLoadingProviders } = useQuery({
		...getKeyProvidersOptions(),
	})

	const {
		data: keys,
		isLoading: isLoadingKeys,
		refetch,
	} = useQuery({
		...getKeysOptions({
			query: {
				// providerId: selectedProvider || undefined,
				page: currentPage,
				pageSize: pageSize,
			},
		}),
	})

	const createKey = useMutation({
		...postKeysMutation(),
		onSuccess: () => {
			toast.success('Key created successfully')
			refetch()
			setOpen(false)
		},
		onError: (error) => {
			if (error instanceof Error) {
				toast.error(`An error occurred: ${error.message}`)
			} else {
				toast.error('An unknown error occurred')
			}
		},
	})

	const deleteKey = useMutation({
		...deleteKeysByIdMutation(),
		onSuccess: () => {
			toast.success('Key deleted successfully')
			refetch()
			setKeyToDelete(null)
		},
		onError: (error) => {
			if (error instanceof Error) {
				toast.error(`An error occurred: ${error.message}`)
			} else {
				toast.error('An unknown error occurred')
			}
		},
	})

	const onSubmit = (data: KeyFormValues) => {
		createKey.mutate({
			body: {
				name: data.name,
				algorithm: data.type,
				curve: data.curve,
				keySize: data.keySize,
				providerId: data.providerId,
			},
		})
	}

	const handleDelete = (key: ModelsKeyResponse) => {
		setKeyToDelete(key)
	}

	const handlePageChange = (page: number) => {
		setCurrentPage(page)
	}

	const EmptyState = () => {
		if (selectedProvider) {
			const providerName = providers?.find((p) => p.id === selectedProvider)?.name
			return (
				<div className="text-center">
					<div className="flex justify-center mb-4">
						<Key className="h-12 w-12 text-muted-foreground" />
					</div>
					<h1 className="text-2xl font-semibold mb-2">No keys found</h1>
					<p className="text-muted-foreground mb-8">No cryptographic keys found for provider "{providerName}".</p>
					<div className="flex items-center justify-center gap-4">
						<Button variant="outline" onClick={() => setSelectedProvider(null)}>
							Show all providers
						</Button>
						<CreateKeyDialog open={open} onOpenChange={setOpen} onSubmit={onSubmit} isSubmitting={createKey.isPending} />
					</div>
				</div>
			)
		}

		return (
			<div className="text-center">
				<div className="flex justify-center mb-4">
					<Key className="h-12 w-12 text-muted-foreground" />
				</div>
				<h1 className="text-2xl font-semibold mb-2">No keys found</h1>
				<p className="text-muted-foreground mb-8">Create your first cryptographic key to get started.</p>
				<CreateKeyDialog open={open} onOpenChange={setOpen} onSubmit={onSubmit} isSubmitting={createKey.isPending} />
			</div>
		)
	}

	return (
		<div className="flex-1 p-8">
			<div className="max-w-4xl mx-auto">
				{isLoadingKeys ? (
					<>
						<div className="flex items-center justify-between mb-8">
							<div className="space-y-2">
								<Skeleton className="h-8 w-32" />
								<Skeleton className="h-5 w-64" />
							</div>
							<div className="flex items-center gap-4">
								<ProviderFilter providers={providers || []} selectedProvider={selectedProvider} onProviderChange={setSelectedProvider} isLoading={isLoadingProviders} />
								<Skeleton className="h-10 w-32" />
							</div>
						</div>
						<div className="space-y-4">
							{[1, 2, 3].map((i) => (
								<KeySkeleton key={i} />
							))}
						</div>
					</>
				) : keys?.items?.length === 0 ? (
					<EmptyState />
				) : (
					<>
						<div className="flex items-center justify-between mb-8">
							<div>
								<h1 className="text-2xl font-semibold">Keys</h1>
								<p className="text-muted-foreground">Manage cryptographic keys for your network</p>
							</div>
							<div className="flex items-center gap-4">
								<ProviderFilter providers={providers} selectedProvider={selectedProvider} onProviderChange={setSelectedProvider} isLoading={isLoadingProviders} />
								<CreateKeyDialog open={open} onOpenChange={setOpen} onSubmit={onSubmit} isSubmitting={createKey.isPending} />
							</div>
						</div>

						<div className="space-y-4">
							{keys?.items?.map((key) => (
								<div key={key.id}>
									<KeyItem 
										key={key.id} 
										keyResponse={key} 
										onDelete={handleDelete}
										createdAt={key.createdAt}
									/>
								</div>
							))}
						</div>

						{keys && keys.totalItems && (
							<div className="mt-8 flex justify-center">
								<Pagination 
									currentPage={currentPage} 
									pageSize={pageSize} 
									totalItems={keys.totalItems} 
									onPageChange={handlePageChange} 
								/>
							</div>
						)}
					</>
				)}
			</div>

			<AlertDialog open={!!keyToDelete} onOpenChange={(open) => !open && setKeyToDelete(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Are you sure?</AlertDialogTitle>
						<AlertDialogDescription>This action cannot be undone. This will permanently delete the key.</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction disabled={deleteKey.isPending} onClick={() => keyToDelete && deleteKey.mutate({ path: { id: keyToDelete.id! } })}>
							Delete
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	)
}

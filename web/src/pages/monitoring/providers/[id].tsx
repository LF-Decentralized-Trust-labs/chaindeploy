import { HttpUpdateProviderRequest } from '@/api/client'
import { getNotificationsProvidersByIdOptions, putNotificationsProvidersByIdMutation } from '@/api/client/@tanstack/react-query.gen'
import { ProviderForm } from '@/components/settings/notifications/provider-form'
import { Skeleton } from '@/components/ui/skeleton'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'

export default function UpdateProviderPage() {
	const navigate = useNavigate()
	const { id } = useParams()

	// Fetch existing provider data
	const { data: provider, isLoading } = useQuery({
		...getNotificationsProvidersByIdOptions({
			path: { id: Number(id) },
			cache: 'no-cache',
		}),
	})

	const mutation = useMutation({
		...putNotificationsProvidersByIdMutation(),
		onSuccess: () => {
			toast.success('Provider updated successfully')
			navigate('/monitoring')
		},
		onError: (error) => {
			if (error instanceof Error) {
				toast.error('Failed to update provider', {
					description: error.message,
				})
			} else if (error.error) {
				toast.error('Failed to update provider', {
					description: (error.error as any).message,
				})
			}
		},
	})
	const providerConfig = useMemo(() => {
		return provider.config as any
	}, [provider])
	if (isLoading) {
		return <Skeleton className="h-48" />
	}

	if (!provider) {
		return (
			<div className="container py-6">
				<p className="text-muted-foreground">Provider not found</p>
			</div>
		)
	}

	return (
		<div className="container space-y-6">
			<div>
				<h1 className="text-2xl font-semibold tracking-tight">Edit Provider</h1>
				<p className="text-sm text-muted-foreground">Update SMTP settings and notification preferences</p>
			</div>

			<ProviderForm
				defaultValues={{
					isDefault: provider?.isDefault ?? false,
					name: provider?.name ?? '',
					type: provider?.type ?? 'SMTP',
					config: {
						host: providerConfig?.host ?? '',
						port: providerConfig?.port ?? 587,
						username: providerConfig?.username ?? '',
						password: providerConfig?.password ?? '',
						from: providerConfig?.from ?? '',
						tls: providerConfig?.tls ?? true,
					},
					notifyNodeDowntime: provider?.notifyNodeDowntime ?? true,
					notifyBackupSuccess: provider?.notifyBackupSuccess ?? false,
					notifyBackupFailure: provider?.notifyBackupFailure ?? true,
					notifyS3ConnIssue: provider?.notifyS3ConnIssue ?? true,
				}}
				onSubmit={async (values) => {
					mutation.mutateAsync({
						path: { id: Number(id) },
						body: values as HttpUpdateProviderRequest,
					})
				}}
				submitText="Update Provider"
				onCancel={() => navigate(-1)}
				isLoading={mutation.isPending}
			/>
		</div>
	)
}

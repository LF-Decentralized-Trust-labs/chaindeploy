import { HttpCreateProviderRequest } from '@/api/client'
import { postNotificationsProvidersMutation } from '@/api/client/@tanstack/react-query.gen'
import { ProviderForm, ProviderFormValues } from '@/components/settings/notifications/provider-form'
import { useMutation } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'

export default function CreateProviderPage() {
	const navigate = useNavigate()

	const mutation = useMutation({
		...postNotificationsProvidersMutation(),
		onSuccess: () => {
			toast.success('Provider created successfully')
			navigate('/monitoring')
		},
		onError: (error) => {
			if (error.error) {
				toast.error('Failed to create provider', {
					description: (error.error as any).message,
				})
			} else {
				toast.error('Failed to create provider', {
					description: error.toString(),
				})
			}
		},
	})

	const defaultValues: ProviderFormValues = {
		name: '',
		type: 'SMTP',
		isDefault: false,
		config: {
			host: '',
			port: 587,
			username: '',
			password: '',
			from: '',
			tls: true,
		},
		notifyNodeDowntime: true,
		notifyBackupSuccess: true,
		notifyBackupFailure: true,
		notifyS3ConnIssue: true,
	}

	return (
		<div className="container space-y-6 p-4">
			<div>
				<h1 className="text-2xl font-semibold tracking-tight">New Provider</h1>
				<p className="text-sm text-muted-foreground">Configure SMTP settings and notification preferences</p>
			</div>

			<ProviderForm
				defaultValues={defaultValues}
				onSubmit={async (values) => {
					await mutation.mutateAsync({ body: values as HttpCreateProviderRequest })
				}}
				submitText="Create Provider"
				onCancel={() => navigate(-1)}
				isLoading={mutation.isPending}
			/>
		</div>
	)
}

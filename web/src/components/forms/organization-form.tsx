import { ModelsProviderResponse } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'

const formSchema = z.object({
	mspId: z.string().min(1, 'MSP ID is required'),
	description: z.string().optional(),
	providerId: z.number().optional(),
})

export type OrganizationFormValues = z.infer<typeof formSchema>

interface OrganizationFormProps {
	onSubmit: (data: OrganizationFormValues) => void
	isSubmitting?: boolean
	providers?: ModelsProviderResponse[]
}

export function OrganizationForm({ onSubmit, isSubmitting, providers }: OrganizationFormProps) {
	const form = useForm<OrganizationFormValues>({
		resolver: zodResolver(formSchema),
		defaultValues: {
			mspId: '',
			description: '',
			providerId: providers && providers.length > 0 ? providers[0].id : undefined,
		},
	})

	// Set the first provider as default when providers are loaded
	useEffect(() => {
		if (providers && providers.length > 0 && !form.getValues('providerId')) {
			form.setValue('providerId', providers[0].id)
		}
	}, [providers, form])

	return (
		<Form {...form}>
			<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
				<FormField
					control={form.control}
					name="mspId"
					render={({ field }) => (
						<FormItem>
							<FormLabel>MSP ID</FormLabel>
							<FormControl>
								<Input autoComplete="off" autoFocus placeholder="Enter MSP ID" {...field} />
							</FormControl>
							<FormMessage />
						</FormItem>
					)}
				/>

				<FormField
					control={form.control}
					name="description"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Description</FormLabel>
							<FormControl>
								<Textarea placeholder="Enter organization description" {...field} />
							</FormControl>
							<FormMessage />
						</FormItem>
					)}
				/>

				{providers && providers.length > 0 && (
					<FormField
						control={form.control}
						name="providerId"
						render={({ field }) => (
							<FormItem>
								<FormLabel>Key Provider</FormLabel>
								<Select onValueChange={(value) => field.onChange(Number(value))} value={field.value?.toString()}>
									<FormControl>
										<SelectTrigger>
											<SelectValue placeholder="Select key provider" />
										</SelectTrigger>
									</FormControl>
									<SelectContent>
										{providers.map((provider) => (
											<SelectItem key={provider.id} value={provider.id!.toString()}>
												{provider.name}
											</SelectItem>
										))}
									</SelectContent>
								</Select>
								<FormMessage />
							</FormItem>
						)}
					/>
				)}

				<Button disabled={isSubmitting} type="submit" className="w-full">
					Create Organization
				</Button>
			</form>
		</Form>
	)
}

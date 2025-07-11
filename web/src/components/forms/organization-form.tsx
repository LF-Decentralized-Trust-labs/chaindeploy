import { ModelsProviderResponse } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect, useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { ArrayFieldInput } from './ArrayFieldInput'

const formSchema = z.object({
	mspId: z.string().min(1, 'MSP ID is required'),
	description: z.string().optional(),
	providerId: z.number().optional(),
	caParams: z
		.object({
			commonName: z.string().optional(),
			country: z.array(z.string()).optional(),
			province: z.array(z.string()).optional(),
			locality: z.array(z.string()).optional(),
			streetAddress: z.array(z.string()).optional(),
			postalCode: z.array(z.string()).optional(),
		})
		.optional(),
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
			caParams: undefined,
		},
	})

	const caEnabled = useMemo(() => form.watch('caParams') !== undefined, [form.watch('caParams')])

	// Set the first provider as default when providers are loaded
	useEffect(() => {
		if (providers && providers.length > 0 && !form.getValues('providerId')) {
			form.setValue('providerId', providers[0].id)
		}
	}, [providers, form])

	return (
		<Form {...form}>
			<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4 max-h-[70vh] overflow-y-auto pr-2">
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

				{/* CA Parameters Switch */}
				<div className="flex items-center gap-2">
					<FormLabel>Customize CA Parameters</FormLabel>
					<FormControl>
						<Switch
							checked={caEnabled}
							onCheckedChange={(checked) => {
								if (checked) {
									form.setValue('caParams', {
										commonName: '',
										country: [],
										province: [],
										locality: [],
										streetAddress: [],
										postalCode: [],
									})
								} else {
									form.setValue('caParams', undefined)
								}
							}}
						/>
					</FormControl>
				</div>

				{caEnabled && (
					<div className="space-y-4 border rounded-lg p-4 bg-muted/20">
						<FormField
							control={form.control}
							name="caParams.commonName"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Common Name</FormLabel>
									<FormControl>
										<Input placeholder="Enter Common Name" {...field} />
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>
						<ArrayFieldInput control={form.control} name="caParams.country" label="Country" placeholder="Enter country" />
						<ArrayFieldInput control={form.control} name="caParams.province" label="Province" placeholder="Enter province" />
						<ArrayFieldInput control={form.control} name="caParams.locality" label="Locality" placeholder="Enter locality" />
						<ArrayFieldInput control={form.control} name="caParams.streetAddress" label="Street Address" placeholder="Enter street address" />
						<ArrayFieldInput control={form.control} name="caParams.postalCode" label="Postal Code" placeholder="Enter postal code" />
					</div>
				)}

				<Button disabled={isSubmitting} type="submit" className="w-full">
					Create Organization
				</Button>
			</form>
		</Form>
	)
}

import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { getKeyProvidersOptions } from '@/api/client/@tanstack/react-query.gen'
import { useQuery } from '@tanstack/react-query'

const keyTypes = ['RSA', 'EC', 'ED25519'] as const
const curves = ['P-256', 'P-384', 'P-521', 'secp256k1'] as const
const keySizes = [2048, 3072, 4096] as const

const baseSchema = z.object({
	name: z.string().min(1, 'Name is required'),
	type: z.enum(keyTypes),
	curve: z.enum(curves).optional().default('P-256'),
	keySize: z.number().optional().default(2048),
	providerId: z.number(),
})

export type KeyFormValues = z.infer<typeof baseSchema>

interface KeyFormProps {
	onSubmit: (data: KeyFormValues) => void
	isSubmitting?: boolean
}

export function KeyForm({ onSubmit, isSubmitting }: KeyFormProps) {
	const form = useForm<KeyFormValues>({
		resolver: zodResolver(baseSchema),
		defaultValues: {
			name: '',
			type: 'RSA',
			keySize: 2048,
			curve: 'P-256',
		},
	})
	const { data: providers, isLoading: isLoadingProviders } = useQuery({
		...getKeyProvidersOptions(),
	})

	const keyType = form.watch('type')

	return (
		<Form {...form}>
			<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
				<FormField
					control={form.control}
					name="name"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Key Name</FormLabel>
							<FormControl>
								<Input autoComplete={'off'} autoFocus={true} placeholder="Enter key name" {...field} />
							</FormControl>
							<FormMessage />
						</FormItem>
					)}
				/>

				<FormField
					control={form.control}
					name="type"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Key Type</FormLabel>
							<Select onValueChange={field.onChange} value={field.value}>
								<FormControl>
									<SelectTrigger>
										<SelectValue placeholder="Select key type" />
									</SelectTrigger>
								</FormControl>
								<SelectContent>
									{keyTypes.map((type) => (
										<SelectItem key={type} value={type}>
											{type}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
							<FormMessage />
						</FormItem>
					)}
				/>

				{keyType === 'EC' && (
					<FormField
						control={form.control}
						name="curve"
						render={({ field }) => (
							<FormItem>
								<FormLabel>Curve</FormLabel>
								<Select onValueChange={field.onChange} value={field.value}>
									<FormControl>
										<SelectTrigger>
											<SelectValue placeholder="Select curve" />
										</SelectTrigger>
									</FormControl>
									<SelectContent>
										{curves.map((curve) => (
											<SelectItem key={curve} value={curve}>
												{curve}
											</SelectItem>
										))}
									</SelectContent>
								</Select>
								<FormMessage />
							</FormItem>
						)}
					/>
				)}

				{keyType === 'RSA' && (
					<FormField
						control={form.control}
						name="keySize"
						render={({ field }) => (
							<FormItem>
								<FormLabel>Key Size</FormLabel>
								<Select onValueChange={(value) => field.onChange(Number(value))} value={field.value?.toString()}>
									<FormControl>
										<SelectTrigger>
											<SelectValue placeholder="Select key size" />
										</SelectTrigger>
									</FormControl>
									<SelectContent>
										{keySizes.map((size) => (
											<SelectItem key={size} value={size.toString()}>
												{size} bits
											</SelectItem>
										))}
									</SelectContent>
								</Select>
								<FormMessage />
							</FormItem>
						)}
					/>
				)}

				<FormField
					control={form.control}
					name="providerId"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Provider</FormLabel>
							<Select disabled={isLoadingProviders} onValueChange={(value) => field.onChange(Number(value))} value={field.value?.toString()}>
								<FormControl>
									<SelectTrigger>
										<SelectValue placeholder="Select provider" />
									</SelectTrigger>
								</FormControl>
								<SelectContent>
									{providers?.map((provider) => (
										<SelectItem key={provider.id} value={provider.id?.toString()!}>
											{provider.name}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
							<FormMessage />
						</FormItem>
					)}
				/>

				<Button disabled={isSubmitting} type="submit" className="w-full">
					Create Key
				</Button>
			</form>
		</Form>
	)
}

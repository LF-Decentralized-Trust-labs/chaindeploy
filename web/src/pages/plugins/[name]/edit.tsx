import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { useNavigate, useParams } from 'react-router-dom'
import { parse, stringify } from 'yaml'
import * as z from 'zod'

import { getPluginsByNameOptions, putPluginsByNameMutation } from '@/api/client/@tanstack/react-query.gen'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Textarea } from '@/components/ui/textarea'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { toast } from 'sonner'

const formSchema = z.object({
	yaml: z
		.string()
		.min(1, 'YAML is required')
		.refine((yaml) => {
			try {
				const parsed = parse(yaml)
				// Add basic validation for required fields
				if (!parsed.apiVersion || !parsed.kind || !parsed.metadata?.name) {
					return false
				}
				return true
			} catch (e) {
				return false
			}
		}, 'Invalid YAML format or missing required fields'),
})

type FormValues = z.infer<typeof formSchema>

export default function EditPluginPage() {
	const { name } = useParams()
	const navigate = useNavigate()

	// Fetch current plugin data
	const { data: plugin, isLoading } = useQuery({
		...getPluginsByNameOptions({
			path: { name: name! },
		}),
	})

	// Update mutation
	const updateMutation = useMutation({
		...putPluginsByNameMutation(),
		onSuccess: () => {
			toast.success('Plugin updated successfully')
			navigate(`/plugins/${name}`)
		},
		onError: (error: any) => {
			if (error?.data?.detail) {
				toast.error(error.data.detail)
			} else if (error.message) {
				toast.error(error.message)
			} else {
				toast.error('Failed to update plugin. Please check your YAML format and try again.')
			}
		},
	})

	const form = useForm<FormValues>({
		resolver: zodResolver(formSchema),
		defaultValues: {
			yaml: plugin ? stringify(plugin) : '',
		},
	})

	// Update form when plugin data is loaded
	useEffect(() => {
		if (plugin) {
			form.reset({ yaml: stringify(plugin) })
		}
	}, [plugin, form])

	async function onSubmit(data: FormValues) {
		try {
			const pluginData = parse(data.yaml)
			await updateMutation.mutateAsync({
				path: { name: name! },
				body: pluginData,
			})
		} catch (error) {
			toast.error('Failed to update plugin. Please check your YAML format and try again.')
		}
	}

	if (isLoading) return <div className="container p-8">Loading...</div>
	if (!plugin) return <div className="container p-8">Plugin not found</div>

	return (
		<div className="container mx-auto py-6">
			<Card>
				<CardHeader>
					<CardTitle>Edit Plugin: {name}</CardTitle>
					<CardDescription>Update the plugin configuration by modifying the YAML below</CardDescription>
				</CardHeader>
				<CardContent>
					<Form {...form}>
						<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
							<FormField
								control={form.control}
								name="yaml"
								render={({ field }) => (
									<FormItem>
										<FormLabel>Plugin Configuration (YAML)</FormLabel>
										<FormControl>
											<Textarea {...field} className="font-mono min-h-[500px]" placeholder="Enter your plugin YAML configuration..." />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>

							<div className="flex justify-end space-x-4">
								<Button type="button" variant="outline" onClick={() => navigate(`/plugins/${name}`)}>
									Cancel
								</Button>
								<Button type="submit">Update Plugin</Button>
							</div>
						</form>
					</Form>
				</CardContent>
			</Card>
		</div>
	)
}

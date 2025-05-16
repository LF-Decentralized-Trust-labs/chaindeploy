import { ServiceSettingConfig } from '@/api/client'
import { getSettingsOptions, postSettingsMutation } from '@/api/client/@tanstack/react-query.gen'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import * as z from 'zod'

const formSchema = z.object({
	peerTemplateCMD: z.string().optional(),
	ordererTemplateCMD: z.string().optional(),
	besuTemplateCMD: z.string().optional(),
})

function TemplatesSection({ form }: { form: any }) {
	return (
		<div className="space-y-6">
			<div>
				<h3 className="text-lg font-medium">Node Templates</h3>
				<p className="text-sm text-muted-foreground">Configure command templates for different node types.</p>
			</div>
			<Separator />
			<div className="space-y-6">
				<FormField
					control={form.control}
					name="peerTemplateCMD"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Peer Template Command</FormLabel>
							<FormControl>
								<Textarea placeholder="Enter peer template command..." {...field} />
							</FormControl>
							<FormDescription>The command template used for Fabric peer nodes.</FormDescription>
							<FormMessage />
						</FormItem>
					)}
				/>

				<FormField
					control={form.control}
					name="ordererTemplateCMD"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Orderer Template Command</FormLabel>
							<FormControl>
								<Textarea placeholder="Enter orderer template command..." {...field} />
							</FormControl>
							<FormDescription>The command template used for Fabric orderer nodes.</FormDescription>
							<FormMessage />
						</FormItem>
					)}
				/>

				<FormField
					control={form.control}
					name="besuTemplateCMD"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Besu Template Command</FormLabel>
							<FormControl>
								<Textarea placeholder="Enter besu template command..." {...field} />
							</FormControl>
							<FormDescription>The command template used for Besu nodes.</FormDescription>
							<FormMessage />
						</FormItem>
					)}
				/>
			</div>
		</div>
	)
}

export default function SettingsPage() {
	const { data: settings, isLoading } = useQuery({
		...getSettingsOptions(),
	})

	const form = useForm<ServiceSettingConfig>({
		resolver: zodResolver(formSchema),
		values: settings?.config || {
			peerTemplateCMD: '',
			ordererTemplateCMD: '',
			besuTemplateCMD: '',
		},
	})
	useEffect(() => {
		form.reset(settings?.config)
	}, [settings])

	const updateSettings = useMutation({
		...postSettingsMutation(),
		onSuccess: () => {
			toast.success('Settings updated successfully')
		},
		onError: (error: any) => {
			if (error instanceof Error) {
				toast.error(`Failed to update settings: ${error.message}`)
			} else if (error.message) {
				toast.error(`Failed to update settings: ${error.message}`)
			} else {
				toast.error('An unknown error occurred')
			}
		},
	})

	function onSubmit(data: ServiceSettingConfig) {
		updateSettings.mutate({
			body: {
				config: data,
			},
		})
	}

	if (isLoading) {
		return (
			<div className="flex-1 space-y-4 p-8 pt-6">
				<div className="flex items-center justify-between space-y-2">
					<h2 className="text-3xl font-bold tracking-tight">Settings</h2>
				</div>
				<div className="hidden h-full flex-1 flex-col space-y-8 md:flex">
					<div>Loading...</div>
				</div>
			</div>
		)
	}

	return (
		<div className="flex-1 space-y-4 p-8 pt-6">
			<div className="flex items-center justify-between space-y-2">
				<h2 className="text-3xl font-bold tracking-tight">Settings</h2>
			</div>
			<Tabs defaultValue="templates" className="space-y-4">
				<TabsList>
					<TabsTrigger value="templates">Templates</TabsTrigger>
					{/* Add more tabs here as needed */}
				</TabsList>
				<TabsContent value="templates" className="space-y-4">
					<div className="grid gap-4 md:grid-cols-2 lg:grid-cols-7">
						<Card className="col-span-4">
							<CardHeader>
								<CardTitle>Settings</CardTitle>
							</CardHeader>
							<CardContent>
								<Form {...form}>
									<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-8">
										<TemplatesSection form={form} />
										<Button type="submit" disabled={updateSettings.isPending}>
											{updateSettings.isPending ? 'Saving...' : 'Save changes'}
										</Button>
									</form>
								</Form>
							</CardContent>
						</Card>
					</div>
				</TabsContent>
			</Tabs>
		</div>
	)
}

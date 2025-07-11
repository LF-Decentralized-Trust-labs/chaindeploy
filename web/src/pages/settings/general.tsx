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
	defaultNodeExposeIP: z.string().optional(),
	peerTemplateCMD: z.string().optional(),
	ordererTemplateCMD: z.string().optional(),
	besuTemplateCMD: z.string().optional(),
})

function ExternalIPSection({ form }: { form: any }) {
	return (
		<div className="space-y-6">
			<div>
				<h3 className="text-lg font-medium">External IP Management</h3>
				<p className="text-sm text-muted-foreground">Configure the default external IP address used for node endpoints. This IP will be used as the default for external endpoints when creating new nodes.</p>
			</div>
			<Separator />
			<FormField
				control={form.control}
				name="defaultNodeExposeIP"
				render={({ field }) => (
					<FormItem>
						<FormLabel>Default External IP</FormLabel>
						<FormControl>
							<input
								type="text"
								placeholder="Enter external IP address..."
								className="input w-full border border-border bg-background rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
								{...field}
							/>
						</FormControl>
						<FormDescription>This IP will be used as the default for external endpoints.</FormDescription>
						<FormMessage />
					</FormItem>
				)}
			/>
		</div>
	)
}

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
			defaultNodeExposeIP: '',
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
										<ExternalIPSection form={form} />
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

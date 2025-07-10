import { getAiBoilerplatesOptions } from '@/api/client/@tanstack/react-query.gen'
import { postChaincodeProjects } from '@/api/client/sdk.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate } from 'react-router-dom'
import * as z from 'zod'

const developChaincodeFormSchema = z.object({
	name: z
		.string()
		.min(1, 'Name is required')
		.regex(/^[a-z0-9_-]+$/, 'Name must be lowercase, no spaces, and can only contain letters, numbers, hyphens, or underscores')
		.transform((val) => val.toLowerCase()),
	networkId: z.string().min(1, 'Network is required'),
	boilerplate: z.string().min(1, 'Boilerplate is required'),
	description: z.string().min(1, 'Description is required'),
	endorsementPolicy: z.string().min(1, 'Endorsement policy is required'),
})

export type DevelopChaincodeFormValues = z.infer<typeof developChaincodeFormSchema>

interface DevelopChaincodeDialogProps {
	networks: { id: number; name: string }[] | undefined
	refetch?: () => void
}

export function DevelopChaincodeDialog({ networks, refetch }: DevelopChaincodeDialogProps) {
	const [open, setOpen] = useState(false)
	const [selectedNetworkId, setSelectedNetworkId] = useState<string>('')
	const [error, setError] = useState<string | null>(null)
	const navigate = useNavigate()

	const form = useForm<DevelopChaincodeFormValues>({
		resolver: zodResolver(developChaincodeFormSchema),
		defaultValues: {
			name: '',
			networkId: '',
			boilerplate: '',
			description: '',
			endorsementPolicy: '',
		},
	})

	// Fetch boilerplates for selected network
	const { data: boilerplates } = useQuery({
		...getAiBoilerplatesOptions({
			query: { network_id: Number(selectedNetworkId) || 0 },
		}),
		enabled: !!selectedNetworkId,
	})

	// Develop chaincode mutation
	const developChaincodeMutation = useMutation({
		mutationFn: async (data: DevelopChaincodeFormValues) => {
			const response = await postChaincodeProjects({
				body: {
					name: data.name,
					networkId: Number(data.networkId),
					boilerplate: data.boilerplate,
					description: data.description,
					endorsementPolicy: data.endorsementPolicy,
				},
			})
			return response.data
		},
		onSuccess: (data) => {
			setOpen(false)
			setError(null)
			form.reset()
			if (refetch) refetch()
			if (data.id) {
				navigate(`/sc/fabric/projects/chaincodes/${data.id}`)
			}
		},
		onError: (error: any) => {
			let message = 'Failed to create chaincode project.'
			if (error?.response?.data?.message) {
				message = error.response.data.message
			} else if (error?.message) {
				message = error.message
			}
			setError(message)
		},
	})

	const handleOpenChange = (open: boolean) => {
		setOpen(open)
		if (!open) {
			setSelectedNetworkId('')
			setError(null)
			form.reset()
		}
	}

	return (
		<Dialog open={open} onOpenChange={handleOpenChange}>
			<DialogTrigger asChild>
				<Button variant="outline">
					<Plus className="mr-2 h-4 w-4" />
					Develop Chaincode
				</Button>
			</DialogTrigger>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>Develop Chaincode</DialogTitle>
					<DialogDescription>Create a new chaincode project with a boilerplate.</DialogDescription>
				</DialogHeader>
				<Form {...form}>
					<form
						onSubmit={form.handleSubmit((data) => {
							setError(null)
							developChaincodeMutation.mutate(data)
						})}
						className="space-y-4"
					>
						<FormField
							control={form.control}
							name="name"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Name</FormLabel>
									<FormControl>
										<Input placeholder="Enter chaincode name" {...field} />
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>
						<FormField
							control={form.control}
							name="networkId"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Network</FormLabel>
									<Select
										onValueChange={(val) => {
											field.onChange(val)
											setSelectedNetworkId(val)
											form.setValue('boilerplate', '')
										}}
										defaultValue={field.value}
									>
										<FormControl>
											<SelectTrigger>
												<SelectValue placeholder="Select a network" />
											</SelectTrigger>
										</FormControl>
										<SelectContent>
											{networks?.map((network) => (
												<SelectItem key={network.id} value={network.id.toString()}>
													{network.name}
												</SelectItem>
											))}
										</SelectContent>
									</Select>
									<FormMessage />
								</FormItem>
							)}
						/>
						<FormField
							control={form.control}
							name="boilerplate"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Boilerplate</FormLabel>
									<Select onValueChange={field.onChange} defaultValue={field.value} disabled={!selectedNetworkId}>
										<FormControl>
											<SelectTrigger>
												<SelectValue placeholder="Select a boilerplate" />
											</SelectTrigger>
										</FormControl>
										<SelectContent>
											{boilerplates
												?.filter((b) => b.id && b.name)
												.map((boilerplate) => (
													<SelectItem key={boilerplate.id} value={boilerplate.id}>
														{boilerplate.name}
													</SelectItem>
												))}
										</SelectContent>
									</Select>
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
										<Input placeholder="Enter chaincode description" {...field} />
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>
						<FormField
							control={form.control}
							name="endorsementPolicy"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Endorsement Policy</FormLabel>
									<FormControl>
										<Input placeholder="e.g. OR('Org1MSP.member')" {...field} />
									</FormControl>
									<FormMessage />
									<div className="text-xs text-muted-foreground mt-1">
										Example policies:
										<ul className="list-disc list-inside mt-1">
											<li>OR('Org1MSP.member') - Any member of Org1</li>
											<li>AND('Org1MSP.member', 'Org2MSP.member') - Both Org1 and Org2 members</li>
											<li>OR('Org1MSP.member', 'Org2MSP.member') - Any member of Org1 or Org2</li>
										</ul>
									</div>
								</FormItem>
							)}
						/>
						{error && <div className="text-sm text-red-500">{error}</div>}
						<DialogFooter>
							<Button type="submit" disabled={developChaincodeMutation.isPending}>
								{developChaincodeMutation.isPending ? 'Creating...' : 'Create'}
							</Button>
						</DialogFooter>
					</form>
				</Form>
			</DialogContent>
		</Dialog>
	)
}

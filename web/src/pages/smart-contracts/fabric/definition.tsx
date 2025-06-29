import { getNetworksFabricByIdNodesOptions, getScFabricChaincodesByIdOptions } from '@/api/client/@tanstack/react-query.gen'
import { getScFabricPeerByPeerIdChaincodeSequence, postScFabricChaincodesByChaincodeIdDefinitions } from '@/api/client/sdk.gen'
import { ChaincodeDefinitionCard } from '@/components/fabric/ChaincodeDefinitionCard'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useNavigate, useParams, Link } from 'react-router-dom'
import { toast } from 'sonner'
import * as z from 'zod'

const versionFormSchema = z.object({
	endorsementPolicy: z.string().min(1, 'Endorsement policy is required'),
	dockerImage: z.string().min(1, 'Docker image is required'),
	version: z.string().min(1, 'Version is required'),
	sequence: z.number().min(1, 'Sequence must be at least 1'),
	chaincodeAddress: z
		.string()
		.min(1, 'Chaincode address is required')
		.regex(/^(\d{1,3}\.){3}\d{1,3}:(\d{1,5})$/, 'Chaincode address must be in the format host:port, e.g., 127.0.0.1:8080'),
})

type VersionFormValues = z.infer<typeof versionFormSchema>

export default function FabricChaincodeDefinitionDetail() {
	const navigate = useNavigate()
	const { id } = useParams<{ id: string }>()

	const [isAddDialogOpen, setIsAddDialogOpen] = useState(false)
	const [sequenceLoading, setSequenceLoading] = useState(false)
	const [sequenceError, setSequenceError] = useState<string | null>(null)

	// Fetch chaincode details
	const {
		data: chaincodeDetail,
		isLoading,
		error,
		refetch,
	} = useQuery({
		...getScFabricChaincodesByIdOptions({ path: { id: Number(id) } }),
		enabled: !!id,
	})

	const def = useMemo(() => chaincodeDetail?.chaincode, [chaincodeDetail])
	const versions = useMemo(() => (chaincodeDetail?.definitions || []).sort((a, b) => b.sequence - a.sequence), [chaincodeDetail?.definitions])

	// Fetch network peers
	const networkId = useMemo(() => def?.network_id, [def])
	const { data: networkNodesResponse } = useQuery({
		...getNetworksFabricByIdNodesOptions(networkId ? { path: { id: networkId } } : { path: { id: 0 } }),
		enabled: !!networkId,
	})

	const availablePeers = useMemo(() => networkNodesResponse?.nodes?.filter((node) => node.node?.nodeType === 'FABRIC_PEER' && node.status === 'joined') || [], [networkNodesResponse?.nodes])

	const form = useForm<VersionFormValues>({
		resolver: zodResolver(versionFormSchema),
		defaultValues: {
			endorsementPolicy: '',
			dockerImage: '',
			version: '1.0',
			sequence: 1,
			chaincodeAddress: '',
		},
	})
	const createDefinitionMutation = useMutation({
		mutationFn: async (data: VersionFormValues) => {
			if (!id) throw new Error('No chaincode id')
			const response = await postScFabricChaincodesByChaincodeIdDefinitions({
				body: {
					chaincode_id: Number(id),
					docker_image: data.dockerImage,
					endorsement_policy: data.endorsementPolicy,
					version: data.version,
					sequence: data.sequence,
					chaincode_address: data.chaincodeAddress,
				},
			})
			return response.data
		},
		onSuccess: () => {
			refetch()
			setIsAddDialogOpen(false)
			form.reset()
		},
		onError: (error: any) => {
			let message = 'Failed to create chaincode definition.'
			if (error?.response?.data?.message) {
				message = error.response.data.message
			} else if (error?.message) {
				message = error.message
			}
			toast.error(message)
		},
	})

	const onSubmit = async (data: VersionFormValues) => {
		toast.promise(createDefinitionMutation.mutateAsync(data), {
			loading: 'Creating chaincode definition...',
			success: 'Chaincode definition created successfully',
			error: (e) => e.message,
		})
	}

	const handleAddDialogOpenChange = async (open: boolean) => {
		setIsAddDialogOpen(open)
		if (open) {
			setSequenceLoading(true)
			setSequenceError(null)
			try {
				// Use first available peer
				const peer = availablePeers[0]
				if (peer && def?.name && chaincodeDetail?.chaincode.network_name) {
					const resp = await getScFabricPeerByPeerIdChaincodeSequence({
						path: { peerId: String(peer.nodeId) },
						query: { chaincodeName: def.name, channelName: chaincodeDetail.chaincode.network_name },
					})
					const seq = resp.data?.sequence
					if (typeof seq === 'number') {
						form.reset({
							...form.getValues(),
							sequence: seq + 1,
						})
					}
				} else {
					setSequenceError('No available peer or channel to fetch sequence.')
				}
			} catch (err: any) {
				setSequenceError('Failed to fetch sequence')
			} finally {
				setSequenceLoading(false)
			}
		} else {
			form.reset()
			setSequenceError(null)
		}
	}

	if (isLoading) {
		return (
			<Card className="p-6">
				<div className="flex flex-col gap-4">
					{[...Array(2)].map((_, i) => (
						<div key={i} className="p-4 mb-4 border rounded-lg bg-background">
							<div className="flex items-center gap-4 mb-2">
								<div className="w-20 h-6">
									<Skeleton className="w-full h-full" />
								</div>
								<div className="w-24 h-6">
									<Skeleton className="w-full h-full" />
								</div>
							</div>
							<div className="mb-1 text-sm w-1/2">
								<Skeleton className="h-4 w-full" />
							</div>
							<div className="mb-1 text-sm w-1/3">
								<Skeleton className="h-4 w-full" />
							</div>
							<div className="mb-1 text-sm w-1/3">
								<Skeleton className="h-4 w-full" />
							</div>
							<div className="mb-2">
								<div className="font-medium text-sm mb-1">Docker Status</div>
								<div className="flex flex-col gap-1 text-sm">
									<div className="flex items-center gap-2">
										<div className="w-32 h-4">
											<Skeleton className="w-full h-full" />
										</div>
										<div className="w-16 h-4">
											<Skeleton className="w-full h-full" />
										</div>
									</div>
									<div className="w-1/3 h-4">
										<Skeleton className="w-full h-full" />
									</div>
									<div className="w-1/4 h-4">
										<Skeleton className="w-full h-full" />
									</div>
								</div>
							</div>
							<div className="mt-4">
								<div className="text-sm font-medium mb-2">Timeline</div>
								<div className="flex flex-col gap-2">
									{[...Array(2)].map((_, j) => (
										<div key={j} className="flex items-center gap-2">
											<Skeleton className="w-6 h-6 rounded-full" />
											<Skeleton className="h-4 w-32" />
											<Skeleton className="h-4 w-24" />
										</div>
									))}
								</div>
							</div>
						</div>
					))}
				</div>
			</Card>
		)
	}
	if (error || !def) {
		return (
			<Card className="p-6">
				Failed to load chaincode definition.{' '}
				<Button variant="link" onClick={() => navigate(-1)}>
					Back
				</Button>
			</Card>
		)
	}

	return (
		<div className="flex-1 p-8 w-full">
			<Button variant="link" onClick={() => navigate(-1)} className="mb-4">
				Back
			</Button>
			{def?.id && (
				<Link to={`/smart-contracts/fabric/${def.id}/playground`}>
					<Button variant="secondary" className="mb-4 ml-2">
						Open Playground
					</Button>
				</Link>
			)}
			<Card className="p-6 mb-6">
				<div className="font-semibold text-lg mb-2">{def.name}</div>
				<div className="text-sm text-muted-foreground mb-1">Network Name: {def.network_name}</div>
			</Card>
			<div className="flex items-center justify-between mb-4">
				<div className="font-semibold">Chaincode Definitions</div>
				<Dialog open={isAddDialogOpen} onOpenChange={handleAddDialogOpenChange}>
					<DialogTrigger asChild>
						<Button size="sm" variant="secondary">
							<Plus className="w-4 h-4 mr-2" />
							Add Definition
						</Button>
					</DialogTrigger>
					<DialogContent>
						<DialogHeader>
							<DialogTitle>Add Chaincode Definition</DialogTitle>
							<DialogDescription>Create a new chaincode definition with version and sequence.</DialogDescription>
						</DialogHeader>
						<Form {...form}>
							<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
								<div className="grid grid-cols-2 gap-4">
									<FormField
										control={form.control}
										name="version"
										render={({ field }) => (
											<FormItem>
												<FormLabel>Version</FormLabel>
												<FormControl>
													<Input {...field} />
												</FormControl>
												<FormMessage />
											</FormItem>
										)}
									/>
									<FormField
										control={form.control}
										name="sequence"
										render={({ field }) => (
											<FormItem>
												<FormLabel>Sequence</FormLabel>
												<FormControl>
													<Input type="number" {...field} onChange={(e) => field.onChange(parseInt(e.target.value))} disabled={sequenceLoading} />
												</FormControl>
												{sequenceLoading && <div className="text-xs text-muted-foreground">Loading sequence...</div>}
												{sequenceError && <div className="text-xs text-red-500">{sequenceError}</div>}
												<FormMessage />
											</FormItem>
										)}
									/>
								</div>
								<FormField
									control={form.control}
									name="endorsementPolicy"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Endorsement Policy</FormLabel>
											<FormControl>
												<Textarea {...field} />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
								<FormField
									control={form.control}
									name="dockerImage"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Docker Image</FormLabel>
											<FormControl>
												<Input {...field} />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
								<FormField
									control={form.control}
									name="chaincodeAddress"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Chaincode Address</FormLabel>
											<FormControl>
												<Input {...field} />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
								<DialogFooter>
									<Button type="submit">Add Definition</Button>
								</DialogFooter>
							</form>
						</Form>
					</DialogContent>
				</Dialog>
			</div>
			{versions.length === 0 ? (
				<Card className="p-6 text-center text-muted-foreground">No chaincode definitions yet.</Card>
			) : (
				versions.map((v, idx) => <ChaincodeDefinitionCard key={v.id} definition={v} availablePeers={availablePeers} refetch={refetch} />)
			)}
		</div>
	)
}

import { getNodesByIdOptions, putNodesByIdMutation } from '@/api/client/@tanstack/react-query.gen'
import { BesuNodeForm, BesuNodeFormValues } from '@/components/nodes/besu-node-form'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { useMutation, useQuery } from '@tanstack/react-query'
import { ArrowLeft } from 'lucide-react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'

export default function EditBesuNodePage() {
	const navigate = useNavigate()
	const { id } = useParams<{ id: string }>()

	// Fetch node data
	const { data: node, isLoading } = useQuery({
		...getNodesByIdOptions({
			path: { id: parseInt(id!) },
		}),
		enabled: !!id,
	})

	// Update node mutation
	const updateNode = useMutation({
		...putNodesByIdMutation(),
		onSuccess: () => {
			toast.success('Node updated successfully')
			navigate(`/nodes/${id}`)
		},
		onError: (error: any) => {
			if (error instanceof Error) {
				toast.error(`Failed to update node: ${error.message}`)
			} else if (error.error.message) {
				toast.error(`Failed to update node: ${error.error.message}`)
			} else {
				toast.error('An unknown error occurred')
			}
		},
	})

	const handleSubmit = async (values: BesuNodeFormValues) => {
		if (!node) return

		await updateNode.mutate({
			path: { id: parseInt(id!) },
			body: {
				name: values.name,
				blockchainPlatform: values.blockchainPlatform,
				besuNode: {
					env: {},
					networkId: values.networkId,
					internalIp: values.internalIp,
					externalIp: values.externalIp,
					p2pHost: values.p2pHost,
					p2pPort: values.p2pPort,
					rpcHost: values.rpcHost,
					rpcPort: values.rpcPort,
					bootnodes: values.bootNodes
						?.split(',')
						.map((node) => node.trim())
						.filter(Boolean),
				},
			},
		})
	}

	if (isLoading) {
		return <div>Loading...</div>
	}

	if (!node) {
		return <div>Node not found</div>
	}

	// Transform API data to form values
	const defaultValues: BesuNodeFormValues = {
		name: node.name!,
		blockchainPlatform: 'BESU',
		type: 'besu',
		requestTimeout: 10,
		mode: node.besuNode?.mode! as 'service' | 'docker',
		networkId: node.besuNode?.networkId!,
		externalIp: node.besuNode?.externalIp!,
		internalIp: node.besuNode?.internalIp!,
		keyId: node.besuNode?.keyId!,
		p2pHost: node.besuNode?.p2pHost!,
		p2pPort: node.besuNode?.p2pPort!,
		rpcHost: node.besuNode?.rpcHost!,
		rpcPort: node.besuNode?.rpcPort!,
		bootNodes: node.besuNode?.bootNodes?.join(',') || '',
	}

	return (
		<div className="flex-1 p-8">
			<div className="max-w-3xl mx-auto">
				<div className="flex items-center gap-2 text-muted-foreground mb-8">
					<Button variant="ghost" size="sm" asChild>
						<Link to="/nodes">
							<ArrowLeft className="mr-2 h-4 w-4" />
							Nodes
						</Link>
					</Button>
				</div>

				<Card className="p-6">
					<h1 className="text-2xl font-semibold mb-6">Edit Besu Node</h1>
					<BesuNodeForm defaultValues={defaultValues} onSubmit={handleSubmit} submitButtonText="Update Node" submitButtonLoadingText="Updating..." />
				</Card>
			</div>
		</div>
	)
}

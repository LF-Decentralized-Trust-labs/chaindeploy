import { getNodesByIdOptions, getOrganizationsOptions, putNodesByIdMutation } from '@/api/client/@tanstack/react-query.gen'
import { FabricNodeForm } from '@/components/nodes/fabric-node-form'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'

type FabricNodeFormValues = {
	name: string
	fabricProperties: {
		nodeType: 'FABRIC_PEER' | 'FABRIC_ORDERER'
		mode: 'docker' | 'service'
		version: string
		organizationId?: number
		listenAddress: string
		operationsListenAddress: string
		externalEndpoint: string
		domains?: string[]
		chaincodeAddress?: string
		eventsAddress?: string
		adminAddress?: string
		addressOverrides?: { from: string; to: string; tlsCACert: string }[]
	}
}

export default function EditFabricNodePage() {
	const navigate = useNavigate()
	const { id } = useParams<{ id: string }>()

	const { data: node } = useQuery({
		...getNodesByIdOptions({
			path: { id: parseInt(id!) },
		}),
		enabled: !!id,
	})

	const { data: organizations } = useQuery({
		...getOrganizationsOptions(),
	})

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

	const onSubmit = (data: FabricNodeFormValues) => {
		const organization = organizations?.find((org) => org.id === data.fabricProperties.organizationId)

		if (data.fabricProperties.nodeType === 'FABRIC_PEER') {
			const fabricPeer = {
				nodeType: 'FABRIC_PEER' as const,
				mode: data.fabricProperties.mode,
				organizationId: data.fabricProperties.organizationId,
				listenAddress: data.fabricProperties.listenAddress,
				operationsListenAddress: data.fabricProperties.operationsListenAddress,
				externalEndpoint: data.fabricProperties.externalEndpoint,
				domainNames: data.fabricProperties.domains || [],
				name: data.name,
				chaincodeAddress: data.fabricProperties.chaincodeAddress || '',
				eventsAddress: data.fabricProperties.eventsAddress || '',
				mspId: organization?.mspId || '',
				version: data.fabricProperties.version || '2.5.12',
				addressOverrides: data.fabricProperties.addressOverrides || [],
			}

			updateNode.mutate({
				path: { id: parseInt(id!) },
				body: {
					name: data.name,
					blockchainPlatform: 'FABRIC',
					fabricPeer,
				},
			})
		} else {
			const fabricOrderer = {
				nodeType: 'FABRIC_ORDERER' as const,
				mode: data.fabricProperties.mode,
				organizationId: data.fabricProperties.organizationId,
				listenAddress: data.fabricProperties.listenAddress,
				operationsListenAddress: data.fabricProperties.operationsListenAddress,
				externalEndpoint: data.fabricProperties.externalEndpoint,
				domainNames: data.fabricProperties.domains || [],
				name: data.name,
				mspId: organization?.mspId || '',
				version: data.fabricProperties.version || '2.5.12',
				addressOverrides: data.fabricProperties.addressOverrides || [],
				adminAddress: data.fabricProperties.adminAddress || '',
			}

			updateNode.mutate({
				path: { id: parseInt(id!) },
				body: {
					name: data.name,
					blockchainPlatform: 'FABRIC',
					fabricOrderer,
				},
			})
		}
	}

	// Convert node data to form values
	const defaultValues = useMemo(() => {
		if (!node) return {} as FabricNodeFormValues

		if (node.fabricPeer) {
			return {
				name: node.name || '',
				fabricProperties: {
					nodeType: 'FABRIC_PEER',
					mode: node.fabricPeer?.mode || 'service',
					version: node.fabricPeer?.version || '3.0.0',
					organizationId: node.fabricPeer?.organizationId,
					listenAddress: node.fabricPeer?.listenAddress || '',
					operationsListenAddress: node.fabricPeer?.operationsAddress || '',
					externalEndpoint: node.fabricPeer?.externalEndpoint || '',
					domains: node.fabricPeer?.domainNames || [],
					chaincodeAddress: node.fabricPeer?.chaincodeAddress || '',
					eventsAddress: node.fabricPeer?.eventsAddress || '',
					addressOverrides:
						node.fabricPeer?.addressOverrides?.map((override) => ({
							from: override.from || '',
							to: override.to || '',
							tlsCACert: override.tlsCACert || '',
						})) || [],
				},
			} as FabricNodeFormValues
		}

		if (node.fabricOrderer) {
			return {
				name: node.name || '',
				fabricProperties: {
					nodeType: 'FABRIC_ORDERER',
					mode: node.fabricOrderer?.mode || 'service',
					version: node.fabricOrderer?.version || '3.0.0',
					organizationId: node.fabricOrderer?.organizationId,
					listenAddress: node.fabricOrderer?.listenAddress || '',
					operationsListenAddress: node.fabricOrderer?.operationsAddress || '',
					externalEndpoint: node.fabricOrderer?.externalEndpoint || '',
					domains: node.fabricOrderer?.domainNames || [],
					addressOverrides: [],
					adminAddress: node.fabricOrderer?.adminAddress || '',
				},
			} as FabricNodeFormValues
		}

		return {} as FabricNodeFormValues
	}, [node])

	if (!node) {
		return (
			<div className="flex-1 p-8">
				<div className="max-w-2xl mx-auto">
					<div className="mb-8">
						<h1 className="text-2xl font-semibold">Loading...</h1>
					</div>
				</div>
			</div>
		)
	}

	if (node.nodeType !== 'FABRIC_PEER' && node.nodeType !== 'FABRIC_ORDERER') {
		return (
			<div className="flex-1 p-8">
				<div className="max-w-2xl mx-auto">
					<div className="mb-8">
						<h1 className="text-2xl font-semibold">Invalid Node Type</h1>
						<p className="text-muted-foreground">This page only supports editing Fabric peer or orderer nodes.</p>
					</div>
				</div>
			</div>
		)
	}

	return (
		<div className="flex-1 p-8">
			<div className="max-w-2xl mx-auto">
				<div className="mb-8">
					<h1 className="text-2xl font-semibold">Edit Fabric Node</h1>
					<p className="text-muted-foreground">Update your Fabric node configuration</p>
				</div>

				<FabricNodeForm
					onSubmit={onSubmit}
					isSubmitting={updateNode.isPending}
					organizations={organizations?.map((org) => ({ id: org.id!, name: org.mspId! })) || []}
					hideNodeType={true}
					submitText="Update Node"
					defaultValues={defaultValues}
				/>
			</div>
		</div>
	)
}

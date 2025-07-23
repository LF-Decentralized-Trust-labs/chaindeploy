import { HttpCreateNodeRequest, TypesFabricOrdererConfig, TypesFabricPeerConfig } from '@/api/client'
import { getNodesDefaultsFabricOrdererOptions, getNodesDefaultsFabricPeerOptions, getOrganizationsOptions, postNodesMutation } from '@/api/client/@tanstack/react-query.gen'

import { FabricNodeForm, FabricNodeFormValues } from '@/components/nodes/fabric-node-form'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'

export default function CreateFabricNodePage() {
	const navigate = useNavigate()
	const [nodeType, setNodeType] = useState<'FABRIC_PEER' | 'FABRIC_ORDERER'>('FABRIC_PEER')

	const { data: organizations } = useQuery({
		...getOrganizationsOptions({query: {limit:1000}}),
	})

	const { data: peerDefaults } = useQuery({
		...getNodesDefaultsFabricPeerOptions(),
	})

	const { data: ordererDefaults } = useQuery({
		...getNodesDefaultsFabricOrdererOptions(),
	})

	const createNode = useMutation({
		...postNodesMutation(),
		onSuccess: (response) => {
			toast.success('Node created successfully')
			navigate(`/nodes/${response.id}`)
		},
		onError: (error: any) => {
			if (error instanceof Error) {
				toast.error(`Failed to create node: ${error.message}`)
			} else if (error.message) {
				toast.error(`Failed to create node: ${error.message}`)
			} else {
				toast.error('An unknown error occurred')
			}
		},
	})

	const onSubmit = (data: FabricNodeFormValues) => {
		let fabricPeer: TypesFabricPeerConfig | undefined
		let fabricOrderer: TypesFabricOrdererConfig | undefined

		if (data.fabricProperties.nodeType === 'FABRIC_PEER') {
			const organization = organizations?.items?.find((org) => org.id === data.fabricProperties.organizationId)
			fabricPeer = {
				nodeType: 'FABRIC_PEER',
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
			} as TypesFabricPeerConfig
		} else {
			const organization = organizations?.items?.find((org) => org.id === data.fabricProperties.organizationId)
			fabricOrderer = {
				nodeType: 'FABRIC_ORDERER',
				mode: data.fabricProperties.mode,
				organizationId: data.fabricProperties.organizationId,
				listenAddress: data.fabricProperties.listenAddress,
				operationsListenAddress: data.fabricProperties.operationsListenAddress,
				externalEndpoint: data.fabricProperties.externalEndpoint,
				domainNames: data.fabricProperties.domains || [],
				name: data.name,
				adminAddress: data.fabricProperties.adminAddress || '',
				mspId: organization?.mspId || '',
				version: data.fabricProperties.version || '2.5.12',
			} as TypesFabricOrdererConfig
		}

		const createNodeDto: HttpCreateNodeRequest = {
			name: data.name,
			blockchainPlatform: 'FABRIC',
			fabricPeer,
			fabricOrderer,
		}

		createNode.mutate({
			body: createNodeDto,
		})
	}

	const handleNodeTypeChange = (type: 'FABRIC_PEER' | 'FABRIC_ORDERER') => {
		setNodeType(type)
	}

	return (
		<div className="flex-1 p-8">
			<div className="max-w-2xl mx-auto">
				<div className="mb-8">
					<h1 className="text-2xl font-semibold">Create Fabric Node</h1>
					<p className="text-muted-foreground">Configure a new Fabric node</p>
				</div>

				<FabricNodeForm
					onSubmit={onSubmit}
					isSubmitting={createNode.isPending}
					organizations={organizations?.items?.map((org) => ({ id: org.id!, name: org.mspId! })) || []}
					defaults={nodeType === 'FABRIC_PEER' ? peerDefaults : ordererDefaults}
					onNodeTypeChange={handleNodeTypeChange}
					submitText="Create Node"
					submitButtonLoadingText="Creating..."
				/>
			</div>
		</div>
	)
}

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useState, useEffect, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getOrganizationsOptions, getNetworksFabricByIdNodesOptions } from '@/api/client/@tanstack/react-query.gen'
import { getKeysById } from '@/api/client/sdk.gen'

interface FabricKeySelectProps {
	value?: { orgId: number; keyId: number }
	onChange: (value: { orgId: number; keyId: number } | undefined) => void
	disabled?: boolean
	networkId?: number
	showErrors?: boolean
}

export const FabricKeySelect = ({ value, onChange, disabled, networkId, showErrors = false }: FabricKeySelectProps) => {
	const [selectedKeys, setSelectedKeys] = useState<Array<{ id: number; name: string; description?: string; algorithm?: string; keySize?: number; curve?: string }>>([])
	const [isLoading, setIsLoading] = useState(false)
	const [isDirty, setIsDirty] = useState(false)

	// Fetch all organizations
	const { data: allOrganizations } = useQuery({
		...getOrganizationsOptions({query: {limit:1000}}),
	})

	// Fetch network nodes if networkId is provided
	const { data: networkNodes } = useQuery({
		...getNetworksFabricByIdNodesOptions({ path: { id: networkId! } }),
		enabled: !!networkId,
	})

	// Filter organizations based on network if networkId is provided
	const organizations = useMemo(() => {
		if (!networkId || !networkNodes?.nodes) {
			return allOrganizations
		}

		// Get unique organization IDs from network nodes
		const networkOrgIds = new Set<number>()
		networkNodes.nodes.forEach(node => {
			if (node.node?.fabricPeer?.organizationId) {
				networkOrgIds.add(node.node.fabricPeer.organizationId)
			}
			if (node.node?.fabricOrderer?.organizationId) {
				networkOrgIds.add(node.node.fabricOrderer.organizationId)
			}
		})

		// Filter organizations
		return {
			items: allOrganizations?.items?.filter(org => networkOrgIds.has(org.id!)) || []
		}
	}, [allOrganizations, networkNodes, networkId])

	// Get the selected organization
	const selectedOrg = useMemo(() => organizations?.items?.find((org) => org.id === value?.orgId), [organizations, value?.orgId])

	// Get key IDs for the selected organization
	const keyIds = useMemo(() => {
		if (!selectedOrg) return []
		const ids: number[] = []
		if (selectedOrg.adminSignKeyId) ids.push(selectedOrg.adminSignKeyId)
		if (selectedOrg.adminTlsKeyId) ids.push(selectedOrg.adminTlsKeyId)
		if (selectedOrg.clientSignKeyId) ids.push(selectedOrg.clientSignKeyId)
		return ids
	}, [selectedOrg])

	const selectedKey = useMemo(() => selectedKeys.find((key) => key.id === value?.keyId), [selectedKeys, value])

	// Fetch key details when organization changes
	const fetchKeyDetails = async () => {
		if (!keyIds.length) {
			setSelectedKeys([])
			return
		}
		setIsLoading(true)
		try {
			const keyDetails = await Promise.all(
				keyIds.map(async (keyId) => {
					const { data } = await getKeysById({ path: { id: keyId } })
					return {
						id: keyId,
						name: data.name || `Key ${keyId}`,
						description: data.description,
						algorithm: data.algorithm,
						keySize: data.keySize,
						curve: data.curve,
					}
				})
			)
			setSelectedKeys(keyDetails)
		} catch (error) {
			setSelectedKeys([])
		} finally {
			setIsLoading(false)
		}
	}

	useEffect(() => {
		if (value?.orgId && keyIds.length > 0) {
			fetchKeyDetails()
		}
	}, [value?.orgId, keyIds])

	// Auto-select first organization if none selected and organizations are available
	useEffect(() => {
		if (!value?.orgId && organizations?.items?.length && organizations.items.length > 0) {
			const firstOrg = organizations.items[0]
			onChange({ orgId: firstOrg.id!, keyId: 0 })
		}
	}, [organizations, value?.orgId])

	// Auto-select client sign key when keys are loaded
	useEffect(() => {
		if (selectedKeys.length > 0 && value?.orgId && !value?.keyId && selectedOrg) {
			// Try to find the client sign key
			const clientSignKey = selectedKeys.find(key => key.id === selectedOrg.clientSignKeyId)
			if (clientSignKey) {
				onChange({ orgId: value.orgId, keyId: clientSignKey.id })
			}
		}
	}, [selectedKeys, value, selectedOrg])

	return (
		<div className="space-y-4">
			<Select
				value={value?.orgId?.toString()}
				onValueChange={(val) => {
					const orgId = Number(val)
					// When org changes, reset key selection to 0
					onChange({ orgId, keyId: 0 })
					setIsDirty(true)
				}}
				disabled={disabled}
			>
				<SelectTrigger
					className={(isDirty || showErrors) && !value?.orgId && !disabled ? 'border-destructive' : ''}
					onClick={() => setIsDirty(true)}
				>
					<SelectValue placeholder="Select an organization" />
				</SelectTrigger>
				<SelectContent>
					<ScrollArea className="max-h-[200px]">
						{organizations?.items?.map((org) => (
							<SelectItem key={org.id} value={org.id?.toString()}>
								{org.mspId}
							</SelectItem>
						))}
					</ScrollArea>
				</SelectContent>
			</Select>

			<Select
				value={value?.keyId ? value.keyId.toString() : undefined}
				onValueChange={(val) => {
					if (value?.orgId) {
						onChange({ orgId: value.orgId, keyId: Number(val) })
						setIsDirty(true)
					}
				}}
				disabled={disabled || !value?.orgId || isLoading}
			>
				<SelectTrigger
					className={(isDirty || showErrors) && value?.orgId && !value?.keyId && !disabled ? 'border-destructive' : ''}
					onClick={() => setIsDirty(true)}
				>
					{selectedKey ? (
						<div className="text-left">
							<div className="font-medium">{selectedKey.name}</div>
							<div className="text-xs text-muted-foreground">
								{selectedKey.description} • {selectedKey.algorithm}
								{selectedKey.keySize && ` (${selectedKey.keySize} bits)`}
								{selectedKey.curve && ` - ${selectedKey.curve}`}
							</div>
						</div>
					) : (
						'Select a key'
					)}
				</SelectTrigger>
				<SelectContent>
					<ScrollArea className="max-h-[200px]">
						{selectedKeys.map((key) => (
							<SelectItem key={key.id} value={key.id.toString()}>
								<div className="flex flex-col">
									<span>{key.name}</span>
									{key.description && <span className="text-xs text-muted-foreground">{key.description}</span>}
									{key.algorithm && (
										<span className="text-xs text-muted-foreground">
											Algorithm: {key.algorithm}
											{key.keySize && ` (${key.keySize} bits)`}
											{key.curve && ` - ${key.curve}`}
										</span>
									)}
								</div>
							</SelectItem>
						))}
					</ScrollArea>
				</SelectContent>
			</Select>
		</div>
	)
}

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useState, useEffect, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getOrganizationsOptions } from '@/api/client/@tanstack/react-query.gen'
import { getKeysById } from '@/api/client/sdk.gen'

interface FabricKeySelectProps {
	value?: { orgId: number; keyId: number }
	onChange: (value: { orgId: number; keyId: number }) => void
	disabled?: boolean
}

export const FabricKeySelect = ({ value, onChange, disabled }: FabricKeySelectProps) => {
	const [selectedKeys, setSelectedKeys] = useState<Array<{ id: number; name: string; description?: string; algorithm?: string; keySize?: number; curve?: string }>>([])
	const [isLoading, setIsLoading] = useState(false)

	// Fetch organizations
	const { data: organizations } = useQuery({
		...getOrganizationsOptions(),
	})

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

	return (
		<div className="space-y-4">
			<Select
				value={value?.orgId?.toString()}
				onValueChange={(val) => {
					const orgId = Number(val)
					// When org changes, reset key selection to 0
					onChange({ orgId, keyId: 0 })
				}}
				disabled={disabled}
			>
				<SelectTrigger>
					<SelectValue placeholder="Select an organization" />
				</SelectTrigger>
				<SelectContent>
					<ScrollArea className="h-[200px]">
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
					}
				}}
				disabled={disabled || !value?.orgId || isLoading}
			>
				<SelectTrigger>
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
					<ScrollArea className="h-[200px]">
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

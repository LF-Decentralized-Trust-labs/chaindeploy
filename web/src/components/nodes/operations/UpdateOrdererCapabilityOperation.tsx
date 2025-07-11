import { UpdateCapabilityOperation } from './UpdateCapabilityOperation'

interface UpdateOrdererCapabilityOperationProps {
	index: number
	onRemove: () => void
	channelConfig?: any
}

export function UpdateOrdererCapabilityOperation({ index, onRemove, channelConfig }: UpdateOrdererCapabilityOperationProps) {
	return (
		<UpdateCapabilityOperation
			index={index}
			onRemove={onRemove}
			channelConfig={channelConfig}
			title="Update Orderer Capability"
			capabilitiesPath={['groups', 'Orderer', 'values', 'Capabilities']}
		/>
	)
} 
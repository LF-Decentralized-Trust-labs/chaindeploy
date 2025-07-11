import { UpdateCapabilityOperation } from './UpdateCapabilityOperation'

interface UpdateChannelCapabilityOperationProps {
	index: number
	onRemove: () => void
	channelConfig?: any
}

export function UpdateChannelCapabilityOperation({ index, onRemove, channelConfig }: UpdateChannelCapabilityOperationProps) {
	return (
		<UpdateCapabilityOperation
			index={index}
			onRemove={onRemove}
			channelConfig={channelConfig}
			title="Update Channel Capability"
			capabilitiesPath={['values', 'Capabilities']}
		/>
	)
} 
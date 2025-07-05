import { UpdateCapabilityOperation } from './UpdateCapabilityOperation'

interface UpdateApplicationCapabilityOperationProps {
	index: number
	onRemove: () => void
	channelConfig?: any
}

export function UpdateApplicationCapabilityOperation({ index, onRemove, channelConfig }: UpdateApplicationCapabilityOperationProps) {
	return (
		<UpdateCapabilityOperation
			index={index}
			onRemove={onRemove}
			channelConfig={channelConfig}
			title="Update Application Capability"
			capabilitiesPath={['groups', 'Application', 'values', 'Capabilities']}
		/>
	)
} 
import { ValidatorItem } from './validator-item'

interface ValidatorListProps {
	validatorIds: number[]
}

export function ValidatorList({ validatorIds }: ValidatorListProps) {
	return (
		<div className="space-y-2">
			{validatorIds.map((keyId, index) => (
				<ValidatorItem key={keyId} keyId={keyId} index={index} />
			))}
		</div>
	)
}

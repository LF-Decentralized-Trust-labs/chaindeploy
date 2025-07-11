import { Button } from '@/components/ui/button'
import { FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Trash2, X } from 'lucide-react'
import { useFormContext } from 'react-hook-form'
import { Badge } from '@/components/ui/badge'

interface UpdateCapabilityOperationProps {
	index: number
	onRemove: () => void
	channelConfig?: any
	title: string
	capabilitiesPath: string[]
}

export function UpdateCapabilityOperation({ index, onRemove, channelConfig, title, capabilitiesPath }: UpdateCapabilityOperationProps) {
	const form = useFormContext()

	// Get existing capabilities from channel config
	const existingCapabilities = capabilitiesPath.reduce((obj, key) => obj?.[key], channelConfig?.config?.data?.data?.[0]?.payload?.data?.config?.channel_group)?.value?.capabilities || {}

	// Common capability values
	const capabilityOptions = [
		'V2_0',
		'V2_5',
		'V3_0',
	]

	const selectedCapabilities = form.watch(`operations.${index}.payload.capability`) || []

	const addCapability = (capability: string) => {
		if (!selectedCapabilities.includes(capability)) {
			form.setValue(`operations.${index}.payload.capability`, [...selectedCapabilities, capability])
		}
	}

	const removeCapability = (capability: string) => {
		form.setValue(
			`operations.${index}.payload.capability`,
			selectedCapabilities.filter((c) => c !== capability)
		)
	}

	return (
		<div className="space-y-4 p-4 border rounded-lg">
			<div className="flex items-center justify-between">
				<h3 className="text-sm font-medium">{title}</h3>
				<Button type="button" variant="ghost" size="icon" onClick={onRemove}>
					<Trash2 className="h-4 w-4" />
				</Button>
			</div>

			<FormField
				control={form.control}
				name={`operations.${index}.payload.capability`}
				render={({ field }) => (
					<FormItem>
						<FormLabel>Capabilities</FormLabel>
						<Select onValueChange={addCapability} value="">
							<FormControl>
								<SelectTrigger>
									<SelectValue placeholder="Select capability" />
								</SelectTrigger>
							</FormControl>
							<SelectContent>
								{capabilityOptions
									.filter((capability) => !selectedCapabilities.includes(capability))
									.map((capability) => (
										<SelectItem key={capability} value={capability}>
											{capability}
										</SelectItem>
									))}
							</SelectContent>
						</Select>
						<div className="flex flex-wrap gap-2 mt-2">
							{selectedCapabilities.map((capability) => (
								<Badge key={capability} variant="secondary" className="flex items-center gap-1">
									{capability}
									<Button
										type="button"
										variant="ghost"
										size="icon"
										className="h-4 w-4 p-0 hover:bg-transparent"
										onClick={() => removeCapability(capability)}
									>
										<X className="h-3 w-3" />
									</Button>
								</Badge>
							))}
						</div>
						<FormMessage />
					</FormItem>
				)}
			/>
		</div>
	)
} 
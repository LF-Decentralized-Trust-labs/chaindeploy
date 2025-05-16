import { Button } from '@/components/ui/button'
import { FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { X } from 'lucide-react'
import { useEffect } from 'react'
import { useFormContext } from 'react-hook-form'
import { z } from 'zod'

// Schema for the UpdateBatchTimeoutPayload
export const updateBatchTimeoutSchema = z.object({
	timeout: z.string().min(1, 'Timeout is required'),
})

export type UpdateBatchTimeoutFormValues = z.infer<typeof updateBatchTimeoutSchema>

interface UpdateBatchTimeoutOperationProps {
	index: number
	onRemove: () => void
	channelConfig?: any
}

export function UpdateBatchTimeoutOperation({ index, onRemove, channelConfig }: UpdateBatchTimeoutOperationProps) {
	const form = useFormContext()
	// Set the default value from channel config if available
	useEffect(() => {
		const batchTimeout = channelConfig?.config.data.data[0].payload.data.config.channel_group?.groups?.Orderer?.values?.BatchTimeout?.value?.timeout

		if (batchTimeout) {
			form.setValue(`operations.${index}.payload.timeout`, batchTimeout)
		}
	}, [channelConfig])

	return (
		<div className="rounded-lg border p-4 space-y-4">
			<div className="flex items-center justify-between">
				<h3 className="font-medium">Update Batch Timeout</h3>
				<Button type="button" variant="ghost" size="icon" onClick={onRemove}>
					<X className="h-4 w-4" />
				</Button>
			</div>
			<FormField
				control={form.control}
				name={`operations.${index}.payload.timeout`}
				render={({ field }) => (
					<FormItem>
						<FormLabel>Timeout</FormLabel>
						<FormControl>
							<Input placeholder="Enter timeout (e.g. 2s)" {...field} />
						</FormControl>
						<FormMessage />
					</FormItem>
				)}
			/>
		</div>
	)
}

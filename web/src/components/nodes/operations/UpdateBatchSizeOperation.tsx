import { Button } from '@/components/ui/button'
import { FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { X } from 'lucide-react'
import { useEffect } from 'react'
import { useFormContext } from 'react-hook-form'
import { z } from 'zod'

// Schema for the UpdateBatchSizePayload
export const updateBatchSizeSchema = z.object({
	absolute_max_bytes: z.number().int().positive('Absolute max bytes must be a positive integer'),
	max_message_count: z.number().int().positive('Max message count must be a positive integer'),
	preferred_max_bytes: z.number().int().positive('Preferred max bytes must be a positive integer'),
})

export type UpdateBatchSizeFormValues = z.infer<typeof updateBatchSizeSchema>

interface UpdateBatchSizeOperationProps {
	index: number
	onRemove: () => void
	channelConfig?: any
}

export function UpdateBatchSizeOperation({ index, onRemove, channelConfig }: UpdateBatchSizeOperationProps) {
	const form = useFormContext()

	// Set the default values from channel config if available
	useEffect(() => {
		const batchSize = channelConfig?.config.data.data[0].payload.data.config.channel_group?.groups?.Orderer?.values?.BatchSize?.value
		if (batchSize) {
			const { absolute_max_bytes, max_message_count, preferred_max_bytes } = batchSize
			if (absolute_max_bytes) form.setValue(`operations.${index}.payload.absolute_max_bytes`, absolute_max_bytes)
			if (max_message_count) form.setValue(`operations.${index}.payload.max_message_count`, max_message_count)
			if (preferred_max_bytes) form.setValue(`operations.${index}.payload.preferred_max_bytes`, preferred_max_bytes)
		}
	}, [channelConfig])

	return (
		<div className="rounded-lg border p-4 space-y-4">
			<div className="flex items-center justify-between">
				<h3 className="font-medium">Update Batch Size</h3>
				<Button type="button" variant="ghost" size="icon" onClick={onRemove}>
					<X className="h-4 w-4" />
				</Button>
			</div>
			<FormField
				control={form.control}
				name={`operations.${index}.payload.absolute_max_bytes`}
				render={({ field }) => (
					<FormItem>
						<FormLabel>Absolute Max Bytes</FormLabel>
						<FormControl>
							<Input type="number" placeholder="Enter absolute max bytes" {...field} onChange={(e) => field.onChange(parseInt(e.target.value) || 0)} />
						</FormControl>
						<FormMessage />
					</FormItem>
				)}
			/>
			<FormField
				control={form.control}
				name={`operations.${index}.payload.max_message_count`}
				render={({ field }) => (
					<FormItem>
						<FormLabel>Max Message Count</FormLabel>
						<FormControl>
							<Input type="number" placeholder="Enter max message count" {...field} onChange={(e) => field.onChange(parseInt(e.target.value) || 0)} />
						</FormControl>
						<FormMessage />
					</FormItem>
				)}
			/>
			<FormField
				control={form.control}
				name={`operations.${index}.payload.preferred_max_bytes`}
				render={({ field }) => (
					<FormItem>
						<FormLabel>Preferred Max Bytes</FormLabel>
						<FormControl>
							<Input type="number" placeholder="Enter preferred max bytes" {...field} onChange={(e) => field.onChange(parseInt(e.target.value) || 0)} />
						</FormControl>
						<FormMessage />
					</FormItem>
				)}
			/>
		</div>
	)
} 
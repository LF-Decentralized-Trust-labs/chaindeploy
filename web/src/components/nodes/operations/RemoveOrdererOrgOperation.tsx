import { Button } from '@/components/ui/button'
import { FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Trash2 } from 'lucide-react'
import { useFormContext } from 'react-hook-form'

interface RemoveOrdererOrgOperationProps {
	index: number
	onRemove: () => void
}

export function RemoveOrdererOrgOperation({ index, onRemove }: RemoveOrdererOrgOperationProps) {
	const form = useFormContext()

	return (
		<div className="space-y-4 p-4 border rounded-lg">
			<div className="flex items-center justify-between">
				<h3 className="text-sm font-medium">Remove Orderer Organization</h3>
				<Button type="button" variant="ghost" size="icon" onClick={onRemove}>
					<Trash2 className="h-4 w-4" />
				</Button>
			</div>
			<FormField
				control={form.control}
				name={`operations.${index}.payload.msp_id`}
				render={({ field }) => (
					<FormItem>
						<FormLabel>MSP ID</FormLabel>
						<FormControl>
							<Input {...field} placeholder="Enter MSP ID to remove" />
						</FormControl>
						<FormMessage />
					</FormItem>
				)}
			/>
		</div>
	)
} 
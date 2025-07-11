import { Control, useFieldArray, useFormContext } from 'react-hook-form'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { FormField, FormItem, FormLabel, FormControl, FormMessage } from '@/components/ui/form'

type ArrayFieldInputProps = {
	control: Control<any>
	name: string
	label: string
	placeholder?: string
	disabled?: boolean
}

export function ArrayFieldInput({ control, name, label, placeholder, disabled }: ArrayFieldInputProps) {
	const { fields, append, remove } = useFieldArray({ control, name })
	const { register } = useFormContext()

	return (
		<FormField
			control={control}
			name={name}
			render={() => (
				<FormItem>
					<FormLabel>{label}</FormLabel>
					<div className="space-y-2">
						{fields.map((field, idx) => (
							<div key={field.id} className="flex gap-2 items-center">
								<FormControl>
									<Input
										{...register(`${name}.${idx}`)}
										disabled={disabled}
										placeholder={placeholder}
									/>
								</FormControl>
								<Button
									type="button"
									variant="ghost"
									size="icon"
									onClick={() => remove(idx)}
									disabled={disabled}
								>
									<span className="sr-only">Remove</span>×
								</Button>
							</div>
						))}
						<Button
							type="button"
							variant="outline"
							size="sm"
							onClick={() => append('')}
							disabled={disabled}
						>
							Add {label}
						</Button>
					</div>
					<FormMessage />
				</FormItem>
			)}
		/>
	)
}

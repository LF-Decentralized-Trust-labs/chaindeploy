import { postScFabricDefinitionsByDefinitionIdDeployMutation } from '@/api/client/@tanstack/react-query.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Form, FormLabel } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { Minus, Plus } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import * as z from 'zod'

const deployFormSchema = z.object({
	environmentVariables: z.array(
		z.object({
			key: z.string().min(1, 'Key is required'),
			value: z.string().min(1, 'Value is required'),
		})
	).optional(),
})

type DeployFormValues = z.infer<typeof deployFormSchema>

interface DeployChaincodeDialogProps {
	deployDialogOpen: boolean
	setDeployDialogOpen: (open: boolean) => void
	definitionId: number
	onSuccess?: () => void
	onError?: (error: any) => void
}

function DeployChaincodeDialog({ deployDialogOpen, setDeployDialogOpen, definitionId, onSuccess, onError }: DeployChaincodeDialogProps) {
	const [envVars, setEnvVars] = useState<Array<{ key: string; value: string }>>([])

	const form = useForm<DeployFormValues>({
		resolver: zodResolver(deployFormSchema),
		defaultValues: {
			environmentVariables: [],
		},
	})

	const deployMutation = useMutation({
		...postScFabricDefinitionsByDefinitionIdDeployMutation(),
		onSuccess: () => {
			toast.success('Chaincode deployed successfully')
			setDeployDialogOpen(false)
			onSuccess?.()
		},
		onError: (error: any) => {
			onError?.(error)
		},
	})

	const addEnvVar = () => {
		setEnvVars([...envVars, { key: '', value: '' }])
	}

	const removeEnvVar = (index: number) => {
		setEnvVars(envVars.filter((_, i) => i !== index))
	}

	const updateEnvVar = (index: number, field: 'key' | 'value', value: string) => {
		const updated = [...envVars]
		updated[index] = { ...updated[index], [field]: value }
		setEnvVars(updated)
	}

	const handleSubmit = async () => {
		// Convert array of key-value pairs to object
		const envVarsObject: { [key: string]: string } = {}
		envVars.forEach(({ key, value }) => {
			if (key.trim() && value.trim()) {
				envVarsObject[key.trim()] = value.trim()
			}
		})

		try {
			await toast.promise(
				deployMutation.mutateAsync({
					path: { definitionId },
					body: {
						environment_variables: Object.keys(envVarsObject).length > 0 ? envVarsObject : undefined,
					},
				}),
				{
					loading: 'Deploying chaincode...',
					success: 'Chaincode deployed successfully',
					error: (err) => {
						if (err?.response?.data?.message) {
							return err.response.data.message
						}
						return 'Failed to deploy chaincode'
					},
				}
			)
		} catch (error) {
			// Error is already handled by toast.promise
		}
	}

	return (
		<Dialog open={deployDialogOpen} onOpenChange={setDeployDialogOpen}>
			<DialogContent className="max-w-lg">
				<DialogHeader>
					<DialogTitle>Deploy Chaincode</DialogTitle>
					<DialogDescription>Configure environment variables for the chaincode deployment (optional).</DialogDescription>
				</DialogHeader>
				<Form {...form}>
					<form onSubmit={form.handleSubmit(handleSubmit)} className="space-y-4">
						<div className="space-y-4">
							<div className="flex items-center justify-between">
								<FormLabel>Environment Variables</FormLabel>
								<Button
									type="button"
									variant="outline"
									size="sm"
									onClick={addEnvVar}
									disabled={deployMutation.isPending}
								>
									<Plus className="w-4 h-4 mr-1" />
									Add Variable
								</Button>
							</div>
							
							{envVars.length === 0 && (
								<div className="text-sm text-muted-foreground text-center py-4">
									No environment variables added. Click "Add Variable" to add some.
								</div>
							)}

							<div className="space-y-3">
								{envVars.map((envVar, index) => (
									<div key={index} className="flex gap-2 items-start">
										<div className="flex-1">
											<Input
												placeholder="Variable name"
												value={envVar.key}
												onChange={(e) => updateEnvVar(index, 'key', e.target.value)}
												disabled={deployMutation.isPending}
											/>
										</div>
										<div className="flex-1">
											<Input
												placeholder="Variable value"
												value={envVar.value}
												onChange={(e) => updateEnvVar(index, 'value', e.target.value)}
												disabled={deployMutation.isPending}
											/>
										</div>
										<Button
											type="button"
											variant="ghost"
											size="icon"
											onClick={() => removeEnvVar(index)}
											disabled={deployMutation.isPending}
										>
											<Minus className="w-4 h-4" />
										</Button>
									</div>
								))}
							</div>
						</div>

						{deployMutation.error && (
							<div className="text-red-500 text-sm mt-2 break-words max-w-full">
								{deployMutation.error.message}
							</div>
						)}

						<DialogFooter>
							<Button
								type="submit"
								disabled={deployMutation.isPending}
							>
								{deployMutation.isPending ? 'Deploying...' : 'Deploy'}
							</Button>
						</DialogFooter>
					</form>
				</Form>
			</DialogContent>
		</Dialog>
	)
}

export { DeployChaincodeDialog }

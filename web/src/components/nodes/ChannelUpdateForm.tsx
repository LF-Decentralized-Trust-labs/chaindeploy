import { HttpChannelResponse } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Form } from '@/components/ui/form'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { Loader2, PlusCircle } from 'lucide-react'
import { useState } from 'react'
import { useFieldArray, useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

// Import operation components
import { postNetworksFabricByIdUpdateConfigMutation } from '@/api/client/@tanstack/react-query.gen'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { AlertCircle } from 'lucide-react'
import { AddConsenterOperation } from './operations/AddConsenterOperation'
import { AddOrgOperation } from './operations/AddOrgOperation'
import { RemoveConsenterOperation } from './operations/RemoveConsenterOperation'
import { RemoveOrgOperation } from './operations/RemoveOrgOperation'
import { UpdateBatchSizeOperation } from './operations/UpdateBatchSizeOperation'
import { UpdateBatchTimeoutOperation } from './operations/UpdateBatchTimeoutOperation'
import { UpdateConsenterOperation } from './operations/UpdateConsenterOperation'
import { UpdateEtcdRaftOptionsOperation } from './operations/UpdateEtcdRaftOptionsOperation'
import { UpdateOrgMSPOperation } from './operations/UpdateOrgMSPOperation'

// Operation type mapping
const operationTypes = {
	add_org: 'Add Organization',
	remove_org: 'Remove Organization',
	update_org_msp: 'Update Organization MSP',
	add_consenter: 'Add Consenter',
	remove_consenter: 'Remove Consenter',
	update_consenter: 'Update Consenter',
	update_etcd_raft_options: 'Update Etcd Raft Options',
	update_batch_size: 'Update Batch Size',
	update_batch_timeout: 'Update Batch Timeout',
} as const

type OperationType = keyof typeof operationTypes

// Define the schema for each operation type

// Define default values for each operation type
const getDefaultPayloadForType = (type: OperationType) => {
	switch (type) {
		case 'add_org':
			return { msp_id: '', root_certs: [], tls_root_certs: [] }
		case 'remove_org':
			return { msp_id: '' }
		case 'update_org_msp':
			return { msp_id: '', root_certs: [], tls_root_certs: [] }
		case 'add_consenter':
			return { host: '', port: 7050, client_tls_cert: '', server_tls_cert: '' }
		case 'remove_consenter':
			return { host: '', port: 7050 }
		case 'update_consenter':
			return { host: '', port: 7050, new_host: '', new_port: 7050, client_tls_cert: '', server_tls_cert: '' }
		case 'update_etcd_raft_options':
			return { election_tick: 10, heartbeat_tick: 1, max_inflight_blocks: 5, snapshot_interval_size: 16777216, tick_interval: '500ms' }
		case 'update_batch_size':
			return { absolute_max_bytes: 10485760, max_message_count: 500, preferred_max_bytes: 2097152 }
		case 'update_batch_timeout':
			return { timeout: '2s' }
		default:
			return {}
	}
}

// Define the form schema
const formSchema = z.object({
	operations: z
		.array(
			z.object({
				type: z.enum([
					'add_org',
					'remove_org',
					'update_org_msp',
					'set_anchor_peers',
					'add_consenter',
					'remove_consenter',
					'update_consenter',
					'update_etcd_raft_options',
					'update_batch_size',
					'update_batch_timeout',
				]),
				payload: z.any(),
			})
		)
		.min(1, 'At least one operation is required'),
})

type FormValues = z.infer<typeof formSchema>

interface ChannelUpdateFormProps {
	network: HttpChannelResponse
	onSuccess?: () => void
	channelConfig?: any
}

export function ChannelUpdateForm({ network, onSuccess, channelConfig }: ChannelUpdateFormProps) {
	const [selectedOperationType, setSelectedOperationType] = useState<OperationType | ''>('')

	const form = useForm<FormValues>({
		resolver: zodResolver(formSchema),
		defaultValues: {
			operations: [],
		},
	})

	const { fields, append, remove } = useFieldArray({
		control: form.control,
		name: 'operations',
	})

	const prepareUpdate = useMutation({
		...postNetworksFabricByIdUpdateConfigMutation(),
		onSuccess: () => {
			toast.success('Channel updated')
			if (onSuccess) onSuccess()
		},
		onError: (error) => {
			toast.error(`Failed to create channel update proposal: ${error.message}`)
		},
	})

	const handleAddOperation = () => {
		if (!selectedOperationType) return

		append({
			type: selectedOperationType,
			payload: getDefaultPayloadForType(selectedOperationType),
		})

		setSelectedOperationType('')
	}

	const onSubmit = (data: FormValues) => {
		// Convert the payload to the format expected by the API
		const operations = data.operations.map((op) => {
			// In a real implementation, we would need to convert the payload to the format expected by the API
			// For now, we'll just pass it as is
			return {
				type: op.type,
				payload: op.payload,
			}
		})

		prepareUpdate.mutate({
			path: { id: Number((network as any).id) },
			body: { operations },
		})
	}

	const renderOperationComponent = (type: OperationType, index: number) => {
		switch (type) {
			case 'add_org':
				return <AddOrgOperation key={index} index={index} onRemove={() => remove(index)} />
			case 'remove_org':
				return <RemoveOrgOperation key={index} index={index} onRemove={() => remove(index)} />
			case 'update_org_msp':
				return <UpdateOrgMSPOperation key={index} index={index} onRemove={() => remove(index)} />
			case 'add_consenter':
				return <AddConsenterOperation key={index} index={index} onRemove={() => remove(index)} />
			case 'remove_consenter':
				return <RemoveConsenterOperation key={index} index={index} onRemove={() => remove(index)} />
			case 'update_consenter':
				return <UpdateConsenterOperation key={index} index={index} onRemove={() => remove(index)} />
			case 'update_etcd_raft_options':
				return <UpdateEtcdRaftOptionsOperation key={index} index={index} onRemove={() => remove(index)} channelConfig={channelConfig} />
			case 'update_batch_size':
				return <UpdateBatchSizeOperation key={index} index={index} onRemove={() => remove(index)} channelConfig={channelConfig} />
			case 'update_batch_timeout':
				return <UpdateBatchTimeoutOperation key={index} index={index} onRemove={() => remove(index)} channelConfig={channelConfig} />
			default:
				return null
		}
	}

	return (
		<Form {...form}>
			<form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
				<div>
					<div className="mb-6">
						<h2 className="text-lg font-semibold">Channel Update Proposal</h2>
						<p className="text-sm text-muted-foreground">Create a proposal to update the channel configuration. You can add multiple operations to be included in a single proposal.</p>
					</div>
					<div className="space-y-6">
						{fields.length === 0 && (
							<Alert>
								<AlertCircle className="h-4 w-4" />
								<AlertTitle>No operations added</AlertTitle>
								<AlertDescription>Add at least one operation to create a channel update proposal.</AlertDescription>
							</Alert>
						)}

						{fields.map((field, index) => renderOperationComponent(field.type as OperationType, index))}

						<div className="flex items-end gap-4">
							<div className="flex-1">
								<label className="text-sm font-medium mb-2 block">Add Operation</label>
								<Select value={selectedOperationType} onValueChange={(value) => setSelectedOperationType(value as OperationType)}>
									<SelectTrigger>
										<SelectValue placeholder="Select operation type" />
									</SelectTrigger>
									<SelectContent>
										{Object.entries(operationTypes).map(([value, label]) => (
											<SelectItem key={value} value={value}>
												{label}
											</SelectItem>
										))}
									</SelectContent>
								</Select>
							</div>
							<Button type="button" onClick={handleAddOperation} disabled={!selectedOperationType}>
								<PlusCircle className="h-4 w-4 mr-2" />
								Add Operation
							</Button>
						</div>
					</div>
					<div className="flex justify-between mt-6">
						<Button type="button" variant="outline" onClick={() => form.reset()}>
							Reset
						</Button>
						<Button type="submit" disabled={fields.length === 0 || prepareUpdate.isPending}>
							{prepareUpdate.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
							Update Channel
						</Button>
					</div>
				</div>
			</form>
		</Form>
	)
}

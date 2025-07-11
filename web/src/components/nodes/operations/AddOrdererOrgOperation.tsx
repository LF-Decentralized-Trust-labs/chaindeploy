import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Trash2, Plus } from 'lucide-react'
import { useFormContext, useFieldArray } from 'react-hook-form'
import { z } from 'zod'

// Schema for the AddOrdererOrgPayload
export const addOrdererOrgSchema = z.object({
	msp_id: z.string().min(1, "MSP ID is required"),
	root_certs: z.array(z.string()).min(1, "At least one root certificate is required"),
	tls_root_certs: z.array(z.string()).min(1, "At least one TLS root certificate is required"),
	orderer_endpoints: z.array(z.string()).min(1, "At least one orderer endpoint is required")
})

export type AddOrdererOrgFormValues = z.infer<typeof addOrdererOrgSchema>

interface AddOrdererOrgOperationProps {
	index: number
	onRemove: () => void
}

export function AddOrdererOrgOperation({ index, onRemove }: AddOrdererOrgOperationProps) {
	const formContext = useFormContext()
	const [newRootCert, setNewRootCert] = useState('')
	const [newTlsRootCert, setNewTlsRootCert] = useState('')
	const [newEndpoint, setNewEndpoint] = useState('')

	const { fields: rootCertsFields, append: appendRootCert, remove: removeRootCert } = 
		useFieldArray({
			name: `operations.${index}.payload.root_certs`,
			control: formContext.control
		})

	const { fields: tlsRootCertsFields, append: appendTlsRootCert, remove: removeTlsRootCert } = 
		useFieldArray({
			name: `operations.${index}.payload.tls_root_certs`,
			control: formContext.control
		})

	const { fields: endpointFields, append: appendEndpoint, remove: removeEndpoint } = 
		useFieldArray({
			name: `operations.${index}.payload.orderer_endpoints`,
			control: formContext.control
		})

	const handleAddRootCert = () => {
		if (newRootCert.trim()) {
			appendRootCert(newRootCert.trim())
			setNewRootCert('')
		}
	}

	const handleAddTlsRootCert = () => {
		if (newTlsRootCert.trim()) {
			appendTlsRootCert(newTlsRootCert.trim())
			setNewTlsRootCert('')
		}
	}

	const handleAddEndpoint = () => {
		if (newEndpoint.trim()) {
			appendEndpoint(newEndpoint.trim())
			setNewEndpoint('')
		}
	}

	return (
		<Card className="mb-6">
			<CardHeader className="pb-3">
				<div className="flex justify-between items-center">
					<CardTitle className="text-lg font-medium">Add Orderer Organization</CardTitle>
					<Button 
						variant="ghost" 
						size="icon" 
						onClick={onRemove}
						className="h-8 w-8 text-destructive"
					>
						<Trash2 className="h-4 w-4" />
					</Button>
				</div>
			</CardHeader>
			<CardContent>
				<div className="space-y-4">
					<FormField
						control={formContext.control}
						name={`operations.${index}.payload.msp_id`}
						render={({ field }) => (
							<FormItem>
								<FormLabel>MSP ID</FormLabel>
								<FormControl>
									<Input placeholder="OrdererOrgMSP" {...field} />
								</FormControl>
								<FormMessage />
							</FormItem>
						)}
					/>

					<div className="space-y-2">
						<FormLabel>Orderer Endpoints</FormLabel>
						{endpointFields.map((field, i) => (
							<div key={field.id} className="flex gap-2">
								<Input 
									{...formContext.register(`operations.${index}.payload.orderer_endpoints.${i}`)}
									placeholder="orderer.example.com:7050"
								/>
								<Button 
									type="button" 
									variant="ghost" 
									size="icon" 
									onClick={() => removeEndpoint(i)}
									className="h-10 w-10 text-destructive"
								>
									<Trash2 className="h-4 w-4" />
								</Button>
							</div>
						))}
						<div className="flex gap-2">
							<Input 
								value={newEndpoint}
								onChange={(e) => setNewEndpoint(e.target.value)}
								placeholder="orderer.example.com:7050"
							/>
							<Button 
								type="button" 
								onClick={handleAddEndpoint}
								className="whitespace-nowrap"
							>
								<Plus className="h-4 w-4 mr-2" />
								Add Endpoint
							</Button>
						</div>
					</div>

					<div className="space-y-2">
						<FormLabel>Root Certificates</FormLabel>
						{rootCertsFields.map((field, i) => (
							<div key={field.id} className="flex gap-2">
								<Textarea 
									{...formContext.register(`operations.${index}.payload.root_certs.${i}`)}
									className="flex-1 min-h-[100px]"
									placeholder="-----BEGIN CERTIFICATE-----
MIICJzCCAc2gAwIBAgIUMR9...
-----END CERTIFICATE-----"
								/>
								<Button 
									type="button" 
									variant="ghost" 
									size="icon" 
									onClick={() => removeRootCert(i)}
									className="h-10 w-10 text-destructive self-start"
								>
									<Trash2 className="h-4 w-4" />
								</Button>
							</div>
						))}
						<div className="flex gap-2">
							<Textarea 
								value={newRootCert}
								onChange={(e) => setNewRootCert(e.target.value)}
								placeholder="Paste PEM certificate"
								className="flex-1 min-h-[100px]"
							/>
							<Button 
								type="button" 
								onClick={handleAddRootCert}
								className="whitespace-nowrap self-start"
							>
								Add Certificate
							</Button>
						</div>
					</div>

					<div className="space-y-2">
						<FormLabel>TLS Root Certificates</FormLabel>
						{tlsRootCertsFields.map((field, i) => (
							<div key={field.id} className="flex gap-2">
								<Textarea 
									{...formContext.register(`operations.${index}.payload.tls_root_certs.${i}`)}
									className="flex-1 min-h-[100px]"
									placeholder="-----BEGIN CERTIFICATE-----
MIICJzCCAc2gAwIBAgIUMR9...
-----END CERTIFICATE-----"
								/>
								<Button 
									type="button" 
									variant="ghost" 
									size="icon" 
									onClick={() => removeTlsRootCert(i)}
									className="h-10 w-10 text-destructive self-start"
								>
									<Trash2 className="h-4 w-4" />
								</Button>
							</div>
						))}
						<div className="flex gap-2">
							<Textarea 
								value={newTlsRootCert}
								onChange={(e) => setNewTlsRootCert(e.target.value)}
								placeholder="Paste PEM certificate"
								className="flex-1 min-h-[100px]"
							/>
							<Button 
								type="button" 
								onClick={handleAddTlsRootCert}
								className="whitespace-nowrap self-start"
							>
								Add Certificate
							</Button>
						</div>
					</div>
				</div>
			</CardContent>
		</Card>
	)
} 
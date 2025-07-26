import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Trash2 } from 'lucide-react'
import { useMemo, useEffect, useRef } from 'react'
import { useFormContext } from 'react-hook-form'

interface UpdateApplicationACLOperationProps {
	index: number
	onRemove: () => void
	channelConfig?: any
}

export function UpdateApplicationACLOperation({ index, onRemove, channelConfig }: UpdateApplicationACLOperationProps) {
	const form = useFormContext()
	const selectedACLName = form.watch(`operations.${index}.payload.acl_name`)
	const hasSetInitialValue = useRef(false)

	// Get existing ACLs from channel config
	const existingACLs = channelConfig?.config?.data?.data?.[0]?.payload?.data?.config?.channel_group?.groups?.Application?.values?.ACLs?.value?.acls || {}

	// Get current ACL policy reference
	const getCurrentACLPolicy = (aclName: string) => {
		if (!aclName || !existingACLs[aclName]) return ''
		return existingACLs[aclName].policy || ''
	}

	// Auto-populate policy reference when ACL name changes
	useEffect(() => {
		if (selectedACLName && !hasSetInitialValue.current) {
			const currentPolicy = getCurrentACLPolicy(selectedACLName)
			form.setValue(`operations.${index}.payload.policy`, currentPolicy)
			hasSetInitialValue.current = true
		}
	}, [selectedACLName, index])

	// Get available policy names from the channel config
	const availablePolicies = useMemo(() => {
		const policies: string[] = []

		// Add application policies
		const appPolicies = channelConfig?.config?.data?.data?.[0]?.payload?.data?.config?.channel_group?.groups?.Application?.policies || {}
		Object.keys(appPolicies).forEach((policy) => policies.push(policy))

		return policies
	}, [channelConfig])

	return (
		<Card className="mb-6">
			<CardHeader className="pb-3">
				<div className="flex justify-between items-center">
					<CardTitle className="text-lg font-medium">Update Application ACL</CardTitle>
					<Button variant="ghost" size="icon" onClick={onRemove} className="h-8 w-8 text-destructive">
						<Trash2 className="h-4 w-4" />
					</Button>
				</div>
			</CardHeader>
			<CardContent>
				<div className="space-y-4">
					<FormField
						control={form.control}
						name={`operations.${index}.payload.acl_name`}
						render={({ field }) => (
							<FormItem>
								<FormLabel>ACL Name</FormLabel>
								<Select onValueChange={field.onChange} value={field.value}>
									<FormControl>
										<SelectTrigger>
											<SelectValue placeholder="Select ACL to update" />
										</SelectTrigger>
									</FormControl>
									<SelectContent>
										{Object.keys(existingACLs).map((aclName) => (
											<SelectItem key={aclName} value={aclName}>
												{aclName}
											</SelectItem>
										))}
									</SelectContent>
								</Select>
								<FormMessage />
							</FormItem>
						)}
					/>

					<FormField
						control={form.control}
						name={`operations.${index}.payload.policy`}
						render={({ field }) => (
							<FormItem>
								<FormLabel>Policy Reference</FormLabel>
								<Select onValueChange={field.onChange} value={field.value}>
									<FormControl>
										<SelectTrigger>
											<SelectValue placeholder="Select policy reference" />
										</SelectTrigger>
									</FormControl>
									<SelectContent>
										{availablePolicies.map((policy) => (
											<SelectItem key={policy} value={`/Channel/Application/${policy}`}>
												{`/Channel/Application/${policy}`}
											</SelectItem>
										))}
									</SelectContent>
								</Select>
								<FormMessage />
							</FormItem>
						)}
					/>
				</div>
			</CardContent>
		</Card>
	)
}

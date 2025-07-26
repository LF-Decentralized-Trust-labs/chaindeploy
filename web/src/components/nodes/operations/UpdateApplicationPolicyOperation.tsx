import { Button } from '@/components/ui/button'
import { FormControl, FormField, FormItem, FormLabel, FormMessage, FormDescription } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Checkbox } from '@/components/ui/checkbox'
import { Trash2 } from 'lucide-react'
import { useFormContext } from 'react-hook-form'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useEffect } from 'react'

interface UpdateApplicationPolicyOperationProps {
	index: number
	onRemove: () => void
	channelConfig?: any
}

export function UpdateApplicationPolicyOperation({ index, onRemove, channelConfig }: UpdateApplicationPolicyOperationProps) {
	const form = useFormContext()
	const policyType = form.watch(`operations.${index}.payload.policy.type`)
	const signatureOperator = form.watch(`operations.${index}.payload.policy.signatureOperator`)
	const selectedPolicyName = form.watch(`operations.${index}.payload.policy_name`)

	// Get current policy from channel config
	const getCurrentPolicy = (policyName: string) => {
		const policies = channelConfig?.config?.data?.data?.[0]?.payload?.data?.config?.channel_group?.groups?.Application?.policies || {}
		return policies[policyName]
	}

	// Populate form with current policy values when policy name changes
	useEffect(() => {
		if (selectedPolicyName && channelConfig) {
			const currentPolicy = getCurrentPolicy(selectedPolicyName)
			if (currentPolicy?.policy) {
				const policy = currentPolicy.policy

				// Set policy type
				if (policy.type === 1) {
					form.setValue(`operations.${index}.payload.policy.type`, 'Signature')

					// Parse signature policy
					const rule = policy.value?.rule
					if (rule?.n_out_of) {
						const n = rule.n_out_of.n
						const rules = rule.n_out_of.rules || []

						if (n === 1 && rules.length === 1) {
							form.setValue(`operations.${index}.payload.policy.signatureOperator`, 'OR')
						} else if (n === rules.length) {
							form.setValue(`operations.${index}.payload.policy.signatureOperator`, 'AND')
						} else {
							form.setValue(`operations.${index}.payload.policy.signatureOperator`, 'OUTOF')
							form.setValue(`operations.${index}.payload.policy.signatureN`, n)
						}

						// Extract organization MSP IDs
						const orgs = rules.filter((r: any) => r.signed_by !== undefined).map((r: any) => r.signed_by)
						form.setValue(`operations.${index}.payload.policy.organizations`, orgs)
					}
				} else if (policy.type === 3) {
					form.setValue(`operations.${index}.payload.policy.type`, 'ImplicitMeta')

					// Parse implicit meta policy
					const rule = policy.value?.rule
					if (rule) {
						const parts = rule.split(' ')
						if (parts.length === 2) {
							form.setValue(`operations.${index}.payload.policy.implicitMetaOperator`, parts[0])
							form.setValue(`operations.${index}.payload.policy.implicitMetaRole`, parts[1])
						}
					}
				}
			}
		}
	}, [selectedPolicyName, channelConfig, form, index])

	return (
		<Card>
			<CardHeader className="flex-row items-center justify-between space-y-0 pb-2">
				<CardTitle className="text-base font-medium">Update Application Policy</CardTitle>
				<Button type="button" variant="ghost" size="icon" onClick={onRemove} className="text-muted-foreground hover:text-destructive">
					<Trash2 className="h-4 w-4" />
				</Button>
			</CardHeader>
			<CardContent className="space-y-4">
				<FormField
					control={form.control}
					name={`operations.${index}.payload.policy_name`}
					render={({ field }) => (
						<FormItem>
							<FormLabel>Policy Name</FormLabel>
							<Select onValueChange={field.onChange} value={field.value}>
								<FormControl>
									<SelectTrigger>
										<SelectValue placeholder="Select policy" />
									</SelectTrigger>
								</FormControl>
								<SelectContent>
									<SelectItem value="Admins">Admins</SelectItem>
									<SelectItem value="Readers">Readers</SelectItem>
									<SelectItem value="Writers">Writers</SelectItem>
									<SelectItem value="LifecycleEndorsement">LifecycleEndorsement</SelectItem>
									<SelectItem value="Endorsement">Endorsement</SelectItem>
								</SelectContent>
							</Select>
							{selectedPolicyName && (
								<div className="text-sm text-muted-foreground mt-1">
									Current:{' '}
									{(() => {
										const currentPolicy = getCurrentPolicy(selectedPolicyName)
										if (currentPolicy?.policy?.type === 1) {
											const rule = currentPolicy.policy.value?.rule
											if (rule?.n_out_of) {
												const n = rule.n_out_of.n
												const rules = rule.n_out_of.rules || []
												if (n === 1 && rules.length === 1) {
													return `OR(${rules[0]?.signed_by || 'unknown'})`
												} else if (n === rules.length) {
													return `AND(${rules.map((r: any) => r.signed_by).join(', ')})`
												} else {
													return `OutOf(${n}, ${rules.map((r: any) => r.signed_by).join(', ')})`
												}
											}
										} else if (currentPolicy?.policy?.type === 3) {
											return currentPolicy.policy.value?.rule || 'Unknown'
										}
										return 'Unknown policy type'
									})()}
								</div>
							)}
							<FormMessage />
						</FormItem>
					)}
				/>

				<FormField
					control={form.control}
					name={`operations.${index}.payload.policy.type`}
					render={({ field }) => (
						<FormItem>
							<FormLabel>Type</FormLabel>
							<Select onValueChange={field.onChange} value={field.value}>
								<FormControl>
									<SelectTrigger>
										<SelectValue placeholder="Select type" />
									</SelectTrigger>
								</FormControl>
								<SelectContent>
									<SelectItem value="ImplicitMeta">ImplicitMeta</SelectItem>
									<SelectItem value="Signature">Signature</SelectItem>
								</SelectContent>
							</Select>
							<FormMessage />
						</FormItem>
					)}
				/>

				{policyType === 'ImplicitMeta' ? (
					<div className="flex gap-4">
						<FormField
							control={form.control}
							name={`operations.${index}.payload.policy.implicitMetaOperator`}
							render={({ field }) => (
								<FormItem className="flex-1">
									<FormLabel>Operator</FormLabel>
									<Select onValueChange={field.onChange} value={field.value}>
										<FormControl>
											<SelectTrigger>
												<SelectValue placeholder="Select operator" />
											</SelectTrigger>
										</FormControl>
										<SelectContent>
											<SelectItem value="ANY">ANY</SelectItem>
											<SelectItem value="MAJORITY">MAJORITY</SelectItem>
										</SelectContent>
									</Select>
									<FormMessage />
								</FormItem>
							)}
						/>
						<FormField
							control={form.control}
							name={`operations.${index}.payload.policy.implicitMetaRole`}
							render={({ field }) => (
								<FormItem className="flex-1">
									<FormLabel>Role</FormLabel>
									<Select onValueChange={field.onChange} value={field.value}>
										<FormControl>
											<SelectTrigger>
												<SelectValue placeholder="Select role" />
											</SelectTrigger>
										</FormControl>
										<SelectContent>
											<SelectItem value="Readers">Readers</SelectItem>
											<SelectItem value="Writers">Writers</SelectItem>
											<SelectItem value="Admins">Admins</SelectItem>
											<SelectItem value="LifecycleEndorsement">LifecycleEndorsement</SelectItem>
											<SelectItem value="Endorsement">Endorsement</SelectItem>
										</SelectContent>
									</Select>
									<FormMessage />
								</FormItem>
							)}
						/>
					</div>
				) : (
					<div className="space-y-4">
						<FormField
							control={form.control}
							name={`operations.${index}.payload.policy.signatureOperator`}
							render={({ field }) => (
								<FormItem>
									<FormLabel>Operator</FormLabel>
									<Select onValueChange={field.onChange} value={field.value}>
										<FormControl>
											<SelectTrigger>
												<SelectValue placeholder="Select operator" />
											</SelectTrigger>
										</FormControl>
										<SelectContent>
											<SelectItem value="OR">OR</SelectItem>
											<SelectItem value="AND">AND</SelectItem>
											<SelectItem value="OUTOF">OUTOF</SelectItem>
										</SelectContent>
									</Select>
									<FormMessage />
								</FormItem>
							)}
						/>

						{signatureOperator === 'OUTOF' && (
							<FormField
								control={form.control}
								name={`operations.${index}.payload.policy.signatureN`}
								render={({ field }) => (
									<FormItem>
										<FormLabel>Number of Required Signatures</FormLabel>
										<FormControl>
											<Input type="number" min={1} value={field.value} onChange={(e) => field.onChange(parseInt(e.target.value))} />
										</FormControl>
										<FormDescription>Number of organizations that must sign</FormDescription>
										<FormMessage />
									</FormItem>
								)}
							/>
						)}

						<FormField
							control={form.control}
							name={`operations.${index}.payload.policy.organizations`}
							render={({ field }) => (
								<FormItem>
									<FormLabel>Organizations</FormLabel>
									<div className="space-y-2">
										{Object.entries(channelConfig?.config?.data?.data?.[0]?.payload?.data?.config?.channel_group?.groups?.Application?.groups || {}).map(
											([mspId, org]: [string, any]) => (
												<FormItem key={mspId} className="flex items-center gap-2">
													<FormControl>
														<Checkbox
															checked={field.value?.includes(mspId)}
															onCheckedChange={(checked) => {
																const value = checked ? [...(field.value || []), mspId] : (field.value || []).filter((id: string) => id !== mspId)
																field.onChange(value)
															}}
														/>
													</FormControl>
													<FormLabel className="!mt-0">{mspId}</FormLabel>
												</FormItem>
											)
										)}
									</div>
									<FormMessage />
								</FormItem>
							)}
						/>
					</div>
				)}
			</CardContent>
		</Card>
	)
}

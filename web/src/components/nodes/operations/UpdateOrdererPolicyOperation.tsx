import { Button } from '@/components/ui/button'
import { FormControl, FormField, FormItem, FormLabel, FormMessage, FormDescription } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Checkbox } from '@/components/ui/checkbox'
import { Trash2 } from 'lucide-react'
import { useFormContext } from 'react-hook-form'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

interface UpdateOrdererPolicyOperationProps {
	index: number
	onRemove: () => void
	channelConfig?: any
}

export function UpdateOrdererPolicyOperation({ index, onRemove, channelConfig }: UpdateOrdererPolicyOperationProps) {
	const form = useFormContext()
	const policyType = form.watch(`operations.${index}.payload.policy.type`)
	const signatureOperator = form.watch(`operations.${index}.payload.policy.signatureOperator`)

	return (
		<Card>
			<CardHeader className="flex-row items-center justify-between space-y-0 pb-2">
				<CardTitle className="text-base font-medium">Update Orderer Policy</CardTitle>
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
									<SelectItem value="BlockValidation">BlockValidation</SelectItem>
								</SelectContent>
							</Select>
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
											<SelectItem value="BlockValidation">BlockValidation</SelectItem>
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
											<Input
												type="number"
												min={1}
												value={field.value}
												onChange={(e) => field.onChange(parseInt(e.target.value))}
											/>
										</FormControl>
										<FormDescription>
											Number of organizations that must sign
										</FormDescription>
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
										{Object.entries(channelConfig?.config?.data?.data?.[0]?.payload?.data?.config?.channel_group?.groups?.Orderer?.groups || {}).map(([mspId, org]: [string, any]) => (
											<FormItem key={mspId} className="flex items-center gap-2">
												<FormControl>
													<Checkbox
														checked={field.value?.includes(mspId)}
														onCheckedChange={(checked) => {
															const value = checked
																? [...(field.value || []), mspId]
																: (field.value || []).filter((id: string) => id !== mspId)
															field.onChange(value)
														}}
													/>
												</FormControl>
												<FormLabel className="!mt-0">{mspId}</FormLabel>
											</FormItem>
										))}
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
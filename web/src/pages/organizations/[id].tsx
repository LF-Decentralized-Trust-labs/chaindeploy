import {
	deleteOrganizationsByIdCrlRevokeSerialMutation,
	getOrganizationsByIdKeysOptions,
	getOrganizationsByIdOptions,
	getOrganizationsByIdRevokedCertificatesOptions,
	postOrganizationsByIdCrlRevokePemMutation,
	postOrganizationsByIdCrlRevokeSerialMutation,
	postOrganizationsByIdKeysMutation,
	postOrganizationsByIdKeysRenewMutation,
} from '@/api/client/@tanstack/react-query.gen'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { CertificateViewer } from '@/components/ui/certificate-viewer'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { TimeAgo } from '@/components/ui/time-ago'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { ArrowLeft, Building2, Key as KeyIcon, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { useCallback, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import * as z from 'zod'

// Add these form schemas
const serialNumberSchema = z.object({
	serialNumber: z.string().min(1, 'Serial number is required'),
})

const pemSchema = z.object({
	pem: z.string().min(1, 'PEM certificate is required'),
})

const createKeySchema = z.object({
	name: z.string().min(1, 'Name is required'),
	description: z.string().optional(),
	role: z.enum(['admin', 'client']),
	dnsNames: z.string().optional(),
	ipAddresses: z.string().optional(),
})

const renewKeySchema = z.object({
	caType: z.enum(['tls', 'sign']),
	role: z.enum(['admin', 'client']),
	validFor: z.string().optional(),
	dnsNames: z.string().optional(),
	ipAddresses: z.string().optional(),
})

// Add this component after your existing cards
function CRLManagement({ orgId }: { orgId: number }) {
	// Query for getting CRL
	const {
		data: crl,
		refetch,
		isLoading: isCrlLoading,
	} = useQuery({
		...getOrganizationsByIdRevokedCertificatesOptions({
			path: { id: orgId },
		}),
	})

	// Form for serial number
	const serialForm = useForm<z.infer<typeof serialNumberSchema>>({
		resolver: zodResolver(serialNumberSchema),
	})

	// Form for PEM
	const pemForm = useForm<z.infer<typeof pemSchema>>({
		resolver: zodResolver(pemSchema),
	})

	// Mutation for adding by serial number
	const addBySerialMutation = useMutation({
		...postOrganizationsByIdCrlRevokeSerialMutation(),
		onSuccess: () => {
			toast.success('Certificate revoked successfully')
			refetch()
			serialForm.reset()
			setSerialDialogOpen(false)
		},
		onError: (e) => {
			toast.error(`Error revoking certificate: ${(e.error as any).message}`)
		},
	})

	// Mutation for adding by PEM
	const addByPemMutation = useMutation({
		...postOrganizationsByIdCrlRevokePemMutation(),
		onSuccess: () => {
			toast.success('Certificate revoked successfully')
			pemForm.reset()
			setPemDialogOpen(false)
		},
		onError: (e) => {
			toast.error(`Error revoking certificate: ${(e.error as any).message}`)
		},
	})

	// Add unrevoke mutation
	const unrevokeMutation = useMutation({
		...deleteOrganizationsByIdCrlRevokeSerialMutation(),
		onSuccess: () => {
			toast.success('Certificate unrevoked successfully')
			refetch()
			setCertificateToDelete(null)
		},
		onError: (e) => {
			toast.error(`Error unrevoking certificate: ${(e.error as any).message}`)
		},
	})

	const [serialDialogOpen, setSerialDialogOpen] = useState(false)
	const [pemDialogOpen, setPemDialogOpen] = useState(false)
	const [certificateToDelete, setCertificateToDelete] = useState<string | null>(null)

	// Update the revoked certificates list rendering
	const RevokedCertificatesList = () => {
		if (isCrlLoading) {
			return <Skeleton className="h-32 w-full" />
		}

		return (
			<div className="bg-muted rounded-lg p-4">
				<h3 className="text-sm font-medium mb-2">Revoked Certificates</h3>
				{crl?.length ? (
					<div className="space-y-2">
						{crl.map((cert) => (
							<div key={cert.serialNumber} className="flex items-center justify-between text-sm p-2 rounded-md hover:bg-muted-foreground/5">
								<div>
									<span className="font-mono">{cert.serialNumber}</span>
									<span className="text-muted-foreground ml-2">
										<TimeAgo date={cert.revocationTime!} />
									</span>
								</div>
								<Button variant="destructive" size="icon" onClick={() => setCertificateToDelete(cert.serialNumber!)}>
									<Trash2 className="h-4 w-4" />
								</Button>
							</div>
						))}
					</div>
				) : (
					<p className="text-sm text-muted-foreground">No certificates have been revoked</p>
				)}
			</div>
		)
	}

	return (
		<Card className="p-6">
			<div className="flex items-center gap-4 mb-6">
				<div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center">
					<KeyIcon className="h-6 w-6 text-primary" />
				</div>
				<div>
					<h2 className="text-lg font-semibold">Certificate Revocation List</h2>
					<p className="text-sm text-muted-foreground">Manage revoked certificates</p>
				</div>
			</div>

			<div className="space-y-4">
				<div className="flex gap-4">
					<Dialog open={serialDialogOpen} onOpenChange={setSerialDialogOpen}>
						<DialogTrigger asChild>
							<Button>Revoke by Serial Number</Button>
						</DialogTrigger>
						<DialogContent>
							<DialogHeader>
								<DialogTitle>Revoke Certificate by Serial Number</DialogTitle>
								<DialogDescription>Enter the serial number of the certificate to revoke</DialogDescription>
							</DialogHeader>
							<Form {...serialForm}>
								<form
									onSubmit={serialForm.handleSubmit((data) =>
										addBySerialMutation.mutate({
											path: { id: orgId },
											body: { serialNumber: data.serialNumber },
										})
									)}
								>
									<FormField
										control={serialForm.control}
										name="serialNumber"
										render={({ field }) => (
											<FormItem>
												<FormLabel>Serial Number</FormLabel>
												<FormControl>
													<Input {...field} />
												</FormControl>
												<FormMessage />
											</FormItem>
										)}
									/>
									<DialogFooter className="mt-4">
										<Button type="submit" disabled={addBySerialMutation.isPending}>
											Revoke Certificate
										</Button>
									</DialogFooter>
								</form>
							</Form>
						</DialogContent>
					</Dialog>

					<Dialog open={pemDialogOpen} onOpenChange={setPemDialogOpen}>
						<DialogTrigger asChild>
							<Button>Revoke by PEM</Button>
						</DialogTrigger>
						<DialogContent>
							<DialogHeader>
								<DialogTitle>Revoke Certificate by PEM</DialogTitle>
								<DialogDescription>Paste the PEM certificate to revoke</DialogDescription>
							</DialogHeader>
							<Form {...pemForm}>
								<form
									onSubmit={pemForm.handleSubmit((data) =>
										addByPemMutation.mutate({
											path: { id: orgId },
											body: { certificate: data.pem },
										})
									)}
								>
									<FormField
										control={pemForm.control}
										name="pem"
										render={({ field }) => (
											<FormItem>
												<FormLabel>PEM Certificate</FormLabel>
												<FormControl>
													<Textarea {...field} rows={8} />
												</FormControl>
												<FormMessage />
											</FormItem>
										)}
									/>
									<DialogFooter className="mt-4">
										<Button type="submit" disabled={addByPemMutation.isPending}>
											Revoke Certificate
										</Button>
									</DialogFooter>
								</form>
							</Form>
						</DialogContent>
					</Dialog>
				</div>

				<RevokedCertificatesList />

				{/* Add confirmation dialog */}
				<AlertDialog open={Boolean(certificateToDelete)} onOpenChange={(open) => !open && setCertificateToDelete(null)}>
					<AlertDialogContent>
						<AlertDialogHeader>
							<AlertDialogTitle>Unrevoke Certificate</AlertDialogTitle>
							<AlertDialogDescription>
								Are you sure you want to unrevoke this certificate? This action cannot be undone.
								<div className="mt-2 p-2 bg-muted rounded-md">
									<code className="text-sm">{certificateToDelete}</code>
								</div>
							</AlertDialogDescription>
						</AlertDialogHeader>
						<AlertDialogFooter>
							<AlertDialogCancel>Cancel</AlertDialogCancel>
							<AlertDialogAction
								onClick={() => {
									if (certificateToDelete) {
										unrevokeMutation.mutate({
											path: {
												id: orgId,
											},
											body: {
												serialNumber: certificateToDelete,
											},
										})
									}
								}}
								className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
							>
								Unrevoke
							</AlertDialogAction>
						</AlertDialogFooter>
					</AlertDialogContent>
				</AlertDialog>
			</div>
		</Card>
	)
}

// Add the Keys Management component
function KeysManagement({ orgId }: { orgId: number }) {
	const {
		data: keysData,
		refetch,
		isLoading: isKeysLoading,
	} = useQuery({
		...getOrganizationsByIdKeysOptions({
			path: { id: orgId },
		}),
	})

	const navigate = useNavigate()

	const createKeyForm = useForm<z.infer<typeof createKeySchema>>({
		resolver: zodResolver(createKeySchema),
		defaultValues: {
			role: 'client',
		},
	})

	const renewKeyForm = useForm<z.infer<typeof renewKeySchema>>({
		resolver: zodResolver(renewKeySchema),
		defaultValues: {
			caType: 'sign',
			role: 'client',
		},
	})

	const createKeyMutation = useMutation({
		...postOrganizationsByIdKeysMutation(),
		onSuccess: (data) => {
			toast.success('Key created successfully')
			refetch()
			createKeyForm.reset()
			setCreateDialogOpen(false)
			// Redirect to the key detail page
			if (data.id) {
				navigate(`/settings/keys/${data.id}`)
			}
		},
		onError: (e) => {
			toast.error(`Error creating key: ${(e.error as any).message}`)
		},
	})

	const renewKeyMutation = useMutation({
		...postOrganizationsByIdKeysRenewMutation(),
		onSuccess: () => {
			toast.success('Key renewed successfully')
			refetch()
			renewKeyForm.reset()
			setRenewDialogOpen(false)
		},
		onError: (e) => {
			toast.error(`Error renewing key: ${(e.error as any).message}`)
		},
	})

	const [createDialogOpen, setCreateDialogOpen] = useState(false)
	const [renewDialogOpen, setRenewDialogOpen] = useState(false)
	const [selectedKeyForRenew, setSelectedKeyForRenew] = useState<{ id: number; name: string } | null>(null)

	// Helper function to check if a key is renewable
	const isKeyRenewable = useCallback((keyString: string) => {
		return keyString.includes('admin') || keyString.includes('client')
	}, [])

	const handleRenewKey = useCallback((key: { id: number; name: string }) => {
		setSelectedKeyForRenew(key)
		setRenewDialogOpen(true)
	}, [])

	const handleRenewSubmit = useCallback((data: z.infer<typeof renewKeySchema>) => {
		if (!selectedKeyForRenew) return

		const payload = {
			keyId: selectedKeyForRenew.id,
			caType: data.caType,
			role: data.role,
			...(data.validFor && { validFor: data.validFor }),
			...(data.dnsNames && { dnsNames: data.dnsNames.split(',').map(s => s.trim()) }),
			...(data.ipAddresses && { ipAddresses: data.ipAddresses.split(',').map(s => s.trim()) }),
		}

		renewKeyMutation.mutate({
			path: { id: orgId },
			body: payload,
		})
	}, [selectedKeyForRenew, renewKeyMutation, orgId])

	const handleCreateKey = useCallback((data: z.infer<typeof createKeySchema>) => {
		const payload = {
			name: data.name,
			description: data.description,
			role: data.role,
			...(data.dnsNames && { dnsNames: data.dnsNames.split(',').map(s => s.trim()) }),
			...(data.ipAddresses && { ipAddresses: data.ipAddresses.split(',').map(s => s.trim()) }),
		}

		createKeyMutation.mutate({
			path: { id: orgId },
			body: payload,
		})
	}, [createKeyMutation, orgId])
	const keys = useMemo(() => keysData?.keys ? Object.entries(keysData.keys)
		.map(([key, value]) => ({ key, ...value }))
		.filter(key => isKeyRenewable(key.key)) : [], [keysData])	


	return (
		<Card className="p-6">
			<div className="flex items-center justify-between mb-6">
				<div className="flex items-center gap-4">
					<div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center">
						<KeyIcon className="h-6 w-6 text-primary" />
					</div>
					<div>
						<h2 className="text-lg font-semibold">Keys Management</h2>
						<p className="text-sm text-muted-foreground">Manage organization keys and certificates</p>
					</div>
				</div>
				<Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
					<DialogTrigger asChild>
						<Button>
							<Plus className="mr-2 h-4 w-4" />
							Create Key
						</Button>
					</DialogTrigger>
					<DialogContent className="max-w-md">
						<DialogHeader>
							<DialogTitle>Create New Key</DialogTitle>
							<DialogDescription>Create a new key for this organization</DialogDescription>
						</DialogHeader>
						<Form {...createKeyForm}>
							<form onSubmit={createKeyForm.handleSubmit(handleCreateKey)} className="space-y-4">
								<FormField
									control={createKeyForm.control}
									name="name"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Name</FormLabel>
											<FormControl>
												<Input {...field} placeholder="Enter key name" />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
								<FormField
									control={createKeyForm.control}
									name="description"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Description (Optional)</FormLabel>
											<FormControl>
												<Input {...field} placeholder="Enter description" />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
								<FormField
									control={createKeyForm.control}
									name="role"
									render={({ field }) => (
										<FormItem>
											<FormLabel>Role</FormLabel>
											<Select onValueChange={field.onChange} defaultValue={field.value}>
												<FormControl>
													<SelectTrigger>
														<SelectValue placeholder="Select role" />
													</SelectTrigger>
												</FormControl>
												<SelectContent>
													<SelectItem value="admin">Admin</SelectItem>
													<SelectItem value="client">Client</SelectItem>
												</SelectContent>
											</Select>
											<FormMessage />
										</FormItem>
									)}
								/>
								<FormField
									control={createKeyForm.control}
									name="dnsNames"
									render={({ field }) => (
										<FormItem>
											<FormLabel>DNS Names (Optional)</FormLabel>
											<FormControl>
												<Input {...field} placeholder="example.com,api.example.com" />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
								<FormField
									control={createKeyForm.control}
									name="ipAddresses"
									render={({ field }) => (
										<FormItem>
											<FormLabel>IP Addresses (Optional)</FormLabel>
											<FormControl>
												<Input {...field} placeholder="192.168.1.1,10.0.0.1" />
											</FormControl>
											<FormMessage />
										</FormItem>
									)}
								/>
								<DialogFooter>
									<Button type="submit" disabled={createKeyMutation.isPending}>
										Create Key
									</Button>
								</DialogFooter>
							</form>
						</Form>
					</DialogContent>
				</Dialog>
			</div>

			{isKeysLoading ? (
				<div className="space-y-4">
					<Skeleton className="h-20 w-full" />
					<Skeleton className="h-20 w-full" />
					<Skeleton className="h-20 w-full" />
				</div>
			) : keys.length === 0 ? (
				<div className="text-center py-8">
					<KeyIcon className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
					<h3 className="text-lg font-medium mb-2">No Keys Found</h3>
					<p className="text-sm text-muted-foreground mb-4">Create your first key to get started</p>
					<Button onClick={() => setCreateDialogOpen(true)}>
						<Plus className="mr-2 h-4 w-4" />
						Create Key
					</Button>
				</div>
			) : (
				<div className="space-y-4">
					{keys.map((key) => (
						<Card key={key.id} className="p-4">
							<div className="flex items-center justify-between mb-4">
								<div>
									<h3 className="font-medium">{key.name}</h3>
									{key.description && <p className="text-sm text-muted-foreground">{key.description}</p>}
								</div>
								<div className="flex items-center gap-2">
									<Badge variant={key.status === 'active' ? 'default' : 'secondary'}>
										{key.status}
									</Badge>
									{key.key.includes('admin') && (
										<Badge variant="outline" className="bg-blue-50 text-blue-700 border-blue-200">
											Admin
										</Badge>
									)}
									{key.key.includes('client') && (
										<Badge variant="outline" className="bg-green-50 text-green-700 border-green-200">
											Client
										</Badge>
									)}
								</div>
							</div>
							<div className="grid grid-cols-2 gap-4 text-sm mb-4">
								<div>
									<p className="text-muted-foreground">Algorithm</p>
									<p className="font-mono">{key.algorithm || 'N/A'}</p>
								</div>
								<div>
									<p className="text-muted-foreground">Created</p>
									<p>{key.createdAt ? <TimeAgo date={key.createdAt} /> : 'N/A'}</p>
								</div>
								{key.expiresAt && (
									<div>
										<p className="text-muted-foreground">Expires</p>
										<p>{<TimeAgo date={key.expiresAt} />}</p>
									</div>
								)}
								{key.lastRotatedAt && (
									<div>
										<p className="text-muted-foreground">Last Rotated</p>
										<p>{<TimeAgo date={key.lastRotatedAt} />}</p>
									</div>
								)}
							</div>
							{key.certificate && (
								<div className="mb-4">
									<CertificateViewer certificate={key.certificate} label="Certificate" />
								</div>
							)}
							<div className="flex justify-end gap-2">
								<Button
									variant="outline"
									size="sm"
									onClick={() => handleRenewKey({ id: key.id!, name: key.name! })}
									disabled={renewKeyMutation.isPending}
								>
									<RefreshCw className="mr-2 h-4 w-4" />
									Renew
								</Button>
							</div>
						</Card>
					))}
				</div>
			)}

			{/* Renew Key Dialog */}
			<Dialog open={renewDialogOpen} onOpenChange={setRenewDialogOpen}>
				<DialogContent className="max-w-md">
					<DialogHeader>
						<DialogTitle>Renew Key</DialogTitle>
						<DialogDescription>
							Renew the certificate for key: <strong>{selectedKeyForRenew?.name}</strong>
						</DialogDescription>
					</DialogHeader>
					<Form {...renewKeyForm}>
						<form onSubmit={renewKeyForm.handleSubmit(handleRenewSubmit)} className="space-y-4">
							<FormField
								control={renewKeyForm.control}
								name="caType"
								render={({ field }) => (
									<FormItem>
										<FormLabel>CA Type</FormLabel>
										<Select onValueChange={field.onChange} defaultValue={field.value}>
											<FormControl>
												<SelectTrigger>
													<SelectValue placeholder="Select CA type" />
												</SelectTrigger>
											</FormControl>
											<SelectContent>
												<SelectItem value="sign">Sign</SelectItem>
												<SelectItem value="tls">TLS</SelectItem>
											</SelectContent>
										</Select>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={renewKeyForm.control}
								name="role"
								render={({ field }) => (
									<FormItem>
										<FormLabel>Role</FormLabel>
										<Select onValueChange={field.onChange} defaultValue={field.value}>
											<FormControl>
												<SelectTrigger>
													<SelectValue placeholder="Select role" />
												</SelectTrigger>
											</FormControl>
											<SelectContent>
												<SelectItem value="admin">Admin</SelectItem>
												<SelectItem value="client">Client</SelectItem>
											</SelectContent>
										</Select>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={renewKeyForm.control}
								name="validFor"
								render={({ field }) => (
									<FormItem>
										<FormLabel>Valid For (Optional)</FormLabel>
										<FormControl>
											<Input {...field} placeholder="8760h (1 year)" />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={renewKeyForm.control}
								name="dnsNames"
								render={({ field }) => (
									<FormItem>
										<FormLabel>DNS Names (Optional)</FormLabel>
										<FormControl>
											<Input {...field} placeholder="example.com,api.example.com" />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<FormField
								control={renewKeyForm.control}
								name="ipAddresses"
								render={({ field }) => (
									<FormItem>
										<FormLabel>IP Addresses (Optional)</FormLabel>
										<FormControl>
											<Input {...field} placeholder="192.168.1.1,10.0.0.1" />
										</FormControl>
										<FormMessage />
									</FormItem>
								)}
							/>
							<DialogFooter>
								<Button type="submit" disabled={renewKeyMutation.isPending}>
									Renew Key
								</Button>
							</DialogFooter>
						</form>
					</Form>
				</DialogContent>
			</Dialog>
		</Card>
	)
}

export default function OrganizationDetailPage() {
	const { id } = useParams()
	const { data: org, isLoading } = useQuery({
		...getOrganizationsByIdOptions({
			path: { id: Number(id) },
		}),
	})

	if (isLoading) {
		return (
			<div className="flex-1 p-8">
				<div className="max-w-4xl mx-auto">
					<div className="mb-8">
						<Skeleton className="h-8 w-32 mb-2" />
						<Skeleton className="h-5 w-64" />
					</div>
					<div className="space-y-8">
						<Card className="p-6">
							<div className="space-y-4">
								<div className="flex items-center gap-4">
									<Skeleton className="h-12 w-12 rounded-lg" />
									<div>
										<Skeleton className="h-6 w-48 mb-2" />
										<Skeleton className="h-4 w-32" />
									</div>
								</div>
								<Skeleton className="h-24 w-full" />
							</div>
						</Card>
					</div>
				</div>
			</div>
		)
	}

	if (!org) {
		return (
			<div className="flex-1 p-8">
				<div className="max-w-4xl mx-auto text-center">
					<Building2 className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
					<h1 className="text-2xl font-semibold mb-2">Organization not found</h1>
					<p className="text-muted-foreground mb-8">The organization you're looking for doesn't exist or you don't have access to it.</p>
					<Button asChild>
						<Link to="/fabric/organizations">
							<ArrowLeft className="mr-2 h-4 w-4" />
							Back to Organizations
						</Link>
					</Button>
				</div>
			</div>
		)
	}

	return (
		<div className="flex-1 p-8">
			<div className="max-w-4xl mx-auto">
				<div className="flex items-center gap-2 text-muted-foreground mb-8">
					<Button variant="ghost" size="sm" asChild>
						<Link to="/fabric/organizations">
							<ArrowLeft className="mr-2 h-4 w-4" />
							Organizations
						</Link>
					</Button>
				</div>

				<div className="flex items-center justify-between mb-8">
					<div>
						<h1 className="text-2xl font-semibold mb-1">{org.mspId}</h1>
						<p className="text-muted-foreground">
							Created <TimeAgo date={org.createdAt!} />
						</p>
					</div>
				</div>

				<Tabs defaultValue="overview" className="space-y-6">
					<TabsList>
						<TabsTrigger value="overview">Overview</TabsTrigger>
						<TabsTrigger value="keys">Keys</TabsTrigger>
						<TabsTrigger value="crl">Certificate Revocation</TabsTrigger>
					</TabsList>

					<TabsContent value="overview" className="space-y-8">
						{/* Organization Info Card */}
						<Card className="p-6">
							<div className="flex items-center gap-4 mb-6">
								<div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center">
									<Building2 className="h-6 w-6 text-primary" />
								</div>
								<div>
									<h2 className="text-lg font-semibold">Organization Information</h2>
									<p className="text-sm text-muted-foreground">Details about your organization</p>
								</div>
							</div>

							<div className="space-y-6">
								<div>
									<h3 className="text-sm font-medium mb-2">MSP ID</h3>
									<p className="text-sm text-muted-foreground">{org.mspId}</p>
								</div>

								{org.description && (
									<div>
										<h3 className="text-sm font-medium mb-2">Description</h3>
										<p className="text-sm text-muted-foreground">{org.description}</p>
									</div>
								)}
							</div>
						</Card>

						<Card className="p-4">
							<div className="flex items-center justify-between">
								<div>
									<h3 className="font-medium mb-1">Sign Certificate</h3>
									<p className="text-sm text-muted-foreground">Organization signing certificate</p>
								</div>
								<Badge variant="outline">Active</Badge>
							</div>
							<div className="mt-4">
								<p className="text-xs text-muted-foreground mb-1">Certificate</p>
								<CertificateViewer certificate={org.signCertificate!} label="Sign Certificate" className="w-full" />
							</div>
							<div className="mt-4">
								<p className="text-xs text-muted-foreground mb-1">Public Key</p>
								<pre className="text-sm font-mono bg-muted p-4 rounded-lg overflow-x-auto whitespace-pre-wrap break-all">{org.signPublicKey}</pre>
							</div>
						</Card>

						{/* TLS Certificate */}
						<Card className="p-4">
							<div className="flex items-center justify-between">
								<div>
									<h3 className="font-medium mb-1">TLS Certificate</h3>
									<p className="text-sm text-muted-foreground">Organization TLS certificate</p>
								</div>
								<Badge variant="outline">Active</Badge>
							</div>
							<div className="mt-4">
								<p className="text-xs text-muted-foreground mb-1">Certificate</p>
								<CertificateViewer certificate={org.tlsCertificate!} label="TLS Certificate" className="w-full" />
							</div>
							<div className="mt-4">
								<p className="text-xs text-muted-foreground mb-1">Public Key</p>
								<pre className="text-sm font-mono bg-muted p-4 rounded-lg overflow-x-auto whitespace-pre-wrap break-all">{org.tlsPublicKey}</pre>
							</div>
						</Card>
					</TabsContent>

					<TabsContent value="keys">
						<KeysManagement orgId={Number(id)} />
					</TabsContent>

					<TabsContent value="crl">
						<CRLManagement orgId={Number(id)} />
					</TabsContent>
				</Tabs>
			</div>
		</div>
	)
}

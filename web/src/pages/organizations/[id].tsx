import {
	deleteOrganizationsByIdCrlRevokeSerialMutation,
	getOrganizationsByIdOptions,
	getOrganizationsByIdRevokedCertificatesOptions,
	postOrganizationsByIdCrlRevokePemMutation,
	postOrganizationsByIdCrlRevokeSerialMutation,
} from '@/api/client/@tanstack/react-query.gen'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { CertificateViewer } from '@/components/ui/certificate-viewer'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { TimeAgo } from '@/components/ui/time-ago'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { ArrowLeft, Building2, Key as KeyIcon, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useParams } from 'react-router-dom'
import { toast } from 'sonner'
import * as z from 'zod'

// Add these form schemas
const serialNumberSchema = z.object({
	serialNumber: z.string().min(1, 'Serial number is required'),
})

const pemSchema = z.object({
	pem: z.string().min(1, 'PEM certificate is required'),
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

				<div className="space-y-8">
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

					{/* Add the CRL Management section */}
					<CRLManagement orgId={Number(id)} />
				</div>
			</div>
		</div>
	)
}

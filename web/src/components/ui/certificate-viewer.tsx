import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { cn } from '@/lib/utils'
import { X509Certificate } from '@peculiar/x509'
import { format } from 'date-fns'
import { Calendar, CheckCircle2, Copy, Download, Eye, Key, Shield, XCircle } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Badge } from '../ui/badge'

interface CertificateViewerProps {
	label: string
	certificate: string
	className?: string
}

interface SubjectAlternativeName {
	type: string
	value: string
}

function parseSANs(cert: X509Certificate): SubjectAlternativeName[] {
	try {
		const sanExtension = cert.extensions.find((ext) => ext.type === '2.5.29.17')
		if (!sanExtension) return []

		// The SAN extension data is available in the 'parsedValue' property
		const sans =
			(sanExtension as any).names?.items.map((item: any) => ({
				type: item.type,
				value: item.value,
			})) || []

		return sans.map((san: any) => ({
			type: san.type,
			value: san.value,
		}))
	} catch (error) {
		console.error('Error parsing SANs:', error)
		return []
	}
}

export function CertificateViewer({ label, certificate, className }: CertificateViewerProps) {
	const [dialogOpen, setDialogOpen] = useState(false)

	let cert: X509Certificate | null = null
	let error: string | null = null
	let status = 'Valid'
	let isExpired = false
	let isNotYetValid = false

	try {
		cert = new X509Certificate(certificate)
		const now = new Date()
		isExpired = now > cert.notAfter
		isNotYetValid = now < cert.notBefore

		if (isExpired) status = 'Expired'
		if (isNotYetValid) status = 'Not Yet Valid'
	} catch (e) {
		error = (e as Error).message
	}

	const getStatusColor = () => {
		if (isExpired) return 'destructive'
		if (isNotYetValid) return 'outline'
		return 'default'
	}

	const getDaysRemaining = () => {
		if (!cert) return 0
		const days = Math.ceil((cert.notAfter.getTime() - new Date().getTime()) / (1000 * 60 * 60 * 24))
		return days
	}

	const copyToClipboard = async () => {
		try {
			await navigator.clipboard.writeText(certificate)
			toast.success('Certificate copied to clipboard')
		} catch (error) {
			toast.error('Failed to copy certificate')
		}
	}

	const downloadCertificate = () => {
		try {
			const blob = new Blob([certificate], { type: 'text/plain' })
			const url = window.URL.createObjectURL(blob)
			const a = document.createElement('a')
			a.href = url
			a.download = `${label.toLowerCase().replace(/\s+/g, '-')}.pem`
			document.body.appendChild(a)
			a.click()
			window.URL.revokeObjectURL(url)
			document.body.removeChild(a)
			toast.success('Certificate downloaded successfully')
		} catch (error) {
			toast.error('Failed to download certificate')
		}
	}

	if (error) {
		return (
			<Card className="p-4 bg-destructive/10 text-destructive">
				<p>Failed to decode certificate: {error}</p>
			</Card>
		)
	}

	if (!cert) {
		return null
	}

	const isCA = cert.extensions.some((ext) => ext.type === '2.5.29.19' && (ext as any).ca)
	const sans = parseSANs(cert)

	const CertificateDetails = () => (
		<div className="space-y-6">
			<div>
				<div className="flex items-center gap-2 mb-2">
					<Shield className="h-4 w-4 text-muted-foreground" />
					<h4 className="text-sm font-medium">Certificate Details</h4>
				</div>
				<Card className="p-4 bg-muted/50">
					<div className="grid gap-4 text-sm">
						<div>
							<span className="text-muted-foreground">Serial Number:</span> <code className="text-xs bg-muted px-2 py-1 rounded font-mono">{cert.serialNumber}</code>
						</div>
						<div>
							<span className="text-muted-foreground">Signature Algorithm:</span> <span className="font-medium">{cert.signatureAlgorithm?.name}</span>
						</div>
						<div>
							<span className="text-muted-foreground">Certificate Type:</span> <span className="font-medium">{isCA ? 'Certificate Authority (CA)' : 'End Entity'}</span>
						</div>
					</div>
				</Card>
			</div>

			<div>
				<div className="flex items-center gap-2 mb-2">
					<Key className="h-4 w-4 text-muted-foreground" />
					<h4 className="text-sm font-medium">Public Key</h4>
				</div>
				<Card className="p-4 bg-muted/50">
					<div className="grid gap-4 text-sm">
						<div>
							<span className="text-muted-foreground">Algorithm:</span> <span className="font-medium">{cert.publicKey.algorithm?.name}</span>
						</div>
						{cert.publicKey.algorithm?.name === 'EC' && (
							<div>
								<span className="text-muted-foreground">Curve:</span> <span className="font-medium">{(cert.publicKey as any).namedCurve}</span>
							</div>
						)}
						{cert.publicKey.algorithm?.name === 'RSA' && (
							<div>
								<span className="text-muted-foreground">Key Size:</span> <span className="font-medium">{(cert.publicKey as any).keySize} bits</span>
							</div>
						)}
					</div>
				</Card>
			</div>

			<div>
				<div className="flex items-center gap-2 mb-2">
					<Calendar className="h-4 w-4 text-muted-foreground" />
					<h4 className="text-sm font-medium">Validity Period</h4>
				</div>
				<Card className="p-4 bg-muted/50">
					<div className="grid gap-4 text-sm">
						<div className="flex items-center justify-between">
							<span className="text-muted-foreground">Valid From</span>
							<span className="font-medium">{format(cert.notBefore, 'PPpp')}</span>
						</div>
						<div className="flex items-center justify-between">
							<span className="text-muted-foreground">Valid To</span>
							<div className="flex items-center gap-2">
								{isExpired ? <XCircle className="h-4 w-4 text-destructive" /> : <CheckCircle2 className="h-4 w-4 text-success" />}
								<span>
									{format(cert.notAfter, 'PPpp')} ({getDaysRemaining()} days)
								</span>
							</div>
						</div>
					</div>
				</Card>
			</div>

			<div>
				<div className="flex items-center gap-2 mb-2">
					<Shield className="h-4 w-4 text-muted-foreground" />
					<h4 className="text-sm font-medium">Subject & Issuer</h4>
				</div>
				<div className="grid gap-4">
					<Card className="p-4 bg-muted/50">
						<div className="grid gap-2 text-sm">
							<span className="text-muted-foreground">Subject:</span>
							<pre className="text-xs bg-muted px-2 py-1 rounded block overflow-x-auto">{cert.subject}</pre>
						</div>
					</Card>
					<Card className="p-4 bg-muted/50">
						<div className="grid gap-2 text-sm">
							<span className="text-muted-foreground">Issuer:</span>
							<pre className="text-xs bg-muted px-2 py-1 rounded block overflow-x-auto">{cert.issuer}</pre>
						</div>
					</Card>
				</div>
			</div>

			{sans.length > 0 && (
				<div>
					<div className="flex items-center gap-2 mb-2">
						<Shield className="h-4 w-4 text-muted-foreground" />
						<h4 className="text-sm font-medium">Subject Alternative Names</h4>
					</div>
					<Card className="p-4 bg-muted/50">
						<div className="grid gap-2 text-sm">
							{sans.map((san, index) => (
								<div key={index} className="flex items-center gap-2">
									<Badge variant="outline" className="text-xs">
										{san.type}
									</Badge>
									<span className="font-mono text-xs">{san.value}</span>
								</div>
							))}
						</div>
					</Card>
				</div>
			)}

			{cert.extensions.length > 0 && (
				<div>
					<div className="flex items-center gap-2 mb-2">
						<Shield className="h-4 w-4 text-muted-foreground" />
						<h4 className="text-sm font-medium">Extensions</h4>
					</div>
					<Card className="p-4 bg-muted/50">
						<div className="grid gap-4 text-sm">
							{cert.extensions.map((ext, index) => (
								<div key={index}>
									<div className="font-medium">{ext.type}</div>
									<div className="text-muted-foreground ml-4">Critical: {ext.critical ? 'Yes' : 'No'}</div>
								</div>
							))}
						</div>
					</Card>
				</div>
			)}
		</div>
	)

	return (
		<>
			<Card className={cn('', className)}>
				<div className="flex items-center justify-between border-b p-4">
					<div className="flex items-center gap-2">
						<h3 className="font-medium">{label}</h3>
						<Badge variant={getStatusColor()}>{status}</Badge>
						{isCA && <Badge variant="secondary">CA</Badge>}
					</div>
					<div className="flex gap-2">
						<Button variant="ghost" size="sm" onClick={() => setDialogOpen(true)}>
							<Eye className="h-4 w-4 mr-2" />
							View Details
						</Button>
						<Button variant="ghost" size="sm" onClick={copyToClipboard}>
							<Copy className="h-4 w-4 mr-2" />
							Copy
						</Button>
						<Button variant="ghost" size="sm" onClick={downloadCertificate}>
							<Download className="h-4 w-4 mr-2" />
							Download
						</Button>
					</div>
				</div>

				<div className="p-4 font-mono text-xs">
					<pre className="whitespace-pre-wrap break-all bg-muted p-4 rounded-md">{certificate}</pre>
				</div>
			</Card>

			<Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
				<DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
					<DialogHeader>
						<DialogTitle>Certificate Details - {label}</DialogTitle>
					</DialogHeader>
					<div className="mt-4">
						<div className="flex items-center gap-2 mb-4">
							<Badge variant={getStatusColor()}>{status}</Badge>
							{isCA && <Badge variant="secondary">CA</Badge>}
							<Badge variant="outline">
								{getDaysRemaining()} days {isExpired ? 'expired' : 'remaining'}
							</Badge>
						</div>
						<CertificateDetails />
					</div>
				</DialogContent>
			</Dialog>
		</>
	)
}

import { ServiceFabricPeerProperties } from '@/api/client'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Separator } from '@/components/ui/separator'
import { CertificateViewer } from '@/components/ui/certificate-viewer'
import { Link } from 'react-router-dom'
import { useState } from 'react'
import { Eye } from 'lucide-react'

interface FabricPeerConfigProps {
	config: ServiceFabricPeerProperties
}

interface AddressOverrideModalProps {
	open: boolean
	onOpenChange: (open: boolean) => void
	override: {
		from: string
		to: string
		tlsCACert: string
	}
}

function AddressOverrideModal({ open, onOpenChange, override }: AddressOverrideModalProps) {
	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="max-w-2xl">
				<DialogHeader>
					<DialogTitle>Address Override Details</DialogTitle>
					<DialogDescription>Certificate and routing information</DialogDescription>
				</DialogHeader>
				<div className="space-y-4">
					<div className="grid grid-cols-2 gap-4">
						<div>
							<p className="text-sm font-medium text-muted-foreground">From Address</p>
							<p className="text-sm font-mono">{override.from}</p>
						</div>
						<div>
							<p className="text-sm font-medium text-muted-foreground">To Address</p>
							<p className="text-sm font-mono">{override.to}</p>
						</div>
					</div>
					<Separator />
					<div className="space-y-2">
						<p className="text-sm font-medium text-muted-foreground">TLS CA Certificate</p>
						<CertificateViewer label="TLS CA Certificate" certificate={override.tlsCACert} />
					</div>
				</div>
			</DialogContent>
		</Dialog>
	)
}

export function FabricPeerConfig({ config }: FabricPeerConfigProps) {
	const [selectedOverride, setSelectedOverride] = useState<{
		from: string
		to: string
		tlsCACert: string
	} | null>(null)

	return (
		<Card>
			<CardHeader>
				<CardTitle>Fabric Peer Configuration</CardTitle>
				<CardDescription>Peer-specific node settings</CardDescription>
			</CardHeader>
			<CardContent className="space-y-6">
				<div className="grid grid-cols-3 gap-4">
					<div>
						<p className="text-sm font-medium text-muted-foreground">Organization</p>
						<p>MSP ID: {config.mspId}</p>
						<p className="text-sm text-muted-foreground">Org ID: {config.organizationId}</p>
					</div>
					<div>
						<p className="text-sm font-medium text-muted-foreground">Key IDs</p>
						<p>
							Sign Key:{' '}
							<Link to={`/settings/keys/${config.signKeyId}`} className="text-blue-500 hover:underline">
								{config.signKeyId}
							</Link>
						</p>
						<p>
							TLS Key:{' '}
							<Link to={`/settings/keys/${config.tlsKeyId}`} className="text-blue-500 hover:underline">
								{config.tlsKeyId}
							</Link>
						</p>
					</div>
					{config.version && (
						<div>
							<p className="text-sm font-medium text-muted-foreground">Version</p>
							<p className="text-sm">{config.version}</p>
						</div>
					)}
				</div>

				<Separator />

				<div className="space-y-2">
					<p className="text-sm font-medium text-muted-foreground">Network Configuration</p>
					<div className="grid grid-cols-2 gap-4">
						<div>
							<p className="text-sm font-medium text-muted-foreground">Listen Address</p>
							<p className="text-sm">{config.listenAddress}</p>
						</div>
						<div>
							<p className="text-sm font-medium text-muted-foreground">Operations Address</p>
							<p className="text-sm">{config.operationsAddress}</p>
						</div>
						<div>
							<p className="text-sm font-medium text-muted-foreground">Chaincode Address</p>
							<p className="text-sm">{config.chaincodeAddress}</p>
						</div>
						<div>
							<p className="text-sm font-medium text-muted-foreground">Events Address</p>
							<p className="text-sm">{config.eventsAddress}</p>
						</div>
					</div>
				</div>

				{config.externalEndpoint && (
					<>
						<Separator />
						<div>
							<p className="text-sm font-medium text-muted-foreground">External Endpoint</p>
							<p className="text-sm">{config.externalEndpoint}</p>
						</div>
					</>
				)}

				{config.domainNames && config.domainNames.length > 0 && (
					<>
						<Separator />
						<div className="space-y-2">
							<p className="text-sm font-medium text-muted-foreground">Domains</p>
							<div className="flex flex-wrap gap-2">
								{config.domainNames.map((domain) => (
									<Badge key={domain} variant="outline">
										{domain}
									</Badge>
								))}
							</div>
						</div>
					</>
				)}

				{config.addressOverrides && config.addressOverrides.length > 0 && (
					<>
						<Separator />
						<div className="space-y-2">
							<p className="text-sm font-medium text-muted-foreground">Address Overrides</p>
							<div className="space-y-2">
								{config.addressOverrides.map((override, index) => (
									<div key={index} className="flex items-center justify-between rounded-lg border p-3">
										<div className="space-y-1">
											<p className="text-sm font-medium">
												{override.from} → {override.to}
											</p>
										</div>
										<Button
											variant="ghost"
											size="icon"
											onClick={() =>
												setSelectedOverride({
													from: override.from,
													to: override.to,
													tlsCACert: override.tlsCACert,
												})
											}
										>
											<Eye className="h-4 w-4" />
										</Button>
									</div>
								))}
							</div>
						</div>
					</>
				)}
			</CardContent>

			{selectedOverride && <AddressOverrideModal open={!!selectedOverride} onOpenChange={(open) => !open && setSelectedOverride(null)} override={selectedOverride} />}
		</Card>
	)
}

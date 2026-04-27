import { HttpNodeResponse, ServiceFabricXChildProperties, ServiceFabricXCommitterProperties, ServiceFabricXOrdererGroupProperties } from '@/api/client'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { CertificateViewer } from '@/components/ui/certificate-viewer'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { LogViewer } from '@/components/nodes/LogViewer'
import { Activity, Copy, ExternalLink, Globe, Key, Layers, Network, Server, Shield } from 'lucide-react'
import { toast } from 'sonner'

interface FabricXNodeDetailsProps {
	node: HttpNodeResponse
	logs: string
	events: any
	activeTab: string
	onTabChange: (value: string) => void
}

function Port({ label, host, port }: { label: string; host?: string; port?: number }) {
	if (!port) return null
	return (
		<div>
			<p className="text-sm font-medium text-muted-foreground">{label}</p>
			<p className="font-mono text-sm">
				{host ? `${host}:${port}` : port}
			</p>
		</div>
	)
}

// MetricsRow renders a per-role Prometheus /metrics URL with a copy
// button and external-link icon. Returns null when the URL is empty,
// which handles legacy nodes that predate monitoring port allocation.
function MetricsRow({ label, url }: { label: string; url?: string }) {
	if (!url) return null
	const onCopy = async () => {
		try {
			await navigator.clipboard.writeText(url)
			toast.success(`${label} metrics URL copied`)
		} catch {
			toast.error('Failed to copy to clipboard')
		}
	}
	return (
		<div className="flex items-center justify-between gap-2 rounded-md border bg-muted/30 px-3 py-2">
			<div className="min-w-0">
				<p className="text-xs font-medium text-muted-foreground">{label}</p>
				<p className="font-mono text-xs truncate">{url}</p>
			</div>
			<div className="flex shrink-0 items-center gap-1">
				<Button size="icon" variant="ghost" className="h-7 w-7" onClick={onCopy} title="Copy URL">
					<Copy className="h-3.5 w-3.5" />
				</Button>
				<Button size="icon" variant="ghost" className="h-7 w-7" asChild title="Open in new tab">
					<a href={url} target="_blank" rel="noreferrer">
						<ExternalLink className="h-3.5 w-3.5" />
					</a>
				</Button>
			</div>
		</div>
	)
}

function OrdererMetricsCard({ config }: { config: ServiceFabricXOrdererGroupProperties }) {
	const rows: { label: string; url?: string }[] = [
		{ label: 'Router', url: config.routerMetricsUrl },
		{ label: 'Batcher', url: config.batcherMetricsUrl },
		{ label: 'Consenter', url: config.consenterMetricsUrl },
		{ label: 'Assembler', url: config.assemblerMetricsUrl },
	]
	const visible = rows.filter((r) => !!r.url)
	if (visible.length === 0) return null
	return (
		<Card>
			<CardHeader>
				<div className="flex items-center gap-2">
					<Activity className="h-4 w-4 text-muted-foreground" />
					<CardTitle>Prometheus Metrics</CardTitle>
				</div>
				<CardDescription>Scrape these endpoints from Prometheus or curl them directly.</CardDescription>
			</CardHeader>
			<CardContent className="space-y-2">
				{visible.map((r) => (
					<MetricsRow key={r.label} label={r.label} url={r.url} />
				))}
			</CardContent>
		</Card>
	)
}

function CommitterMetricsCard({ config }: { config: ServiceFabricXCommitterProperties }) {
	const rows: { label: string; url?: string }[] = [
		{ label: 'Sidecar', url: config.sidecarMetricsUrl },
		{ label: 'Coordinator', url: config.coordinatorMetricsUrl },
		{ label: 'Validator', url: config.validatorMetricsUrl },
		{ label: 'Verifier', url: config.verifierMetricsUrl },
		{ label: 'Query Service', url: config.queryServiceMetricsUrl },
	]
	const visible = rows.filter((r) => !!r.url)
	if (visible.length === 0) return null
	return (
		<Card>
			<CardHeader>
				<div className="flex items-center gap-2">
					<Activity className="h-4 w-4 text-muted-foreground" />
					<CardTitle>Prometheus Metrics</CardTitle>
				</div>
				<CardDescription>Scrape these endpoints from Prometheus or curl them directly.</CardDescription>
			</CardHeader>
			<CardContent className="space-y-2">
				{visible.map((r) => (
					<MetricsRow key={r.label} label={r.label} url={r.url} />
				))}
			</CardContent>
		</Card>
	)
}

// ChildMetricsCard renders the leaf-row metrics URL for a single FabricX
// child node (router/batcher/.../query-service). The parent group cards
// already list every role; this is the focused view for one container.
function ChildMetricsCard({ config }: { config: ServiceFabricXChildProperties }) {
	if (!config.metricsUrl) return null
	const role = config.role ?? 'role'
	return (
		<Card>
			<CardHeader>
				<div className="flex items-center gap-2">
					<Activity className="h-4 w-4 text-muted-foreground" />
					<CardTitle>Prometheus Metrics</CardTitle>
				</div>
				<CardDescription>{`This ${role} container exposes /metrics on the host below.`}</CardDescription>
			</CardHeader>
			<CardContent>
				<MetricsRow label={role} url={config.metricsUrl} />
			</CardContent>
		</Card>
	)
}

function OrdererGroupHeader({ config }: { config: ServiceFabricXOrdererGroupProperties }) {
	return (
		<div className="grid gap-6 md:grid-cols-4">
			<Card>
				<CardHeader className="pb-3">
					<div className="flex items-center gap-2">
						<Server className="h-4 w-4 text-muted-foreground" />
						<CardTitle className="text-base">Node Type</CardTitle>
					</div>
				</CardHeader>
				<CardContent>
					<Badge variant="secondary" className="font-mono">
						ORDERER GROUP
					</Badge>
				</CardContent>
			</Card>

			<Card>
				<CardHeader className="pb-3">
					<div className="flex items-center gap-2">
						<Network className="h-4 w-4 text-muted-foreground" />
						<CardTitle className="text-base">Organization</CardTitle>
					</div>
				</CardHeader>
				<CardContent>
					<p className="font-medium">{config.mspId || 'N/A'}</p>
					<p className="text-xs text-muted-foreground">MSP ID</p>
				</CardContent>
			</Card>

			<Card>
				<CardHeader className="pb-3">
					<div className="flex items-center gap-2">
						<Layers className="h-4 w-4 text-muted-foreground" />
						<CardTitle className="text-base">Party</CardTitle>
					</div>
				</CardHeader>
				<CardContent>
					<p className="font-medium">{config.partyId ?? 'N/A'}</p>
					<p className="text-xs text-muted-foreground">Party ID</p>
				</CardContent>
			</Card>

			<Card>
				<CardHeader className="pb-3">
					<div className="flex items-center gap-2">
						<Shield className="h-4 w-4 text-muted-foreground" />
						<CardTitle className="text-base">Version</CardTitle>
					</div>
				</CardHeader>
				<CardContent>
					<Badge variant="outline">{config.version || 'N/A'}</Badge>
				</CardContent>
			</Card>
		</div>
	)
}

function CommitterHeader({ config }: { config: ServiceFabricXCommitterProperties }) {
	return (
		<div className="grid gap-6 md:grid-cols-3">
			<Card>
				<CardHeader className="pb-3">
					<div className="flex items-center gap-2">
						<Server className="h-4 w-4 text-muted-foreground" />
						<CardTitle className="text-base">Node Type</CardTitle>
					</div>
				</CardHeader>
				<CardContent>
					<Badge variant="secondary" className="font-mono">
						COMMITTER
					</Badge>
				</CardContent>
			</Card>

			<Card>
				<CardHeader className="pb-3">
					<div className="flex items-center gap-2">
						<Network className="h-4 w-4 text-muted-foreground" />
						<CardTitle className="text-base">Organization</CardTitle>
					</div>
				</CardHeader>
				<CardContent>
					<p className="font-medium">{config.mspId || 'N/A'}</p>
					<p className="text-xs text-muted-foreground">MSP ID</p>
				</CardContent>
			</Card>

			<Card>
				<CardHeader className="pb-3">
					<div className="flex items-center gap-2">
						<Shield className="h-4 w-4 text-muted-foreground" />
						<CardTitle className="text-base">Version</CardTitle>
					</div>
				</CardHeader>
				<CardContent>
					<Badge variant="outline">{config.version || 'N/A'}</Badge>
				</CardContent>
			</Card>
		</div>
	)
}

function OrdererGroupConfig({ config }: { config: ServiceFabricXOrdererGroupProperties }) {
	return (
		<Card>
			<CardHeader>
				<CardTitle>Orderer Group Configuration</CardTitle>
				<CardDescription>SmartBFT orderer roles and settings</CardDescription>
			</CardHeader>
			<CardContent className="space-y-4">
				<div className="grid grid-cols-2 gap-4">
					<div>
						<p className="text-sm font-medium text-muted-foreground">MSP ID</p>
						<p className="text-sm">{config.mspId || 'N/A'}</p>
					</div>
					<div>
						<p className="text-sm font-medium text-muted-foreground">Organization ID</p>
						<p className="text-sm">{config.organizationId ?? 'N/A'}</p>
					</div>
					<div>
						<p className="text-sm font-medium text-muted-foreground">Party ID</p>
						<p className="text-sm">{config.partyId ?? 'N/A'}</p>
					</div>
					<div>
						<p className="text-sm font-medium text-muted-foreground">Version</p>
						<p className="text-sm">{config.version || 'N/A'}</p>
					</div>
				</div>

				<Separator />

				<div>
					<p className="text-sm font-medium text-muted-foreground mb-2">Role Ports</p>
					<div className="grid grid-cols-2 gap-4">
						<Port label="Router" host={config.externalIp} port={config.routerPort} />
						<Port label="Batcher" host={config.externalIp} port={config.batcherPort} />
						<Port label="Consenter" host={config.externalIp} port={config.consenterPort} />
						<Port label="Assembler" host={config.externalIp} port={config.assemblerPort} />
					</div>
				</div>
			</CardContent>
		</Card>
	)
}

function CommitterConfig({ config }: { config: ServiceFabricXCommitterProperties }) {
	return (
		<Card>
			<CardHeader>
				<CardTitle>Committer Configuration</CardTitle>
				<CardDescription>Committer sidecar, coordinator and validator settings</CardDescription>
			</CardHeader>
			<CardContent className="space-y-4">
				<div className="grid grid-cols-2 gap-4">
					<div>
						<p className="text-sm font-medium text-muted-foreground">MSP ID</p>
						<p className="text-sm">{config.mspId || 'N/A'}</p>
					</div>
					<div>
						<p className="text-sm font-medium text-muted-foreground">Organization ID</p>
						<p className="text-sm">{config.organizationId ?? 'N/A'}</p>
					</div>
					<div>
						<p className="text-sm font-medium text-muted-foreground">Version</p>
						<p className="text-sm">{config.version || 'N/A'}</p>
					</div>
				</div>

				<Separator />

				<div>
					<p className="text-sm font-medium text-muted-foreground mb-2">Service Ports</p>
					<div className="grid grid-cols-2 gap-4">
						<Port label="Sidecar" host={config.externalIp} port={config.sidecarPort} />
						<Port label="Coordinator" host={config.externalIp} port={config.coordinatorPort} />
						<Port label="Validator" host={config.externalIp} port={config.validatorPort} />
						<Port label="Verifier" host={config.externalIp} port={config.verifierPort} />
						<Port label="Query Service" host={config.externalIp} port={config.queryServicePort} />
					</div>
				</div>
			</CardContent>
		</Card>
	)
}

function EndpointCard({ externalIp }: { externalIp?: string }) {
	return (
		<Card>
			<CardHeader>
				<div className="flex items-center gap-2">
					<Globe className="h-4 w-4 text-muted-foreground" />
					<CardTitle>Service Endpoints</CardTitle>
				</div>
				<CardDescription>Network addresses exposed by this node</CardDescription>
			</CardHeader>
			<CardContent className="space-y-3">
				<div>
					<p className="text-sm font-medium text-muted-foreground">External IP</p>
					<p className="font-mono text-sm">{externalIp || 'N/A'}</p>
				</div>
			</CardContent>
		</Card>
	)
}

export function FabricXNodeDetails({ node, logs, events, activeTab, onTabChange }: FabricXNodeDetailsProps) {
	const isOrdererGroup = node.fabricXOrdererGroup !== undefined
	const isCommitter = node.fabricXCommitter !== undefined
	const ordererGroup = node.fabricXOrdererGroup
	const committer = node.fabricXCommitter
	const child = node.fabricXChild

	const signCert = ordererGroup?.signCert
	const tlsCert = ordererGroup?.tlsCert
	const caCert = ordererGroup?.caCert
	const tlsCaCert = ordererGroup?.tlsCaCert
	const externalIp = ordererGroup?.externalIp || committer?.externalIp
	const hasCerts = isOrdererGroup && (signCert || tlsCert || caCert || tlsCaCert)

	return (
		<div className="space-y-6">
			{isOrdererGroup && ordererGroup && <OrdererGroupHeader config={ordererGroup} />}
			{isCommitter && committer && <CommitterHeader config={committer} />}

			<div className="grid gap-6 md:grid-cols-2">
				{isOrdererGroup && ordererGroup && <OrdererGroupConfig config={ordererGroup} />}
				{isCommitter && committer && <CommitterConfig config={committer} />}
				<EndpointCard externalIp={externalIp} />
				{/* Prefer the per-child metrics card on leaf nodes; fall back
				    to the parent group's full per-role list otherwise. */}
				{child && <ChildMetricsCard config={child} />}
				{!child && isOrdererGroup && ordererGroup && <OrdererMetricsCard config={ordererGroup} />}
				{!child && isCommitter && committer && <CommitterMetricsCard config={committer} />}
			</div>

			<Tabs value={activeTab} onValueChange={onTabChange} className="space-y-4">
				<TabsList className={`grid w-full ${hasCerts ? 'grid-cols-3' : 'grid-cols-2'}`}>
					<TabsTrigger value="logs">Logs</TabsTrigger>
					{hasCerts && <TabsTrigger value="crypto">Certificates</TabsTrigger>}
					<TabsTrigger value="events">Events</TabsTrigger>
				</TabsList>

				<TabsContent value="logs" className="space-y-4">
					<Card>
						<CardHeader>
							<CardTitle>Container Logs</CardTitle>
							<CardDescription>Real-time logs from the Fabric-X node</CardDescription>
						</CardHeader>
						<CardContent>
							<LogViewer logs={logs} onScroll={() => {}} />
						</CardContent>
					</Card>
				</TabsContent>

				{hasCerts && (
					<TabsContent value="crypto" className="space-y-4">
						<div className="grid gap-4">
							<Card>
								<CardHeader>
									<div className="flex items-center gap-2">
										<Key className="h-4 w-4 text-muted-foreground" />
										<CardTitle>Identity Certificates</CardTitle>
									</div>
									<CardDescription>MSP signing certificates</CardDescription>
								</CardHeader>
								<CardContent className="space-y-4">
									{signCert && <CertificateViewer label="Signing Certificate" certificate={signCert} />}
									{caCert && <CertificateViewer label="CA Certificate" certificate={caCert} />}
								</CardContent>
							</Card>

							<Card>
								<CardHeader>
									<div className="flex items-center gap-2">
										<Shield className="h-4 w-4 text-muted-foreground" />
										<CardTitle>TLS Certificates</CardTitle>
									</div>
									<CardDescription>Transport layer security certificates</CardDescription>
								</CardHeader>
								<CardContent className="space-y-4">
									{tlsCert && <CertificateViewer label="TLS Certificate" certificate={tlsCert} />}
									{tlsCaCert && <CertificateViewer label="TLS CA Certificate" certificate={tlsCaCert} />}
								</CardContent>
							</Card>
						</div>
					</TabsContent>
				)}

				<TabsContent value="events">
					<Card>
						<CardHeader>
							<CardTitle>Event History</CardTitle>
							<CardDescription>Lifecycle events and operations</CardDescription>
						</CardHeader>
						<CardContent>{events}</CardContent>
					</Card>
				</TabsContent>
			</Tabs>
		</div>
	)
}

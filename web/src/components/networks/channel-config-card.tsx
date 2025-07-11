import { Card } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { ChevronRight, Settings, Shield, Users, Network, Server, FileText, Lock, Hash, Clock, ListChecks } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useState } from 'react'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { PolicyCard } from '@/components/networks/PolicyCard'
import { CertificateViewer } from '@/components/networks/certificate-viewer'

interface ChannelConfigCardProps {
	config: any
}

export function ChannelConfigCard({ config }: ChannelConfigCardProps) {
	const [openSections, setOpenSections] = useState<string[]>([])
	const toggleSection = (section: string) => {
		setOpenSections((prev) => (prev.includes(section) ? prev.filter((s) => s !== section) : [...prev, section]))
	}

	const channelGroup = config?.data?.data?.[0]?.payload?.data?.config?.channel_group
	const consensusType = channelGroup?.groups?.Orderer?.values?.ConsensusType?.value
	const batchSize = channelGroup?.groups?.Orderer?.values?.BatchSize?.value
	const batchTimeout = channelGroup?.groups?.Orderer?.values?.BatchTimeout?.value
	const capabilities = channelGroup?.values?.Capabilities?.value?.capabilities
	const appCapabilities = channelGroup?.groups?.Application?.values?.Capabilities?.value?.capabilities
	const ordererCapabilities = channelGroup?.groups?.Orderer?.values?.Capabilities?.value?.capabilities
	const hashingAlgorithm = channelGroup?.values?.HashingAlgorithm?.value
	const blockDataHashing = channelGroup?.values?.BlockDataHashingStructure?.value
	const acls = channelGroup?.groups?.Application?.values?.ACLs?.value?.acls

	if (!channelGroup) return null

	const renderPolicies = (policies: any) => {
		if (!policies) return null
		return Object.entries(policies).map(([name, policy]: [string, any]) => (
			<div key={name} className="space-y-1">
				<div className="flex items-center justify-between">
					<div className="flex items-center gap-2">
						<Shield className="h-4 w-4 text-muted-foreground" />
						<span className="font-medium">{name}</span>
					</div>
					<div className="flex items-center gap-2">
						<Badge variant="outline">{policy?.policy?.type === 1 ? 'Signature' : policy?.policy?.type === 3 ? 'ImplicitMeta' : 'Unknown'}</Badge>
						<Dialog>
							<DialogTrigger asChild>
								<Button variant="ghost" size="sm" className="h-6 px-2">
									<FileText className="h-4 w-4" />
								</Button>
							</DialogTrigger>
							<DialogContent className="max-w-2xl">
								<DialogHeader>
									<DialogTitle>Policy Details: {name}</DialogTitle>
								</DialogHeader>
								<PolicyCard name={name} policy={policy?.policy} modPolicy={policy?.mod_policy} />
							</DialogContent>
						</Dialog>
					</div>
				</div>
				{policy?.policy?.type === 3 && (
					<div className="text-sm text-muted-foreground pl-6">
						Rule: {policy.policy.value?.rule} of {policy.policy.value?.sub_policy}
					</div>
				)}
				{policy?.policy?.type === 1 && (
					<div className="text-sm text-muted-foreground pl-6">
						Rule: {policy.policy.value?.rule?.n_out_of?.n} out of {policy.policy.value?.rule?.n_out_of?.rules?.length || 0} signatures required
					</div>
				)}
			</div>
		))
	}

	const renderEndpoints = (endpoints: string[]) => {
		if (!endpoints || !Array.isArray(endpoints)) return null
		return (
			<div className="space-y-2 pl-6">
				{endpoints.map((endpoint, index) => (
					<div key={index} className="flex items-center gap-2 text-sm text-muted-foreground">
						<Server className="h-4 w-4" />
						<span>{endpoint}</span>
					</div>
				))}
			</div>
		)
	}

	const renderConsenters = (consenters: any[]) => {
		if (!consenters || !Array.isArray(consenters)) return null
		return (
			<div className="space-y-2 pl-6">
				{consenters.map((consenter, index) => (
					<div key={index} className="flex items-center gap-2 text-sm text-muted-foreground">
						<Network className="h-4 w-4" />
						<span>{`${consenter?.host || 'unknown'}:${consenter?.port || 'unknown'}`}</span>
					</div>
				))}
			</div>
		)
	}

	const renderACLs = (acls: any) => {
		if (!acls) return null
		return Object.entries(acls).map(([name, acl]: [string, any]) => (
			<div key={name} className="flex items-center justify-between py-1">
				<div className="flex items-center gap-2">
					<Lock className="h-4 w-4 text-muted-foreground" />
					<span className="font-medium">{name}</span>
				</div>
				<Badge variant="outline">{acl?.policy_ref || 'Unknown'}</Badge>
			</div>
		))
	}

	const decodeBase64Certificate = (base64Cert: string) => {
		if (!base64Cert) return 'No certificate provided'
		try {
			const decoded = atob(base64Cert)
			return decoded
		} catch (error) {
			console.error('Error decoding certificate:', error)
			return 'Error decoding certificate'
		}
	}

	const formatCertificate = (cert: string) => {
		if (!cert) return ''
		const decoded = decodeBase64Certificate(cert)
		const lines = decoded.split('\n')
		return lines.join('\n')
	}

	const renderOrganizations = (organizations: any) => {
		if (!organizations) return null
		return Object.entries(organizations).map(([mspId, org]: [string, any]) => (
			<Collapsible key={mspId} open={openSections.includes(mspId)} onOpenChange={() => toggleSection(mspId)}>
				<CollapsibleTrigger className="flex items-center gap-2 w-full hover:bg-muted/50 p-2 rounded-md">
					<ChevronRight className={cn('h-4 w-4 transition-transform', openSections.includes(mspId) && 'transform rotate-90')} />
					<Users className="h-4 w-4" />
					<span className="font-medium">{mspId}</span>
				</CollapsibleTrigger>
				<CollapsibleContent className="pl-8 pr-4 pb-2 space-y-4">
					<div className="space-y-4 pt-2">
						<div>
							<h4 className="text-sm font-medium mb-2">Policies</h4>
							<div className="space-y-3">{renderPolicies(org?.policies)}</div>
						</div>
						{org?.values?.Endpoints && (
							<div>
								<h4 className="text-sm font-medium mb-2">Endpoints</h4>
								{renderEndpoints(org.values.Endpoints.value?.addresses)}
							</div>
						)}
						{org?.values?.AnchorPeers && (
							<div>
								<h4 className="text-sm font-medium mb-2">Anchor Peers</h4>
								<div className="space-y-2 pl-6">
									{org.values.AnchorPeers.value?.anchor_peers?.map((peer: any, index: number) => (
										<div key={index} className="flex items-center gap-2 text-sm text-muted-foreground">
											<Network className="h-4 w-4" />
											<span>{`${peer?.host || 'unknown'}:${peer?.port || 'unknown'}`}</span>
										</div>
									))}
								</div>
							</div>
						)}
						{org?.values?.MSP && (
							<div>
								<h4 className="text-sm font-medium mb-2">MSP Configuration</h4>
								<div className="space-y-2">
									<div className="flex items-center gap-2">
										<FileText className="h-4 w-4 text-muted-foreground" />
										<span className="text-sm">Name: {org.values.MSP.value?.config?.name || 'Unknown'}</span>
									</div>
									<Dialog>
										<DialogTrigger asChild>
											<Button variant="outline" size="sm" className="w-full">
												View Certificates
											</Button>
										</DialogTrigger>
										<DialogContent className="max-w-3xl">
											<DialogHeader>
												<DialogTitle>MSP Certificates</DialogTitle>
											</DialogHeader>
											<div className="space-y-4">
												<div>
													<h4 className="text-sm font-medium mb-2">Root Certificates</h4>
													<div className="space-y-2">
														{org.values.MSP.value?.config?.root_certs?.map((cert: string, index: number) => (
															<div key={index} className="space-y-2">
																<div className="flex justify-between items-center">
																	<span className="text-xs font-medium">Certificate {index + 1}</span>
																</div>
																<CertificateViewer title={`Root Certificate ${index + 1}`} certificate={decodeBase64Certificate(cert)} />
															</div>
														))}
													</div>
												</div>
												<div>
													<h4 className="text-sm font-medium mb-2">TLS Root Certificates</h4>
													<div className="space-y-2">
														{org.values.MSP.value?.config?.tls_root_certs?.map((cert: string, index: number) => (
															<div key={index} className="space-y-2">
																<div className="flex justify-between items-center">
																	<span className="text-xs font-medium">TLS Certificate {index + 1}</span>
																</div>
																<CertificateViewer title={`TLS Root Certificate ${index + 1}`} certificate={decodeBase64Certificate(cert)} />
															</div>
														))}
													</div>
												</div>
											</div>
										</DialogContent>
									</Dialog>
								</div>
							</div>
						)}
					</div>
				</CollapsibleContent>
			</Collapsible>
		))
	}

	return (
		<Card className="p-6">
			<div className="flex items-center gap-4 mb-6">
				<div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center">
					<Settings className="h-6 w-6 text-primary" />
				</div>
				<div>
					<h2 className="text-lg font-semibold">Channel Configuration</h2>
					<p className="text-sm text-muted-foreground">Channel policies and organization details</p>
				</div>
			</div>

			<ScrollArea className="h-[600px] pr-4">
				<div className="space-y-6">
					{/* Channel Level Configuration */}
					<div>
						<h3 className="text-sm font-medium mb-3">Channel Level Configuration</h3>
						<div className="space-y-4">
							{capabilities && (
								<div className="space-y-2">
									<div className="flex items-center gap-2">
										<ListChecks className="h-4 w-4 text-muted-foreground" />
										<span className="font-medium">Capabilities</span>
									</div>
									<div className="pl-6">
										{Object.keys(capabilities).map((cap) => (
											<Badge key={cap} variant="secondary" className="mr-2">
												{cap}
											</Badge>
										))}
									</div>
								</div>
							)}
							{hashingAlgorithm && (
								<div className="space-y-2">
									<div className="flex items-center gap-2">
										<Hash className="h-4 w-4 text-muted-foreground" />
										<span className="font-medium">Hashing Algorithm</span>
									</div>
									<div className="pl-6 text-sm text-muted-foreground">{hashingAlgorithm.name}</div>
								</div>
							)}
							{blockDataHashing && (
								<div className="space-y-2">
									<div className="flex items-center gap-2">
										<Hash className="h-4 w-4 text-muted-foreground" />
										<span className="font-medium">Block Data Hashing Structure</span>
									</div>
									<div className="pl-6 text-sm text-muted-foreground">Width: {blockDataHashing.width}</div>
								</div>
							)}
						</div>
					</div>

					{/* Channel Policies */}
					<div>
						<h3 className="text-sm font-medium mb-3">Channel Policies</h3>
						<div className="space-y-3">{renderPolicies(channelGroup?.policies)}</div>
					</div>

					{/* Application Section */}
					<div>
						<h3 className="text-sm font-medium mb-3">Application Configuration</h3>
						<div className="space-y-4">
							{appCapabilities && (
								<div className="space-y-2">
									<div className="flex items-center gap-2">
										<ListChecks className="h-4 w-4 text-muted-foreground" />
										<span className="font-medium">Capabilities</span>
									</div>
									<div className="pl-6">
										{Object.keys(appCapabilities).map((cap) => (
											<Badge key={cap} variant="secondary" className="mr-2">
												{cap}
											</Badge>
										))}
									</div>
								</div>
							)}
							{acls && (
								<div>
									<h4 className="text-sm font-medium mb-2">ACLs</h4>
									<div className="space-y-2">{renderACLs(acls)}</div>
								</div>
							)}
							<div>
								<h4 className="text-sm font-medium mb-2">Application Policies</h4>
								<div className="space-y-3">{renderPolicies(channelGroup?.groups?.Application?.policies)}</div>
							</div>
							<div>
								<h4 className="text-sm font-medium mb-2">Application Organizations</h4>
								<div className="space-y-2">{renderOrganizations(channelGroup?.groups?.Application?.groups)}</div>
							</div>
						</div>
					</div>

					{/* Orderer Section */}
					<div>
						<h3 className="text-sm font-medium mb-3">Orderer Configuration</h3>
						<div className="space-y-4">
							{ordererCapabilities && (
								<div className="space-y-2">
									<div className="flex items-center gap-2">
										<ListChecks className="h-4 w-4 text-muted-foreground" />
										<span className="font-medium">Capabilities</span>
									</div>
									<div className="pl-6">
										{Object.keys(ordererCapabilities).map((cap) => (
											<Badge key={cap} variant="secondary" className="mr-2">
												{cap}
											</Badge>
										))}
									</div>
								</div>
							)}
							{consensusType && (
								<div>
									<h4 className="text-sm font-medium mb-2">Consensus Configuration</h4>
									<div className="space-y-3">
										<div className="space-y-2">
											<div className="flex items-center gap-2">
												<Clock className="h-4 w-4 text-muted-foreground" />
												<span className="font-medium">Type: {consensusType.type}</span>
											</div>
											<div className="flex items-center gap-2">
												<Clock className="h-4 w-4 text-muted-foreground" />
												<span className="font-medium">State: {consensusType.state}</span>
											</div>
											{consensusType.metadata?.options && (
												<div className="space-y-1 pl-6">
													<div className="text-sm font-medium">Options:</div>
													<div className="space-y-1 text-sm text-muted-foreground">
														<div>Election Tick: {consensusType.metadata.options.election_tick}</div>
														<div>Heartbeat Tick: {consensusType.metadata.options.heartbeat_tick}</div>
														<div>Max Inflight Blocks: {consensusType.metadata.options.max_inflight_blocks}</div>
														<div>Tick Interval: {consensusType.metadata.options.tick_interval}</div>
													</div>
												</div>
											)}
											{consensusType.metadata?.consenters && (
												<div>
													<div className="text-sm font-medium mb-2">Consenters:</div>
													{renderConsenters(consensusType.metadata.consenters)}
												</div>
											)}
										</div>
									</div>
								</div>
							)}
							{batchSize && (
								<div>
									<h4 className="text-sm font-medium mb-2">Batch Size</h4>
									<div className="space-y-1 pl-6">
										<div className="text-sm text-muted-foreground">Max Message Count: {batchSize.max_message_count}</div>
										<div className="text-sm text-muted-foreground">Absolute Max Bytes: {batchSize.absolute_max_bytes}</div>
										<div className="text-sm text-muted-foreground">Preferred Max Bytes: {batchSize.preferred_max_bytes}</div>
									</div>
								</div>
							)}
							{batchTimeout && (
								<div>
									<h4 className="text-sm font-medium mb-2">Batch Timeout</h4>
									<div className="pl-6 text-sm text-muted-foreground">{batchTimeout.timeout}</div>
								</div>
							)}
							<div>
								<h4 className="text-sm font-medium mb-2">Orderer Policies</h4>
								<div className="space-y-3">{renderPolicies(channelGroup?.groups?.Orderer?.policies)}</div>
							</div>
							<div>
								<h4 className="text-sm font-medium mb-2">Orderer Organizations</h4>
								<div className="space-y-2">{renderOrganizations(channelGroup?.groups?.Orderer?.groups)}</div>
							</div>
						</div>
					</div>
				</div>
			</ScrollArea>
		</Card>
	)
}

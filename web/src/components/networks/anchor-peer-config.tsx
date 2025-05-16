import { ServiceNetworkNode } from '@/api/client/types.gen'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { AlertCircle, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'

interface AnchorPeerConfigProps {
	organization: {
		id: number
		name: string
		mspId: string
	}
	peers: ServiceNetworkNode[]
	currentAnchorPeers: { host: string; port: number }[]
	onUpdateAnchorPeers: (newAnchorPeers: { host: string; port: number }[]) => Promise<void>
}

function parseEndpoint(endpoint: string): { host: string; port: number } | null {
	try {
		const [host, portStr] = endpoint.split(':')
		const port = parseInt(portStr, 10)
		if (host && !isNaN(port)) {
			return { host, port }
		}
		return null
	} catch {
		return null
	}
}

export function AnchorPeerConfig({ organization, peers, currentAnchorPeers, onUpdateAnchorPeers }: AnchorPeerConfigProps) {
	const [isDialogOpen, setIsDialogOpen] = useState(false)
	const [selectedPeerIds, setSelectedPeerIds] = useState<Set<string>>(new Set())

	const availablePeers = peers.filter((peer) => {
		if (peer.node?.nodeType !== 'FABRIC_PEER' || peer.status !== 'joined') {
			return false
		}
		const endpoint = peer.node?.fabricPeer?.externalEndpoint
		return endpoint ? parseEndpoint(endpoint) !== null : false
	})

	const handleAddAnchorPeer = async () => {
		try {
			const selectedPeers = availablePeers.filter((p) => selectedPeerIds.has(p.node!.id!.toString()))
			const newEndpoints = selectedPeers
				.map((peer) => parseEndpoint(peer.node!.fabricPeer!.externalEndpoint!))
				.filter((endpoint): endpoint is { host: string; port: number } => endpoint !== null)

			const newAnchorPeers = [...currentAnchorPeers, ...newEndpoints]
			await onUpdateAnchorPeers(newAnchorPeers)
			setIsDialogOpen(false)
			setSelectedPeerIds(new Set())
		} catch (error: any) {
			toast.error('Failed to add anchor peers', {
				description: error.message,
			})
		}
	}

	const handleRemoveAnchorPeer = async (host: string, port: number) => {
		const newAnchorPeers = currentAnchorPeers.filter((peer) => !(peer.host === host && peer.port === port))
		await onUpdateAnchorPeers(newAnchorPeers)
	}

	const availableToAdd = availablePeers.filter((peer) => {
		const endpoint = parseEndpoint(peer.node!.fabricPeer!.externalEndpoint!)
		return !currentAnchorPeers.some((ap) => ap.host === endpoint?.host && ap.port === endpoint?.port)
	})

	return (
		<Card className="p-4">
			<div className="flex items-center justify-between mb-4">
				<div>
					<h4 className="font-medium">{organization.name}</h4>
					<p className="text-sm text-muted-foreground">MSP ID: {organization.mspId}</p>
				</div>

				<Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
					<DialogTrigger asChild>
						<Button variant="outline" size="sm" disabled={availableToAdd.length === 0}>
							<Plus className="h-4 w-4 mr-2" />
							Add Anchor Peer
						</Button>
					</DialogTrigger>
					<DialogContent>
						<DialogHeader>
							<DialogTitle>Add Anchor Peer</DialogTitle>
							<DialogDescription>Select a peer to add as an anchor peer for {organization.name}</DialogDescription>
						</DialogHeader>

						{availableToAdd.length === 0 ? (
							<Alert>
								<AlertCircle className="h-4 w-4" />
								<AlertDescription>No peers available to add as anchor peers. All eligible peers have already been added.</AlertDescription>
							</Alert>
						) : (
							<div className="space-y-6">
								<div className="flex flex-col gap-4">
									{availableToAdd.map((peer) => {
										const endpoint = parseEndpoint(peer.node!.fabricPeer!.externalEndpoint!)
										const peerId = peer.node!.id!.toString()
										return (
											<div key={peer.node!.id} className="flex items-center space-x-3">
												<Checkbox
													id={`peer-${peer.node!.id}`}
													checked={selectedPeerIds.has(peerId)}
													onCheckedChange={(checked) => {
														const newSelected = new Set(selectedPeerIds)
														if (checked) {
															newSelected.add(peerId)
														} else {
															newSelected.delete(peerId)
														}
														setSelectedPeerIds(newSelected)
													}}
												/>
												<Label htmlFor={`peer-${peer.node!.id}`} className="flex flex-col cursor-pointer">
													<span className="font-medium">{peer.node!.name}</span>
													<span className="text-sm text-muted-foreground">
														{endpoint?.host}:{endpoint?.port}
													</span>
												</Label>
											</div>
										)
									})}
								</div>

								<div className="flex justify-end gap-2">
									<Button variant="outline" onClick={() => setIsDialogOpen(false)}>
										Cancel
									</Button>
									<Button onClick={handleAddAnchorPeer} disabled={selectedPeerIds.size === 0}>
										Add Peers
									</Button>
								</div>
							</div>
						)}
					</DialogContent>
				</Dialog>
			</div>

			<div className="space-y-2">
				{currentAnchorPeers.length === 0 ? (
					<p className="text-sm text-muted-foreground">No anchor peers configured</p>
				) : (
					currentAnchorPeers.map(({ host, port }, idx) => {
						const peer = availablePeers.find((p) => {
							const endpoint = parseEndpoint(p.node!.fabricPeer!.externalEndpoint!)
							return endpoint?.host === host && endpoint?.port === port
						})

						return (
							<div key={idx} className="flex items-center justify-between p-2 rounded-md border bg-background">
								<div className="flex items-center gap-2">
									<span className="text-sm font-medium">
										{host}:{port}
									</span>
									{peer && peer.status !== 'joined' && (
										<Badge variant="secondary" className="text-xs">
											{peer.status}
										</Badge>
									)}
								</div>
								<Button variant="ghost" size="sm" onClick={() => handleRemoveAnchorPeer(host, port)}>
									<Trash2 className="h-4 w-4 text-muted-foreground hover:text-destructive" />
								</Button>
							</div>
						)
					})
				)}
			</div>
		</Card>
	)
}

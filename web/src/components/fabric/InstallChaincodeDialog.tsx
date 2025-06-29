import { ServiceNetworkNode } from '@/api/client'
import { postScFabricDefinitionsByDefinitionIdInstallMutation } from '@/api/client/@tanstack/react-query.gen'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'

interface InstallChaincodeDialogProps {
	installDialogOpen: boolean
	setInstallDialogOpen: (open: boolean) => void
	availablePeers: ServiceNetworkNode[]
	definitionId: number
}
function InstallChaincodeDialog({ installDialogOpen, setInstallDialogOpen, availablePeers, definitionId }: InstallChaincodeDialogProps) {
	const [selectedPeers, setSelectedPeers] = useState<Set<string>>(new Set())

	const installMutation = useMutation({
		...postScFabricDefinitionsByDefinitionIdInstallMutation(),
		onSuccess: () => {
			toast.success('Chaincode installed successfully')
			setInstallDialogOpen(false)
		},
	})
	return (
		<Dialog open={installDialogOpen} onOpenChange={setInstallDialogOpen}>
			<DialogContent className="max-w-lg">
				<DialogHeader>
					<DialogTitle>Install Chaincode</DialogTitle>
					<DialogDescription>Select the peers where you want to install the chaincode.</DialogDescription>
				</DialogHeader>
				<div className="space-y-4 max-h-[50vh] overflow-y-auto pr-2">
					{availablePeers.map((peer) => (
						<div key={peer.nodeId} className="flex items-center space-x-2">
							<Checkbox
								id={`peer-${peer.nodeId}`}
								checked={selectedPeers.has(peer.nodeId!.toString())}
								onCheckedChange={(checked) => {
									setSelectedPeers((prev) => {
										const next = new Set(prev)
										if (checked) {
											next.add(peer.nodeId!.toString())
										} else {
											next.delete(peer.nodeId!.toString())
										}
										return next
									})
								}}
							/>
							<label htmlFor={`peer-${peer.nodeId}`} className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
								{peer.node?.name} ({peer.node?.fabricPeer?.mspId})
							</label>
						</div>
					))}
				</div>
				{installMutation.error && <div className="text-red-500 text-sm mt-2 break-words max-w-full">{installMutation.error.message}</div>}
				<DialogFooter>
					<Button
						onClick={() => installMutation.mutate({ path: { definitionId }, body: { peer_ids: Array.from(selectedPeers).map(Number) } })}
						disabled={selectedPeers.size === 0 || installMutation.isPending}
					>
						{installMutation.isPending ? 'Installing...' : 'Install'}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	)
}

export { InstallChaincodeDialog }

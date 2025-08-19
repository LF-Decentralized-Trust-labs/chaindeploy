import { ServiceNetworkNode } from '@/api/client'
import { postScFabricDefinitionsByDefinitionIdCommitMutation } from '@/api/client/@tanstack/react-query.gen'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'

interface CommitChaincodeDialogProps {
	commitDialogOpen: boolean
	setCommitDialogOpen: (open: boolean) => void
	availablePeers: ServiceNetworkNode[]
	definitionId: number
	onSuccess?: () => void
	onError?: (error: any) => void
}

function CommitChaincodeDialog({ commitDialogOpen, setCommitDialogOpen, availablePeers, definitionId, onSuccess, onError }: CommitChaincodeDialogProps) {
	const [selectedPeerId, setSelectedPeerId] = useState<string | null>(() => {
		// Select the first peer by default if available
		return availablePeers.length > 0 ? availablePeers[0].nodeId!.toString() : null
	})
	const commitMutation = useMutation({
		...postScFabricDefinitionsByDefinitionIdCommitMutation(),
		onSuccess: () => {
			toast.success('Chaincode committed successfully')
			setCommitDialogOpen(false)
			onSuccess?.()
		},
		onError: (error: any) => {
			onError?.(error)
		},
	})
	return (
		<Dialog open={commitDialogOpen} onOpenChange={setCommitDialogOpen}>
			<DialogContent className="max-w-lg">
				<DialogHeader>
					<DialogTitle>Commit Chaincode</DialogTitle>
					<DialogDescription>Select the peer to commit the chaincode.</DialogDescription>
				</DialogHeader>
				<div className="space-y-4 max-h-[50vh] overflow-y-auto pr-2">
					{availablePeers.map((peer) => (
						<div key={peer.nodeId} className="flex items-center space-x-2">
							<Checkbox
								id={`peer-${peer.nodeId}`}
								checked={selectedPeerId === peer.nodeId!.toString()}
								onCheckedChange={(checked) => {
									setSelectedPeerId(checked ? peer.nodeId!.toString() : null)
								}}
							/>
							<label htmlFor={`peer-${peer.nodeId}`} className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
								{peer.node?.name} ({peer.node?.fabricPeer?.mspId})
							</label>
						</div>
					))}
				</div>
				{commitMutation.error && <div className="text-red-500 text-sm mt-2 break-words max-w-full">{commitMutation.error.message}</div>}
				<DialogFooter>
					<Button
						onClick={() => {
							if (selectedPeerId) {
								commitMutation.mutate({ path: { definitionId }, body: { peer_id: Number(selectedPeerId) } })
							}
						}}
						disabled={selectedPeerId === null || commitMutation.isPending}
					>
						{commitMutation.isPending ? 'Committing...' : 'Commit'}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	)
}

export { CommitChaincodeDialog }

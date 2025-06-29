import { ServiceNetworkNode } from '@/api/client'
import { postScFabricDefinitionsByDefinitionIdApproveMutation } from '@/api/client/@tanstack/react-query.gen'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
interface ApproveChaincodeDialogProps {
	approveDialogOpen: boolean
	setApproveDialogOpen: (open: boolean) => void
	availablePeers: ServiceNetworkNode[]
	definitionId: number
}
function ApproveChaincodeDialog({ approveDialogOpen, setApproveDialogOpen, availablePeers, definitionId }: ApproveChaincodeDialogProps) {
	const [selectedPeerId, setSelectedPeerId] = useState<string | null>(null)
	const approveMutation = useMutation({
		...postScFabricDefinitionsByDefinitionIdApproveMutation(),
		onSuccess: () => {
			toast.success('Chaincode approved successfully')
		},
	})
	return (
		<Dialog open={approveDialogOpen} onOpenChange={setApproveDialogOpen}>
			<DialogContent className="max-w-lg">
				<DialogHeader>
					<DialogTitle>Approve Chaincode</DialogTitle>
					<DialogDescription>Select the peer to approve the chaincode.</DialogDescription>
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
				{approveMutation.error && <div className="text-red-500 text-sm mt-2 break-words max-w-full">{approveMutation.error.message}</div>}
				<DialogFooter>
					<Button
						onClick={() => {
							if (selectedPeerId) {
								toast.promise(approveMutation.mutateAsync({ path: { definitionId }, body: { peer_id: Number(selectedPeerId) } }), {
									loading: 'Approving chaincode...',
									success: 'Chaincode approved successfully',
									error: (error) => {
										if (error?.response?.data?.message) {
											return error.response.data.message
										}
										return 'Failed to approve chaincode'
									},
								})
							}
						}}
						disabled={selectedPeerId === null || approveMutation.isPending}
					>
						{approveMutation.isPending ? 'Approving...' : 'Approve'}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	)
}

export { ApproveChaincodeDialog }

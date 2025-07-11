import { useState } from 'react'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { ScrollArea } from '@/components/ui/scroll-area'
import { PlusCircle, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { useMutation } from '@tanstack/react-query'
import { postNetworksFabricByIdNodesMutation } from '@/api/client/@tanstack/react-query.gen'

interface Node {
	id: number
	name: string
	nodeType: string
	mspId?: string
	externalEndpoint?: string
}

interface AddMultipleNodesDialogProps {
	networkId: number
	availableNodes: Node[]
	onNodesAdded: () => void
}

export function AddMultipleNodesDialog({ networkId, availableNodes, onNodesAdded }: AddMultipleNodesDialogProps) {
	const [selectedNodes, setSelectedNodes] = useState<number[]>([])
	const [isOpen, setIsOpen] = useState(false)

	const addNodes = useMutation({
		...postNetworksFabricByIdNodesMutation(),
		onSuccess: () => {
			toast.success('Nodes added to network successfully')
			setSelectedNodes([])
			setIsOpen(false)
			onNodesAdded()
		},
		onError: (error: any) => {
			toast.error('Failed to add nodes to network', {
				description: error.message,
			})
		},
	})

	// Select all nodes by default when dialog opens
	const handleOpenChange = (open: boolean) => {
		setIsOpen(open)
		if (open) {
			setSelectedNodes(availableNodes.map((node) => node.id))
		}
	}

	const handleAddNodes = async () => {
		if (selectedNodes.length === 0) {
			toast.error('Please select at least one node')
			return
		}

		const selectedNodesData = availableNodes.filter((node) => selectedNodes.includes(node.id))
		for (const node of selectedNodesData) {
			try {
				await addNodes.mutateAsync({
					path: { id: networkId },
					body: {
						nodeId: node.id,
						role: node.nodeType === 'FABRIC_PEER' ? 'peer' : 'orderer',
					},
				})
			} catch (error: any) {
				// Error is handled in the mutation's onError callback
			}
		}
	}

	return (
		<Dialog open={isOpen} onOpenChange={handleOpenChange}>
			<DialogTrigger asChild>
				<Button variant="outline" size="sm">
					<PlusCircle className="h-4 w-4 mr-2" />
					Add Multiple Nodes
				</Button>
			</DialogTrigger>
			<DialogContent className="sm:max-w-[425px]">
				<DialogHeader>
					<DialogTitle>Add Multiple Nodes</DialogTitle>
					<DialogDescription>Select multiple nodes to add to the network at once</DialogDescription>
				</DialogHeader>
				<div className="py-4">
					<ScrollArea className="h-[300px] pr-4">
						<div className="space-y-4">
							{availableNodes.map((node) => (
								<div key={node.id} className="space-y-1">
									<div className="flex items-center space-x-2">
										<Checkbox
											id={`node-${node.id}`}
											checked={selectedNodes.includes(node.id)}
											onCheckedChange={(checked) => {
												if (checked) {
													setSelectedNodes([...selectedNodes, node.id])
												} else {
													setSelectedNodes(selectedNodes.filter((id) => id !== node.id))
												}
											}}
										/>
										<label htmlFor={`node-${node.id}`} className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
											{node.name}
											<span className="ml-2 text-xs text-muted-foreground">({node.nodeType === 'FABRIC_PEER' ? 'Peer' : 'Orderer'})</span>
										</label>
									</div>
									<div className="ml-7 text-xs text-muted-foreground flex flex-col gap-0.5">
										<span>MSP: {node.mspId || <span className="italic">N/A</span>}</span>
										<span>Endpoint: {node.externalEndpoint || <span className="italic">N/A</span>}</span>
									</div>
								</div>
							))}
						</div>
					</ScrollArea>
				</div>
				<DialogFooter>
					<Button onClick={handleAddNodes} disabled={selectedNodes.length === 0 || addNodes.isPending}>
						{addNodes.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
						{addNodes.isPending ? 'Adding Nodes...' : 'Add Selected Nodes'}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	)
}

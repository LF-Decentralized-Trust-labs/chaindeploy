import { Button } from '@/components/ui/button'
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from '@/components/ui/dialog'
import { CheckCircle2, ArrowRight } from 'lucide-react'
import { Link } from 'react-router-dom'

interface NetworkCreatedDialogProps {
	open: boolean
	onOpenChange: (open: boolean) => void
	networkName: string
	networkId?: number
	platform: 'fabric' | 'besu'
}

export function NetworkCreatedDialog({
	open,
	onOpenChange,
	networkName,
	networkId,
	platform,
}: NetworkCreatedDialogProps) {
	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="sm:max-w-md">
				<DialogHeader className="text-center">
					<div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-green-100 dark:bg-green-900/30">
						<CheckCircle2 className="h-8 w-8 text-green-600 dark:text-green-400" />
					</div>
					<DialogTitle className="text-center text-xl">
						Network Created!
					</DialogTitle>
					<DialogDescription className="text-center">
						Your {platform === 'fabric' ? 'Fabric' : 'Besu'} network{' '}
						<span className="font-medium text-foreground">{networkName}</span>{' '}
						is being set up. Nodes will start automatically.
					</DialogDescription>
				</DialogHeader>

				<div className="rounded-lg border bg-muted/50 p-4 my-2">
					<p className="text-sm font-medium mb-2">What's next?</p>
					<ul className="text-sm text-muted-foreground space-y-1.5">
						<li className="flex items-start gap-2">
							<ArrowRight className="h-4 w-4 mt-0.5 shrink-0 text-primary" />
							{platform === 'besu'
								? 'Deploy a Solidity smart contract to your network'
								: 'Install and deploy chaincode to your channel'}
						</li>
						<li className="flex items-start gap-2">
							<ArrowRight className="h-4 w-4 mt-0.5 shrink-0 text-primary" />
							Monitor your nodes from the dashboard
						</li>
						<li className="flex items-start gap-2">
							<ArrowRight className="h-4 w-4 mt-0.5 shrink-0 text-primary" />
							Use the CLI or API for automation
						</li>
					</ul>
				</div>

				<DialogFooter className="flex-col gap-2 sm:flex-col">
					{networkId && (
						<Button asChild className="w-full">
							<Link to={`/networks/${networkId}/${platform}`}>
								View Network
								<ArrowRight className="ml-2 h-4 w-4" />
							</Link>
						</Button>
					)}
					<Button
						variant="outline"
						className="w-full"
						onClick={() => onOpenChange(false)}
					>
						Back to Dashboard
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	)
}

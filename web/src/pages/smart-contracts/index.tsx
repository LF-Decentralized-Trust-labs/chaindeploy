import { BesuIcon } from '@/components/icons/besu-icon'
import { FabricIcon } from '@/components/icons/fabric-icon'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Plus, Upload } from 'lucide-react'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'

export default function SmartContractsPage() {
	const [isDeployDialogOpen, setIsDeployDialogOpen] = useState(false)
	const navigate = useNavigate()

	return (
		<div className="flex-1 p-8">
			<div className="mb-6">
				<div className="flex items-center justify-between">
					<div>
						<h1 className="text-2xl font-semibold">Smart Contracts</h1>
						<p className="text-muted-foreground">Manage and deploy smart contracts across your networks</p>
					</div>
				</div>
			</div>

			<div className="grid gap-6 md:grid-cols-2">
				<Card className="cursor-pointer hover:bg-accent/50 transition-colors" onClick={() => navigate('/smart-contracts/fabric')}>
					<CardHeader>
						<CardTitle className="flex items-center gap-2">
							<FabricIcon className="h-6 w-6" />
							Fabric Chaincodes
						</CardTitle>
						<CardDescription>Manage Hyperledger Fabric chaincodes across your networks</CardDescription>
					</CardHeader>
					<CardContent>
						<div className="space-y-2">
							<p className="text-sm text-muted-foreground">• Install and manage chaincodes</p>
							<p className="text-sm text-muted-foreground">• Handle lifecycle operations</p>
							<p className="text-sm text-muted-foreground">• Monitor chaincode status</p>
						</div>
					</CardContent>
				</Card>
			</div>
		</div>
	)
}

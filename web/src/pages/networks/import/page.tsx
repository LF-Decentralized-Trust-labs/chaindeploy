import { ImportNetworkForm } from '@/components/network-import/ImportNetworkForm'

export default function ImportNetworkPage() {
	return (
		<div className="flex-1 p-8">
			<div className="max-w-4xl mx-auto">
				<div className="mb-8">
					<div className="flex items-center justify-between">
						<div>
							<h1 className="text-2xl font-semibold">Import Network</h1>
							<p className="text-muted-foreground">Import an existing blockchain network using a genesis block</p>
						</div>
					</div>
				</div>

				<ImportNetworkForm />
			</div>
		</div>
	)
}

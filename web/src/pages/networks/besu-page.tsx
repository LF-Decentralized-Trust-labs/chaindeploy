import { getNetworksBesuByIdOptions } from '@/api/client/@tanstack/react-query.gen'
import { BesuNetworkDetails } from '@/components/networks/BesuNetworkDetails'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, Network } from 'lucide-react'
import { Link, useParams } from 'react-router-dom'

export default function NetworkDetailPage() {
	const { id } = useParams()
	const { data: network, isLoading } = useQuery({
		...getNetworksBesuByIdOptions({
			path: { id: Number(id) },
		}),
	})

	if (isLoading) {
		return (
			<div className="flex-1 p-8">
				<div className="max-w-4xl mx-auto">
					<div className="mb-8">
						<Skeleton className="h-8 w-32 mb-2" />
						<Skeleton className="h-5 w-64" />
					</div>
					<div className="space-y-8">
						<Card className="p-6">
							<div className="space-y-4">
								<div className="flex items-center gap-4">
									<Skeleton className="h-12 w-12 rounded-lg" />
									<div>
										<Skeleton className="h-6 w-48 mb-2" />
										<Skeleton className="h-4 w-32" />
									</div>
								</div>
								<Skeleton className="h-24 w-full" />
							</div>
						</Card>
					</div>
				</div>
			</div>
		)
	}

	if (!network) {
		return (
			<div className="flex-1 p-8">
				<div className="max-w-4xl mx-auto text-center">
					<Network className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
					<h1 className="text-2xl font-semibold mb-2">Network not found</h1>
					<p className="text-muted-foreground mb-8">The network you're looking for doesn't exist or you don't have access to it.</p>
					<Button asChild>
						<Link to="/networks">
							<ArrowLeft className="mr-2 h-4 w-4" />
							Back to Networks
						</Link>
					</Button>
				</div>
			</div>
		)
	}

	return <BesuNetworkDetails network={network as any} />
}

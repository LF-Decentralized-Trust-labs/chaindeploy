import { HttpBesuNetworkResponse } from '@/api/client'
import { ValidatorList } from '@/components/networks/validator-list'
import { Activity, ArrowLeft, Code, Copy, Network } from 'lucide-react'
import { Link, useSearchParams } from 'react-router-dom'
import { BesuIcon } from '../icons/besu-icon'
import { Badge } from '../ui/badge'
import { Button } from '../ui/button'
import { Card } from '../ui/card'
import { TimeAgo } from '../ui/time-ago'
import { BesuNetworkTabs, BesuTabValue } from './besu-network-tabs'

// Add these interfaces to properly type the config and genesis config
interface BesuConfig {
	type: string
	networkId: number
	chainId: number
	consensus: string
	initialValidators: number[]
	blockPeriod: number
	epochLength: number
	requestTimeout: number
	nonce: string
	timestamp: string
	gasLimit: string
	difficulty: string
	mixHash: string
	coinbase: string
}

interface BesuGenesisConfig {
	config: {
		chainId: number
		berlinBlock: number
		qbft: {
			blockperiodseconds: number
			epochlength: number
			requesttimeoutseconds: number
			startBlock: number
		}
	}
	nonce: string
	timestamp: string
	gasLimit: string
	difficulty: string
	mixHash: string
	coinbase: string
	alloc: Record<string, { balance: string }>
	extraData: string
	number: string
	gasUsed: string
	parentHash: string
}

interface BesuNetworkDetailsProps {
	network: HttpBesuNetworkResponse & {
		platform: string
		config: BesuConfig
		genesisConfig: BesuGenesisConfig
	}
}

export function BesuNetworkDetails({ network }: BesuNetworkDetailsProps) {
	const [searchParams, setSearchParams] = useSearchParams()
	const currentTab = (searchParams.get('tab') || 'details') as BesuTabValue

	const handleTabChange = (newTab: BesuTabValue) => {
		setSearchParams({ tab: newTab })
	}

	const handleCopyGenesis = () => {
		navigator.clipboard.writeText(JSON.stringify(JSON.parse(genesisConfig as any), null, 2))
	}

	// Update the genesisConfig and initialConfig typing
	const genesisConfig = network.genesisConfig as BesuGenesisConfig
	const initialConfig = network.config as BesuConfig
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

	return (
		<div className="flex-1 p-8">
			<div className="max-w-4xl mx-auto">
				<div className="flex items-center gap-2 text-muted-foreground mb-8">
					<Button variant="ghost" size="sm" asChild>
						<Link to="/networks">
							<ArrowLeft className="mr-2 h-4 w-4" />
							Networks
						</Link>
					</Button>
				</div>

				<div className="mb-4">
					<div className="flex items-center justify-between">
						<div>
							<div className="flex items-center gap-3 mb-1">
								<h1 className="text-2xl font-semibold">{network.name}</h1>
								<Badge className="gap-1">
									<Activity className="h-3 w-3" />
									{network.status}
								</Badge>
							</div>
							<p className="text-muted-foreground">
								Created <TimeAgo date={network.createdAt!} />
							</p>
						</div>

						<div className="flex items-center gap-2">
							<Badge variant="outline" className="text-sm">
								{network.platform}
							</Badge>
						</div>
					</div>
				</div>

				<Card className="p-6">
					<BesuNetworkTabs
						tab={currentTab}
						setTab={handleTabChange}
						networkDetails={
							<div className="space-y-6">
								<div className="flex items-center gap-4 mb-6">
									<div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center">
										<BesuIcon className="h-6 w-6 text-primary" />
									</div>
									<div>
										<h2 className="text-lg font-semibold">Network Information</h2>
										<p className="text-sm text-muted-foreground">Details about your Besu network</p>
									</div>
								</div>

								<div>
									<h3 className="text-sm font-medium mb-2">Network ID</h3>
									<p className="text-sm text-muted-foreground">{genesisConfig?.config?.chainId || 'Not specified'}</p>
								</div>

								<div>
									<h3 className="text-sm font-medium mb-2">Consensus</h3>
									<p className="text-sm text-muted-foreground">{initialConfig?.consensus || 'Not specified'}</p>
								</div>

								{initialConfig?.initialValidators && (
									<div>
										<h3 className="text-sm font-medium mb-2">Validators</h3>
										<ValidatorList validatorIds={initialConfig.initialValidators} />
									</div>
								)}
							</div>
						}
						genesis={
							<div className="space-y-4">
								<div className="flex items-center gap-4 mb-6">
									<div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center">
										<Code className="h-6 w-6 text-primary" />
									</div>
									<div>
										<h2 className="text-lg font-semibold">Genesis Configuration</h2>
										<p className="text-sm text-muted-foreground">Network genesis block configuration</p>
									</div>
								</div>

								<Card className="p-4">
									<div className="flex justify-between items-center mb-2">
										<h3 className="text-sm font-medium">Genesis Configuration</h3>
										<Button variant="ghost" size="sm" onClick={handleCopyGenesis} className="h-8 w-8 p-0">
											<Copy className="h-4 w-4" />
										</Button>
									</div>
									<pre className="text-sm overflow-auto">
										<code>{JSON.stringify(JSON.parse(genesisConfig as any), null, 2)}</code>
									</pre>
								</Card>
							</div>
						}
					/>
				</Card>
			</div>
		</div>
	)
}

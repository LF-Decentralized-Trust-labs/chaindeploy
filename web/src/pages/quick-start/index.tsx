import { postKeys, postNodes, postOrganizations, postNetworksFabric, postNetworksBesu, getKeyProviders, getNodesDefaultsFabricPeer, getNodesDefaultsFabricOrderer } from '@/api/client'
import { BesuIcon } from '@/components/icons/besu-icon'
import { FabricIcon } from '@/components/icons/fabric-icon'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { usePageTitle } from '@/hooks/use-page-title'
import { numberToHex } from '@/utils'
import { ArrowLeft, ArrowRight, CheckCircle2, ChevronLeft, Loader2, Network, Rocket, Server } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { toast } from 'sonner'

type Platform = 'fabric' | 'besu' | null

interface BesuConfig {
	networkName: string
	validatorCount: number
}

interface FabricConfig {
	networkName: string
	orgName: string
	peerCount: number
	ordererCount: number
}

interface ProgressStep {
	label: string
	status: 'pending' | 'running' | 'done' | 'error'
}

interface CreatedResult {
	platform: Platform
	networkName: string
	networkId?: number
}

export default function QuickStartPage() {
	usePageTitle('Quick Start')
	const [platform, setPlatform] = useState<Platform>(null)
	const [step, setStep] = useState(0) // 0: choose platform, 1: configure, 2: creating, 3: done
	const [besuConfig, setBesuConfig] = useState<BesuConfig>({ networkName: '', validatorCount: 4 })
	const [fabricConfig, setFabricConfig] = useState<FabricConfig>({ networkName: '', orgName: '', peerCount: 1, ordererCount: 1 })
	const [progressSteps, setProgressSteps] = useState<ProgressStep[]>([])
	const [progressPercent, setProgressPercent] = useState(0)
	const [result, setResult] = useState<CreatedResult | null>(null)
	const [error, setError] = useState<string | null>(null)

	const updateStep = (index: number, status: ProgressStep['status']) => {
		setProgressSteps((prev) => prev.map((s, i) => (i === index ? { ...s, status } : s)))
	}

	const runBesuSetup = async () => {
		setStep(2)
		setError(null)
		const steps: ProgressStep[] = [
			{ label: 'Finding key provider', status: 'running' },
			{ label: `Generating ${besuConfig.validatorCount} validator keys`, status: 'pending' },
			{ label: 'Creating Besu network', status: 'pending' },
			{ label: 'Starting validator nodes', status: 'pending' },
		]
		setProgressSteps(steps)
		setProgressPercent(5)

		try {
			// Step 1: Get key provider
			const providersResp = await getKeyProviders()
			const providers = providersResp.data
			if (!providers || providers.length === 0) {
				throw new Error('No key providers found. Please configure a key provider in Settings first.')
			}
			const providerId = providers[0].id!
			updateStep(0, 'done')
			setProgressPercent(15)

			// Step 2: Generate validator keys
			updateStep(1, 'running')
			const keyIds: number[] = []
			for (let i = 0; i < besuConfig.validatorCount; i++) {
				const keyResp = await postKeys({
					body: {
						name: `${besuConfig.networkName}-validator-${i + 1}`,
						providerId,
						algorithm: 'EC',
						curve: 'secp256k1',
						description: `Validator key ${i + 1} for ${besuConfig.networkName}`,
					},
				})
				keyIds.push(keyResp.data!.id!)
				setProgressPercent(15 + ((i + 1) / besuConfig.validatorCount) * 35)
			}
			updateStep(1, 'done')
			setProgressPercent(50)

			// Step 3: Create network
			updateStep(2, 'running')
			const networkResp = await postNetworksBesu({
				body: {
					name: besuConfig.networkName,
					description: `Besu network created via Quick Start with ${besuConfig.validatorCount} validators`,
					config: {
						blockPeriod: 5,
						chainId: 1337,
						coinbase: '0x0000000000000000000000000000000000000000',
						consensus: 'qbft',
						difficulty: numberToHex(1),
						epochLength: 30000,
						gasLimit: numberToHex(700000000),
						initialValidatorsKeyIds: keyIds,
						mixHash: '0x63746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365',
						nonce: numberToHex(0),
						requestTimeout: 10,
						timestamp: numberToHex(Math.floor(Date.now() / 1000)),
					},
				},
			})
			updateStep(2, 'done')
			setProgressPercent(80)

			// Step 4: Nodes are auto-started by the network creation
			updateStep(3, 'running')
			await new Promise((r) => setTimeout(r, 1000))
			updateStep(3, 'done')
			setProgressPercent(100)

			setResult({
				platform: 'besu',
				networkName: besuConfig.networkName,
				networkId: networkResp.data?.id,
			})
			setStep(3)
		} catch (err: any) {
			const msg = err?.error?.message || err?.message || 'An unexpected error occurred'
			setError(msg)
			toast.error('Setup failed', { description: msg })
			// Mark current running step as error
			setProgressSteps((prev) => prev.map((s) => (s.status === 'running' ? { ...s, status: 'error' } : s)))
		}
	}

	const runFabricSetup = async () => {
		setStep(2)
		setError(null)
		const steps: ProgressStep[] = [
			{ label: 'Finding key provider', status: 'running' },
			{ label: `Creating organization "${fabricConfig.orgName}"`, status: 'pending' },
			{ label: `Creating ${fabricConfig.ordererCount} orderer node(s)`, status: 'pending' },
			{ label: `Creating ${fabricConfig.peerCount} peer node(s)`, status: 'pending' },
			{ label: 'Creating Fabric network', status: 'pending' },
		]
		setProgressSteps(steps)
		setProgressPercent(5)

		try {
			// Step 1: Get key provider
			const providersResp = await getKeyProviders()
			const providers = providersResp.data
			if (!providers || providers.length === 0) {
				throw new Error('No key providers found. Please configure a key provider in Settings first.')
			}
			const providerId = providers[0].id!
			updateStep(0, 'done')
			setProgressPercent(10)

			// Step 2: Create organization
			updateStep(1, 'running')
			const orgResp = await postOrganizations({
				body: {
					name: fabricConfig.orgName,
					mspId: fabricConfig.orgName,
					description: `Organization created via Quick Start for ${fabricConfig.networkName}`,
					providerId,
				},
			})
			const orgId = orgResp.data!.id!
			updateStep(1, 'done')
			setProgressPercent(25)

			// Step 3: Create orderer nodes
			updateStep(2, 'running')
			const ordererDefaults = await getNodesDefaultsFabricOrderer()
			const ordererIds: number[] = []
			for (let i = 0; i < fabricConfig.ordererCount; i++) {
				const nodeResp = await postNodes({
					body: {
						name: `${fabricConfig.networkName}-orderer-${i + 1}`,
						blockchainPlatform: 'FABRIC',
						fabricOrderer: {
							name: `${fabricConfig.networkName}-orderer-${i + 1}`,
							mode: ordererDefaults.data?.mode || 'service',
							organizationId: orgId,
							listenAddress: ordererDefaults.data?.listenAddress || `0.0.0.0:${7050 + i}`,
							operationsListenAddress: ordererDefaults.data?.operationsListenAddress || `0.0.0.0:${9443 + i}`,
							externalEndpoint: ordererDefaults.data?.externalEndpoint || `127.0.0.1:${7050 + i}`,
							domainNames: [],
							adminAddress: ordererDefaults.data?.adminAddress || `0.0.0.0:${7053 + i}`,
							mspId: fabricConfig.orgName,
							version: '2.5.12',
						},
					},
				})
				ordererIds.push(nodeResp.data!.id!)
				setProgressPercent(25 + ((i + 1) / fabricConfig.ordererCount) * 20)
			}
			updateStep(2, 'done')
			setProgressPercent(45)

			// Step 4: Create peer nodes
			updateStep(3, 'running')
			const peerDefaults = await getNodesDefaultsFabricPeer()
			const peerIds: number[] = []
			for (let i = 0; i < fabricConfig.peerCount; i++) {
				const nodeResp = await postNodes({
					body: {
						name: `${fabricConfig.networkName}-peer-${i + 1}`,
						blockchainPlatform: 'FABRIC',
						fabricPeer: {
							name: `${fabricConfig.networkName}-peer-${i + 1}`,
							mode: peerDefaults.data?.mode || 'service',
							organizationId: orgId,
							listenAddress: peerDefaults.data?.listenAddress || `0.0.0.0:${7051 + i}`,
							operationsListenAddress: peerDefaults.data?.operationsListenAddress || `0.0.0.0:${9444 + i}`,
							externalEndpoint: peerDefaults.data?.externalEndpoint || `127.0.0.1:${7051 + i}`,
							domainNames: [],
							chaincodeAddress: peerDefaults.data?.chaincodeAddress || `0.0.0.0:${7052 + i}`,
							eventsAddress: peerDefaults.data?.eventsAddress || '',
							mspId: fabricConfig.orgName,
							version: '2.5.12',
						},
					},
				})
				peerIds.push(nodeResp.data!.id!)
				setProgressPercent(45 + ((i + 1) / fabricConfig.peerCount) * 20)
			}
			updateStep(3, 'done')
			setProgressPercent(65)

			// Step 5: Create Fabric network (channel)
			updateStep(4, 'running')
			const networkResp = await postNetworksFabric({
				body: {
					name: fabricConfig.networkName,
					description: `Fabric network created via Quick Start`,
					config: {
						consensusType: 'etcdraft',
						peerOrganizations: [{ id: orgId, nodeIds: [] }],
						ordererOrganizations: [{ id: orgId, nodeIds: ordererIds }],
						batchSize: {
							maxMessageCount: 10,
							absoluteMaxBytes: 99 * 1024 * 1024,
							preferredMaxBytes: 512 * 1024,
						},
						batchTimeout: '2s',
						channelCapabilities: ['V2_0'],
						applicationCapabilities: ['V2_0'],
						ordererCapabilities: ['V2_0'],
						etcdRaftOptions: {
							tickInterval: '500ms',
							electionTick: 10,
							heartbeatTick: 1,
							maxInflightBlocks: 5,
							snapshotIntervalSize: 16 * 1024 * 1024,
						},
					},
				},
			})
			updateStep(4, 'done')
			setProgressPercent(100)

			setResult({
				platform: 'fabric',
				networkName: fabricConfig.networkName,
				networkId: networkResp.data?.id,
			})
			setStep(3)
		} catch (err: any) {
			const msg = err?.error?.message || err?.message || 'An unexpected error occurred'
			setError(msg)
			toast.error('Setup failed', { description: msg })
			setProgressSteps((prev) => prev.map((s) => (s.status === 'running' ? { ...s, status: 'error' } : s)))
		}
	}

	// Step 0: Choose platform
	if (step === 0) {
		return (
			<div className="flex-1 p-4 md:p-8">
				<div className="max-w-4xl mx-auto">
					<div className="mb-8">
						<Button variant="ghost" size="sm" asChild className="mb-4">
							<Link to="/dashboard">
								<ChevronLeft className="mr-1 h-4 w-4" />
								Dashboard
							</Link>
						</Button>
						<div className="text-center">
							<div className="flex items-center justify-center gap-2 mb-2">
								<Rocket className="h-6 w-6 text-primary" />
								<h1 className="text-2xl md:text-3xl font-bold">Quick Start</h1>
							</div>
							<p className="text-muted-foreground">
								Get a working blockchain network in under a minute. We'll handle organizations, keys, nodes, and network configuration automatically.
							</p>
						</div>
					</div>

					<div className="grid gap-6 md:grid-cols-3 max-w-5xl mx-auto">
						<Card
							className="relative overflow-hidden hover:shadow-lg transition-shadow cursor-pointer group border-2 hover:border-primary/50"
							onClick={() => {
								setPlatform('besu')
								setStep(1)
							}}
						>
							<div className="absolute inset-0 bg-gradient-to-br from-purple-500/10 to-transparent opacity-0 group-hover:opacity-100 transition-opacity" />
							<CardHeader className="pb-4">
								<div className="flex items-center justify-between mb-2">
									<BesuIcon className="h-12 w-12" />
									<ArrowRight className="h-5 w-5 text-muted-foreground group-hover:translate-x-1 transition-transform" />
								</div>
								<CardTitle className="text-xl">Hyperledger Besu</CardTitle>
								<CardDescription>Ethereum-compatible with QBFT consensus</CardDescription>
							</CardHeader>
							<CardContent>
								<div className="space-y-2 text-sm text-muted-foreground">
									<p className="font-medium text-foreground">What gets created:</p>
									<ul className="space-y-1">
										<li className="flex items-center gap-2">
											<CheckCircle2 className="h-3.5 w-3.5 text-green-500" />
											Validator keys (secp256k1)
										</li>
										<li className="flex items-center gap-2">
											<CheckCircle2 className="h-3.5 w-3.5 text-green-500" />
											QBFT network with genesis block
										</li>
										<li className="flex items-center gap-2">
											<CheckCircle2 className="h-3.5 w-3.5 text-green-500" />
											Validator nodes (auto-started)
										</li>
									</ul>
								</div>
								<div className="mt-4 pt-3 border-t text-xs text-muted-foreground">
									Best for: EVM smart contracts, tokenization, DeFi
								</div>
							</CardContent>
						</Card>

						<Card
							className="relative overflow-hidden hover:shadow-lg transition-shadow cursor-pointer group border-2 hover:border-primary/50"
							onClick={() => {
								setPlatform('fabric')
								setStep(1)
							}}
						>
							<div className="absolute inset-0 bg-gradient-to-br from-blue-500/10 to-transparent opacity-0 group-hover:opacity-100 transition-opacity" />
							<CardHeader className="pb-4">
								<div className="flex items-center justify-between mb-2">
									<FabricIcon className="h-12 w-12" />
									<ArrowRight className="h-5 w-5 text-muted-foreground group-hover:translate-x-1 transition-transform" />
								</div>
								<CardTitle className="text-xl">Hyperledger Fabric</CardTitle>
								<CardDescription>Enterprise permissioned with Raft consensus</CardDescription>
							</CardHeader>
							<CardContent>
								<div className="space-y-2 text-sm text-muted-foreground">
									<p className="font-medium text-foreground">What gets created:</p>
									<ul className="space-y-1">
										<li className="flex items-center gap-2">
											<CheckCircle2 className="h-3.5 w-3.5 text-green-500" />
											Organization with MSP & certificates
										</li>
										<li className="flex items-center gap-2">
											<CheckCircle2 className="h-3.5 w-3.5 text-green-500" />
											Orderer & peer nodes
										</li>
										<li className="flex items-center gap-2">
											<CheckCircle2 className="h-3.5 w-3.5 text-green-500" />
											Channel with default policies
										</li>
									</ul>
								</div>
								<div className="mt-4 pt-3 border-t text-xs text-muted-foreground">
									Best for: Supply chain, identity, permissioned networks
								</div>
							</CardContent>
						</Card>

						<Link to="/networks/fabricx/quickstart" className="block">
							<Card className="relative overflow-hidden hover:shadow-lg transition-shadow cursor-pointer group border-2 hover:border-primary/50 h-full">
								<div className="absolute inset-0 bg-gradient-to-br from-amber-500/10 to-transparent opacity-0 group-hover:opacity-100 transition-opacity" />
								<CardHeader className="pb-4">
									<div className="flex items-center justify-between mb-2">
										<FabricIcon className="h-12 w-12" />
										<ArrowRight className="h-5 w-5 text-muted-foreground group-hover:translate-x-1 transition-transform" />
									</div>
									<div className="flex items-center gap-2">
										<CardTitle className="text-xl">FabricX (MVP)</CardTitle>
										<span className="text-[10px] font-medium bg-amber-500/20 text-amber-700 dark:text-amber-400 px-1.5 py-0.5 rounded uppercase tracking-wide">Preview</span>
									</div>
									<CardDescription>4-party Arma consensus network</CardDescription>
								</CardHeader>
								<CardContent>
									<div className="space-y-2 text-sm text-muted-foreground">
										<p className="font-medium text-foreground">What gets created:</p>
										<ul className="space-y-1">
											<li className="flex items-center gap-2">
												<CheckCircle2 className="h-3.5 w-3.5 text-green-500" />
												4 organizations (Party1–4 MSP)
											</li>
											<li className="flex items-center gap-2">
												<CheckCircle2 className="h-3.5 w-3.5 text-green-500" />
												4 orderer groups + 4 committers
											</li>
											<li className="flex items-center gap-2">
												<CheckCircle2 className="h-3.5 w-3.5 text-green-500" />
												Genesis block + all nodes joined
											</li>
										</ul>
									</div>
									<div className="mt-4 pt-3 border-t text-xs text-muted-foreground">
										Best for: High-throughput BFT, research, Arma testing
									</div>
								</CardContent>
							</Card>
						</Link>
					</div>

					<div className="mt-8 text-center">
						<p className="text-xs text-muted-foreground">
							Want more control? Use the{' '}
							<Link to="/networks/besu/create" className="text-primary hover:underline">
								advanced network creation
							</Link>{' '}
							instead.
						</p>
					</div>
				</div>
			</div>
		)
	}

	// Step 1: Configure
	if (step === 1) {
		const isBesu = platform === 'besu'
		const canProceed = isBesu
			? besuConfig.networkName.trim().length > 0 && besuConfig.validatorCount >= 1
			: fabricConfig.networkName.trim().length > 0 && fabricConfig.orgName.trim().length > 0 && fabricConfig.peerCount >= 1 && fabricConfig.ordererCount >= 1

		return (
			<div className="flex-1 p-4 md:p-8">
				<div className="max-w-2xl mx-auto">
					<Button variant="ghost" size="sm" onClick={() => setStep(0)} className="mb-4">
						<ChevronLeft className="mr-1 h-4 w-4" />
						Back
					</Button>

					<Card>
						<CardHeader>
							<div className="flex items-center gap-3">
								{isBesu ? <BesuIcon className="h-8 w-8" /> : <FabricIcon className="h-8 w-8" />}
								<div>
									<CardTitle>{isBesu ? 'Besu' : 'Fabric'} Quick Start</CardTitle>
									<CardDescription>
										{isBesu
											? 'Configure your Besu network. We\'ll generate keys and create validator nodes automatically.'
											: 'Configure your Fabric network. We\'ll create the organization, nodes, and channel automatically.'}
									</CardDescription>
								</div>
							</div>
						</CardHeader>
						<CardContent className="space-y-6">
							<div className="space-y-2">
								<Label htmlFor="networkName">Network Name</Label>
								<Input
									id="networkName"
									placeholder={isBesu ? 'my-besu-network' : 'my-fabric-channel'}
									value={isBesu ? besuConfig.networkName : fabricConfig.networkName}
									onChange={(e) =>
										isBesu
											? setBesuConfig((c) => ({ ...c, networkName: e.target.value }))
											: setFabricConfig((c) => ({ ...c, networkName: e.target.value }))
									}
									autoFocus
								/>
							</div>

							{isBesu ? (
								<div className="space-y-2">
									<Label htmlFor="validatorCount">Number of Validators</Label>
									<Input
										id="validatorCount"
										type="number"
										min={1}
										max={10}
										value={besuConfig.validatorCount}
										onChange={(e) => setBesuConfig((c) => ({ ...c, validatorCount: Math.max(1, Math.min(10, parseInt(e.target.value) || 1)) }))}
									/>
									<p className="text-xs text-muted-foreground">
										Recommended: 4 validators for QBFT consensus (tolerates 1 faulty node). Range: 1-10.
									</p>
								</div>
							) : (
								<>
									<div className="space-y-2">
										<Label htmlFor="orgName">Organization Name (MSP ID)</Label>
										<Input
											id="orgName"
											placeholder="Org1MSP"
											value={fabricConfig.orgName}
											onChange={(e) => setFabricConfig((c) => ({ ...c, orgName: e.target.value }))}
										/>
										<p className="text-xs text-muted-foreground">
											The MSP (Membership Service Provider) ID for your organization. This identifies your org in the network.
										</p>
									</div>

									<div className="grid grid-cols-2 gap-4">
										<div className="space-y-2">
											<Label htmlFor="ordererCount">Orderer Nodes</Label>
											<Input
												id="ordererCount"
												type="number"
												min={1}
												max={5}
												value={fabricConfig.ordererCount}
												onChange={(e) => setFabricConfig((c) => ({ ...c, ordererCount: Math.max(1, Math.min(5, parseInt(e.target.value) || 1)) }))}
											/>
											<p className="text-xs text-muted-foreground">Orders transactions into blocks</p>
										</div>
										<div className="space-y-2">
											<Label htmlFor="peerCount">Peer Nodes</Label>
											<Input
												id="peerCount"
												type="number"
												min={1}
												max={5}
												value={fabricConfig.peerCount}
												onChange={(e) => setFabricConfig((c) => ({ ...c, peerCount: Math.max(1, Math.min(5, parseInt(e.target.value) || 1)) }))}
											/>
											<p className="text-xs text-muted-foreground">Maintains ledger & runs chaincode</p>
										</div>
									</div>
								</>
							)}

							<div className="rounded-lg border bg-muted/50 p-4">
								<p className="text-sm font-medium mb-2">What will be created:</p>
								<ul className="text-sm text-muted-foreground space-y-1">
									{isBesu ? (
										<>
											<li>- {besuConfig.validatorCount} EC/secp256k1 validator key(s)</li>
											<li>- 1 QBFT Besu network (chain ID: 1337)</li>
											<li>- {besuConfig.validatorCount} validator node(s), auto-started</li>
										</>
									) : (
										<>
											<li>- 1 organization "{fabricConfig.orgName || '...'}" with certificates</li>
											<li>- {fabricConfig.ordererCount} orderer node(s) with Raft consensus</li>
											<li>- {fabricConfig.peerCount} peer node(s)</li>
											<li>- 1 channel "{fabricConfig.networkName || '...'}" with default policies</li>
										</>
									)}
								</ul>
							</div>

							<Button className="w-full" size="lg" disabled={!canProceed} onClick={() => (isBesu ? runBesuSetup() : runFabricSetup())}>
								<Rocket className="mr-2 h-4 w-4" />
								Create Everything
							</Button>
						</CardContent>
					</Card>
				</div>
			</div>
		)
	}

	// Step 2: Creating (progress)
	if (step === 2) {
		return (
			<div className="flex-1 p-4 md:p-8">
				<div className="max-w-2xl mx-auto">
					<Card>
						<CardHeader className="text-center">
							<div className="flex items-center justify-center gap-2 mb-2">
								{platform === 'besu' ? <BesuIcon className="h-8 w-8" /> : <FabricIcon className="h-8 w-8" />}
								<CardTitle>Setting up your network</CardTitle>
							</div>
							<CardDescription>This usually takes less than a minute</CardDescription>
						</CardHeader>
						<CardContent className="space-y-6">
							<Progress value={progressPercent} className="h-2" />

							<div className="space-y-3">
								{progressSteps.map((s, i) => (
									<div key={i} className="flex items-center gap-3">
										{s.status === 'done' && <CheckCircle2 className="h-5 w-5 text-green-500 shrink-0" />}
										{s.status === 'running' && <Loader2 className="h-5 w-5 text-primary animate-spin shrink-0" />}
										{s.status === 'pending' && <div className="h-5 w-5 rounded-full border-2 border-muted-foreground/30 shrink-0" />}
										{s.status === 'error' && <div className="h-5 w-5 rounded-full bg-destructive text-destructive-foreground flex items-center justify-center text-xs shrink-0">!</div>}
										<span className={`text-sm ${s.status === 'done' ? 'text-foreground' : s.status === 'running' ? 'text-foreground font-medium' : s.status === 'error' ? 'text-destructive' : 'text-muted-foreground'}`}>
											{s.label}
										</span>
									</div>
								))}
							</div>

							{error && (
								<div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4 space-y-3">
									<p className="text-sm text-destructive font-medium">Setup failed</p>
									<p className="text-sm text-destructive/80">{error}</p>
									<div className="flex gap-2">
										<Button variant="outline" size="sm" onClick={() => setStep(1)}>
											<ArrowLeft className="mr-1 h-3 w-3" />
											Go Back
										</Button>
										<Button
											variant="outline"
											size="sm"
											onClick={() => (platform === 'besu' ? runBesuSetup() : runFabricSetup())}
										>
											Retry
										</Button>
									</div>
								</div>
							)}
						</CardContent>
					</Card>
				</div>
			</div>
		)
	}

	// Step 3: Success
	if (step === 3 && result) {
		return (
			<div className="flex-1 p-4 md:p-8">
				<div className="max-w-2xl mx-auto">
					<Card>
						<CardHeader className="text-center">
							<div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-green-100 dark:bg-green-900/30">
								<CheckCircle2 className="h-8 w-8 text-green-600 dark:text-green-400" />
							</div>
							<CardTitle className="text-xl">Network Created!</CardTitle>
							<CardDescription>
								Your {result.platform === 'fabric' ? 'Fabric' : 'Besu'} network{' '}
								<span className="font-medium text-foreground">{result.networkName}</span> is ready.
								{result.platform === 'besu' ? ' Validator nodes are starting automatically.' : ' Nodes are provisioned and the channel is configured.'}
							</CardDescription>
						</CardHeader>
						<CardContent className="space-y-4">
							<div className="rounded-lg border bg-muted/50 p-4">
								<p className="text-sm font-medium mb-2">What's next?</p>
								<ul className="text-sm text-muted-foreground space-y-1.5">
									{result.platform === 'besu' ? (
										<>
											<li className="flex items-start gap-2">
												<ArrowRight className="h-4 w-4 mt-0.5 shrink-0 text-primary" />
												Deploy a Solidity smart contract to your network
											</li>
											<li className="flex items-start gap-2">
												<ArrowRight className="h-4 w-4 mt-0.5 shrink-0 text-primary" />
												Connect MetaMask or other wallets using Chain ID 1337
											</li>
										</>
									) : (
										<>
											<li className="flex items-start gap-2">
												<ArrowRight className="h-4 w-4 mt-0.5 shrink-0 text-primary" />
												Install and deploy chaincode to your channel
											</li>
											<li className="flex items-start gap-2">
												<ArrowRight className="h-4 w-4 mt-0.5 shrink-0 text-primary" />
												Add more organizations to your network
											</li>
										</>
									)}
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

							<div className="flex flex-col gap-2">
								{result.networkId && (
									<Button asChild className="w-full">
										<Link to={`/networks/${result.networkId}/${result.platform}`}>
											<Network className="mr-2 h-4 w-4" />
											View Network
										</Link>
									</Button>
								)}
								<Button variant="outline" asChild className="w-full">
									<Link to="/nodes">
										<Server className="mr-2 h-4 w-4" />
										View Nodes
									</Link>
								</Button>
								<Button variant="ghost" asChild className="w-full">
									<Link to="/dashboard">Back to Dashboard</Link>
								</Button>
							</div>
						</CardContent>
					</Card>
				</div>
			</div>
		)
	}

	return null
}

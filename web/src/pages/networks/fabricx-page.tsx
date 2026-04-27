import {
	deleteNetworksFabricxByIdNamespacesByNamespaceIdMutation,
	getNetworksFabricxByIdBlocksOptions,
	getNetworksFabricxByIdChainInfoOptions,
	getNetworksFabricxByIdNamespacePoliciesOptions,
	getNetworksFabricxByIdNamespacesOptions,
	getNetworksFabricxByIdNodesOptions,
	getNetworksFabricxByIdOptions,
	getOrganizationsOptions,
	postNetworksFabricxByIdNamespacesMutation,
} from '@/api/client/@tanstack/react-query.gen'
import { HttpFabricXNamespaceResponse } from '@/api/client'
import { FabricIcon } from '@/components/icons/fabric-icon'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { TimeAgo } from '@/components/ui/time-ago'
import { useMutation, useQuery, keepPreviousData } from '@tanstack/react-query'
import { ArrowLeft, ArrowUpRight, ChevronLeft, ChevronRight, Copy, Network, Plus, RefreshCw, Search, Trash } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'

const ROLE_LABELS: Record<string, string> = {
	router: 'Router',
	batcher: 'Batcher',
	consenter: 'Consenter',
	assembler: 'Assembler',
	committer: 'Committer',
	orderer: 'Orderer',
}

function deriveRole(n: any): string {
	const nodeType: string | undefined = n?.node?.nodeType
	if (nodeType) {
		if (nodeType === 'FABRICX_ORDERER_ROUTER') return 'router'
		if (nodeType === 'FABRICX_ORDERER_BATCHER') return 'batcher'
		if (nodeType === 'FABRICX_ORDERER_CONSENTER') return 'consenter'
		if (nodeType === 'FABRICX_ORDERER_ASSEMBLER') return 'assembler'
		if (nodeType === 'FABRICX_ORDERER_GROUP') return 'orderer'
		if (nodeType.startsWith('FABRICX_COMMITTER')) return 'committer'
	}
	return (n?.role || 'unknown').toLowerCase()
}

function formatHash(hash?: string | null) {
	if (!hash) return '—'
	if (hash.length <= 16) return hash
	return `${hash.slice(0, 10)}…${hash.slice(-6)}`
}

function copyText(text: string | null | undefined) {
	if (!text) return
	navigator.clipboard.writeText(text)
	toast.success('Copied to clipboard')
}

export default function FabricXNetworkDetailPage() {
	const { id: idParam } = useParams()
	const id = Number(idParam)

	const { data: network, isLoading } = useQuery({
		...getNetworksFabricxByIdOptions({ path: { id } }),
		enabled: Number.isFinite(id),
	})

	const {
		data: nodes,
		refetch: refetchNodes,
		isFetching: isFetchingNodes,
	} = useQuery({
		...getNetworksFabricxByIdNodesOptions({ path: { id } }),
		enabled: Number.isFinite(id),
	})

	const {
		data: namespacesResponse,
		isLoading: isLoadingNamespaces,
		refetch: refetchNamespaces,
		isFetching: isFetchingNamespaces,
	} = useQuery({
		...getNetworksFabricxByIdNamespacesOptions({ path: { id } }),
		enabled: Number.isFinite(id),
	})
	// Server returns { namespaces, chainError }; chainError is set when the
	// committer query-service is unreachable — we still get DB rows in that case.
	const namespaces = namespacesResponse?.namespaces
	const namespacesChainError = namespacesResponse?.chainError

	const { data: allOrgs } = useQuery({
		...getOrganizationsOptions({ query: { limit: 100, offset: 0 } } as any),
	})

	const nodeOrgIds = new Set(
		(nodes?.nodes || [])
			.map((n) => n.node?.fabricXOrdererGroup?.organizationId ?? n.node?.fabricXCommitter?.organizationId)
			.filter((v): v is number => typeof v === 'number')
	)
	const selectableOrgs = (allOrgs?.items || []).filter((o) => o.id != null && (nodeOrgIds.size === 0 || nodeOrgIds.has(o.id as number)))

	const [isCreateOpen, setIsCreateOpen] = useState(false)
	const [nsName, setNsName] = useState('')
	const [submitterOrgId, setSubmitterOrgId] = useState<string>('')
	const [waitForFinality, setWaitForFinality] = useState(true)
	const [namespaceToDelete, setNamespaceToDelete] = useState<HttpFabricXNamespaceResponse | null>(null)
	const [tab, setTab] = useState('overview')

	const navigate = useNavigate()
	const [txLookup, setTxLookup] = useState('')
	const [blockLookup, setBlockLookup] = useState('')
	const [lastLookup, setLastLookup] = useState<'tx' | 'block'>('tx')

	const { data: chainInfo, refetch: refetchChainInfo, isLoading: isLoadingChainInfo, isFetching: isFetchingChainInfo } = useQuery({
		...getNetworksFabricxByIdChainInfoOptions({ path: { id } }),
		enabled: Number.isFinite(id),
		refetchInterval: 5000,
	})

	const {
		data: latestBlocks,
		refetch: refetchBlocks,
		isLoading: isLoadingBlocks,
		isFetching: isFetchingLatestBlocks,
	} = useQuery({
		...getNetworksFabricxByIdBlocksOptions({ path: { id }, query: { limit: 10, reverse: true } } as any),
		enabled: Number.isFinite(id) && (chainInfo?.height ?? 0) > 0,
		refetchInterval: 5000,
	})

	const [blocksPage, setBlocksPage] = useState(0)
	const [blocksPageSize, setBlocksPageSize] = useState(10)
	const height = chainInfo?.height ?? 0
	const totalBlocksPages = height > 0 ? Math.max(1, Math.ceil(height / blocksPageSize)) : 1
	const blocksOffset = blocksPage * blocksPageSize

	useEffect(() => {
		// Clamp when the chain shrinks, page size changes, or page drifts past the end.
		if (blocksPage > 0 && (blocksOffset >= height || blocksPage >= totalBlocksPages)) {
			setBlocksPage(Math.max(0, totalBlocksPages - 1))
		}
	}, [height, blocksOffset, blocksPage, totalBlocksPages])

	const {
		data: pagedBlocks,
		refetch: refetchPagedBlocks,
		isLoading: isLoadingPagedBlocks,
		isFetching: isFetchingPagedBlocks,
	} = useQuery({
		...getNetworksFabricxByIdBlocksOptions({
			path: { id },
			query: { limit: blocksPageSize, offset: blocksOffset, reverse: true },
		} as any),
		enabled: Number.isFinite(id) && height > 0 && tab === 'blocks',
		placeholderData: keepPreviousData,
	})

	const { data: namespacePolicies } = useQuery({
		...getNetworksFabricxByIdNamespacePoliciesOptions({ path: { id } }),
		enabled: Number.isFinite(id),
	})

	const createNamespace = useMutation({
		...postNetworksFabricxByIdNamespacesMutation(),
		onSuccess: () => {
			toast.success('Namespace created')
			setIsCreateOpen(false)
			setNsName('')
			setSubmitterOrgId('')
			setWaitForFinality(true)
			refetchNamespaces()
		},
		onError: (err: any) => {
			toast.error('Failed to create namespace', { description: err?.message })
		},
	})

	const deleteNamespace = useMutation({
		...deleteNetworksFabricxByIdNamespacesByNamespaceIdMutation(),
		onSuccess: () => {
			toast.success('Namespace deleted')
			setNamespaceToDelete(null)
			refetchNamespaces()
		},
		onError: (err: any) => {
			toast.error('Failed to delete namespace', { description: err?.message })
		},
	})

	const handleCreateNamespace = () => {
		const orgId = Number(submitterOrgId)
		if (!nsName.trim()) {
			toast.error('Namespace name required')
			return
		}
		if (!Number.isFinite(orgId) || orgId <= 0) {
			toast.error('Submitter organization ID must be a positive integer')
			return
		}
		createNamespace.mutate({
			path: { id },
			body: {
				name: nsName.trim(),
				submitterOrgId: orgId,
				waitForFinality,
			},
		})
	}

	const nodeList = nodes?.nodes || []
	const groupedNodes = useMemo(() => {
		const groups: Record<string, typeof nodeList> = {}
		for (const n of nodeList) {
			const key = deriveRole(n)
			if (!groups[key]) groups[key] = []
			groups[key].push(n)
		}
		return groups
	}, [nodeList])

	const ordererRoles = ['router', 'batcher', 'consenter', 'assembler', 'orderer']
	const ordererCount = ordererRoles.reduce((acc, r) => acc + (groupedNodes[r]?.length || 0), 0)
	const committerCount = groupedNodes['committer']?.length || 0
	const totalTxs = useMemo(() => {
		if (!latestBlocks?.blocks) return null
		return latestBlocks.blocks.reduce((acc, b) => acc + (b.txCount ?? 0), 0)
	}, [latestBlocks])

	const refreshAll = () => {
		refetchChainInfo()
		refetchBlocks()
		refetchPagedBlocks()
		refetchNodes()
		refetchNamespaces()
	}

	if (!Number.isFinite(id)) {
		return (
			<div className="flex-1 p-4 sm:p-8">
				<div className="max-w-4xl mx-auto text-center">
					<h1 className="text-2xl font-semibold mb-2">Invalid network ID</h1>
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

	if (isLoading) {
		return (
			<div className="flex-1 p-4 sm:p-8">
				<div className="max-w-6xl mx-auto">
					<Skeleton className="h-8 w-64 mb-2" />
					<Skeleton className="h-5 w-80 mb-8" />
					<Skeleton className="h-32 w-full mb-8" />
					<Skeleton className="h-96 w-full" />
				</div>
			</div>
		)
	}

	if (!network) {
		return (
			<div className="flex-1 p-4 sm:p-8">
				<div className="max-w-4xl mx-auto text-center">
					<Network className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
					<h1 className="text-2xl font-semibold mb-2">Network not found</h1>
					<p className="text-muted-foreground mb-8">The FabricX network you're looking for doesn't exist or you don't have access to it.</p>
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
		<div className="flex-1 min-w-0 overflow-x-hidden">
			<div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 py-6 sm:py-8">
				<Button variant="ghost" size="sm" asChild className="mb-4 -ml-2">
					<Link to="/networks">
						<ArrowLeft className="mr-2 h-4 w-4" />
						Back to Networks
					</Link>
				</Button>

				<div className="flex flex-col gap-4 pb-6 md:flex-row md:items-start md:justify-between">
					<div className="flex items-start gap-3 min-w-0 sm:gap-4">
						<FabricIcon className="size-9 sm:size-10 shrink-0 mt-0.5 sm:mt-1" />
						<div className="min-w-0 flex-1">
							<div className="flex flex-wrap items-center gap-x-2 gap-y-1">
								<h1 className="text-xl sm:text-2xl font-semibold tracking-tight break-all">{network.name}</h1>
								<Badge variant="secondary">FabricX</Badge>
								{network.status && (
									<Badge variant={network.status === 'running' ? 'default' : 'outline'}>
										{network.status}
									</Badge>
								)}
							</div>
							<div className="mt-1 flex flex-wrap items-center gap-x-4 gap-y-1 text-base sm:text-sm text-muted-foreground">
								<span className="min-w-0 truncate">
									Channel <span className="font-mono text-foreground">{network.channelName || '—'}</span>
								</span>
								{network.createdAt && (
									<span>
										Created <TimeAgo date={network.createdAt} />
									</span>
								)}
							</div>
							{network.description && <p className="mt-2 text-base/6 sm:text-sm text-muted-foreground text-pretty max-w-[68ch]">{network.description}</p>}
						</div>
					</div>
					<div className="flex items-center justify-between gap-2 shrink-0 md:justify-end">
						<div className="flex items-center gap-2 text-xs text-muted-foreground">
							<span className="relative flex size-2">
								<span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
								<span className="relative inline-flex size-2 rounded-full bg-emerald-500" />
							</span>
							Live · 5s
						</div>
						<Button variant="outline" size="sm" onClick={refreshAll}>
							<RefreshCw
								className={`h-4 w-4 sm:mr-2 ${
									isFetchingChainInfo || isFetchingLatestBlocks || isFetchingPagedBlocks || isFetchingNodes || isFetchingNamespaces
										? 'animate-spin'
										: ''
								}`}
							/>
							<span className="hidden sm:inline">Refresh</span>
						</Button>
					</div>
				</div>

				<dl className="grid grid-cols-2 border-y border-border sm:grid-cols-4">
					<div className="px-3 py-4 sm:py-5 sm:pl-0 sm:pr-4 [&:nth-child(n+3)]:border-t sm:[&:nth-child(n+3)]:border-t-0 sm:[&:not(:first-child)]:border-l border-border">
						<dt className="text-sm font-medium text-muted-foreground truncate">Chain height</dt>
						<dd className="mt-1 font-mono text-2xl sm:text-2xl font-semibold tabular-nums">
							{isLoadingChainInfo ? <Skeleton className="h-7 w-16" /> : height}
						</dd>
					</div>
					<div className="px-3 py-4 sm:py-5 sm:px-4 [&:nth-child(n+3)]:border-t sm:[&:nth-child(n+3)]:border-t-0 sm:[&:not(:first-child)]:border-l border-border">
						<dt className="text-sm font-medium text-muted-foreground truncate">Transactions</dt>
						<dd className="mt-1 font-mono text-2xl sm:text-2xl font-semibold tabular-nums">
							{totalTxs == null ? <Skeleton className="h-7 w-16" /> : totalTxs}
							<span className="ml-2 text-xs font-normal text-muted-foreground hidden lg:inline">
								last {latestBlocks?.blocks?.length ?? 0} blocks
							</span>
						</dd>
					</div>
					<div className="px-3 py-4 sm:py-5 sm:px-4 [&:nth-child(n+3)]:border-t sm:[&:nth-child(n+3)]:border-t-0 sm:[&:not(:first-child)]:border-l border-border">
						<dt className="text-sm font-medium text-muted-foreground truncate">Nodes</dt>
						<dd className="mt-1 font-mono text-2xl sm:text-2xl font-semibold tabular-nums">
							{nodeList.length}
							<span className="ml-2 text-xs font-normal text-muted-foreground hidden lg:inline">
								{ordererCount} orderer · {committerCount} committer
							</span>
						</dd>
						<p className="mt-1 text-xs text-muted-foreground lg:hidden">
							{ordererCount} orderer · {committerCount} committer
						</p>
					</div>
					<div className="px-3 py-4 sm:py-5 sm:pl-4 sm:pr-0 [&:nth-child(n+3)]:border-t sm:[&:nth-child(n+3)]:border-t-0 sm:[&:not(:first-child)]:border-l border-border">
						<dt className="text-sm font-medium text-muted-foreground truncate">Namespaces</dt>
						<dd className="mt-1 font-mono text-2xl sm:text-2xl font-semibold tabular-nums">
							{namespaces?.length ?? 0}
							<span className="ml-2 text-xs font-normal text-muted-foreground hidden lg:inline">
								{namespaces?.filter((n) => n.onChain).length ?? 0} on-chain
							</span>
						</dd>
						<p className="mt-1 text-xs text-muted-foreground lg:hidden">
							{namespaces?.filter((n) => n.onChain).length ?? 0} on-chain
						</p>
					</div>
				</dl>

				<div className="sticky top-0 z-10 bg-background/80 backdrop-blur supports-[backdrop-filter]:bg-background/60 border-b border-border">
					<div className="py-3 flex flex-col gap-2 sm:flex-row sm:items-center">
						<div className="relative flex-1 sm:max-w-md">
							<Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
							<Input
								placeholder="Lookup transaction by ID"
								value={txLookup}
								onChange={(e) => {
									setTxLookup(e.target.value)
									setLastLookup('tx')
								}}
								onKeyDown={(e) => {
									if (e.key === 'Enter' && txLookup.trim()) {
										navigate(`/networks/${id}/fabricx/transactions/${txLookup.trim()}`)
									}
								}}
								className="pl-9"
							/>
						</div>
						<div className="flex gap-2">
							<Input
								placeholder="Block #"
								inputMode="numeric"
								pattern="[0-9]*"
								value={blockLookup}
								onChange={(e) => {
									setBlockLookup(e.target.value.replace(/[^0-9]/g, ''))
									setLastLookup('block')
								}}
								onKeyDown={(e) => {
									if (e.key === 'Enter' && blockLookup.trim()) {
										navigate(`/networks/${id}/fabricx/blocks/${blockLookup.trim()}`)
									}
								}}
								className="flex-1 sm:w-36 sm:flex-none"
							/>
							<Button
								variant="secondary"
								size="sm"
								className="shrink-0"
								onClick={() => {
									const tx = txLookup.trim()
									const block = blockLookup.trim()
									if (tx && block) {
										if (lastLookup === 'block') navigate(`/networks/${id}/fabricx/blocks/${block}`)
										else navigate(`/networks/${id}/fabricx/transactions/${tx}`)
									} else if (tx) navigate(`/networks/${id}/fabricx/transactions/${tx}`)
									else if (block) navigate(`/networks/${id}/fabricx/blocks/${block}`)
								}}
								disabled={!txLookup.trim() && !blockLookup.trim()}
							>
								<Search className="h-4 w-4 sm:mr-2" />
								<span className="hidden sm:inline">Find</span>
							</Button>
						</div>
					</div>
				</div>

				<Tabs value={tab} onValueChange={setTab} className="mt-6">
					<div className="-mx-4 sm:-mx-6 lg:-mx-8 overflow-x-auto">
						<div className="px-4 sm:px-6 lg:px-8">
							<TabsList>
								<TabsTrigger value="overview">Overview</TabsTrigger>
								<TabsTrigger value="blocks">Blocks</TabsTrigger>
								<TabsTrigger value="namespaces">Namespaces</TabsTrigger>
								<TabsTrigger value="nodes">Nodes</TabsTrigger>
							</TabsList>
						</div>
					</div>

					<TabsContent value="overview" className="mt-6 space-y-8 sm:space-y-10">
						<section>
							<div className="flex items-end justify-between gap-3 mb-3">
								<div className="min-w-0">
									<h2 className="text-base font-semibold">Latest blocks</h2>
									<p className="text-base/6 sm:text-sm text-muted-foreground">Most recent committed blocks. Click a row to inspect.</p>
								</div>
								<Button variant="ghost" size="sm" className="shrink-0" onClick={() => setTab('blocks')}>
									<span className="hidden sm:inline">View all</span>
									<span className="sm:hidden">All</span>
									<ArrowUpRight className="ml-1 h-4 w-4" />
								</Button>
							</div>
							<BlocksTable
								blocks={latestBlocks?.blocks || []}
								loading={isLoadingBlocks}
								height={height}
								onClick={(num) => navigate(`/networks/${id}/fabricx/blocks/${num}`)}
							/>
						</section>

						<section>
							<div className="flex items-end justify-between gap-3 mb-3">
								<div className="min-w-0">
									<h2 className="text-base font-semibold">Network topology</h2>
									<p className="text-base/6 sm:text-sm text-muted-foreground">Orderer group and committer layout.</p>
								</div>
								<Button variant="ghost" size="sm" className="shrink-0" onClick={() => setTab('nodes')}>
									<span className="hidden sm:inline">All nodes</span>
									<span className="sm:hidden">All</span>
									<ArrowUpRight className="ml-1 h-4 w-4" />
								</Button>
							</div>
							<div className="grid grid-cols-1 md:grid-cols-2 gap-4 sm:gap-6">
								<TopologyGroup
									title="Orderer group"
									subtitle="Router · Batcher · Consenter · Assembler"
									nodes={ordererRoles.flatMap((r) => groupedNodes[r] || [])}
									emptyLabel="No orderer nodes yet"
								/>
								<TopologyGroup
									title="Committer"
									subtitle="Sidecar + validator + query service"
									nodes={groupedNodes['committer'] || []}
									emptyLabel="No committer nodes yet"
								/>
							</div>
						</section>

						{namespacePolicies && namespacePolicies.length > 0 && (
							<section>
								<h2 className="text-base font-semibold mb-3">On-chain namespace policies</h2>
								<div className="flex flex-wrap gap-2">
									{namespacePolicies.map((p) => (
										<Badge key={p.namespace} variant="outline" className="font-mono">
											{p.namespace}
											<span className="ml-1 text-muted-foreground">v{p.version}</span>
										</Badge>
									))}
								</div>
							</section>
						)}
					</TabsContent>

					<TabsContent value="blocks" className="mt-6">
						<div className="flex items-end justify-between gap-3 mb-3">
							<div className="min-w-0">
								<h2 className="text-base font-semibold">Blocks</h2>
								{height > 0 ? (
									<p className="text-base/6 sm:text-sm text-muted-foreground">
										Showing {blocksOffset + 1}–{Math.min(blocksOffset + (pagedBlocks?.blocks?.length ?? 0), height)} of {height} blocks.
									</p>
								) : (
									<p className="text-base/6 sm:text-sm text-muted-foreground">Paginate through committed blocks.</p>
								)}
							</div>
							<div className="flex items-center gap-2 shrink-0">
								<Select
									value={String(blocksPageSize)}
									onValueChange={(v) => {
										setBlocksPageSize(Number(v))
										setBlocksPage(0)
									}}
								>
									<SelectTrigger className="w-[88px] h-9" aria-label="Blocks per page">
										<SelectValue />
									</SelectTrigger>
									<SelectContent>
										<SelectItem value="10">10 / page</SelectItem>
										<SelectItem value="25">25 / page</SelectItem>
										<SelectItem value="50">50 / page</SelectItem>
										<SelectItem value="100">100 / page</SelectItem>
									</SelectContent>
								</Select>
								<Button variant="outline" size="sm" onClick={() => refetchPagedBlocks()}>
									<RefreshCw className={`h-4 w-4 sm:mr-2 ${isFetchingPagedBlocks ? 'animate-spin' : ''}`} />
									<span className="hidden sm:inline">Refresh</span>
								</Button>
							</div>
						</div>
						<BlocksTable
							blocks={pagedBlocks?.blocks || []}
							loading={isLoadingPagedBlocks && !pagedBlocks}
							height={height}
							showHash
							onClick={(num) => navigate(`/networks/${id}/fabricx/blocks/${num}`)}
						/>
						{height > 0 && (
							<div className="mt-4 flex items-center justify-between gap-3">
								<p className="text-xs text-muted-foreground tabular-nums">
									Page {blocksPage + 1} of {totalBlocksPages}
								</p>
								<div className="flex items-center gap-2">
									<Button
										variant="outline"
										size="sm"
										onClick={() => setBlocksPage(0)}
										disabled={blocksPage === 0 || isFetchingPagedBlocks}
									>
										<span className="hidden sm:inline">Newest</span>
										<span className="sm:hidden">First</span>
									</Button>
									<Button
										variant="outline"
										size="sm"
										onClick={() => setBlocksPage((p) => Math.max(0, p - 1))}
										disabled={blocksPage === 0 || isFetchingPagedBlocks}
									>
										<ChevronLeft className="h-4 w-4 sm:mr-1" />
										<span className="hidden sm:inline">Previous</span>
									</Button>
									<Button
										variant="outline"
										size="sm"
										onClick={() => setBlocksPage((p) => Math.min(totalBlocksPages - 1, p + 1))}
										disabled={blocksPage >= totalBlocksPages - 1 || isFetchingPagedBlocks}
									>
										<span className="hidden sm:inline">Next</span>
										<ChevronRight className="h-4 w-4 sm:ml-1" />
									</Button>
									<Button
										variant="outline"
										size="sm"
										onClick={() => setBlocksPage(totalBlocksPages - 1)}
										disabled={blocksPage >= totalBlocksPages - 1 || isFetchingPagedBlocks}
									>
										<span className="hidden sm:inline">Oldest</span>
										<span className="sm:hidden">Last</span>
									</Button>
								</div>
							</div>
						)}
					</TabsContent>

					<TabsContent value="namespaces" className="mt-6">
						<div className="flex items-end justify-between gap-3 mb-3">
							<div className="min-w-0">
								<h2 className="text-base font-semibold">Namespaces</h2>
								<p className="text-base/6 sm:text-sm text-muted-foreground">Create and manage FabricX namespaces on this channel.</p>
							</div>
							<Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
								<DialogTrigger asChild>
									<Button size="sm" className="shrink-0">
										<Plus className="h-4 w-4 sm:mr-2" />
										<span className="hidden sm:inline">Create namespace</span>
										<span className="sm:hidden">Create</span>
									</Button>
								</DialogTrigger>
								<DialogContent>
									<DialogHeader>
										<DialogTitle>Create namespace</DialogTitle>
										<DialogDescription>
											Submits a namespace creation transaction through the submitter organization. Namespaces control chaincode isolation on FabricX channels.
										</DialogDescription>
									</DialogHeader>
									<div className="space-y-4 py-2">
										<div className="space-y-2">
											<Label htmlFor="ns-name">Namespace name</Label>
											<Input id="ns-name" value={nsName} onChange={(e) => setNsName(e.target.value)} placeholder="e.g. supply-chain" />
										</div>
										<div className="space-y-2">
											<Label htmlFor="ns-org">Submitter organization</Label>
											<Select value={submitterOrgId} onValueChange={setSubmitterOrgId}>
												<SelectTrigger id="ns-org">
													<SelectValue placeholder="Select an organization" />
												</SelectTrigger>
												<SelectContent>
													{selectableOrgs.length === 0 ? (
														<div className="px-2 py-1.5 text-sm text-muted-foreground">No organizations available</div>
													) : (
														selectableOrgs.map((o) => (
															<SelectItem key={o.id} value={String(o.id)}>
																{o.mspId || `Org #${o.id}`}
															</SelectItem>
														))
													)}
												</SelectContent>
											</Select>
											<p className="text-xs text-muted-foreground">The organization whose identity signs the namespace-create transaction.</p>
										</div>
										<label className="flex items-center gap-2 text-sm">
											<input type="checkbox" checked={waitForFinality} onChange={(e) => setWaitForFinality(e.target.checked)} />
											Wait for finality
										</label>
									</div>
									<DialogFooter>
										<Button variant="outline" onClick={() => setIsCreateOpen(false)} disabled={createNamespace.isPending}>
											Cancel
										</Button>
										<Button onClick={handleCreateNamespace} disabled={createNamespace.isPending}>
											{createNamespace.isPending ? 'Creating…' : 'Create'}
										</Button>
									</DialogFooter>
								</DialogContent>
							</Dialog>
						</div>

						{namespacesChainError && (
							<div className="mb-3 rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-xs text-destructive">
								Could not reach the committer query-service — showing DB rows only.
								<span className="block text-muted-foreground mt-1 font-mono">{namespacesChainError}</span>
							</div>
						)}
						{isLoadingNamespaces ? (
							<div className="space-y-2">
								<Skeleton className="h-10 w-full" />
								<Skeleton className="h-10 w-full" />
							</div>
						) : !namespaces?.length ? (
							<div className="py-16 text-center border border-dashed border-border rounded-md">
								<p className="text-base sm:text-sm text-muted-foreground">No namespaces yet.</p>
								<Button variant="link" size="sm" onClick={() => setIsCreateOpen(true)}>
									Create the first one
								</Button>
							</div>
						) : (
							<>
								<ul role="list" className="divide-y divide-border border-y border-border md:hidden">
									{namespaces.map((ns) => (
										<li key={ns.id ?? `chain:${ns.name}`} className="py-4 flex items-start gap-3">
											<div className="min-w-0 flex-1">
												<div className="flex flex-wrap items-center gap-x-2 gap-y-1">
													<span className="font-medium break-all">{ns.name}</span>
													{ns.onChain && (
														<Badge variant="outline" className="text-xs">
															on-chain
														</Badge>
													)}
													{ns.source === 'chain' && (
														<Badge variant="secondary" className="text-xs">
															external
														</Badge>
													)}
													<Badge variant={ns.status === 'error' || ns.status === 'FAILED' ? 'destructive' : 'secondary'} className="text-xs">
														{ns.status || '—'}
													</Badge>
												</div>
												<dl className="mt-2 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm">
													<dt className="text-muted-foreground">Submitter</dt>
													<dd className="min-w-0 break-words">
														{ns.submitterMspId || '—'}
														{ns.submitterOrgId ? <span className="text-muted-foreground"> (#{ns.submitterOrgId})</span> : null}
													</dd>
													<dt className="text-muted-foreground">Created</dt>
													<dd className="text-muted-foreground">{ns.createdAt ? <TimeAgo date={ns.createdAt} /> : '—'}</dd>
												</dl>
												{ns.error && <p className="mt-2 text-xs text-destructive whitespace-normal break-words">{ns.error}</p>}
											</div>
											{ns.id != null && (
												<Button
													variant="ghost"
													size="icon"
													className="relative shrink-0"
													onClick={() => setNamespaceToDelete(ns)}
													aria-label={`Delete namespace ${ns.name}`}
												>
													<Trash className="h-4 w-4" />
													<span
														aria-hidden="true"
														className="pointer-fine:hidden absolute top-1/2 left-1/2 size-[max(100%,3rem)] -translate-x-1/2 -translate-y-1/2"
													/>
												</Button>
											)}
										</li>
									))}
								</ul>
								<div className="hidden md:block -mx-4 -my-2 overflow-x-auto whitespace-nowrap sm:-mx-6 lg:-mx-8">
									<div className="inline-block min-w-full px-4 py-2 align-middle sm:px-6 lg:px-8">
										<Table className="w-full">
											<TableHeader>
												<TableRow>
													<TableHead className="whitespace-nowrap">Name</TableHead>
													<TableHead className="whitespace-nowrap">Submitter</TableHead>
													<TableHead className="whitespace-nowrap">Status</TableHead>
													<TableHead className="whitespace-nowrap">Created</TableHead>
													<TableHead className="w-10" />
												</TableRow>
											</TableHeader>
											<TableBody>
												{namespaces.map((ns) => (
													<TableRow key={ns.id ?? `chain:${ns.name}`}>
														<TableCell className="font-medium">
															<div className="flex items-center gap-2">
																{ns.name}
																{ns.onChain && (
																	<Badge variant="outline" className="text-xs">
																		on-chain
																	</Badge>
																)}
																{ns.source === 'chain' && (
																	<Badge variant="secondary" className="text-xs" title="On-chain namespace with no local submission record">
																		external
																	</Badge>
																)}
															</div>
														</TableCell>
														<TableCell className="text-sm">
															{ns.submitterMspId || '—'}
															{ns.submitterOrgId ? <span className="text-muted-foreground"> (#{ns.submitterOrgId})</span> : null}
														</TableCell>
														<TableCell>
															<Badge variant={ns.status === 'error' || ns.status === 'FAILED' ? 'destructive' : 'secondary'}>{ns.status || '—'}</Badge>
															{ns.error && <p className="text-xs text-destructive mt-1 whitespace-normal">{ns.error}</p>}
														</TableCell>
														<TableCell className="text-sm text-muted-foreground">{ns.createdAt ? <TimeAgo date={ns.createdAt} /> : '—'}</TableCell>
														<TableCell>
															{ns.id != null ? (
																<Button
																	variant="ghost"
																	size="icon"
																	className="relative"
																	onClick={() => setNamespaceToDelete(ns)}
																	aria-label={`Delete namespace ${ns.name}`}
																>
																	<Trash className="h-4 w-4" />
																	<span
																		aria-hidden="true"
																		className="pointer-fine:hidden absolute top-1/2 left-1/2 size-[max(100%,3rem)] -translate-x-1/2 -translate-y-1/2"
																	/>
																</Button>
															) : null}
														</TableCell>
													</TableRow>
												))}
											</TableBody>
										</Table>
									</div>
								</div>
							</>
						)}
					</TabsContent>

					<TabsContent value="nodes" className="mt-6">
						<div className="flex items-end justify-between gap-3 mb-3">
							<div className="min-w-0">
								<h2 className="text-base font-semibold">Nodes</h2>
								<p className="text-base/6 sm:text-sm text-muted-foreground">All nodes associated with this network.</p>
							</div>
							<Button variant="outline" size="sm" className="shrink-0" onClick={() => refetchNodes()}>
								<RefreshCw className="h-4 w-4 sm:mr-2" />
								<span className="hidden sm:inline">Refresh</span>
							</Button>
						</div>
						{!nodeList.length ? (
							<div className="py-16 text-center border border-dashed border-border rounded-md">
								<p className="text-base sm:text-sm text-muted-foreground">No nodes associated with this network.</p>
							</div>
						) : (
							<div className="-mx-4 -my-2 overflow-x-auto whitespace-nowrap sm:-mx-6 lg:-mx-8">
								<div className="inline-block min-w-full px-4 py-2 align-middle sm:px-6 lg:px-8">
									<Table className="w-full">
										<TableHeader>
											<TableRow>
												<TableHead className="whitespace-nowrap">Name</TableHead>
												<TableHead className="whitespace-nowrap">Role</TableHead>
												<TableHead className="whitespace-nowrap">Status</TableHead>
												<TableHead className="whitespace-nowrap">Joined</TableHead>
											</TableRow>
										</TableHeader>
										<TableBody>
											{nodeList.map((n) => {
												const role = deriveRole(n)
												return (
													<TableRow key={n.id}>
														<TableCell className="font-medium">{n.node?.name || `Node #${n.nodeId}`}</TableCell>
														<TableCell>
															<Badge variant="outline">{ROLE_LABELS[role] || role || '—'}</Badge>
														</TableCell>
														<TableCell>
															<Badge variant={n.status === 'running' ? 'default' : 'secondary'}>{n.status || '—'}</Badge>
														</TableCell>
														<TableCell className="text-sm text-muted-foreground">{n.createdAt ? <TimeAgo date={n.createdAt} /> : '—'}</TableCell>
													</TableRow>
												)
											})}
										</TableBody>
									</Table>
								</div>
							</div>
						)}
					</TabsContent>
				</Tabs>
			</div>

			<AlertDialog open={!!namespaceToDelete} onOpenChange={(open) => !open && setNamespaceToDelete(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Delete namespace</AlertDialogTitle>
						<AlertDialogDescription>
							Are you sure you want to delete the namespace <span className="font-medium">{namespaceToDelete?.name}</span>? This removes the local record only.
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction
							onClick={() =>
								namespaceToDelete?.id != null &&
								deleteNamespace.mutate({
									path: { id, namespaceId: namespaceToDelete.id },
								})
							}
							disabled={deleteNamespace.isPending}
							className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
						>
							{deleteNamespace.isPending ? 'Deleting…' : 'Delete'}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	)
}

function BlocksTable({
	blocks,
	loading,
	height,
	showHash,
	onClick,
}: {
	blocks: Array<{ number?: number; txCount?: number; dataHash?: string; previousHash?: string }>
	loading: boolean
	height: number
	showHash?: boolean
	onClick: (num: number) => void
}) {
	if (loading) {
		return (
			<div className="space-y-2">
				<Skeleton className="h-10 w-full" />
				<Skeleton className="h-10 w-full" />
				<Skeleton className="h-10 w-full" />
			</div>
		)
	}
	if (!blocks.length) {
		return (
			<div className="py-12 text-center border border-dashed border-border rounded-md">
				<p className="text-base sm:text-sm text-muted-foreground">
					{height === 0 ? 'No blocks yet — waiting for the first transaction.' : 'No blocks to display.'}
				</p>
			</div>
		)
	}
	return (
		<div className="-mx-4 -my-2 overflow-x-auto whitespace-nowrap sm:-mx-6 lg:-mx-8">
			<div className="inline-block min-w-full px-4 py-2 align-middle sm:px-6 lg:px-8">
				<Table className="w-full">
					<TableHeader>
						<TableRow>
							<TableHead className="whitespace-nowrap w-20 sm:w-28">Block</TableHead>
							<TableHead className="whitespace-nowrap w-16 sm:w-24">Txs</TableHead>
							<TableHead className="whitespace-nowrap">Data hash</TableHead>
							{showHash && <TableHead className="whitespace-nowrap hidden md:table-cell">Previous hash</TableHead>}
						</TableRow>
					</TableHeader>
					<TableBody>
						{blocks.map((block) => (
							<TableRow
								key={block.number}
								className="group cursor-pointer hover:bg-muted/50"
								onClick={() => block.number != null && onClick(block.number)}
							>
								<TableCell className="font-mono tabular-nums">#{block.number}</TableCell>
								<TableCell className="tabular-nums">{block.txCount ?? 0}</TableCell>
								<TableCell className="font-mono text-xs text-muted-foreground">
									<span className="inline-flex items-center gap-2">
										{formatHash(block.dataHash)}
										{block.dataHash && (
											<button
												type="button"
												className="sm:opacity-0 group-hover:opacity-100 transition-opacity hover:text-foreground"
												onClick={(e) => {
													e.stopPropagation()
													copyText(block.dataHash)
												}}
												aria-label="Copy data hash"
											>
												<Copy className="h-3 w-3" />
											</button>
										)}
									</span>
								</TableCell>
								{showHash && (
									<TableCell className="font-mono text-xs text-muted-foreground hidden md:table-cell">{formatHash(block.previousHash)}</TableCell>
								)}
							</TableRow>
						))}
					</TableBody>
				</Table>
			</div>
		</div>
	)
}

function TopologyGroup({
	title,
	subtitle,
	nodes,
	emptyLabel,
}: {
	title: string
	subtitle: string
	nodes: Array<any>
	emptyLabel: string
}) {
	return (
		<div className="border border-border rounded-md p-4 sm:p-5">
			<div className="flex items-baseline justify-between gap-2 mb-4">
				<div className="min-w-0">
					<h3 className="text-sm font-semibold truncate">{title}</h3>
					<p className="text-xs text-muted-foreground truncate">{subtitle}</p>
				</div>
				<span className="font-mono text-sm tabular-nums text-muted-foreground shrink-0">{nodes.length}</span>
			</div>
			{!nodes.length ? (
				<p className="text-base sm:text-sm text-muted-foreground">{emptyLabel}</p>
			) : (
				<ul role="list" className="space-y-2.5 sm:space-y-2">
					{nodes.map((n) => {
						const role = deriveRole(n)
						return (
							<li key={n.id} className="flex items-center justify-between gap-3 text-base sm:text-sm">
								<div className="min-w-0 flex items-center gap-2">
									<span
										className={`h-2 w-2 sm:h-1.5 sm:w-1.5 rounded-full shrink-0 ${
											n.status === 'running' ? 'bg-emerald-500' : n.status === 'error' ? 'bg-red-500' : 'bg-muted-foreground/40'
										}`}
									/>
									<span className="font-medium truncate">{n.node?.name || `Node #${n.nodeId}`}</span>
								</div>
								<Badge variant="outline" className="shrink-0 text-xs">
									{ROLE_LABELS[role] || role || '—'}
								</Badge>
							</li>
						)
					})}
				</ul>
			)}
		</div>
	)
}

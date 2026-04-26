import {
	GithubComChainlaunchChainlaunchPkgServicesTypesService as Service,
	ServiceChild,
	TypesGroupStatus,
	TypesNodeGroup as NodeGroup,
	TypesNodeGroupService as NodeGroupService,
} from '@/api/client'
import {
	deleteNodeGroupsByIdPostgresServiceMutation,
	getNodeGroupsByIdChildrenOptions,
	getNodeGroupsByIdOptions,
	getNodeGroupsByIdServicesOptions,
	getServicesOptions,
	postNodeGroupsByIdRestartMutation,
	postNodeGroupsByIdStartMutation,
	postNodeGroupsByIdStopMutation,
	putNodeGroupsByIdPostgresServiceMutation,
} from '@/api/client/@tanstack/react-query.gen'
import { PageHeader, PageShell } from '@/components/layout/page-shell'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { usePageTitle } from '@/hooks/use-page-title'
import { useMutation, useQuery } from '@tanstack/react-query'
import { ChevronLeft, Link2Off, Play, RefreshCw, Square } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'

type StatusVariant = 'default' | 'secondary' | 'destructive' | 'outline'

function statusVariant(status?: TypesGroupStatus): StatusVariant {
	switch (status) {
		case 'RUNNING':
			return 'default'
		case 'ERROR':
		case 'DEGRADED':
			return 'destructive'
		case 'STOPPED':
		case 'CREATED':
			return 'secondary'
		default:
			return 'outline'
	}
}

function formatRelative(iso?: string): string {
	if (!iso) return '—'
	const d = new Date(iso)
	if (Number.isNaN(d.getTime())) return '—'
	return d.toLocaleString()
}

export default function NodeGroupDetailPage() {
	const { id: idParam } = useParams<{ id: string }>()
	const id = Number(idParam)
	const navigate = useNavigate()

	const { data: group, refetch: refetchGroup, isLoading } = useQuery({
		...getNodeGroupsByIdOptions({ path: { id } }),
		enabled: Number.isFinite(id) && id > 0,
		refetchInterval: 5000,
	})

	const { data: attachedServices, refetch: refetchAttached } = useQuery({
		...getNodeGroupsByIdServicesOptions({ path: { id } }),
		enabled: Number.isFinite(id) && id > 0,
		refetchInterval: 10_000,
	})

	const { data: children } = useQuery({
		...getNodeGroupsByIdChildrenOptions({ path: { id } }),
		enabled: Number.isFinite(id) && id > 0,
		refetchInterval: 5000,
	})

	const g = group as NodeGroup | undefined
	usePageTitle(g?.name ? `${g.name} · Node group` : 'Node group')

	const attached = (attachedServices ?? []) as NodeGroupService[]
	const currentPostgres = attached.find((s) => s.serviceType === 'POSTGRES')
	const childList = (children ?? []) as ServiceChild[]

	const startMut = useMutation({
		...postNodeGroupsByIdStartMutation(),
		onSuccess: () => {
			toast.success('Group start requested')
			refetchGroup()
		},
		onError: (e: Error) => toast.error(`Start failed: ${e.message}`),
	})
	const stopMut = useMutation({
		...postNodeGroupsByIdStopMutation(),
		onSuccess: () => {
			toast.success('Group stop requested')
			refetchGroup()
		},
		onError: (e: Error) => toast.error(`Stop failed: ${e.message}`),
	})
	const restartMut = useMutation({
		...postNodeGroupsByIdRestartMutation(),
		onSuccess: () => {
			toast.success('Group restart requested')
			refetchGroup()
		},
		onError: (e: Error) => toast.error(`Restart failed: ${e.message}`),
	})

	if (!Number.isFinite(id) || id <= 0) {
		return (
			<PageShell maxWidth="detail">
				<p className="text-sm text-destructive">Invalid group id.</p>
				<Button variant="link" className="px-0" onClick={() => navigate('/node-groups')}>
					Back to node groups
				</Button>
			</PageShell>
		)
	}

	const isCommitter = g?.groupType === 'FABRICX_COMMITTER'
	const running = g?.status === 'RUNNING' || g?.status === 'DEGRADED'

	const runningChildren = childList.filter((c) => (c.status || '').toUpperCase() === 'RUNNING').length
	const erroredChildren = childList.filter((c) => {
		const s = (c.status || '').toUpperCase()
		return s === 'ERROR' || s === 'DEGRADED'
	}).length

	return (
		<PageShell>
			<div className="mb-6">
				<Link to="/node-groups" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
					<ChevronLeft className="size-4" />
					Node groups
				</Link>
			</div>

			<PageHeader
				title={g?.name ?? `Group #${id}`}
				description={g ? `${g.groupType ?? 'Group'} · ${g.platform ?? '—'}` : 'Loading group details…'}
			>
				{g?.status && <Badge variant={statusVariant(g.status)}>{g.status}</Badge>}
				<Button variant="outline" size="sm" onClick={() => startMut.mutate({ path: { id } })} disabled={startMut.isPending || running}>
					<Play className="size-4 mr-1.5" /> Start
				</Button>
				<Button variant="outline" size="sm" onClick={() => stopMut.mutate({ path: { id } })} disabled={stopMut.isPending || !running}>
					<Square className="size-4 mr-1.5" /> Stop
				</Button>
				<Button variant="outline" size="sm" onClick={() => restartMut.mutate({ path: { id } })} disabled={restartMut.isPending}>
					<RefreshCw className="size-4 mr-1.5" /> Restart
				</Button>
			</PageHeader>

			{isLoading && !g && <p className="text-sm text-muted-foreground">Loading…</p>}

			{g?.errorMessage && (
				<div className="mb-6 rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
					{g.errorMessage}
				</div>
			)}

			<div className="space-y-8">
				{g && (
					<section className="space-y-4">
						<h2 className="text-lg font-semibold">Overview</h2>
						<dl className="grid grid-cols-2 gap-x-6 gap-y-4 border-t pt-4 sm:grid-cols-4">
							<Stat label="Children" value={String(childList.length)} />
							<Stat label="Running" value={String(runningChildren)} />
							<Stat label="Errored" value={String(erroredChildren)} tone={erroredChildren > 0 ? 'destructive' : undefined} />
							<Stat label="Created" value={formatRelative(g.createdAt)} mono={false} />
						</dl>

						<dl className="grid grid-cols-1 gap-x-6 gap-y-4 border-t pt-4 sm:grid-cols-2">
							<DescriptionItem label="Type" value={g.groupType} />
							<DescriptionItem label="Platform" value={g.platform} />
							<DescriptionItem label="MSP ID" value={g.mspId} mono />
							<DescriptionItem label="Version" value={g.version} mono />
							<DescriptionItem label="External IP" value={g.externalIp} mono />
							<DescriptionItem label="Group ID" value={`#${g.id}`} mono />
						</dl>
					</section>
				)}

				{isCommitter && (
					<PostgresSection
						groupId={id}
						current={currentPostgres}
						groupRunning={running}
						onChanged={() => {
							refetchGroup()
							refetchAttached()
						}}
					/>
				)}

				<ChildrenSection children={childList} />
			</div>
		</PageShell>
	)
}

function Stat({ label, value, tone, mono = true }: { label: string; value: string; tone?: 'destructive'; mono?: boolean }) {
	return (
		<div className="min-w-0">
			<dt className="truncate text-sm text-muted-foreground">{label}</dt>
			<dd className={`mt-1 text-2xl font-semibold ${mono ? 'tabular-nums' : ''} ${tone === 'destructive' ? 'text-destructive' : ''}`}>
				{value}
			</dd>
		</div>
	)
}

function DescriptionItem({ label, value, mono = false }: { label: string; value?: string; mono?: boolean }) {
	return (
		<div className="min-w-0">
			<dt className="text-sm font-medium text-foreground">{label}</dt>
			<dd className={`mt-1 text-sm text-muted-foreground ${mono ? 'font-mono' : ''}`}>{value || '—'}</dd>
		</div>
	)
}

// --- children -------------------------------------------------------

function ChildrenSection({ children }: { children: ServiceChild[] }) {
	return (
		<section className="space-y-4">
			<div>
				<h2 className="text-lg font-semibold">Children</h2>
				<p className="mt-1 text-sm text-muted-foreground">
					Nodes owned by this group in canonical role order. Click a row to drill into the node's logs, config, and metrics.
				</p>
			</div>
			{children.length === 0 ? (
				<p className="text-sm text-muted-foreground">No children yet. They appear as soon as the group is prepared.</p>
			) : (
				<div className="-mx-4 overflow-x-auto sm:-mx-6 lg:-mx-8">
					<div className="inline-block min-w-full align-middle px-4 sm:px-6 lg:px-8">
						<Table>
							<TableHeader>
								<TableRow>
									<TableHead className="whitespace-nowrap">Name</TableHead>
									<TableHead className="whitespace-nowrap">Role</TableHead>
									<TableHead className="whitespace-nowrap">Status</TableHead>
									<TableHead className="whitespace-nowrap">Endpoint</TableHead>
									<TableHead className="whitespace-nowrap">Updated</TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{children.map((c) => (
									<TableRow key={c.id}>
										<TableCell className="font-medium">
											<Link to={`/nodes/${c.id}`} className="hover:underline">
												{c.name}
											</Link>
										</TableCell>
										<TableCell className="text-sm text-muted-foreground">{c.nodeType || '—'}</TableCell>
										<TableCell>
											<Badge variant={statusVariant(c.status as TypesGroupStatus | undefined)}>{c.status}</Badge>
											{c.errorMessage && <div className="mt-1 text-xs text-destructive">{c.errorMessage}</div>}
										</TableCell>
										<TableCell className="font-mono text-sm text-muted-foreground">{c.endpoint || '—'}</TableCell>
										<TableCell className="text-sm text-muted-foreground">
											{c.updatedAt ? new Date(c.updatedAt).toLocaleString() : c.createdAt ? new Date(c.createdAt).toLocaleString() : '—'}
										</TableCell>
									</TableRow>
								))}
							</TableBody>
						</Table>
					</div>
				</div>
			)}
		</section>
	)
}

// --- postgres attach/detach -----------------------------------------

function PostgresSection({
	groupId,
	current,
	groupRunning,
	onChanged,
}: {
	groupId: number
	current: NodeGroupService | undefined
	groupRunning: boolean
	onChanged: () => void
}) {
	const [pickerValue, setPickerValue] = useState<string>('')
	const [detachOpen, setDetachOpen] = useState(false)

	const { data: services } = useQuery({
		...getServicesOptions({ query: { serviceType: 'POSTGRES' } }),
		refetchInterval: 15_000,
	})

	const postgresServices = useMemo(() => {
		const list = (services ?? []) as Service[]
		return list.filter((s) => s.serviceType === 'POSTGRES')
	}, [services])

	const attachMut = useMutation({
		...putNodeGroupsByIdPostgresServiceMutation(),
		onSuccess: () => {
			toast.success('Postgres service attached')
			setPickerValue('')
			onChanged()
		},
		onError: (e: Error) => toast.error(`Attach failed: ${e.message}`),
	})

	const detachMut = useMutation({
		...deleteNodeGroupsByIdPostgresServiceMutation(),
		onSuccess: () => {
			toast.success('Postgres service detached')
			setDetachOpen(false)
			onChanged()
		},
		onError: (e: Error) => toast.error(`Detach failed: ${e.message}`),
	})

	const onAttach = () => {
		const svcId = Number(pickerValue)
		if (!Number.isFinite(svcId) || svcId <= 0) return
		attachMut.mutate({ path: { id: groupId }, body: { serviceId: svcId } })
	}

	return (
		<section className="space-y-4">
			<div>
				<h2 className="text-lg font-semibold">Postgres service</h2>
				<p className="mt-1 text-sm text-muted-foreground">
					Committer groups need a postgres backend. Attach an existing service below — create one at{' '}
					<Link to="/services" className="underline">
						/services
					</Link>{' '}
					if you don't have one yet.
				</p>
			</div>

			{current ? (
				<div className="flex items-center justify-between rounded-md border p-4">
					<div className="min-w-0">
						<div className="truncate text-sm font-medium">{current.name}</div>
						<div className="mt-1 text-xs text-muted-foreground">
							{current.serviceType} · {current.status || '—'}
						</div>
					</div>
					<Button
						variant="outline"
						size="sm"
						onClick={() => setDetachOpen(true)}
						disabled={detachMut.isPending || groupRunning}
						title={groupRunning ? 'Stop the group before detaching' : undefined}
					>
						<Link2Off className="size-4 mr-1.5" /> Detach
					</Button>
				</div>
			) : (
				<div className="flex flex-wrap items-center gap-2">
					<Select value={pickerValue} onValueChange={setPickerValue}>
						<SelectTrigger className="w-full sm:w-80">
							<SelectValue placeholder="Pick a postgres service…" />
						</SelectTrigger>
						<SelectContent>
							{postgresServices.length === 0 && (
								<SelectItem value="__none" disabled>
									No postgres services available
								</SelectItem>
							)}
							{postgresServices.map((s) => (
								<SelectItem key={s.id} value={String(s.id)}>
									{s.name} <span className="ml-2 text-xs text-muted-foreground">{s.status}</span>
								</SelectItem>
							))}
						</SelectContent>
					</Select>
					<Button onClick={onAttach} disabled={attachMut.isPending || !pickerValue || groupRunning}>
						{attachMut.isPending ? 'Attaching…' : 'Attach'}
					</Button>
					{groupRunning && <span className="text-xs text-muted-foreground">Stop the group to attach a service.</span>}
				</div>
			)}

			<AlertDialog open={detachOpen} onOpenChange={setDetachOpen}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Detach "{current?.name}"?</AlertDialogTitle>
						<AlertDialogDescription>
							Clears this group's postgres pointer. The underlying container is <strong>not stopped</strong> — other groups may still use it. Stop it explicitly from{' '}
							<Link to="/services" className="underline">
								/services
							</Link>{' '}
							if nothing else references it.
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction onClick={() => detachMut.mutate({ path: { id: groupId } })} disabled={detachMut.isPending}>
							{detachMut.isPending ? 'Detaching…' : 'Detach'}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</section>
	)
}

import {
	GithubComChainlaunchChainlaunchPkgServicesTypesService as Service,
	TypesServiceStatus,
} from '@/api/client'
import {
	deleteServicesByIdMutation,
	getServicesByIdConsumersOptions,
	getServicesByIdLogsOptions,
	getServicesByIdOptions,
	postServicesByIdPostgresDatabasesMutation,
	postServicesByIdStartMutation,
	postServicesByIdStopMutation,
	putServicesByIdMutation,
} from '@/api/client/@tanstack/react-query.gen'
import { PageHeader, PageShell } from '@/components/layout/page-shell'
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
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { usePageTitle } from '@/hooks/use-page-title'
import { useMutation, useQuery } from '@tanstack/react-query'
import { ChevronLeft, Copy, Eye, EyeOff, Pencil, Play, Plus, RefreshCw, Square, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'

// Service config shapes. The generated client types these as `Array<number>`
// because Swagger doesn't model the oneof, but the API returns structured JSON.
type PostgresConfig = {
	db?: string
	user?: string
	password?: string
	hostPort?: number
}

type PostgresDeploymentConfig = {
	host?: string
	port?: number
	containerName?: string
	networkName?: string
}

type LogsResponse = {
	logs?: string
	tail?: number
}

type StatusVariant = 'default' | 'secondary' | 'destructive' | 'outline'

function statusVariant(status?: TypesServiceStatus): StatusVariant {
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

function formatTimestamp(iso?: string): string {
	if (!iso) return '—'
	const d = new Date(iso)
	if (Number.isNaN(d.getTime())) return '—'
	return d.toLocaleString()
}

function readConfig(svc: Service | undefined): PostgresConfig {
	if (!svc?.config) return {}
	return svc.config as unknown as PostgresConfig
}

function readDeployment(svc: Service | undefined): PostgresDeploymentConfig {
	const d = (svc as unknown as { deploymentConfig?: PostgresDeploymentConfig })?.deploymentConfig
	return d ?? {}
}

function generateSecurePassword(length = 24): string {
	const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_'
	const bytes = new Uint8Array(length)
	crypto.getRandomValues(bytes)
	let out = ''
	for (let i = 0; i < length; i++) out += alphabet[bytes[i] % alphabet.length]
	return out
}

// Mutability matches the backend: only STOPPED/CREATED/ERROR services can be
// edited or deleted; RUNNING services can only be stopped.
const MUTABLE: TypesServiceStatus[] = ['STOPPED', 'CREATED', 'ERROR']
function canMutate(status?: TypesServiceStatus): boolean {
	return !!status && MUTABLE.includes(status)
}

export default function ServiceDetailPage() {
	const { id: idParam } = useParams<{ id: string }>()
	const id = Number(idParam)
	const navigate = useNavigate()

	const { data, refetch, isLoading } = useQuery({
		...getServicesByIdOptions({ path: { id } }),
		enabled: Number.isFinite(id) && id > 0,
		refetchInterval: 5000,
	})

	const { data: consumers } = useQuery({
		...getServicesByIdConsumersOptions({ path: { id } }),
		enabled: Number.isFinite(id) && id > 0,
		refetchInterval: 10_000,
	})

	const svc = data as Service | undefined
	usePageTitle(svc?.name ? `${svc.name} · Service` : 'Service')

	const [startOpen, setStartOpen] = useState(false)
	const [editOpen, setEditOpen] = useState(false)
	const [deleteOpen, setDeleteOpen] = useState(false)
	const [addDbOpen, setAddDbOpen] = useState(false)

	const stopMut = useMutation({
		...postServicesByIdStopMutation(),
		onSuccess: () => {
			toast.success('Service stopped')
			refetch()
		},
		onError: (e: Error) => toast.error(`Stop failed: ${e.message}`),
	})

	const deleteMut = useMutation({
		...deleteServicesByIdMutation(),
		onSuccess: () => {
			toast.success('Service deleted')
			navigate('/services')
		},
		onError: (e: Error) => toast.error(`Delete failed: ${e.message}`),
	})

	if (!Number.isFinite(id) || id <= 0) {
		return (
			<PageShell maxWidth="detail">
				<p className="text-sm text-destructive">Invalid service id.</p>
				<Button variant="link" className="px-0" onClick={() => navigate('/services')}>
					Back to services
				</Button>
			</PageShell>
		)
	}

	const config = readConfig(svc)
	const deployment = readDeployment(svc)
	const running = svc?.status === 'RUNNING' || svc?.status === 'STARTING'
	const mutable = canMutate(svc?.status)
	const consumerList = consumers ?? []
	const blockedByConsumers = consumerList.length > 0

	return (
		<PageShell maxWidth="detail">
			<div className="mb-6">
				<Link to="/services" className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
					<ChevronLeft className="size-4" />
					Services
				</Link>
			</div>

			<PageHeader
				title={svc?.name ?? `Service #${id}`}
				description={
					svc
						? `${svc.serviceType ?? 'Service'}${svc.version ? ` · ${svc.version}` : ''}`
						: 'Loading service details…'
				}
			>
				{svc?.status && <Badge variant={statusVariant(svc.status)}>{svc.status}</Badge>}
				<Button
					variant="outline"
					size="sm"
					onClick={() => setStartOpen(true)}
					disabled={running || svc?.status === 'STARTING' || svc?.status === 'STOPPING'}
				>
					<Play className="size-4 mr-1.5" /> Start
				</Button>
				<Button
					variant="outline"
					size="sm"
					onClick={() => svc?.id && stopMut.mutate({ path: { id: svc.id } })}
					disabled={!running || stopMut.isPending}
				>
					<Square className="size-4 mr-1.5" /> Stop
				</Button>
				<Button variant="outline" size="sm" onClick={() => setEditOpen(true)} disabled={!mutable}>
					<Pencil className="size-4 mr-1.5" /> Edit
				</Button>
				<Button variant="outline" size="sm" onClick={() => setDeleteOpen(true)} disabled={!mutable} className="text-destructive hover:text-destructive">
					<Trash2 className="size-4 mr-1.5" /> Delete
				</Button>
			</PageHeader>

			{isLoading && !svc && <p className="text-sm text-muted-foreground">Loading…</p>}

			{svc?.errorMessage && (
				<div className="mb-6 rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
					{svc.errorMessage}
				</div>
			)}

			{svc && (
				<div className="space-y-8">
					<OverviewSection svc={svc} consumerCount={consumerList.length} />

					<ConnectionSection
						config={config}
						deployment={deployment}
						serviceStatus={svc.status}
					/>

					{svc.serviceType === 'POSTGRES' && (
						<DatabasesSection
							running={running}
							onAdd={() => setAddDbOpen(true)}
						/>
					)}

					{svc.serviceType === 'POSTGRES' && <LogsSection serviceId={id} status={svc.status} />}

					<ConsumersSection consumers={consumerList} />
				</div>
			)}

			<StartServiceDialog
				service={svc ?? null}
				open={startOpen}
				onOpenChange={setStartOpen}
				onStarted={() => {
					setStartOpen(false)
					refetch()
				}}
			/>

			<EditServiceDialog
				service={svc ?? null}
				open={editOpen}
				onOpenChange={setEditOpen}
				onSaved={() => {
					setEditOpen(false)
					refetch()
				}}
			/>

			<DeleteServiceDialog
				service={svc ?? null}
				open={deleteOpen}
				onOpenChange={setDeleteOpen}
				isPending={deleteMut.isPending}
				blocked={blockedByConsumers}
				consumers={consumerList}
				onConfirm={() => svc?.id && deleteMut.mutate({ path: { id: svc.id } })}
			/>

			<AddDatabaseDialog
				serviceId={svc?.id}
				open={addDbOpen}
				onOpenChange={setAddDbOpen}
				onAdded={() => setAddDbOpen(false)}
			/>
		</PageShell>
	)
}

// --- sections --------------------------------------------------------

function OverviewSection({ svc, consumerCount }: { svc: Service; consumerCount: number }) {
	return (
		<section className="space-y-4">
			<h2 className="text-lg font-semibold">Overview</h2>
			<dl className="grid grid-cols-1 gap-x-6 gap-y-4 border-t pt-4 sm:grid-cols-2">
				<DescriptionItem label="Name" value={svc.name} />
				<DescriptionItem label="Type" value={svc.serviceType} />
				<DescriptionItem label="Version" value={svc.version} mono />
				<DescriptionItem label="Service ID" value={`#${svc.id}`} mono />
				<DescriptionItem label="Consumers" value={String(consumerCount)} mono />
				<DescriptionItem label="Status" value={svc.status} />
				<DescriptionItem label="Created" value={formatTimestamp(svc.createdAt)} />
				<DescriptionItem label="Updated" value={formatTimestamp(svc.updatedAt)} />
			</dl>
		</section>
	)
}

function ConnectionSection({
	config,
	deployment,
	serviceStatus,
}: {
	config: PostgresConfig
	deployment: PostgresDeploymentConfig
	serviceStatus?: TypesServiceStatus
}) {
	const [showPassword, setShowPassword] = useState(false)
	const hasDeployment = !!deployment.containerName || !!deployment.host
	const dsn = useMemo(() => {
		if (!config.user || !config.db) return ''
		const host = deployment.host || 'localhost'
		const port = config.hostPort ?? deployment.port ?? 5432
		return `postgres://${config.user}:${showPassword && config.password ? config.password : '••••••'}@${host}:${port}/${config.db}`
	}, [config, deployment, showPassword])

	return (
		<section className="space-y-4">
			<div>
				<h2 className="text-lg font-semibold">Connection</h2>
				<p className="mt-1 text-sm text-muted-foreground">
					How siblings and host tools dial this service. Siblings on the same docker network reach it at{' '}
					<code className="rounded bg-muted px-1 font-mono text-xs">container:internal-port</code>; tooling on the host uses the published port.
				</p>
			</div>

			<dl className="grid grid-cols-1 gap-x-6 gap-y-4 border-t pt-4 sm:grid-cols-2">
				<DescriptionItem label="Admin database" value={config.db} mono />
				<DescriptionItem label="Admin user" value={config.user} mono />
				<DescriptionItem label="Host port (published)" value={config.hostPort ? String(config.hostPort) : undefined} mono />
				<DescriptionItem
					label="Internal port (container)"
					value={deployment.port ? String(deployment.port) : serviceStatus === 'RUNNING' ? '5432' : undefined}
					mono
				/>
				<DescriptionItem label="Container name" value={deployment.containerName} mono />
				<DescriptionItem label="Docker network" value={deployment.networkName} mono />
				<DescriptionItem label="Sibling hostname" value={deployment.host} mono />
			</dl>

			{hasDeployment && config.user && config.db && (
				<div className="space-y-2 border-t pt-4">
					<div className="flex items-center justify-between">
						<Label className="text-sm font-medium">Connection string</Label>
						<div className="flex gap-2">
							<Button
								type="button"
								variant="ghost"
								size="sm"
								onClick={() => setShowPassword((v) => !v)}
								title={showPassword ? 'Hide password' : 'Show password'}
							>
								{showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
							</Button>
							<Button
								type="button"
								variant="ghost"
								size="sm"
								onClick={() => {
									if (!config.user || !config.db) return
									const host = deployment.host || 'localhost'
									const port = config.hostPort ?? deployment.port ?? 5432
									const real = `postgres://${config.user}:${config.password ?? ''}@${host}:${port}/${config.db}`
									navigator.clipboard.writeText(real).then(() => toast.success('Connection string copied'))
								}}
								title="Copy with real password"
							>
								<Copy className="size-4" />
							</Button>
						</div>
					</div>
					<div className="overflow-x-auto rounded-md border bg-muted/50 px-3 py-2">
						<code className="font-mono text-xs whitespace-nowrap">{dsn}</code>
					</div>
					<p className="text-xs text-muted-foreground">Copy always includes the real password regardless of visibility toggle.</p>
				</div>
			)}
		</section>
	)
}

function DatabasesSection({ running, onAdd }: { running: boolean; onAdd: () => void }) {
	return (
		<section className="space-y-4">
			<div className="flex items-start justify-between gap-4">
				<div>
					<h2 className="text-lg font-semibold">Databases</h2>
					<p className="mt-1 text-sm text-muted-foreground">
						Provision additional roles + databases inside this container. Idempotent — re-running safely updates the password.
					</p>
				</div>
				<Button size="sm" onClick={onAdd} disabled={!running}>
					<Plus className="size-4 mr-1.5" />
					Add database
				</Button>
			</div>
			{!running && (
				<div className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-sm">
					Service must be RUNNING to provision databases.
				</div>
			)}
		</section>
	)
}

function LogsSection({ serviceId, status }: { serviceId: number; status?: TypesServiceStatus }) {
	const [tail, setTail] = useState(200)
	const { data, refetch, isFetching } = useQuery({
		...getServicesByIdLogsOptions({ path: { id: serviceId }, query: { tail } }),
		enabled: Number.isFinite(serviceId) && serviceId > 0,
		refetchInterval: status === 'RUNNING' ? 5000 : false,
	})
	const logs = (data as LogsResponse | undefined)?.logs ?? ''
	const empty = !logs.trim()

	return (
		<section className="space-y-4">
			<div className="flex items-start justify-between gap-4">
				<div>
					<h2 className="text-lg font-semibold">Logs</h2>
					<p className="mt-1 text-sm text-muted-foreground">
						Tail of stdout/stderr from the container. Auto-refreshes every 5s while RUNNING.
					</p>
				</div>
				<div className="flex items-center gap-2">
					<select
						value={tail}
						onChange={(e) => setTail(Number(e.target.value))}
						className="h-8 rounded-md border bg-background px-2 text-sm"
						aria-label="Log tail size"
					>
						<option value={50}>50 lines</option>
						<option value={200}>200 lines</option>
						<option value={500}>500 lines</option>
						<option value={2000}>2000 lines</option>
					</select>
					<Button size="sm" variant="outline" onClick={() => refetch()} disabled={isFetching}>
						<RefreshCw className={`size-4 mr-1.5 ${isFetching ? 'animate-spin' : ''}`} /> Refresh
					</Button>
					<Button
						size="sm"
						variant="outline"
						disabled={empty}
						onClick={() => navigator.clipboard.writeText(logs).then(() => toast.success('Logs copied'))}
					>
						<Copy className="size-4 mr-1.5" /> Copy
					</Button>
				</div>
			</div>
			{empty ? (
				<div className="rounded-md border bg-muted/40 p-4 text-sm text-muted-foreground">
					{status === 'CREATED' || status === 'STOPPED'
						? 'No logs yet. Start the service to see output here.'
						: 'No logs available.'}
				</div>
			) : (
				<pre className="max-h-96 overflow-auto rounded-md border bg-muted/30 p-3 font-mono text-xs leading-relaxed whitespace-pre-wrap break-all">
					{logs}
				</pre>
			)}
		</section>
	)
}

function ConsumersSection({ consumers }: { consumers: Array<{ id?: number; name?: string; groupType?: string; status?: string }> }) {
	return (
		<section className="space-y-4">
			<div>
				<h2 className="text-lg font-semibold">Consumers</h2>
				<p className="mt-1 text-sm text-muted-foreground">
					Node groups that reference this service via <code className="rounded bg-muted px-1 font-mono text-xs">postgres_service_id</code>. The service cannot be deleted while any consumer is attached.
				</p>
			</div>
			{consumers.length === 0 ? (
				<p className="text-sm text-muted-foreground">No consumers. Safe to delete.</p>
			) : (
				<div className="-mx-4 -my-2 overflow-x-auto whitespace-nowrap sm:-mx-6 lg:-mx-8">
					<div className="inline-block min-w-full px-4 py-2 align-middle sm:px-6 lg:px-8">
						<Table>
							<TableHeader>
								<TableRow>
									<TableHead className="whitespace-nowrap">Name</TableHead>
									<TableHead className="whitespace-nowrap">Type</TableHead>
									<TableHead className="whitespace-nowrap">Status</TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{consumers.map((c) => (
									<TableRow key={c.id}>
										<TableCell className="font-medium">
											<Link to={`/node-groups/${c.id}`} className="hover:underline">
												{c.name}
											</Link>
										</TableCell>
										<TableCell className="text-sm text-muted-foreground">{c.groupType || '—'}</TableCell>
										<TableCell>
											<Badge variant="outline">{c.status || '—'}</Badge>
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

// --- shared primitives ----------------------------------------------

function DescriptionItem({ label, value, mono = false }: { label: string; value?: string; mono?: boolean }) {
	return (
		<div className="min-w-0">
			<dt className="text-sm font-medium text-foreground">{label}</dt>
			<dd className={`mt-1 text-sm text-muted-foreground ${mono ? 'font-mono' : ''} break-all`}>{value || '—'}</dd>
		</div>
	)
}

// --- dialogs ---------------------------------------------------------

function StartServiceDialog({
	service,
	open,
	onOpenChange,
	onStarted,
}: {
	service: Service | null
	open: boolean
	onOpenChange: (v: boolean) => void
	onStarted: () => void
}) {
	const [networkName, setNetworkName] = useState('')
	const startMut = useMutation({
		...postServicesByIdStartMutation(),
		onSuccess: () => {
			toast.success('Service started')
			setNetworkName('')
			onStarted()
		},
		onError: (e: Error) => toast.error(`Start failed: ${e.message}`),
	})

	const onSubmit = (e: React.FormEvent) => {
		e.preventDefault()
		if (!service?.id) return
		startMut.mutate({ path: { id: service.id }, body: { networkName } })
	}

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>Start &ldquo;{service?.name}&rdquo;</DialogTitle>
					<DialogDescription>Runs the container on the chosen docker network so siblings can dial it by container name.</DialogDescription>
				</DialogHeader>
				<form onSubmit={onSubmit} className="space-y-3">
					<div className="space-y-1">
						<Label htmlFor="detail-start-net">Docker network</Label>
						<Input
							id="detail-start-net"
							required
							value={networkName}
							onChange={(e) => setNetworkName(e.target.value)}
							placeholder="e.g. chainlaunch-committer-org1"
						/>
					</div>
					<DialogFooter>
						<Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
							Cancel
						</Button>
						<Button type="submit" disabled={startMut.isPending}>
							{startMut.isPending ? 'Starting…' : 'Start'}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	)
}

function EditServiceDialog({
	service,
	open,
	onOpenChange,
	onSaved,
}: {
	service: Service | null
	open: boolean
	onOpenChange: (v: boolean) => void
	onSaved: () => void
}) {
	const [name, setName] = useState(service?.name ?? '')
	const [version, setVersion] = useState(service?.version ?? '')
	const [password, setPassword] = useState('')
	const [showPassword, setShowPassword] = useState(false)
	const [hostPort, setHostPort] = useState('')

	useMaybeReset(service?.id, () => {
		setName(service?.name ?? '')
		setVersion(service?.version ?? '')
		setPassword('')
		setShowPassword(false)
		setHostPort('')
	})

	const updateMut = useMutation({
		...putServicesByIdMutation(),
		onSuccess: () => {
			toast.success('Service updated')
			onSaved()
		},
		onError: (e: Error) => toast.error(`Update failed: ${e.message}`),
	})

	const onSubmit = (e: React.FormEvent) => {
		e.preventDefault()
		if (!service?.id) return
		updateMut.mutate({
			path: { id: service.id },
			body: {
				name: name !== (service.name ?? '') ? name : undefined,
				version: version !== (service.version ?? '') ? version : undefined,
				password: password ? password : undefined,
				hostPort: hostPort ? Number(hostPort) : undefined,
			},
		})
	}

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>Edit &ldquo;{service?.name}&rdquo;</DialogTitle>
					<DialogDescription>Only allowed while the service is not RUNNING or STARTING.</DialogDescription>
				</DialogHeader>
				<form onSubmit={onSubmit} className="space-y-3">
					<div className="space-y-1">
						<Label htmlFor="detail-edit-name">Name</Label>
						<Input id="detail-edit-name" value={name} onChange={(e) => setName(e.target.value)} />
					</div>
					<div className="grid grid-cols-2 gap-3">
						<div className="space-y-1">
							<Label htmlFor="detail-edit-version">Version</Label>
							<Input id="detail-edit-version" value={version} onChange={(e) => setVersion(e.target.value)} />
						</div>
						<div className="space-y-1">
							<Label htmlFor="detail-edit-port">Host port</Label>
							<Input id="detail-edit-port" type="number" value={hostPort} onChange={(e) => setHostPort(e.target.value)} placeholder="unchanged" />
						</div>
					</div>
					<div className="space-y-1">
						<Label htmlFor="detail-edit-pw">New password (optional)</Label>
						<div className="flex gap-2">
							<Input
								id="detail-edit-pw"
								type={showPassword ? 'text' : 'password'}
								value={password}
								onChange={(e) => setPassword(e.target.value)}
								placeholder="leave blank to keep current"
								className="font-mono"
							/>
							<Button
								type="button"
								variant="outline"
								size="icon"
								onClick={() => setShowPassword((v) => !v)}
								title={showPassword ? 'Hide' : 'Show'}
							>
								{showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
							</Button>
							<Button
								type="button"
								variant="outline"
								size="icon"
								onClick={() => {
									setPassword(generateSecurePassword())
									setShowPassword(true)
								}}
								title="Generate secure password"
							>
								<RefreshCw className="size-4" />
							</Button>
						</div>
					</div>
					<DialogFooter>
						<Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
							Cancel
						</Button>
						<Button type="submit" disabled={updateMut.isPending}>
							{updateMut.isPending ? 'Saving…' : 'Save'}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	)
}

function DeleteServiceDialog({
	service,
	open,
	onOpenChange,
	isPending,
	blocked,
	consumers,
	onConfirm,
}: {
	service: Service | null
	open: boolean
	onOpenChange: (v: boolean) => void
	isPending: boolean
	blocked: boolean
	consumers: Array<{ id?: number; name?: string; groupType?: string; status?: string }>
	onConfirm: () => void
}) {
	return (
		<AlertDialog open={open} onOpenChange={onOpenChange}>
			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>Delete service &ldquo;{service?.name}&rdquo;?</AlertDialogTitle>
					<AlertDialogDescription asChild>
						<div className="space-y-2">
							<p>The services row is removed from the database. The container must be stopped first.</p>
							{blocked && (
								<div className="rounded border border-destructive/50 bg-destructive/10 p-2 text-sm text-destructive">
									<div className="font-medium">Blocked: {consumers.length} node group(s) still reference this service.</div>
									<ul role="list" className="list-disc pl-5 mt-1">
										{consumers.map((c) => (
											<li key={c.id}>
												{c.name} <span className="text-xs opacity-75">({c.groupType} · {c.status})</span>
											</li>
										))}
									</ul>
									<div className="mt-1 text-xs">Detach the service from these groups before deleting.</div>
								</div>
							)}
						</div>
					</AlertDialogDescription>
				</AlertDialogHeader>
				<AlertDialogFooter>
					<AlertDialogCancel>Cancel</AlertDialogCancel>
					<AlertDialogAction
						className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
						onClick={onConfirm}
						disabled={isPending || blocked}
					>
						{isPending ? 'Deleting…' : 'Delete'}
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	)
}

function AddDatabaseDialog({
	serviceId,
	open,
	onOpenChange,
	onAdded,
}: {
	serviceId?: number
	open: boolean
	onOpenChange: (v: boolean) => void
	onAdded: () => void
}) {
	const [db, setDb] = useState('')
	const [user, setUser] = useState('')
	const [password, setPassword] = useState(() => generateSecurePassword())
	const [showPassword, setShowPassword] = useState(false)

	useMaybeReset(serviceId, () => {
		setDb('')
		setUser('')
		setPassword(generateSecurePassword())
		setShowPassword(false)
	})

	const addMut = useMutation({
		...postServicesByIdPostgresDatabasesMutation(),
		onSuccess: () => {
			toast.success('Database provisioned')
			setDb('')
			setUser('')
			setPassword(generateSecurePassword())
			onAdded()
		},
		onError: (e: Error) => toast.error(`Provision failed: ${e.message}`),
	})

	const onSubmit = (e: React.FormEvent) => {
		e.preventDefault()
		if (!serviceId) return
		addMut.mutate({
			path: { id: serviceId },
			body: { databases: [{ db, user, password }] },
		})
	}

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>Add database</DialogTitle>
					<DialogDescription>
						Creates a role + database inside this container and grants all privileges on the database to the role. Idempotent: re-running with the same names updates the password.
					</DialogDescription>
				</DialogHeader>
				<form onSubmit={onSubmit} className="space-y-3">
					<div className="space-y-1">
						<Label htmlFor="add-db">Database name</Label>
						<Input id="add-db" required value={db} onChange={(e) => setDb(e.target.value)} placeholder="e.g. party5_fabricx" />
					</div>
					<div className="space-y-1">
						<Label htmlFor="add-user">Role / user</Label>
						<Input id="add-user" required value={user} onChange={(e) => setUser(e.target.value)} placeholder="e.g. party5" />
					</div>
					<div className="space-y-1">
						<Label htmlFor="add-pw">Password</Label>
						<div className="flex gap-2">
							<Input
								id="add-pw"
								type={showPassword ? 'text' : 'password'}
								required
								value={password}
								onChange={(e) => setPassword(e.target.value)}
								className="font-mono"
							/>
							<Button
								type="button"
								variant="outline"
								size="icon"
								onClick={() => setShowPassword((v) => !v)}
								title={showPassword ? 'Hide' : 'Show'}
							>
								{showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
							</Button>
							<Button
								type="button"
								variant="outline"
								size="icon"
								onClick={() => navigator.clipboard.writeText(password).then(() => toast.success('Password copied'))}
								title="Copy password"
							>
								<Copy className="size-4" />
							</Button>
							<Button
								type="button"
								variant="outline"
								size="icon"
								onClick={() => setPassword(generateSecurePassword())}
								title="Generate new password"
							>
								<RefreshCw className="size-4" />
							</Button>
						</div>
					</div>
					<DialogFooter>
						<Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
							Cancel
						</Button>
						<Button type="submit" disabled={addMut.isPending || !db || !user || !password}>
							{addMut.isPending ? 'Creating…' : 'Create'}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	)
}

// useMaybeReset runs `fn` whenever `key` changes — resets local dialog form
// state when the surfaced service changes.
function useMaybeReset(key: number | undefined, fn: () => void) {
	const [last, setLast] = useState<number | undefined>(key)
	if (key !== last) {
		setLast(key)
		fn()
	}
}

import { GithubComChainlaunchChainlaunchPkgServicesTypesService as Service, TypesServiceStatus } from '@/api/client'
import {
	deleteServicesByIdMutation,
	getServicesByIdConsumersOptions,
	getServicesOptions,
	postServicesByIdStartMutation,
	postServicesByIdStopMutation,
	postServicesPostgresMutation,
	putServicesByIdMutation,
} from '@/api/client/@tanstack/react-query.gen'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useMutation, useQuery } from '@tanstack/react-query'
import { Copy, Database, Eye, EyeOff, MoreVertical, Plus, RefreshCw } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { toast } from 'sonner'

// generateSecurePassword returns a 24-char URL-safe password built from the
// web crypto RNG. 144 bits of entropy — fine for a managed-service secret
// the operator can rotate later via Edit.
function generateSecurePassword(length = 24): string {
	const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_'
	const bytes = new Uint8Array(length)
	crypto.getRandomValues(bytes)
	let out = ''
	for (let i = 0; i < length; i++) out += alphabet[bytes[i] % alphabet.length]
	return out
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

function actionsFor(status?: TypesServiceStatus): Array<'start' | 'stop' | 'edit' | 'delete'> {
	switch (status) {
		case 'RUNNING':
			return ['stop']
		case 'STARTING':
		case 'STOPPING':
			return []
		case 'STOPPED':
		case 'CREATED':
		case 'ERROR':
			return ['start', 'edit', 'delete']
		default:
			return ['start', 'edit', 'delete']
	}
}

// --- main page -------------------------------------------------------

export default function ServicesPage() {
	const { data: services, refetch, isLoading } = useQuery({
		...getServicesOptions(),
		refetchInterval: 5000,
	})

	const [createOpen, setCreateOpen] = useState(false)
	const [startTarget, setStartTarget] = useState<Service | null>(null)
	const [editTarget, setEditTarget] = useState<Service | null>(null)
	const [deleteTarget, setDeleteTarget] = useState<Service | null>(null)

	const stopSvc = useMutation({
		...postServicesByIdStopMutation(),
		onSuccess: () => {
			toast.success('Service stopped')
			refetch()
		},
		onError: (err: Error) => toast.error(`Stop failed: ${err.message}`),
	})

	const deleteSvc = useMutation({
		...deleteServicesByIdMutation(),
		onSuccess: () => {
			toast.success('Service deleted')
			setDeleteTarget(null)
			refetch()
		},
		onError: (err: Error) => toast.error(`Delete failed: ${err.message}`),
	})

	const onAction = (svc: Service, action: 'start' | 'stop' | 'edit' | 'delete') => {
		if (!svc.id) return
		switch (action) {
			case 'start':
				setStartTarget(svc)
				return
			case 'stop':
				stopSvc.mutate({ path: { id: svc.id } })
				return
			case 'edit':
				setEditTarget(svc)
				return
			case 'delete':
				setDeleteTarget(svc)
				return
		}
	}

	const list = services ?? []

	return (
		<div className="p-6 space-y-4">
			<div className="flex items-center justify-between">
				<div className="flex items-center gap-2">
					<Database className="size-5" />
					<h1 className="text-2xl font-semibold">Services</h1>
				</div>
				<Button onClick={() => setCreateOpen(true)}>
					<Plus className="size-4 mr-2" />
					New PostgreSQL
				</Button>
			</div>
			<p className="text-sm text-muted-foreground">
				Managed supporting services (PostgreSQL, etc.). Services are standalone — a node group references the service it needs via its{' '}
				<code className="rounded bg-muted px-1">postgres-service</code> endpoint.
			</p>

			<Card>
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Name</TableHead>
							<TableHead>Type</TableHead>
							<TableHead>Version</TableHead>
							<TableHead>Status</TableHead>
							<TableHead>Consumers</TableHead>
							<TableHead>Created</TableHead>
							<TableHead className="w-12" />
						</TableRow>
					</TableHeader>
					<TableBody>
						{isLoading && (
							<TableRow>
								<TableCell colSpan={7} className="text-center text-muted-foreground">
									Loading…
								</TableCell>
							</TableRow>
						)}
						{!isLoading && list.length === 0 && (
							<TableRow>
								<TableCell colSpan={7} className="text-center text-muted-foreground py-8">
									No services yet. Create one with the button above.
								</TableCell>
							</TableRow>
						)}
						{list.map((svc) => (
							<TableRow key={svc.id}>
								<TableCell className="font-medium">
									{svc.id ? (
										<Link to={`/services/${svc.id}`} className="hover:underline">
											{svc.name}
										</Link>
									) : (
										svc.name
									)}
								</TableCell>
								<TableCell>{svc.serviceType}</TableCell>
								<TableCell>{svc.version || '—'}</TableCell>
								<TableCell>
									<Badge variant={statusVariant(svc.status)}>{svc.status}</Badge>
									{svc.errorMessage && <span className="ml-2 text-xs text-destructive">{svc.errorMessage}</span>}
								</TableCell>
								<TableCell>
									<ConsumersCell serviceId={svc.id} />
								</TableCell>
								<TableCell className="text-sm text-muted-foreground">{svc.createdAt ? new Date(svc.createdAt).toLocaleString() : ''}</TableCell>
								<TableCell>
									<DropdownMenu>
										<DropdownMenuTrigger asChild>
											<Button variant="ghost" size="icon">
												<MoreVertical className="size-4" />
											</Button>
										</DropdownMenuTrigger>
										<DropdownMenuContent align="end">
											{actionsFor(svc.status).map((a) => (
												<DropdownMenuItem
													key={a}
													onClick={() => onAction(svc, a)}
													className={a === 'delete' ? 'text-destructive focus:text-destructive' : ''}
												>
													{a[0].toUpperCase() + a.slice(1)}
												</DropdownMenuItem>
											))}
											{actionsFor(svc.status).length === 0 && <DropdownMenuItem disabled>No actions</DropdownMenuItem>}
										</DropdownMenuContent>
									</DropdownMenu>
								</TableCell>
							</TableRow>
						))}
					</TableBody>
				</Table>
			</Card>

			<CreatePostgresDialog
				open={createOpen}
				onOpenChange={setCreateOpen}
				onCreated={() => {
					setCreateOpen(false)
					refetch()
				}}
			/>

			<StartServiceDialog
				service={startTarget}
				onOpenChange={(v) => !v && setStartTarget(null)}
				onStarted={() => {
					setStartTarget(null)
					refetch()
				}}
			/>

			<EditServiceDialog
				service={editTarget}
				onOpenChange={(v) => !v && setEditTarget(null)}
				onSaved={() => {
					setEditTarget(null)
					refetch()
				}}
			/>

			<DeleteServiceDialog
				target={deleteTarget}
				onOpenChange={(v) => !v && setDeleteTarget(null)}
				isPending={deleteSvc.isPending}
				onConfirm={() => deleteTarget?.id && deleteSvc.mutate({ path: { id: deleteTarget.id } })}
			/>
		</div>
	)
}

// --- consumers cell --------------------------------------------------

function ConsumersCell({ serviceId }: { serviceId?: number }) {
	const { data, isLoading } = useQuery({
		...getServicesByIdConsumersOptions({ path: { id: serviceId ?? 0 } }),
		enabled: !!serviceId,
		refetchInterval: 10_000,
	})
	if (!serviceId || isLoading) return <span className="text-xs text-muted-foreground">—</span>
	const consumers = data ?? []
	if (consumers.length === 0) return <span className="text-xs text-muted-foreground">None</span>
	return (
		<div className="flex flex-wrap gap-1">
			{consumers.map((c) => (
				<Badge key={c.id} variant="outline" title={`${c.groupType ?? ''} · ${c.status ?? ''}`}>
					{c.name}
				</Badge>
			))}
		</div>
	)
}

// --- delete dialog ---------------------------------------------------

function DeleteServiceDialog({
	target,
	onOpenChange,
	isPending,
	onConfirm,
}: {
	target: Service | null
	onOpenChange: (v: boolean) => void
	isPending: boolean
	onConfirm: () => void
}) {
	const { data: consumers } = useQuery({
		...getServicesByIdConsumersOptions({ path: { id: target?.id ?? 0 } }),
		enabled: !!target?.id,
	})
	const attached = consumers ?? []
	const blocked = attached.length > 0
	return (
		<AlertDialog open={!!target} onOpenChange={onOpenChange}>
			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>Delete service “{target?.name}”?</AlertDialogTitle>
					<AlertDialogDescription asChild>
						<div className="space-y-2">
							<p>The services row is removed from the database.</p>
							{blocked && (
								<div className="rounded border border-destructive/50 bg-destructive/10 p-2 text-sm text-destructive">
									<div className="font-medium">Blocked: {attached.length} node group(s) still reference this service.</div>
									<ul className="list-disc pl-5 mt-1">
										{attached.map((c) => (
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

// --- create ----------------------------------------------------------

function CreatePostgresDialog({ open, onOpenChange, onCreated }: { open: boolean; onOpenChange: (v: boolean) => void; onCreated: () => void }) {
	const [name, setName] = useState('')
	const [db, setDb] = useState('')
	const [user, setUser] = useState('')
	const [password, setPassword] = useState(() => generateSecurePassword())
	const [showPassword, setShowPassword] = useState(false)
	const [version, setVersion] = useState('')
	const [hostPort, setHostPort] = useState('')

	const createSvc = useMutation({
		...postServicesPostgresMutation(),
		onSuccess: () => {
			toast.success('Service created')
			setName('')
			setDb('')
			setUser('')
			setPassword(generateSecurePassword())
			setVersion('')
			setHostPort('')
			onCreated()
		},
		onError: (err: Error) => toast.error(`Create failed: ${err.message}`),
	})

	const onSubmit = (e: React.FormEvent) => {
		e.preventDefault()
		createSvc.mutate({
			body: {
				name,
				db,
				user,
				password,
				version: version || undefined,
				hostPort: hostPort ? Number(hostPort) : undefined,
			},
		})
	}

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>New PostgreSQL service</DialogTitle>
					<DialogDescription>Creates a CREATED-state service. Start it on a docker network when you're ready.</DialogDescription>
				</DialogHeader>
				<form onSubmit={onSubmit} className="space-y-3">
					<div className="space-y-1">
						<Label htmlFor="svc-name">Name</Label>
						<Input id="svc-name" required value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. committer-org1-pg" />
					</div>
					<div className="grid grid-cols-2 gap-3">
						<div className="space-y-1">
							<Label htmlFor="svc-db">Database</Label>
							<Input id="svc-db" required value={db} onChange={(e) => setDb(e.target.value)} />
						</div>
						<div className="space-y-1">
							<Label htmlFor="svc-user">User</Label>
							<Input id="svc-user" required value={user} onChange={(e) => setUser(e.target.value)} />
						</div>
					</div>
					<div className="space-y-1">
						<Label htmlFor="svc-password">Password</Label>
						<div className="flex gap-2">
							<Input
								id="svc-password"
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
								title={showPassword ? 'Hide password' : 'Show password'}
							>
								{showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
							</Button>
							<Button
								type="button"
								variant="outline"
								size="icon"
								onClick={() => {
									navigator.clipboard.writeText(password).then(() => toast.success('Password copied'))
								}}
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
						<p className="text-xs text-muted-foreground">Auto-generated. Copy it now — you can rotate it later via Edit.</p>
					</div>
					<div className="grid grid-cols-2 gap-3">
						<div className="space-y-1">
							<Label htmlFor="svc-version">Version (optional)</Label>
							<Input id="svc-version" value={version} onChange={(e) => setVersion(e.target.value)} placeholder="16" />
						</div>
						<div className="space-y-1">
							<Label htmlFor="svc-port">Host port (optional)</Label>
							<Input id="svc-port" type="number" value={hostPort} onChange={(e) => setHostPort(e.target.value)} placeholder="5432" />
						</div>
					</div>
					<DialogFooter>
						<Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
							Cancel
						</Button>
						<Button type="submit" disabled={createSvc.isPending}>
							{createSvc.isPending ? 'Creating…' : 'Create'}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	)
}

// --- start -----------------------------------------------------------

function StartServiceDialog({ service, onOpenChange, onStarted }: { service: Service | null; onOpenChange: (v: boolean) => void; onStarted: () => void }) {
	const [networkName, setNetworkName] = useState('')

	const startSvc = useMutation({
		...postServicesByIdStartMutation(),
		onSuccess: () => {
			toast.success('Service started')
			setNetworkName('')
			onStarted()
		},
		onError: (err: Error) => toast.error(`Start failed: ${err.message}`),
	})

	const onSubmit = (e: React.FormEvent) => {
		e.preventDefault()
		if (!service?.id) return
		startSvc.mutate({ path: { id: service.id }, body: { networkName } })
	}

	return (
		<Dialog open={!!service} onOpenChange={onOpenChange}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>Start “{service?.name}”</DialogTitle>
					<DialogDescription>Runs the container on the chosen docker network so siblings can dial it by container name.</DialogDescription>
				</DialogHeader>
				<form onSubmit={onSubmit} className="space-y-3">
					<div className="space-y-1">
						<Label htmlFor="start-net">Docker network</Label>
						<Input id="start-net" required value={networkName} onChange={(e) => setNetworkName(e.target.value)} placeholder="e.g. chainlaunch-committer-org1" />
					</div>
					<DialogFooter>
						<Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
							Cancel
						</Button>
						<Button type="submit" disabled={startSvc.isPending}>
							{startSvc.isPending ? 'Starting…' : 'Start'}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	)
}

// --- edit ------------------------------------------------------------

function EditServiceDialog({ service, onOpenChange, onSaved }: { service: Service | null; onOpenChange: (v: boolean) => void; onSaved: () => void }) {
	const [name, setName] = useState(service?.name ?? '')
	const [version, setVersion] = useState(service?.version ?? '')
	const [password, setPassword] = useState('')
	const [hostPort, setHostPort] = useState('')

	// Reset local form when target changes.
	useMaybeReset(service?.id, () => {
		setName(service?.name ?? '')
		setVersion(service?.version ?? '')
		setPassword('')
		setHostPort('')
	})

	const updateSvc = useMutation({
		...putServicesByIdMutation(),
		onSuccess: () => {
			toast.success('Service updated')
			onSaved()
		},
		onError: (err: Error) => toast.error(`Update failed: ${err.message}`),
	})

	const onSubmit = (e: React.FormEvent) => {
		e.preventDefault()
		if (!service?.id) return
		updateSvc.mutate({
			path: { id: service.id },
			body: {
				name: name !== service.name ? name : undefined,
				version: version !== (service.version ?? '') ? version : undefined,
				password: password ? password : undefined,
				hostPort: hostPort ? Number(hostPort) : undefined,
			},
		})
	}

	return (
		<Dialog open={!!service} onOpenChange={onOpenChange}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>Edit “{service?.name}”</DialogTitle>
					<DialogDescription>Only allowed while the service is not RUNNING or STARTING.</DialogDescription>
				</DialogHeader>
				<form onSubmit={onSubmit} className="space-y-3">
					<div className="space-y-1">
						<Label htmlFor="edit-name">Name</Label>
						<Input id="edit-name" value={name} onChange={(e) => setName(e.target.value)} />
					</div>
					<div className="grid grid-cols-2 gap-3">
						<div className="space-y-1">
							<Label htmlFor="edit-version">Version</Label>
							<Input id="edit-version" value={version} onChange={(e) => setVersion(e.target.value)} />
						</div>
						<div className="space-y-1">
							<Label htmlFor="edit-port">Host port</Label>
							<Input id="edit-port" type="number" value={hostPort} onChange={(e) => setHostPort(e.target.value)} placeholder="unchanged" />
						</div>
					</div>
					<div className="space-y-1">
						<Label htmlFor="edit-pw">New password (optional)</Label>
						<Input id="edit-pw" type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="leave blank to keep current" />
					</div>
					<DialogFooter>
						<Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
							Cancel
						</Button>
						<Button type="submit" disabled={updateSvc.isPending}>
							{updateSvc.isPending ? 'Saving…' : 'Save'}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	)
}

// useMaybeReset runs `fn` whenever `key` changes (including null → id).
// A tiny helper to avoid useEffect noise at the top of the component.
function useMaybeReset(key: number | undefined, fn: () => void) {
	const [last, setLast] = useState<number | undefined>(key)
	if (key !== last) {
		setLast(key)
		fn()
	}
}

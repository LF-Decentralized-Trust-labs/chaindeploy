import { TypesNodeGroup as NodeGroup, TypesGroupStatus } from '@/api/client'
import {
	deleteNodeGroupsByIdMutation,
	getNodeGroupsOptions,
	postNodeGroupsByIdRestartMutation,
	postNodeGroupsByIdStartMutation,
	postNodeGroupsByIdStopMutation,
} from '@/api/client/@tanstack/react-query.gen'
import { PageHeader, PageShell } from '@/components/layout/page-shell'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { usePageTitle } from '@/hooks/use-page-title'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronDown, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { toast } from 'sonner'

type StatusVariant = 'default' | 'secondary' | 'destructive' | 'outline'
type BulkAction = 'start' | 'stop' | 'restart' | 'delete'

const ACTION_VERBS: Record<BulkAction, { progressive: string; past: string; present: string }> = {
	start: { progressive: 'Starting', past: 'started', present: 'start' },
	stop: { progressive: 'Stopping', past: 'stopped', present: 'stop' },
	restart: { progressive: 'Restarting', past: 'restarted', present: 'restart' },
	delete: { progressive: 'Deleting', past: 'deleted', present: 'delete' },
}

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

export default function NodeGroupsPage() {
	usePageTitle('Node groups')
	const queryClient = useQueryClient()
	const { data: groups, isLoading } = useQuery({
		...getNodeGroupsOptions(),
		refetchInterval: 5000,
	})

	const list = useMemo(() => (groups ?? []) as NodeGroup[], [groups])

	const [groupToDelete, setGroupToDelete] = useState<NodeGroup | null>(null)
	const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
	const [bulkConfirm, setBulkConfirm] = useState<{ action: BulkAction; groups: NodeGroup[] } | null>(null)

	const invalidate = () => queryClient.invalidateQueries({ queryKey: getNodeGroupsOptions().queryKey })

	const startMut = useMutation({ ...postNodeGroupsByIdStartMutation(), onSuccess: invalidate })
	const stopMut = useMutation({ ...postNodeGroupsByIdStopMutation(), onSuccess: invalidate })
	const restartMut = useMutation({ ...postNodeGroupsByIdRestartMutation(), onSuccess: invalidate })
	const deleteMut = useMutation({ ...deleteNodeGroupsByIdMutation(), onSuccess: invalidate })

	const anyPending = startMut.isPending || stopMut.isPending || restartMut.isPending || deleteMut.isPending

	const selectedGroups = useMemo(() => list.filter((g) => g.id !== undefined && selectedIds.has(g.id)), [list, selectedIds])

	const toggleOne = (id: number, checked: boolean) => {
		setSelectedIds((prev) => {
			const next = new Set(prev)
			if (checked) next.add(id)
			else next.delete(id)
			return next
		})
	}

	const selectableIds = useMemo(() => list.filter((g) => g.id !== undefined).map((g) => g.id as number), [list])
	const allSelected = selectableIds.length > 0 && selectableIds.every((id) => selectedIds.has(id))

	const toggleAll = (checked: boolean) => {
		if (checked) setSelectedIds(new Set(selectableIds))
		else setSelectedIds(new Set())
	}

	const runBulk = async (action: BulkAction, groups: NodeGroup[]) => {
		if (groups.length === 0) return
		const verb = ACTION_VERBS[action]
		const mut = { start: startMut, stop: stopMut, restart: restartMut, delete: deleteMut }[action]
		const promise = Promise.all(
			groups.map((g) => mut.mutateAsync({ path: { id: g.id! } }))
		)
		toast.promise(promise, {
			loading: `${verb.progressive} ${groups.length} group${groups.length > 1 ? 's' : ''}…`,
			success: `Successfully ${verb.past} ${groups.length} group${groups.length > 1 ? 's' : ''}`,
			error: (err: unknown) => {
				const msg = err instanceof Error ? err.message : 'unknown error'
				return `Failed to ${verb.present} groups: ${msg}`
			},
		})
		try {
			await promise
		} catch {
			// toast.promise already surfaced it
		}
		setSelectedIds(new Set())
		invalidate()
	}

	const handleBulkAction = (action: BulkAction) => {
		if (selectedGroups.length === 0) return
		if (action === 'delete') {
			setBulkConfirm({ action, groups: selectedGroups })
		} else {
			void runBulk(action, selectedGroups)
		}
	}

	return (
		<PageShell>
			<PageHeader
				title="Node groups"
				description="Groups bundle children that share identity and deployment context (e.g. a FabricX committer is a group of sidecar/coordinator/validator/verifier/query children)."
			>
				{selectedGroups.length > 0 && (
					<DropdownMenu>
						<DropdownMenuTrigger asChild>
							<Button variant="outline" disabled={anyPending}>
								Bulk actions ({selectedGroups.length})
								<ChevronDown className="ml-2 h-4 w-4" />
							</Button>
						</DropdownMenuTrigger>
						<DropdownMenuContent align="end">
							<DropdownMenuItem onClick={() => handleBulkAction('start')}>Start</DropdownMenuItem>
							<DropdownMenuItem onClick={() => handleBulkAction('stop')}>Stop</DropdownMenuItem>
							<DropdownMenuItem onClick={() => handleBulkAction('restart')}>Restart</DropdownMenuItem>
							<DropdownMenuItem onClick={() => handleBulkAction('delete')} className="text-destructive focus:text-destructive">
								Delete
							</DropdownMenuItem>
						</DropdownMenuContent>
					</DropdownMenu>
				)}
			</PageHeader>

			<div className="-mx-4 overflow-x-auto sm:-mx-6 lg:-mx-8">
				<div className="inline-block min-w-full align-middle px-4 sm:px-6 lg:px-8">
					<Table>
						<TableHeader>
							<TableRow>
								<TableHead className="w-[40px]">
									<Checkbox
										checked={allSelected}
										onCheckedChange={(v) => toggleAll(Boolean(v))}
										disabled={selectableIds.length === 0}
										aria-label="Select all"
									/>
								</TableHead>
								<TableHead className="whitespace-nowrap">Name</TableHead>
								<TableHead className="whitespace-nowrap">Type</TableHead>
								<TableHead className="whitespace-nowrap">Platform</TableHead>
								<TableHead className="whitespace-nowrap">MSP ID</TableHead>
								<TableHead className="whitespace-nowrap">Status</TableHead>
								<TableHead className="whitespace-nowrap">Created</TableHead>
								<TableHead className="w-[60px]"></TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{isLoading && (
								<TableRow>
									<TableCell colSpan={8} className="text-center text-muted-foreground">
										Loading…
									</TableCell>
								</TableRow>
							)}
							{!isLoading && list.length === 0 && (
								<TableRow>
									<TableCell colSpan={8} className="text-center text-muted-foreground py-8">
										No node groups yet.
								</TableCell>
								</TableRow>
							)}
							{list.map((g) => {
								const id = g.id
								const checked = id !== undefined && selectedIds.has(id)
								return (
									<TableRow key={id} data-state={checked ? 'selected' : undefined}>
										<TableCell>
											<Checkbox
												checked={checked}
												onCheckedChange={(v) => id !== undefined && toggleOne(id, Boolean(v))}
												aria-label={`Select ${g.name}`}
											/>
										</TableCell>
										<TableCell className="font-medium">
											<Link to={`/node-groups/${id}`} className="hover:underline">
												{g.name}
											</Link>
										</TableCell>
										<TableCell className="text-sm text-muted-foreground">{g.groupType}</TableCell>
										<TableCell className="text-sm text-muted-foreground">{g.platform}</TableCell>
										<TableCell className="font-mono text-sm text-muted-foreground">{g.mspId || '—'}</TableCell>
										<TableCell>
											<Badge variant={statusVariant(g.status)}>{g.status}</Badge>
											{g.errorMessage && <span className="ml-2 text-xs text-destructive">{g.errorMessage}</span>}
										</TableCell>
										<TableCell className="text-sm text-muted-foreground">{g.createdAt ? new Date(g.createdAt).toLocaleString() : ''}</TableCell>
										<TableCell>
											<Button
												variant="ghost"
												size="icon"
												className="text-muted-foreground hover:text-destructive"
												onClick={() => setGroupToDelete(g)}
												aria-label={`Delete ${g.name}`}
											>
												<Trash2 className="size-4" />
											</Button>
										</TableCell>
									</TableRow>
								)
							})}
						</TableBody>
					</Table>
				</div>
			</div>

			<AlertDialog open={!!groupToDelete} onOpenChange={(open) => !open && setGroupToDelete(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Delete node group "{groupToDelete?.name}"?</AlertDialogTitle>
						<AlertDialogDescription>
							This removes the group row only. Any running child containers (router, batcher, consenter, assembler, committer components) will NOT be stopped automatically and will become orphans. Stop the children first if they are running.
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel disabled={deleteMut.isPending}>Cancel</AlertDialogCancel>
						<AlertDialogAction
							disabled={deleteMut.isPending}
							onClick={async (e) => {
								e.preventDefault()
								if (!groupToDelete?.id) return
								const name = groupToDelete.name ?? `#${groupToDelete.id}`
								try {
									await deleteMut.mutateAsync({ path: { id: groupToDelete.id } })
									toast.success(`Deleted node group "${name}"`)
									setGroupToDelete(null)
								} catch (err) {
									const msg = err instanceof Error ? err.message : 'unknown error'
									toast.error(`Failed to delete node group: ${msg}`)
								}
							}}
							className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
						>
							{deleteMut.isPending ? 'Deleting…' : 'Delete'}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>

			<AlertDialog open={!!bulkConfirm} onOpenChange={(open) => !open && setBulkConfirm(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Delete {bulkConfirm?.groups.length} node group{(bulkConfirm?.groups.length ?? 0) > 1 ? 's' : ''}?</AlertDialogTitle>
						<AlertDialogDescription asChild>
							<div className="space-y-2">
								<p>The following groups will be removed. Running child containers will NOT be stopped automatically.</p>
								<ul className="list-disc pl-4 space-y-1">
									{bulkConfirm?.groups.map((g) => (
										<li key={g.id} className="text-sm">{g.name}</li>
									))}
								</ul>
								<p className="text-destructive">This action cannot be undone.</p>
							</div>
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>Cancel</AlertDialogCancel>
						<AlertDialogAction
							onClick={async (e) => {
								e.preventDefault()
								if (!bulkConfirm) return
								const groups = bulkConfirm.groups
								setBulkConfirm(null)
								await runBulk('delete', groups)
							}}
							className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
						>
							Delete
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</PageShell>
	)
}

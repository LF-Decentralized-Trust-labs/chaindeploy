import { BackupTargetsCreate } from './backup-targets-create'

export function BackupTargetsEmpty() {
	return (
		<div className="flex min-h-[200px] flex-col items-center justify-center rounded-lg border border-dashed p-8 text-center animate-in fade-in-50">
			<div className="mx-auto flex max-w-[420px] flex-col items-center justify-center text-center">
				<h3 className="mt-4 text-lg font-semibold">No backup targets</h3>
				<p className="mb-4 mt-2 text-sm text-muted-foreground">You haven't created any backup targets yet. Create one to get started.</p>
				<BackupTargetsCreate onSuccess={() => {}} />
			</div>
		</div>
	)
}

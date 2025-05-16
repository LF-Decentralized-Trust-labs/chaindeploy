import { getBackupsTargetsOptions } from '@/api/client/@tanstack/react-query.gen'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useQuery } from '@tanstack/react-query'
import { BackupTargetsCreate } from './backup-targets-create'
import { BackupTargetsEmpty } from './backup-targets-empty'
import { BackupTargetsSkeleton } from './backup-targets-skeleton'
import { BackupTargetsTable } from './backup-targets-table'

export function BackupTargets() {
	const {
		data: targets = [],
		isLoading,
		refetch,
	} = useQuery({
		...getBackupsTargetsOptions(),
	})
	if (isLoading) {
		return <BackupTargetsSkeleton />
	}

	return (
		<Card>
			<CardHeader className="flex flex-row items-center justify-between">
				<div>
					<CardTitle>Backup Targets</CardTitle>
					<CardDescription>Configure S3 backup targets for your backups</CardDescription>
				</div>
				<BackupTargetsCreate onSuccess={refetch} />
			</CardHeader>
			<CardContent>{targets.length === 0 ? <BackupTargetsEmpty /> : <BackupTargetsTable targets={targets} onSuccess={() => refetch()} />}</CardContent>
		</Card>
	)
}

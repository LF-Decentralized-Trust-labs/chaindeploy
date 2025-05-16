import { getAuditLogsOptions } from '@/api/client/@tanstack/react-query.gen'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useQuery } from '@tanstack/react-query'
import { format } from 'date-fns'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'

interface AuditLogDetails {
	method?: string
	path?: string
	user_id?: string
	status?: number
	duration?: string
	query?: string
	user_agent?: string
	[key: string]: any
}

const severityColors = {
	INFO: 'bg-blue-500',
	WARNING: 'bg-yellow-500',
	CRITICAL: 'bg-red-500',
}

const outcomeColors = {
	SUCCESS: 'bg-green-500',
	FAILURE: 'bg-red-500',
	PENDING: 'bg-yellow-500',
}

export default function AuditLogsPage() {
	const navigate = useNavigate()
	const [page, setPage] = useState(1)
	const { data, isLoading } = useQuery({
		...getAuditLogsOptions({
			query: {
				page: page,
				page_size: 10,
			},
		}),
	})

	if (isLoading) {
		return <div>Loading...</div>
	}

	const totalPages = Math.ceil((data?.total_count || 0) / 10)
	const currentPage = page

	return (
		<div className="container mx-auto py-6">
			<Card>
				<CardHeader>
					<CardTitle>Audit Logs</CardTitle>
				</CardHeader>
				<CardContent>
					<Table>
						<TableHeader>
							<TableRow>
								<TableHead>Timestamp</TableHead>
								<TableHead>Method</TableHead>
								<TableHead>Path</TableHead>
								<TableHead>User ID</TableHead>
								<TableHead>Session ID</TableHead>
								<TableHead>Source</TableHead>
								<TableHead>Security</TableHead>
								<TableHead>Severity</TableHead>
								<TableHead>Outcome</TableHead>
								<TableHead>Resource</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{data?.items?.map((log) => {
								const details = log.details as AuditLogDetails | undefined
								return (
									<TableRow key={log.id} className="cursor-pointer hover:bg-muted/50" onClick={() => navigate(`/settings/audit-logs/${log.id}`)}>
										<TableCell>{log.timestamp ? format(new Date(log.timestamp), 'MMM d, yyyy HH:mm:ss') : '-'}</TableCell>
										<TableCell>{details?.method || '-'}</TableCell>
										<TableCell>{details?.path || '-'}</TableCell>
										<TableCell>{log.userIdentity || '-'}</TableCell>
										<TableCell>{details?.session_id || '-'}</TableCell>
										<TableCell>{log.eventSource || '-'}</TableCell>
										<TableCell>{details?.is_security_event && <Badge className="bg-red-500 text-white">Security Event</Badge>}</TableCell>
										<TableCell>{log.severity && <Badge className={`${severityColors[log.severity]} text-white`}>{log.severity}</Badge>}</TableCell>
										<TableCell>{log.eventOutcome && <Badge className={`${outcomeColors[log.eventOutcome]} text-white`}>{log.eventOutcome}</Badge>}</TableCell>
										<TableCell>{log.affectedResource || '-'}</TableCell>
									</TableRow>
								)
							})}
						</TableBody>
					</Table>

					<div className="flex items-center justify-between mt-4">
						<div className="text-sm text-muted-foreground">
							Page {currentPage} of {totalPages}
						</div>
						<div className="flex items-center space-x-2">
							<Button variant="outline" size="sm" onClick={() => setPage((p) => Math.max(1, p - 1))} disabled={currentPage <= 1}>
								<ChevronLeft className="h-4 w-4" />
								Previous
							</Button>
							<Button variant="outline" size="sm" onClick={() => setPage((p) => Math.min(totalPages, p + 1))} disabled={currentPage >= totalPages}>
								Next
								<ChevronRight className="h-4 w-4" />
							</Button>
						</div>
					</div>
				</CardContent>
			</Card>
		</div>
	)
}

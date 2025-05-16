import { getAuditLogsByIdOptions } from '@/api/client/@tanstack/react-query.gen'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useQuery } from '@tanstack/react-query'
import { format } from 'date-fns'
import { ArrowLeft } from 'lucide-react'
import { useNavigate, useParams } from 'react-router-dom'

interface AuditLogDetails {
	method?: string
	path?: string
	user_id?: string
	status?: number
	duration?: string
	query?: string
	user_agent?: string
	session_id?: string
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

export default function AuditLogDetailPage() {
	const { id } = useParams()
	console.log('id', id)
	const navigate = useNavigate()
	const { data, isLoading } = useQuery({
		...getAuditLogsByIdOptions({ path: { id: id! } }),
	})

	if (isLoading) {
		return <div>Loading...</div>
	}

	const details = data?.details as AuditLogDetails | undefined

	return (
		<div className="container mx-auto py-6">
			<Button variant="ghost" className="mb-4" onClick={() => navigate('/settings/audit-logs')}>
				<ArrowLeft className="mr-2 h-4 w-4" />
				Back to Audit Logs
			</Button>

			<Card>
				<CardHeader>
					<CardTitle>Audit Log Details</CardTitle>
				</CardHeader>
				<CardContent>
					<div className="grid gap-4">
						<div className="grid grid-cols-2 gap-4">
							<div>
								<h3 className="text-sm font-medium text-muted-foreground">Timestamp</h3>
								<p>{data?.timestamp ? format(new Date(data.timestamp), 'MMM d, yyyy HH:mm:ss') : '-'}</p>
							</div>
							<div>
								<h3 className="text-sm font-medium text-muted-foreground">Event Type</h3>
								<p>{data?.eventType || '-'}</p>
							</div>
							<div>
								<h3 className="text-sm font-medium text-muted-foreground">User</h3>
								<p>{data?.userIdentity || '-'}</p>
							</div>
							<div>
								<h3 className="text-sm font-medium text-muted-foreground">Source</h3>
								<p>{data?.eventSource || '-'}</p>
							</div>
							<div>
								<h3 className="text-sm font-medium text-muted-foreground">Severity</h3>
								{data?.severity && <Badge className={`${severityColors[data.severity]} text-white`}>{data.severity}</Badge>}
							</div>
							<div>
								<h3 className="text-sm font-medium text-muted-foreground">Outcome</h3>
								{data?.eventOutcome && <Badge className={`${outcomeColors[data.eventOutcome]} text-white`}>{data.eventOutcome}</Badge>}
							</div>
							<div>
								<h3 className="text-sm font-medium text-muted-foreground">Resource</h3>
								<p>{data?.affectedResource || '-'}</p>
							</div>
							<div>
								<h3 className="text-sm font-medium text-muted-foreground">Request ID</h3>
								<p>{data?.requestId || '-'}</p>
							</div>
							<div>
								<h3 className="text-sm font-medium text-muted-foreground">Source IP</h3>
								<p>{data?.sourceIp || '-'}</p>
							</div>
							{details && (
								<>
									<div>
										<h3 className="text-sm font-medium text-muted-foreground">HTTP Method</h3>
										<p>{details.method || '-'}</p>
									</div>
									<div>
										<h3 className="text-sm font-medium text-muted-foreground">HTTP Path</h3>
										<p>{details.path || '-'}</p>
									</div>
									<div>
										<h3 className="text-sm font-medium text-muted-foreground">User ID</h3>
										<p>{details.user_id || '-'}</p>
									</div>
									<div>
										<h3 className="text-sm font-medium text-muted-foreground">Status</h3>
										<p>{details.status || '-'}</p>
									</div>
									<div>
										<h3 className="text-sm font-medium text-muted-foreground">Duration</h3>
										<p>{details.duration || '-'}</p>
									</div>
									<div className="grid gap-2">
										<h3 className="text-sm font-medium">Session ID</h3>
										<p className="text-sm text-muted-foreground">{details?.session_id || '-'}</p>
									</div>
								</>
							)}
						</div>

						{details && Object.keys(details).length > 0 && (
							<div>
								<h3 className="text-sm font-medium text-muted-foreground mb-2">Additional Details</h3>
								<pre className="bg-muted p-4 rounded-lg overflow-auto">{JSON.stringify(details, null, 2)}</pre>
							</div>
						)}
					</div>
				</CardContent>
			</Card>
		</div>
	)
}

import { getChaincodeProjectsByIdOptions, postChaincodeProjectsByIdStartMutation, postChaincodeProjectsByIdStopMutation } from '@/api/client/@tanstack/react-query.gen'
import { CodeEditor } from '@/components/editor/CodeEditor'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useMutation, useQuery } from '@tanstack/react-query'
import { ArrowLeft, Code2, Loader2, PlayCircle, StopCircle } from 'lucide-react'
import { useCallback, useMemo, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'

export default function ChaincodeProjectEditorPage() {
	const { id } = useParams()
	const navigate = useNavigate()
	const [searchParams, setSearchParams] = useSearchParams()
	const projectId = parseInt(id || '0', 10)

	const { data: project, isLoading, error, refetch } = useQuery(getChaincodeProjectsByIdOptions({ path: { id: projectId } }))

	const startMutation = useMutation(postChaincodeProjectsByIdStartMutation())
	const stopMutation = useMutation(postChaincodeProjectsByIdStopMutation())

	const initialMode = useMemo(() => (searchParams.get('mode') === 'playground' ? 'playground' : 'editor') as 'editor' | 'playground', [searchParams])
	const [mode, setModeState] = useState<'editor' | 'playground'>(initialMode)

	const setMode = useCallback(
		(newMode: 'editor' | 'playground') => {
			setModeState(newMode)
			setSearchParams((prev) => {
				const params = new URLSearchParams(prev)
				params.set('mode', newMode)
				return params
			})
		},
		[setSearchParams]
	)

	const handleStart = useCallback(async () => {
		try {
			const toastId = toast.loading('Starting project...')
			await startMutation.mutateAsync({ path: { id: projectId } })
			toast.success('Project started!', { id: toastId })
			await refetch()
		} catch (err: any) {
			toast.error('Failed to start project', { description: err?.message })
		}
	}, [projectId, startMutation, refetch])

	const handleStop = useCallback(async () => {
		try {
			const toastId = toast.loading('Stopping project...')
			await stopMutation.mutateAsync({ path: { id: projectId } })
			toast.success('Project stopped!', { id: toastId })
			await refetch()
		} catch (err: any) {
			toast.error('Failed to stop project', { description: err?.message })
		}
	}, [projectId, stopMutation, refetch])

	if (isLoading) {
		return (
			<div className="h-screen flex flex-col">
				<div className="flex items-center gap-4 p-4 border-b bg-background">
					<Skeleton className="h-10 w-10 rounded-full" />
					<Skeleton className="h-6 w-40 rounded" />
					<div className="ml-auto flex gap-2 items-center">
						<Skeleton className="h-10 w-10 rounded" />
						<Skeleton className="h-10 w-10 rounded" />
						<Skeleton className="h-10 w-28 rounded" />
						<Skeleton className="h-4 w-20 rounded" />
					</div>
				</div>
				<div className="flex-1 p-4">
					<Skeleton className="h-full w-full rounded" />
				</div>
			</div>
		)
	}

	if (error) {
		return (
			<div className="h-screen flex flex-col items-center justify-center">
				<Alert variant="destructive" className="max-w-md w-full">
					<AlertTitle>Error loading project</AlertTitle>
					<AlertDescription>{error instanceof Error ? error.message : 'An unknown error occurred.'}</AlertDescription>
					<Button variant="outline" onClick={() => refetch()} className="mt-4">
						Retry
					</Button>
				</Alert>
			</div>
		)
	}

	return (
		<div className="h-screen flex flex-col">
			<div className="flex items-center gap-4 p-4 border-b bg-background">
				<Button variant="ghost" size="icon" onClick={() => navigate(`/sc/fabric/projects/chaincodes/${projectId}`)}>
					<ArrowLeft className="h-4 w-4" />
				</Button>
				<div className="flex items-center gap-2">
					<h1 className="text-lg font-semibold">{project?.name}</h1>

					<Badge variant={project?.status === 'running' ? 'success' : project?.status === 'stopped' ? 'destructive' : 'secondary'} className="ml-2">
						{project?.status}
					</Badge>
					{project?.status === 'running' ? (
						<Button onClick={handleStop} disabled={stopMutation.isPending} variant="destructive" className='size-8' size="icon">
							{stopMutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <StopCircle className="h-4 w-4" />}
						</Button>
					) : (
						<Button onClick={handleStart} disabled={startMutation.isPending} className='size-8' size="icon">
							{startMutation.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <PlayCircle className="h-4 w-4" />}
						</Button>
					)}
				</div>
				<div className="ml-auto flex gap-2 items-center">
					<Button size="icon" className='size-10' variant={mode === 'editor' ? 'default' : 'outline'} onClick={() => setMode('editor')} title="Editor mode">
						<Code2  />
					</Button>
					<Button size="icon" className='size-10' variant={mode === 'playground' ? 'default' : 'outline'} onClick={() => setMode('playground')} title="Playground mode">
						<PlayCircle  />
					</Button>
				</div>
			</div>
			<div className="flex-1 p-4">
				<CodeEditor mode={mode} projectId={projectId} chaincodeProject={project} />
			</div>
		</div>
	)
}

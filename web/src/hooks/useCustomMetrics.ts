import { useQuery } from '@tanstack/react-query'
import { postApiV1MetricsNodeByIdQuery } from '@/api/client'
import { MetricsDataPoint } from '@/components/metrics/MetricsCard'

interface CustomQueryRequest {
	query: string
	time?: string
	timeout?: string
	start?: string
	end?: string
	step?: string
}

interface UseCustomMetricsOptions {
	nodeId: string
	query: string
	start?: number
	end?: number
	step?: string
	time?: string
	timeout?: string
	enabled?: boolean
	refetchInterval?: number // in milliseconds
}

export function useCustomMetrics({ nodeId, query, start, end, step = '1m', time, timeout, enabled = true, refetchInterval }: UseCustomMetricsOptions) {
	return useQuery<MetricsDataPoint[]>({
		queryKey: ['custom-metrics', nodeId, query, start, end, step, time, timeout],
		queryFn: async () => {
			if (!nodeId || !query) return []

			const request: CustomQueryRequest = {
				query,
				...(time && { time }),
				...(timeout && { timeout }),
				...(start && { start: new Date(start).toISOString() }),
				...(end && { end: new Date(end).toISOString() }),
				...(step && { step }),
			}

			const response = await postApiV1MetricsNodeByIdQuery({
				path: { id: nodeId },
				body: request,
			})

			if (!response.data) return []

			// Transform Prometheus result into MetricsDataPoint[]
			return response.data.data.result.flatMap((item) => {
				if (item.values) {
					// Handle range query (matrix)
					return item.values.map(([timestamp, value]: [number, string]) => ({
						// Convert Unix timestamp (seconds) to milliseconds
						timestamp: timestamp * 1000,
						value: parseFloat(value),
					}))
				} else if (item.value) {
					// Handle instant query (vector)
					const [timestamp, value] = item.value as [number, string]
					return [
						{
							// Convert Unix timestamp (seconds) to milliseconds
							timestamp: timestamp * 1000,
							value: parseFloat(value),
						},
					]
				}
				return []
			})
		},
		enabled: enabled && !!nodeId && !!query,
		refetchInterval,
	})
}

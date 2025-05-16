import { useQuery } from '@tanstack/react-query'
import { getApiV1MetricsNodeByIdLabelByLabelValues } from '@/api/client'

interface UseMetricLabelsOptions {
	nodeId: string
	metric: string
	label: string
	enabled?: boolean
}

export function useMetricLabels({ nodeId, metric, label, enabled = true }: UseMetricLabelsOptions) {
	return useQuery({
		queryKey: ['metric-labels', nodeId, metric, label],
		queryFn: async () => {
			if (!nodeId || !metric || !label) return []

			const response = await getApiV1MetricsNodeByIdLabelByLabelValues({
				path: {
					id: nodeId,
					label: label,
				},
				query: {
					match: [metric],
				},
			})

			if (!response.data?.data) return []

			return response.data.data as string[]
		},
		enabled: enabled && !!nodeId && !!metric && !!label,
	})
}

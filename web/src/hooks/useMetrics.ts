import { useQuery } from "@tanstack/react-query";
import { getApiV1MetricsNodeByIdRange } from "@/api/client";

interface UseMetricsOptions {
  nodeId: string;
  query: string;
  start: number;
  end: number;
  step?: string;
}

interface MetricsDataPoint {
  timestamp: number;
  value: number;
}

interface MetricsResponse {
  data?: {
    result?: Array<{
      values?: Array<[string, string]>;
    }>;
  };
}

export function useMetrics({ nodeId, query, start, end, step = "1m" }: UseMetricsOptions) {
  return useQuery({
    queryKey: ['metrics', nodeId, query, start, end, step],
    queryFn: async () => {
      const response = await getApiV1MetricsNodeByIdRange({
        path: { id: nodeId },
        query: {
          query,
          start: new Date(start).toISOString(),
          end: new Date(end).toISOString(),
          step,
        }
      }) as MetricsResponse;

      if (!response.data?.result?.[0]?.values) {
        return [] as MetricsDataPoint[];
      }

      return response.data.result[0].values.map(([timestamp, value]) => ({
        timestamp: Number(timestamp),
        value: Number(value)
      }));
    }
  });
} 
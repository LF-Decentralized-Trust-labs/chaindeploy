import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { format } from 'date-fns'
import { TrendingUp, TrendingDown } from 'lucide-react'
import { Area, AreaChart, CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

export interface MetricsDataPoint {
	timestamp: number
	value: number
}

export type ChartType = 'line' | 'area'

export interface MetricsCardProps {
	title: string
	data: MetricsDataPoint[]
	color: string
	unit: string
	chartType?: ChartType
	valueFormatter?: (value: number) => string
	description?: string
	trend?: {
		value: number
		label: string
	}
}

export function MetricsCard({ title, data, color, unit, chartType = 'line', valueFormatter = (value) => value.toString(), description, trend }: MetricsCardProps) {
	// Calculate trend if not provided
	const calculatedTrend =
		trend ||
		(() => {
			if (data.length < 2) return null
			const firstValue = data[0].value
			const lastValue = data[data.length - 1].value
			if (firstValue === 0) {
				return {
					value: null,
					label: 'No trend',
				}
			}
			const change = ((lastValue - firstValue) / firstValue) * 100
			return {
				value: change,
				label: change >= 0 ? 'Trending up' : 'Trending down',
			}
		})()

	const chartData = data.map((point) => ({
		...point,
		formattedValue: valueFormatter(point.value),
	}))

	return (
		<Card>
			<CardHeader>
				<CardTitle>{title}</CardTitle>
				{description && <p className="text-sm text-muted-foreground">{description}</p>}
			</CardHeader>
			<CardContent>
				{!data || data.length === 0 ? (
					<div className="h-[300px] flex items-center justify-center text-muted-foreground text-center">
						<span>No data available</span>
					</div>
				) : (
					<>
						<div className="h-[300px]">
							<ResponsiveContainer width="100%" height="100%">
								{chartType === 'line' ? (
									<LineChart data={chartData} margin={{ left: 12, right: 12 }}>
										<CartesianGrid strokeDasharray="3 3" vertical={false} />
										<XAxis dataKey="timestamp" tickFormatter={(value) => format(value, 'HH:mm:ss')} tickLine={false} axisLine={false} tickMargin={8} />
										<YAxis tickFormatter={(value) => valueFormatter(value)} tickLine={false} axisLine={false} tickMargin={8} />
										<Tooltip labelFormatter={(value) => format(value, 'HH:mm:ss')} formatter={(value: number) => [valueFormatter(value), unit]} cursor={false} />
										<Line type="monotone" dataKey="value" stroke={color} dot={false} strokeWidth={2} />
									</LineChart>
								) : (
									<AreaChart data={chartData} margin={{ left: 12, right: 12 }}>
										<CartesianGrid strokeDasharray="3 3" vertical={false} />
										<XAxis dataKey="timestamp" tickFormatter={(value) => format(value, 'HH:mm:ss')} tickLine={false} axisLine={false} tickMargin={8} />
										<YAxis tickFormatter={(value) => valueFormatter(value)} tickLine={false} axisLine={false} tickMargin={8} />
										<Tooltip labelFormatter={(value) => format(value, 'HH:mm:ss')} formatter={(value: number) => [valueFormatter(value), unit]} cursor={false} />
										<Area type="monotone" dataKey="value" stroke={color} fill={color} fillOpacity={0.2} strokeWidth={2} />
									</AreaChart>
								)}
							</ResponsiveContainer>
						</div>
						{calculatedTrend && (
							<div className="mt-4 flex items-center gap-2 text-sm">
								<div className="flex items-center gap-2 font-medium leading-none">
									{calculatedTrend.value === null || isNaN(calculatedTrend.value) ? (
										<span>Trend: N/A</span>
									) : (
										<>
											{calculatedTrend.label} by {Math.abs(calculatedTrend.value).toFixed(1)}%
											{calculatedTrend.value >= 0 ? <TrendingUp className="h-4 w-4 text-green-500" /> : <TrendingDown className="h-4 w-4 text-red-500" />}
										</>
									)}
								</div>
							</div>
						)}
					</>
				)}
			</CardContent>
		</Card>
	)
}

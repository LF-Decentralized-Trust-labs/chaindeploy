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
	data?: MetricsDataPoint[]
	color?: string
	unit: string
	chartType?: ChartType
	valueFormatter?: (value: number) => string
	description?: string
	trend?: {
		value: number
		label: string
	}
	series?: { name: string; color: string; data: MetricsDataPoint[] }[]
}

export function MetricsCard({ title, data = [], color = '#2563eb', unit, chartType = 'line', valueFormatter = (value) => value.toString(), description, trend, series }: MetricsCardProps) {
	// Calculate trend if not provided
	const calculatedTrend =
		trend ||
		(() => {
			if (series && series.length > 0) {
				// Use the first series for trend
				const s = series[0].data
				if (s.length < 2) return null
				const firstValue = s[0].value
				const lastValue = s[s.length - 1].value
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
			}
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

	// For multi-series, merge all timestamps and build chartData with each series as a key
	let chartData: any[] = []
	if (series && series.length > 0) {
		const allTimestamps = Array.from(new Set(series.flatMap((s) => s.data.map((d) => d.timestamp)))).sort((a, b) => a - b)
		chartData = allTimestamps.map((timestamp) => {
			const entry: any = { timestamp }
			series.forEach((s) => {
				const point = s.data.find((d) => d.timestamp === timestamp)
				entry[s.name] = point ? valueFormatter(point.value) : null
			})
			return entry
		})
	} else {
		chartData = data.map((point) => ({
			...point,
			formattedValue: valueFormatter(point.value),
		}))
	}

	return (
		<Card>
			<CardHeader>
				<CardTitle>{title}</CardTitle>
				{description && <p className="text-sm text-muted-foreground">{description}</p>}
			</CardHeader>
			<CardContent>
				{(series && series.length === 0) || (!series && (!data || data.length === 0)) ? (
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
										<Tooltip labelFormatter={(value) => format(value, 'HH:mm:ss')} formatter={(value: number, name: string) => [value, name]} cursor={false} />
										{series ? (
											series.map((s) => <Line key={s.name} type="monotone" dataKey={s.name} stroke={s.color} dot={false} strokeWidth={2} />)
										) : (
											<Line type="monotone" dataKey="value" stroke={color} dot={false} strokeWidth={2} />
										)}
									</LineChart>
								) : (
									<AreaChart data={chartData} margin={{ left: 12, right: 12 }}>
										<CartesianGrid strokeDasharray="3 3" vertical={false} />
										<XAxis dataKey="timestamp" tickFormatter={(value) => format(value, 'HH:mm:ss')} tickLine={false} axisLine={false} tickMargin={8} />
										<YAxis tickFormatter={(value) => valueFormatter(value)} tickLine={false} axisLine={false} tickMargin={8} />
										<Tooltip labelFormatter={(value) => format(value, 'HH:mm:ss')} formatter={(value: number, name: string) => [value, name]} cursor={false} />
										{series ? (
											series.map((s) => <Area key={s.name} type="monotone" dataKey={s.name} stroke={s.color} fill={s.color} fillOpacity={0.2} strokeWidth={2} />)
										) : (
											<Area type="monotone" dataKey="value" stroke={color} fill={color} fillOpacity={0.2} strokeWidth={2} />
										)}
									</AreaChart>
								)}
							</ResponsiveContainer>
						</div>
						{series && series.length > 0 && (
							<div className="mt-4 flex flex-wrap gap-4 text-sm">
								{series.map((s) => (
									<span key={s.name} className="flex items-center gap-2">
										<span className="inline-block w-3 h-3 rounded-full" style={{ backgroundColor: s.color }} />
										{s.name}
									</span>
								))}
							</div>
						)}
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

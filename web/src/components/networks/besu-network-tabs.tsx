import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

export type BesuTabValue = 'details' | 'genesis' | 'validators' | 'explorer'

interface BesuNetworkTabsProps {
	tab: BesuTabValue
	setTab: (tab: BesuTabValue) => void
	networkDetails: React.ReactNode
	genesis: React.ReactNode
	validators: React.ReactNode
	explorer: React.ReactNode
}

export function BesuNetworkTabs({ tab, setTab, networkDetails, genesis, validators, explorer }: BesuNetworkTabsProps) {
	return (
		<Tabs value={tab} onValueChange={(value) => setTab(value as BesuTabValue)}>
			<TabsList>
				<TabsTrigger value="details">Details</TabsTrigger>
				<TabsTrigger value="genesis">Genesis</TabsTrigger>
				<TabsTrigger value="validators">Validators</TabsTrigger>
				<TabsTrigger value="explorer">Explorer</TabsTrigger>
			</TabsList>

			<TabsContent className="mt-8" value="details">
				{networkDetails}
			</TabsContent>

			<TabsContent className="mt-8" value="genesis">
				{genesis}
			</TabsContent>

			<TabsContent className="mt-8" value="validators">
				{validators}
			</TabsContent>

			<TabsContent className="mt-8" value="explorer">
				{explorer}
			</TabsContent>
		</Tabs>
	)
} 
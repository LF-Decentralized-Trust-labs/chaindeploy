import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { LockIcon, StarIcon } from 'lucide-react'

interface ProFeatureGateProps {
	title: string
	description: string
}

export function ProFeatureGate({ title, description }: ProFeatureGateProps) {
	return (
		<div className="min-h-screen flex items-center justify-center p-4">
			<Card className="p-12 flex flex-col items-center text-center max-w-3xl w-full mx-auto shadow-lg">
				<div className="bg-primary/10 p-4 rounded-full mb-6">
					<StarIcon className="h-12 w-12 text-primary" />
				</div>
				<LockIcon className="h-14 w-14 text-muted-foreground mb-6" />
				<h2 className="text-3xl font-bold mb-4">{title}</h2>
				<p className="text-muted-foreground text-lg mb-8 max-w-xl leading-relaxed">{description}</p>
				<Button size="lg" className="text-lg px-8 py-6 font-semibold hover:scale-105 transition-transform" onClick={() => window.open('https://chainlaunch.dev/premium', '_blank')}>
					Upgrade to Pro
				</Button>
			</Card>
		</div>
	)
}

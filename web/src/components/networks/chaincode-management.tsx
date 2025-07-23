import { HttpGetNetworkNodesResponse, HttpNetworkResponse } from '@/api/client'
import { getNodesByIdChannelsByChannelIdChaincodesOptions, getOrganizationsOptions } from '@/api/client/@tanstack/react-query.gen'
import { Card } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useQuery } from '@tanstack/react-query'
import { AlertTriangle, Check, Code, Copy } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import SyntaxHighlighter, { SyntaxHighlighterProps } from 'react-syntax-highlighter'
import { docco } from 'react-syntax-highlighter/dist/esm/styles/hljs'
import rehypeRaw from 'rehype-raw'
import { Skeleton } from '../ui/skeleton'
import { useTheme } from 'next-themes'
import { atomOneDark } from 'react-syntax-highlighter/dist/esm/styles/hljs'
const SyntaxHighlighterComp = SyntaxHighlighter as unknown as React.ComponentType<SyntaxHighlighterProps>

// Update the CHAINCODE_INSTRUCTIONS to be a function that takes parameters
const getChainCodeInstructions = (channelName: string, mspId: string) => {
	// Get the current origin and append /api/v1
	const apiUrl = typeof window !== 'undefined' ? `${window.location.origin}/api/v1` : 'http://localhost:8100/api/v1'

	return `
# Chaincode Installation Guide

## Clone the Repository

First, clone the chaincode repository:

\`\`\`bash
git clone https://github.com/kfs-learn/chaincode-typescript
cd chaincode-typescript
\`\`\`

## Install Required Tools

### Install bun.sh

We need to install bun.sh to run the project:

\`\`\`bash
curl -fsSL https://bun.sh/install | bash
\`\`\`

### Install Node.JS using NVM

First, install NVM:

\`\`\`bash
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash
\`\`\`

Then, install Node.JS using NVM:

\`\`\`bash
nvm install v22
nvm use default v22
\`\`\`

### Install Dependencies

Install the project dependencies:

\`\`\`bash
bun install
\`\`\`

## Start Chaincode

### Pull Network Configuration

First, set up environment variables and pull the network configuration:

\`\`\`bash
export CHANNEL_NAME=${channelName}
export MSP_ID=${mspId}
export URL="${apiUrl}"
export CHAINLAUNCH_USER=admin
export CHAINLAUNCH_PASSWORD="<chainlaunch_password>"

chainlaunch fabric network-config pull \\
    --network=$CHANNEL_NAME \\
    --msp-id=$MSP_ID \\
    --url=$URL \\
    --username="$CHAINLAUNCH_USER" \\
    --password="$CHAINLAUNCH_PASSWORD" \\
    --output=network-config.yaml
\`\`\`

### Start the Chaincode Service

Set up additional environment variables and start the chaincode:

\`\`\`bash
export CHANNEL_NAME=${channelName}
export CHAINCODE_NAME=basic
export CHAINCODE_ADDRESS="localhost:9996"  # Chaincode listening address
export USER_NAME=admin
export MSP_ID=${mspId}

chainlaunch fabric install --local \\
    --config=$PWD/network-config.yaml \\
    --channel=$CHANNEL_NAME \\
    --chaincode=$CHAINCODE_NAME \\
    -o $MSP_ID -u $USER_NAME \\
    --policy="OR('\${MSP_ID}.member')" \\
    --chaincodeAddress="\${CHAINCODE_ADDRESS}" \\
    --envFile=$PWD/.env

bun run build
bun start:dev
\`\`\`

### Initialize and Test the Chaincode

Initialize the ledger and verify it's working:

\`\`\`bash
export CHANNEL_NAME=${channelName}
export CHAINCODE_NAME=basic
export MSP_ID=${mspId}

# Initialize the ledger
chainlaunch fabric invoke \\
    --chaincode=$CHAINCODE_NAME \\
    --config=network-config.yaml \\
    --channel $CHANNEL_NAME \\
    --fcn InitLedger \\
    --user=admin \\
    --mspID=$MSP_ID

# Query all assets to verify
chainlaunch fabric query \\
    --chaincode=$CHAINCODE_NAME \\
    --config=network-config.yaml \\
    --channel $CHANNEL_NAME \\
    --fcn GetAllAssets \\
    --user=admin \\
    --mspID=$MSP_ID
\`\`\`
`
}

function CopyButton({ text }: { text: string }) {
	const [copied, setCopied] = useState(false)

	const copy = () => {
		navigator.clipboard.writeText(text)
		setCopied(true)
		setTimeout(() => setCopied(false), 2000)
	}

	return (
		<button onClick={copy} className="absolute right-2 top-2 p-2 hover:bg-muted-foreground/20 rounded-md transition-colors">
			{copied ? <Check className="h-4 w-4 text-green-500" /> : <Copy className="h-4 w-4 text-muted-foreground" />}
		</button>
	)
}
interface ChaincodeManagementProps {
	network: HttpNetworkResponse
	channelConfig: Record<string, any>
	networkNodes: HttpGetNetworkNodesResponse
}

export function ChaincodeManagement({ networkNodes, network, channelConfig }: ChaincodeManagementProps) {
	const [selectedOrg, setSelectedOrg] = useState<{ id: number; mspId: string } | null>(null)
	const { theme } = useTheme();

	const { data: fabricOrgs } = useQuery({
		...getOrganizationsOptions({query: {limit:1000}}),
	})

	const peerOrgs = useMemo(
		() =>
			Object.keys(channelConfig?.config?.data?.data?.[0]?.payload?.data?.config?.channel_group?.groups?.Application?.groups || {}).filter(
				(mspId) => fabricOrgs?.items?.find((org) => org.mspId === mspId)!!
			),
		[channelConfig, fabricOrgs]
	)
	useEffect(() => {
		if (peerOrgs?.length) {
			const org = fabricOrgs?.items?.find((org) => peerOrgs.includes(org.mspId!))
			if (org) {
				setSelectedOrg({ id: org.id!, mspId: org.mspId! })
			}
		}
	}, [fabricOrgs])
	console.log(selectedOrg)
	return (
		<div className="space-y-4">
			<div className="flex items-center gap-4 mb-6">
				<div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center">
					<Code className="h-6 w-6 text-primary" />
				</div>
				<div>
					<h2 className="text-lg font-semibold">Chaincode Installation</h2>
					<p className="text-sm text-muted-foreground">Instructions for installing and managing chaincode</p>
				</div>
			</div>

			{networkNodes?.nodes?.find((node) => node.status === 'joined' && node.node?.nodeType === 'FABRIC_PEER') && (
				<CommittedChaincodes
					networkId={network.id!}
					channelName={network.name!}
					peerId={networkNodes.nodes.find((node) => node.status === 'joined' && node.node?.nodeType === 'FABRIC_PEER')!.node!.id!}
				/>
			)}

			<Card className="p-6">
				<div className="mb-6">
					<Select
						value={selectedOrg?.mspId}
						onValueChange={(mspId) => {
							const org = fabricOrgs?.items?.find((org) => org.mspId === mspId)
							if (org) {
								setSelectedOrg({
									id: org.id!,
									mspId: org.mspId!,
								})
							}
						}}
					>
						<SelectTrigger>
							<SelectValue placeholder="Select an organization" />
						</SelectTrigger>
						<SelectContent>
							{peerOrgs?.map((mspId) => (
								<SelectItem key={mspId} value={mspId}>
									{mspId}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</div>

				<div className="">
					<ReactMarkdown
						rehypePlugins={[rehypeRaw]}
						components={{
							h1: ({ children }) => <h1 className="text-2xl font-bold mb-4 mt-0">{children}</h1>,
							h2: ({ children }) => <h2 className="text-xl font-semibold mt-6 mb-3">{children}</h2>,
							h3: ({ children }) => <h3 className="text-lg font-semibold mt-4 mb-2">{children}</h3>,
							h4: ({ children }) => <h4 className="text-base font-semibold mt-4 mb-2">{children}</h4>,
							h5: ({ children }) => <h5 className="text-sm font-semibold mt-4 mb-2">{children}</h5>,
							h6: ({ children }) => <h6 className="text-xs font-semibold mt-4 mb-2">{children}</h6>,
							code: ({ node, className, children, ...props }) => {
								const match = /language-(\w+)/.exec(className || '')
								const content = Array.isArray(children) ? children.join('') : String(children)

								return match ? (
									<div className="relative group">
										<CopyButton text={content.replace(/\n$/, '')} />
										<SyntaxHighlighterComp style={theme === 'dark' ? atomOneDark : docco} language="javascript">
											{content}
										</SyntaxHighlighterComp>
									</div>
								) : (
									<code {...props} className={`${className} !bg-muted !text-primary px-1.5 py-0.5 rounded`}>
										{children}
									</code>
								)
							},
							p: ({ children }) => <p className="my-4 leading-7">{children}</p>,
							ul: ({ children }) => <ul className="my-6 ml-6 list-disc [&>li]:mt-2">{children}</ul>,
							ol: ({ children }) => <ol className="my-6 ml-6 list-decimal [&>li]:mt-2">{children}</ol>,
							blockquote: ({ children }) => <blockquote className="mt-6 border-l-2 border-border pl-6 italic">{children}</blockquote>,
						}}
					>
						{getChainCodeInstructions(network.name!, selectedOrg?.mspId || '')}
					</ReactMarkdown>
				</div>
			</Card>
		</div>
	)
}

function CommittedChaincodes({ networkId, channelName, peerId }: { networkId: number; channelName: string; peerId: number }) {
	const { data: chaincodes, isLoading } = useQuery({
		...getNodesByIdChannelsByChannelIdChaincodesOptions({
			path: {
				id: peerId,
				channelID: channelName,
			},
		}),
	})

	if (isLoading) {
		return <Skeleton className="h-32 w-full" />
	}

	if (!chaincodes || chaincodes.length === 0) {
		return (
			<Card className="p-6">
				<div className="flex items-center gap-4">
					<AlertTriangle className="h-5 w-5 text-muted-foreground" />
					<p className="text-sm text-muted-foreground">No chaincodes have been committed to this channel</p>
				</div>
			</Card>
		)
	}

	return (
		<Card className="p-6">
			<div className="space-y-4">
				<div className="flex items-center gap-4">
					<div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center">
						<Code className="h-6 w-6 text-primary" />
					</div>
					<div>
						<h2 className="text-lg font-semibold">Committed Chaincodes</h2>
						<p className="text-sm text-muted-foreground">Chaincodes that have been committed to this channel</p>
					</div>
				</div>

				<div className="border rounded-lg">
					<table className="w-full">
						<thead>
							<tr className="border-b">
								<th className="text-left p-4 font-medium">Name</th>
								<th className="text-left p-4 font-medium">Version</th>
								<th className="text-left p-4 font-medium">Sequence</th>
								<th className="text-left p-4 font-medium">Init Required</th>
							</tr>
						</thead>
						<tbody>
							{chaincodes.map((chaincode) => (
								<tr key={chaincode.name} className="border-b last:border-0">
									<td className="p-4 font-mono">{chaincode.name}</td>
									<td className="p-4 font-mono">{chaincode.version}</td>
									<td className="p-4">{chaincode.sequence}</td>
									<td className="p-4">{chaincode.initRequired ? 'Yes' : 'No'}</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			</div>
		</Card>
	)
}

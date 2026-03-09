import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { CopyButton } from '@/components/ui/copy-button'
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'
import { BookOpen, Code, FileCode, Package, Rocket, Terminal, Wallet, CheckCircle2, ExternalLink, AlertCircle } from 'lucide-react'

interface BesuSmartContractTutorialProps {
	rpcEndpoint?: string
	chainId?: number
}

export function BesuSmartContractTutorial({ rpcEndpoint, chainId }: BesuSmartContractTutorialProps) {
	const sampleContract = `// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

contract SimpleStorage {
    uint256 private storedValue;

    event ValueChanged(uint256 newValue);

    function set(uint256 value) public {
        storedValue = value;
        emit ValueChanged(value);
    }

    function get() public view returns (uint256) {
        return storedValue;
    }
}`

	const hardhatConfig = `// hardhat.config.js
require("@nomicfoundation/hardhat-toolbox");

module.exports = {
  solidity: "0.8.19",
  networks: {
    besu: {
      url: "${rpcEndpoint || 'http://localhost:8545'}",
      chainId: ${chainId || 1337},
      // For development networks, you can use test accounts
      // accounts: ["0xYOUR_PRIVATE_KEY"]
    }
  }
};`

	const foundryConfig = `# foundry.toml
[profile.default]
src = "src"
out = "out"
libs = ["lib"]
solc = "0.8.19"

[rpc_endpoints]
besu = "${rpcEndpoint || 'http://localhost:8545'}"`

	const deployScript = `// scripts/deploy.js
const hre = require("hardhat");

async function main() {
  const SimpleStorage = await hre.ethers.getContractFactory("SimpleStorage");
  const storage = await SimpleStorage.deploy();
  await storage.waitForDeployment();

  console.log("SimpleStorage deployed to:", await storage.getAddress());
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});`

	const foundryDeployScript = `// script/Deploy.s.sol
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import "forge-std/Script.sol";
import "../src/SimpleStorage.sol";

contract DeployScript is Script {
    function run() external {
        vm.startBroadcast();

        SimpleStorage storage_ = new SimpleStorage();
        console.log("SimpleStorage deployed to:", address(storage_));

        vm.stopBroadcast();
    }
}`

	const remixSteps = `1. Open Remix IDE (https://remix.ethereum.org)
2. Create a new file: SimpleStorage.sol
3. Paste your contract code
4. Compile using Solidity Compiler (Ctrl+S)
5. Go to "Deploy & Run Transactions"
6. Select "Injected Provider" or "Custom - External HTTP Provider"
7. Enter your Besu RPC endpoint: ${rpcEndpoint || 'http://localhost:8545'}
8. Click "Deploy"`

	return (
		<div className="space-y-6">
			{/* Header */}
			<div className="flex items-center gap-4">
				<div className="h-12 w-12 rounded-lg bg-primary/10 flex items-center justify-center">
					<BookOpen className="h-6 w-6 text-primary" />
				</div>
				<div>
					<h2 className="text-lg font-semibold">Smart Contract Deployment Tutorial</h2>
					<p className="text-sm text-muted-foreground">Learn how to deploy smart contracts to your Besu network</p>
				</div>
			</div>

			{/* Network Info Alert */}
			{rpcEndpoint && (
				<Alert>
					<Terminal className="h-4 w-4" />
					<AlertDescription className="flex items-center justify-between">
						<span>
							Your network RPC endpoint: <code className="bg-muted px-2 py-1 rounded text-sm">{rpcEndpoint}</code>
						</span>
						<CopyButton textToCopy={rpcEndpoint} label="" variant="ghost" size="sm" className="h-6 w-6 p-0 ml-2" />
					</AlertDescription>
				</Alert>
			)}

			{/* Prerequisites */}
			<Card>
				<CardHeader>
					<CardTitle className="flex items-center gap-2 text-base">
						<CheckCircle2 className="h-5 w-5 text-green-500" />
						Prerequisites
					</CardTitle>
					<CardDescription>Make sure you have these ready before deploying</CardDescription>
				</CardHeader>
				<CardContent>
					<ul className="space-y-3">
						<li className="flex items-start gap-3">
							<Badge variant="outline" className="mt-0.5">1</Badge>
							<div>
								<p className="font-medium">Node.js and npm/yarn</p>
								<p className="text-sm text-muted-foreground">Required for Hardhat or other development frameworks</p>
							</div>
						</li>
						<li className="flex items-start gap-3">
							<Badge variant="outline" className="mt-0.5">2</Badge>
							<div>
								<p className="font-medium">A funded account</p>
								<p className="text-sm text-muted-foreground">You need an account with ETH to pay for gas fees on your network</p>
							</div>
						</li>
						<li className="flex items-start gap-3">
							<Badge variant="outline" className="mt-0.5">3</Badge>
							<div>
								<p className="font-medium">Your Solidity smart contract</p>
								<p className="text-sm text-muted-foreground">The contract code you want to deploy (see example below)</p>
							</div>
						</li>
					</ul>
				</CardContent>
			</Card>

			{/* Sample Contract */}
			<Card>
				<CardHeader>
					<CardTitle className="flex items-center gap-2 text-base">
						<FileCode className="h-5 w-5" />
						Sample Contract: SimpleStorage
					</CardTitle>
					<CardDescription>A basic contract to get started - stores and retrieves a number</CardDescription>
				</CardHeader>
				<CardContent>
					<div className="relative">
						<div className="absolute right-2 top-2">
							<CopyButton textToCopy={sampleContract} label="" variant="ghost" size="sm" className="h-8 w-8" />
						</div>
						<div className="bg-muted/50 dark:bg-slate-950 p-4 rounded-lg overflow-x-auto border">
							<pre className="text-xs font-mono">
								<code>{sampleContract}</code>
							</pre>
						</div>
					</div>
				</CardContent>
			</Card>

			{/* Deployment Methods */}
			<Card>
				<CardHeader>
					<CardTitle className="flex items-center gap-2 text-base">
						<Rocket className="h-5 w-5" />
						Deployment Methods
					</CardTitle>
					<CardDescription>Choose your preferred method to deploy contracts</CardDescription>
				</CardHeader>
				<CardContent>
					<Accordion type="single" collapsible className="w-full">
						{/* Hardhat Method */}
						<AccordionItem value="hardhat">
							<AccordionTrigger>
								<div className="flex items-center gap-2">
									<Package className="h-4 w-4" />
									<span>Using Hardhat</span>
									<Badge variant="secondary" className="ml-2">Recommended</Badge>
								</div>
							</AccordionTrigger>
							<AccordionContent className="space-y-4">
								<div className="space-y-4">
									<div>
										<h4 className="font-medium mb-2 flex items-center gap-2">
											<Badge variant="outline">Step 1</Badge>
											Initialize a Hardhat project
										</h4>
										<div className="bg-muted/50 dark:bg-slate-950 p-3 rounded-lg border">
											<div className="flex items-center justify-between">
												<code className="text-sm font-mono">npx hardhat init</code>
												<CopyButton textToCopy="npx hardhat init" label="" variant="ghost" size="sm" className="h-6 w-6 p-0" />
											</div>
										</div>
									</div>

									<div>
										<h4 className="font-medium mb-2 flex items-center gap-2">
											<Badge variant="outline">Step 2</Badge>
											Install dependencies
										</h4>
										<div className="bg-muted/50 dark:bg-slate-950 p-3 rounded-lg border">
											<div className="flex items-center justify-between">
												<code className="text-sm font-mono">npm install @nomicfoundation/hardhat-toolbox</code>
												<CopyButton textToCopy="npm install @nomicfoundation/hardhat-toolbox" label="" variant="ghost" size="sm" className="h-6 w-6 p-0" />
											</div>
										</div>
									</div>

									<div>
										<h4 className="font-medium mb-2 flex items-center gap-2">
											<Badge variant="outline">Step 3</Badge>
											Configure hardhat.config.js
										</h4>
										<div className="relative">
											<div className="absolute right-2 top-2">
												<CopyButton textToCopy={hardhatConfig} label="" variant="ghost" size="sm" className="h-8 w-8" />
											</div>
											<div className="bg-muted/50 dark:bg-slate-950 p-4 rounded-lg overflow-x-auto border">
												<pre className="text-xs font-mono">
													<code>{hardhatConfig}</code>
												</pre>
											</div>
										</div>
									</div>

									<div>
										<h4 className="font-medium mb-2 flex items-center gap-2">
											<Badge variant="outline">Step 4</Badge>
											Create deployment script (scripts/deploy.js)
										</h4>
										<div className="relative">
											<div className="absolute right-2 top-2">
												<CopyButton textToCopy={deployScript} label="" variant="ghost" size="sm" className="h-8 w-8" />
											</div>
											<div className="bg-muted/50 dark:bg-slate-950 p-4 rounded-lg overflow-x-auto border">
												<pre className="text-xs font-mono">
													<code>{deployScript}</code>
												</pre>
											</div>
										</div>
									</div>

									<div>
										<h4 className="font-medium mb-2 flex items-center gap-2">
											<Badge variant="outline">Step 5</Badge>
											Deploy to your Besu network
										</h4>
										<div className="bg-muted/50 dark:bg-slate-950 p-3 rounded-lg border">
											<div className="flex items-center justify-between">
												<code className="text-sm font-mono">npx hardhat run scripts/deploy.js --network besu</code>
												<CopyButton textToCopy="npx hardhat run scripts/deploy.js --network besu" label="" variant="ghost" size="sm" className="h-6 w-6 p-0" />
											</div>
										</div>
									</div>
								</div>
							</AccordionContent>
						</AccordionItem>

						{/* Foundry Method */}
						<AccordionItem value="foundry">
							<AccordionTrigger>
								<div className="flex items-center gap-2">
									<Terminal className="h-4 w-4" />
									<span>Using Foundry</span>
									<Badge variant="outline" className="ml-2">Advanced</Badge>
								</div>
							</AccordionTrigger>
							<AccordionContent className="space-y-4">
								<div className="space-y-4">
									<div>
										<h4 className="font-medium mb-2 flex items-center gap-2">
											<Badge variant="outline">Step 1</Badge>
											Install Foundry
										</h4>
										<div className="bg-muted/50 dark:bg-slate-950 p-3 rounded-lg border">
											<div className="flex items-center justify-between">
												<code className="text-sm font-mono">curl -L https://foundry.paradigm.xyz | bash && foundryup</code>
												<CopyButton textToCopy="curl -L https://foundry.paradigm.xyz | bash && foundryup" label="" variant="ghost" size="sm" className="h-6 w-6 p-0" />
											</div>
										</div>
									</div>

									<div>
										<h4 className="font-medium mb-2 flex items-center gap-2">
											<Badge variant="outline">Step 2</Badge>
											Initialize a Foundry project
										</h4>
										<div className="bg-muted/50 dark:bg-slate-950 p-3 rounded-lg border">
											<div className="flex items-center justify-between">
												<code className="text-sm font-mono">forge init my-project && cd my-project</code>
												<CopyButton textToCopy="forge init my-project && cd my-project" label="" variant="ghost" size="sm" className="h-6 w-6 p-0" />
											</div>
										</div>
									</div>

									<div>
										<h4 className="font-medium mb-2 flex items-center gap-2">
											<Badge variant="outline">Step 3</Badge>
											Configure foundry.toml
										</h4>
										<div className="relative">
											<div className="absolute right-2 top-2">
												<CopyButton textToCopy={foundryConfig} label="" variant="ghost" size="sm" className="h-8 w-8" />
											</div>
											<div className="bg-muted/50 dark:bg-slate-950 p-4 rounded-lg overflow-x-auto border">
												<pre className="text-xs font-mono">
													<code>{foundryConfig}</code>
												</pre>
											</div>
										</div>
									</div>

									<div>
										<h4 className="font-medium mb-2 flex items-center gap-2">
											<Badge variant="outline">Step 4</Badge>
											Create deployment script (script/Deploy.s.sol)
										</h4>
										<div className="relative">
											<div className="absolute right-2 top-2">
												<CopyButton textToCopy={foundryDeployScript} label="" variant="ghost" size="sm" className="h-8 w-8" />
											</div>
											<div className="bg-muted/50 dark:bg-slate-950 p-4 rounded-lg overflow-x-auto border">
												<pre className="text-xs font-mono">
													<code>{foundryDeployScript}</code>
												</pre>
											</div>
										</div>
									</div>

									<div>
										<h4 className="font-medium mb-2 flex items-center gap-2">
											<Badge variant="outline">Step 5</Badge>
											Deploy using Forge
										</h4>
										<div className="bg-muted/50 dark:bg-slate-950 p-3 rounded-lg border">
											<div className="flex items-center justify-between">
												<code className="text-sm font-mono text-wrap break-all">forge script script/Deploy.s.sol --rpc-url besu --broadcast --private-key YOUR_PRIVATE_KEY</code>
												<CopyButton textToCopy={`forge script script/Deploy.s.sol --rpc-url ${rpcEndpoint || 'http://localhost:8545'} --broadcast --private-key YOUR_PRIVATE_KEY`} label="" variant="ghost" size="sm" className="h-6 w-6 p-0 flex-shrink-0" />
											</div>
										</div>
									</div>
								</div>
							</AccordionContent>
						</AccordionItem>

						{/* Remix Method */}
						<AccordionItem value="remix">
							<AccordionTrigger>
								<div className="flex items-center gap-2">
									<Code className="h-4 w-4" />
									<span>Using Remix IDE</span>
									<Badge variant="outline" className="ml-2">Beginner-friendly</Badge>
								</div>
							</AccordionTrigger>
							<AccordionContent className="space-y-4">
								<Alert>
									<AlertCircle className="h-4 w-4" />
									<AlertDescription>
										Remix IDE is a web-based development environment. No installation required.
									</AlertDescription>
								</Alert>

								<div className="relative">
									<div className="absolute right-2 top-2">
										<CopyButton textToCopy={remixSteps} label="" variant="ghost" size="sm" className="h-8 w-8" />
									</div>
									<div className="bg-muted/50 dark:bg-slate-950 p-4 rounded-lg border">
										<ol className="space-y-2 text-sm">
											<li className="flex items-start gap-2">
												<Badge variant="outline" className="mt-0.5 flex-shrink-0">1</Badge>
												<span>Open <a href="https://remix.ethereum.org" target="_blank" rel="noopener noreferrer" className="text-primary hover:underline inline-flex items-center gap-1">Remix IDE <ExternalLink className="h-3 w-3" /></a></span>
											</li>
											<li className="flex items-start gap-2">
												<Badge variant="outline" className="mt-0.5 flex-shrink-0">2</Badge>
												<span>Create a new file: <code className="bg-muted px-1 rounded">SimpleStorage.sol</code></span>
											</li>
											<li className="flex items-start gap-2">
												<Badge variant="outline" className="mt-0.5 flex-shrink-0">3</Badge>
												<span>Paste your contract code</span>
											</li>
											<li className="flex items-start gap-2">
												<Badge variant="outline" className="mt-0.5 flex-shrink-0">4</Badge>
												<span>Compile using Solidity Compiler (Ctrl+S)</span>
											</li>
											<li className="flex items-start gap-2">
												<Badge variant="outline" className="mt-0.5 flex-shrink-0">5</Badge>
												<span>Go to "Deploy & Run Transactions" tab</span>
											</li>
											<li className="flex items-start gap-2">
												<Badge variant="outline" className="mt-0.5 flex-shrink-0">6</Badge>
												<span>Select "Custom - External HTTP Provider" from Environment dropdown</span>
											</li>
											<li className="flex items-start gap-2">
												<Badge variant="outline" className="mt-0.5 flex-shrink-0">7</Badge>
												<span>Enter your RPC endpoint: <code className="bg-muted px-1 rounded">{rpcEndpoint || 'http://localhost:8545'}</code></span>
											</li>
											<li className="flex items-start gap-2">
												<Badge variant="outline" className="mt-0.5 flex-shrink-0">8</Badge>
												<span>Click "Deploy" button</span>
											</li>
										</ol>
									</div>
								</div>
							</AccordionContent>
						</AccordionItem>

						{/* Web3.js/Ethers.js Method */}
						<AccordionItem value="web3">
							<AccordionTrigger>
								<div className="flex items-center gap-2">
									<Wallet className="h-4 w-4" />
									<span>Using Web3.js / Ethers.js</span>
									<Badge variant="outline" className="ml-2">Programmatic</Badge>
								</div>
							</AccordionTrigger>
							<AccordionContent className="space-y-4">
								<div className="space-y-4">
									<div>
										<h4 className="font-medium mb-2">Using Ethers.js v6</h4>
										<div className="relative">
											<div className="absolute right-2 top-2">
												<CopyButton textToCopy={`const { ethers } = require("ethers");

// Connect to your Besu network
const provider = new ethers.JsonRpcProvider("${rpcEndpoint || 'http://localhost:8545'}");

// Create a wallet (replace with your private key)
const wallet = new ethers.Wallet("YOUR_PRIVATE_KEY", provider);

// Contract ABI and bytecode (from compilation)
const abi = [...]; // Your contract ABI
const bytecode = "0x..."; // Your contract bytecode

async function deploy() {
  const factory = new ethers.ContractFactory(abi, bytecode, wallet);
  const contract = await factory.deploy();
  await contract.waitForDeployment();

  console.log("Contract deployed at:", await contract.getAddress());
}

deploy();`} label="" variant="ghost" size="sm" className="h-8 w-8" />
											</div>
											<div className="bg-muted/50 dark:bg-slate-950 p-4 rounded-lg overflow-x-auto border">
												<pre className="text-xs font-mono">
													<code>{`const { ethers } = require("ethers");

// Connect to your Besu network
const provider = new ethers.JsonRpcProvider("${rpcEndpoint || 'http://localhost:8545'}");

// Create a wallet (replace with your private key)
const wallet = new ethers.Wallet("YOUR_PRIVATE_KEY", provider);

// Contract ABI and bytecode (from compilation)
const abi = [...]; // Your contract ABI
const bytecode = "0x..."; // Your contract bytecode

async function deploy() {
  const factory = new ethers.ContractFactory(abi, bytecode, wallet);
  const contract = await factory.deploy();
  await contract.waitForDeployment();

  console.log("Contract deployed at:", await contract.getAddress());
}

deploy();`}</code>
												</pre>
											</div>
										</div>
									</div>
								</div>
							</AccordionContent>
						</AccordionItem>
					</Accordion>
				</CardContent>
			</Card>

			{/* Tips & Best Practices */}
			<Card>
				<CardHeader>
					<CardTitle className="flex items-center gap-2 text-base">
						<AlertCircle className="h-5 w-5 text-amber-500" />
						Tips & Best Practices
					</CardTitle>
				</CardHeader>
				<CardContent>
					<ul className="space-y-3 text-sm">
						<li className="flex items-start gap-2">
							<CheckCircle2 className="h-4 w-4 text-green-500 mt-0.5 flex-shrink-0" />
							<span><strong>Test first:</strong> Always deploy to a test network before deploying to production.</span>
						</li>
						<li className="flex items-start gap-2">
							<CheckCircle2 className="h-4 w-4 text-green-500 mt-0.5 flex-shrink-0" />
							<span><strong>Secure your keys:</strong> Never commit private keys to version control. Use environment variables or secure key management.</span>
						</li>
						<li className="flex items-start gap-2">
							<CheckCircle2 className="h-4 w-4 text-green-500 mt-0.5 flex-shrink-0" />
							<span><strong>Verify your contract:</strong> After deployment, verify the contract source code for transparency.</span>
						</li>
						<li className="flex items-start gap-2">
							<CheckCircle2 className="h-4 w-4 text-green-500 mt-0.5 flex-shrink-0" />
							<span><strong>Save the address:</strong> Record the deployed contract address - you'll need it to interact with the contract.</span>
						</li>
						<li className="flex items-start gap-2">
							<CheckCircle2 className="h-4 w-4 text-green-500 mt-0.5 flex-shrink-0" />
							<span><strong>Gas estimation:</strong> For permissioned networks, gas limits may differ from public networks.</span>
						</li>
					</ul>
				</CardContent>
			</Card>
		</div>
	)
}

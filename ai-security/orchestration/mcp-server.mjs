#!/usr/bin/env node
/**
 * BETTERGOV AI Security — MCP Server (JSON-RPC 2.0 over stdio)
 * Inspired by T3MP3ST (src/mcp-server.ts)
 *
 * Exposes read-only defensive audit tools to MCP-compatible coding agents.
 * Strictly validates targets and never spawns shell commands.
 */

import readline from 'node:readline';
import { SecurityCoordinatorAgent } from './coordinator.mjs';

const TARGET_RE = /^[A-Za-z0-9._:-]+$/;
const coordinator = new SecurityCoordinatorAgent();

const MCP_TOOLS = [
  {
    name: 'bettergov_security_audit',
    description: 'Run the 9-agent BETTERGOV defensive AI security sweep and return tool-verified findings.',
    inputSchema: {
      type: 'object',
      properties: {
        target: { type: 'string', description: 'Target chain ID or contract identifier ([A-Za-z0-9._:-])' },
      },
      required: ['target'],
    },
  },
];

export function handleMcpRequest(request) {
  if (request.method === 'tools/list') {
    return { jsonrpc: '2.0', id: request.id, result: { tools: MCP_TOOLS } };
  }
  if (request.method === 'tools/call') {
    const { name, arguments: args = {} } = request.params || {};
    if (name !== 'bettergov_security_audit') {
      return { jsonrpc: '2.0', id: request.id, error: { code: -32601, message: `Unknown tool: ${name}` } };
    }
    const target = args.target;
    if (typeof target !== 'string' || !TARGET_RE.test(target) || target.includes('\0')) {
      return {
        jsonrpc: '2.0',
        id: request.id,
        result: {
          content: [{ type: 'text', text: JSON.stringify({ error: 'Invalid target format; rejected by TARGET_RE guard' }) }],
          isError: true,
        },
      };
    }
    const auditResult = coordinator.runFullAuditMission({ missionName: `MCP Audit: ${target}` });
    return {
      jsonrpc: '2.0',
      id: request.id,
      result: {
        content: [{ type: 'text', text: JSON.stringify(auditResult.report, null, 2) }],
      },
    };
  }
  return { jsonrpc: '2.0', id: request.id, error: { code: -32601, message: 'Method not supported' } };
}

if (process.argv[1] && process.argv[1].endsWith('mcp-server.mjs')) {
  const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: false });
  rl.on('line', (line) => {
    if (!line.trim()) return;
    try {
      const req = JSON.parse(line);
      const res = handleMcpRequest(req);
      process.stdout.write(JSON.stringify(res) + '\n');
    } catch {
      process.stdout.write(JSON.stringify({ jsonrpc: '2.0', id: null, error: { code: -32700, message: 'Parse error' } }) + '\n');
    }
  });
}


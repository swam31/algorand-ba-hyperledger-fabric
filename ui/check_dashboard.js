const WebSocket = require('ws');

const ws = new WebSocket('ws://localhost:3001');

let received = false;

ws.on('message', function incoming(data) {
  if (received) return;
  received = true;
  const state = JSON.parse(data);
  console.log("=== DASHBOARD STATE ===");
  console.log("Orderers:", Object.keys(state.orderers).length);
  for (const [id, o] of Object.entries(state.orderers)) {
    console.log(`  ${id}: ${o.up ? 'UP' : 'DOWN'} (nodeId: ${o.nodeId})`);
  }
  
  console.log("Rounds count:", state.rounds.length);
  if (state.rounds.length > 0) {
    const lastRounds = state.rounds.slice(0, 3);
    console.log("Last 3 Rounds:");
    for (const r of lastRounds) {
        console.log(`  Round ${r.round} Steps ${r.steps} by ${r.nodeId} (outcome: ${r.outcome})`);
    }
  }

  console.log("Blocks count:", state.blocks.length);
  if (state.blocks.length > 0) {
    const lastBlocks = state.blocks.slice(0, 3);
    console.log("Last 3 Blocks:");
    for (const b of lastBlocks) {
        console.log(`  Block ${b.number}: tx count ${b.txCount}, tx1: ${b.txCount > 0 ? b.txs[0].id : 'none'}`);
    }
  }

  process.exit(0);
});

ws.on('error', function error(err) {
  console.error("WS Error:", err);
  process.exit(1);
});

setTimeout(() => {
    if (!received) {
        console.error("Timeout: No data received over WS after 5 seconds");
        process.exit(1);
    }
}, 5000);

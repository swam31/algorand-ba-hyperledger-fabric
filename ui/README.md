# Dashboard

`node server.js` serves the dashboard at http://localhost:3001. Start the network
and deploy the chaincodes first (see the main [README](../README.md)).

How it gets its data:

- **Orderer telemetry** (ports 7070-7073): `/status` is polled every second.
  The `/events` (round outcomes) and `/votes` (every vote sent and received)
  streams are replayed on connect and re-opened after an orderer restarts.
- **The ledger**, through the Fabric Gateway on peer0.org1. `history.js` streams
  every block from block 0 and decodes it: round metadata, transactions,
  validation codes and payments. From that it rebuilds the round history and
  account balances. Certificates are re-read on each reconnect, because the
  test network regenerates them.
- **Benchmark results** from `bench/*.json`.

Pages in `public/`: `app.js` (health, rounds, blocks), `flow.js` (live votes),
`rounds.js` (history, accounts), `bench.js`, `compare.js`, `scale.js`.

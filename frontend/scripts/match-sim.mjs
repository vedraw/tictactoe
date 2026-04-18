/**
 * Live Nakama simulation: two device-authenticated sockets exercise match corner cases.
 * Run from repo: `cd frontend && node scripts/match-sim.mjs`
 * Requires Nakama on NAKAMA_HOST:NAKAMA_PORT (default 127.0.0.1:7350).
 *
 * Env:
 *   RUN_TIMED_FORFEIT=1 — run timed-mode timeout scenario (~35s extra). Default: skip.
 */
import { WebSocket as NodeWebSocket } from "ws";
import { Client } from "@heroiclabs/nakama-js";

if (typeof globalThis.WebSocket === "undefined") {
  globalThis.WebSocket = NodeWebSocket;
}

const OP_STATE = 2;
const OP_ERROR = 3;
const OP_MOVE = 1;

const host = process.env.NAKAMA_HOST || "127.0.0.1";
const port = process.env.NAKAMA_PORT || "7350";
const serverKey = process.env.NAKAMA_SERVER_KEY || "defaultkey";
const runTimedForfeit = process.env.RUN_TIMED_FORFEIT === "1";

function randSuffix() {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

function createPlayer(tag) {
  const deviceId = `sim-${tag}-${randSuffix()}`;
  const client = new Client(serverKey, host, port, false);
  client.timeout = 15000;
  let session;
  let socket;
  const states = [];
  const errors = [];
  const systems = [];
  let displayUsername = "";

  return {
    tag,
    getUsername: () => displayUsername,
    userId: () => session?.user_id,
    latestState() {
      return states.length ? states[states.length - 1] : null;
    },
    stateCount() {
      return states.length;
    },
    lastError() {
      return errors.length ? errors[errors.length - 1] : null;
    },
    lastSystem() {
      return systems.length ? systems[systems.length - 1] : null;
    },
    errorCount() {
      return errors.length;
    },
    async connect(username) {
      displayUsername = username;
      session = await client.authenticateDevice(deviceId, true, username);
      socket = client.createSocket(false, false);
      socket.onmatchdata = (md) => {
        const text = new TextDecoder().decode(md.data);
        try {
          const j = JSON.parse(text);
          if (md.op_code === OP_STATE) states.push(j);
          else if (md.op_code === OP_ERROR) errors.push(j);
          else if (md.op_code === 4) systems.push(j);
        } catch {
          /* ignore */
        }
      };
      await socket.connect(session, true);
    },
    async rpcEnvelope(id, input) {
      const r = await client.rpc(session, id, input);
      return r.payload;
    },
    async joinMatch(matchId) {
      await socket.joinMatch(matchId);
    },
    async leaveMatch(matchId) {
      await socket.leaveMatch(matchId);
    },
    sendMove(matchId, position) {
      return socket.sendMatchState(matchId, OP_MOVE, JSON.stringify({ position }));
    },
    sendRaw(matchId, body) {
      return socket.sendMatchState(matchId, OP_MOVE, body);
    },
    sendOpcode(matchId, opCode, body) {
      return socket.sendMatchState(matchId, opCode, body);
    },
    shutdown() {
      try {
        socket?.disconnect(false);
      } catch {
        /* */
      }
    },
  };
}

async function rpcData(player, id, input) {
  const env = await player.rpcEnvelope(id, input);
  if (!env || env.ok !== true) {
    throw new Error(`${id} RPC failed: ${JSON.stringify(env)}`);
  }
  return env.data;
}

async function waitState(player, pred, ms = 8000) {
  const t0 = Date.now();
  while (Date.now() - t0 < ms) {
    const s = player.latestState();
    if (s && pred(s)) return s;
    await new Promise((r) => setTimeout(r, 40));
  }
  throw new Error(`${player.tag}: timeout waiting for state, last=${JSON.stringify(player.latestState())}`);
}

async function waitErrorCount(player, min, ms = 5000) {
  const t0 = Date.now();
  while (Date.now() - t0 < ms) {
    if (player.errorCount() >= min) return;
    await new Promise((r) => setTimeout(r, 40));
  }
  throw new Error(`${player.tag}: expected >=${min} errors, got ${player.errorCount()}`);
}

function assert(cond, msg) {
  if (!cond) throw new Error(msg);
}

async function rpcAny(player, id, input) {
  return player.rpcEnvelope(id, input);
}

function winsOnLeaderboard(lbData, username) {
  const rows = lbData.leaderboard || [];
  const row = rows.find((r) => r.username === username);
  return row ? Number(row.wins || 0) : 0;
}

async function scenario(name, fn) {
  process.stdout.write(`\n== ${name} ==\n`);
  await fn();
  console.log(`OK   ${name}`);
}

async function main() {
  console.log(`Nakama simulation → ws://${host}:${port} (server_key=${serverKey})`);

  await scenario("two players activate match", async () => {
    const a = createPlayer("A");
    const b = createPlayer("B");
    try {
      await a.connect(`SimA-${randSuffix()}`);
      await b.connect(`SimB-${randSuffix()}`);
      const { matchId } = await rpcData(a, "create_match", { mode: "classic" });
      await a.joinMatch(matchId);
      await b.joinMatch(matchId);
      await waitState(a, (s) => s.status === "active" && s.players?.length === 2);
      const s = a.latestState();
      assert(s.turnUserId && s.playerSymbols, "missing turn/symbols");
    } finally {
      a.shutdown();
      b.shutdown();
    }
  });

  await scenario("NOT_YOUR_TURN", async () => {
    const a = createPlayer("A");
    const b = createPlayer("B");
    try {
      await a.connect(`SimA-${randSuffix()}`);
      await b.connect(`SimB-${randSuffix()}`);
      const { matchId } = await rpcData(a, "create_match", { mode: "classic" });
      await a.joinMatch(matchId);
      await b.joinMatch(matchId);
      await waitState(a, (s) => s.status === "active");
      const turn = a.latestState().turnUserId;
      const offender = turn === a.userId() ? b : a;
      const nErr = offender.errorCount();
      await offender.sendMove(matchId, 0);
      await waitErrorCount(offender, nErr + 1);
      const err = offender.lastError()?.error;
      assert(err?.code === "NOT_YOUR_TURN", `want NOT_YOUR_TURN got ${JSON.stringify(err)}`);
    } finally {
      a.shutdown();
      b.shutdown();
    }
  });

  await scenario("INVALID_PAYLOAD (move opcode)", async () => {
    const a = createPlayer("A");
    const b = createPlayer("B");
    try {
      await a.connect(`SimA-${randSuffix()}`);
      await b.connect(`SimB-${randSuffix()}`);
      const { matchId } = await rpcData(a, "create_match", { mode: "classic" });
      await a.joinMatch(matchId);
      await b.joinMatch(matchId);
      await waitState(a, (s) => s.status === "active");
      const turn = a.latestState().turnUserId;
      const p = turn === a.userId() ? a : b;
      const n = p.errorCount();
      await p.sendRaw(matchId, "{");
      await waitErrorCount(p, n + 1);
      assert(p.lastError()?.error?.code === "INVALID_PAYLOAD", JSON.stringify(p.lastError()));
    } finally {
      a.shutdown();
      b.shutdown();
    }
  });

  await scenario("INVALID_POSITION", async () => {
    const a = createPlayer("A");
    const b = createPlayer("B");
    try {
      await a.connect(`SimA-${randSuffix()}`);
      await b.connect(`SimB-${randSuffix()}`);
      const { matchId } = await rpcData(a, "create_match", { mode: "classic" });
      await a.joinMatch(matchId);
      await b.joinMatch(matchId);
      await waitState(a, (s) => s.status === "active");
      const turn = a.latestState().turnUserId;
      const p = turn === a.userId() ? a : b;
      const n = p.errorCount();
      await p.sendMove(matchId, 99);
      await waitErrorCount(p, n + 1);
      assert(p.lastError()?.error?.code === "INVALID_POSITION", JSON.stringify(p.lastError()));
    } finally {
      a.shutdown();
      b.shutdown();
    }
  });

  await scenario("CELL_OCCUPIED", async () => {
    const a = createPlayer("A");
    const b = createPlayer("B");
    try {
      await a.connect(`SimA-${randSuffix()}`);
      await b.connect(`SimB-${randSuffix()}`);
      const { matchId } = await rpcData(a, "create_match", { mode: "classic" });
      await a.joinMatch(matchId);
      await b.joinMatch(matchId);
      await waitState(a, (s) => s.status === "active");
      // First joiner is X; wait for server state after each move so we never stack two moves in one tick.
      await a.sendMove(matchId, 0);
      await waitState(a, (s) => s.moveCount >= 1 && s.board[0] !== "");
      await b.sendMove(matchId, 1);
      await waitState(a, (s) => s.moveCount >= 2);
      const n = a.errorCount();
      await a.sendMove(matchId, 0);
      await waitErrorCount(a, n + 1);
      assert(a.lastError()?.error?.code === "CELL_OCCUPIED", JSON.stringify(a.lastError()));
    } finally {
      a.shutdown();
      b.shutdown();
    }
  });

  await scenario("win row (first player X)", async () => {
    const a = createPlayer("A");
    const b = createPlayer("B");
    try {
      await a.connect(`SimA-${randSuffix()}`);
      await b.connect(`SimB-${randSuffix()}`);
      const { matchId } = await rpcData(a, "create_match", { mode: "classic" });
      await a.joinMatch(matchId);
      await b.joinMatch(matchId);
      await waitState(a, (s) => s.status === "active");
      // A joins first → X; top row win 0-1-2 while O plays 3,4.
      const plan = [
        [a, 0],
        [b, 3],
        [a, 1],
        [b, 4],
        [a, 2],
      ];
      let expectCount = 0;
      for (const [p, cell] of plan) {
        await waitState(a, (s) => s.turnUserId === p.userId());
        await p.sendMove(matchId, cell);
        expectCount += 1;
        await waitState(a, (s) => s.moveCount >= expectCount);
      }
      await waitState(a, (s) => s.status === "finished" && s.winner === a.userId(), 8000);
    } finally {
      a.shutdown();
      b.shutdown();
    }
  });

  await scenario("draw (9 moves, no winner)", async () => {
    const a = createPlayer("A");
    const b = createPlayer("B");
    try {
      await a.connect(`SimA-${randSuffix()}`);
      await b.connect(`SimB-${randSuffix()}`);
      const { matchId } = await rpcData(a, "create_match", { mode: "classic" });
      await a.joinMatch(matchId);
      await b.joinMatch(matchId);
      await waitState(a, (s) => s.status === "active");
      // Cat-game style cell order; follow server turn each step.
      const cells = [0, 1, 2, 4, 3, 5, 7, 6, 8];
      let expectCount = 0;
      for (const cell of cells) {
        const s = a.latestState();
        if (s.status !== "active") break;
        const p = s.turnUserId === a.userId() ? a : b;
        await p.sendMove(matchId, cell);
        expectCount += 1;
        await waitState(a, (st) => st.moveCount >= expectCount);
      }
      await waitState(
        a,
        (s) => s.status === "finished" && !s.winner && s.moveCount === 9,
        12000
      );
    } finally {
      a.shutdown();
      b.shutdown();
    }
  });

  await scenario("waiting room abandoned on sole leave", async () => {
    const a = createPlayer("A");
    try {
      await a.connect(`SimA-${randSuffix()}`);
      const { matchId } = await rpcData(a, "create_match", { mode: "classic" });
      await a.joinMatch(matchId);
      await waitState(a, (s) => s.status === "waiting" && (s.players?.length || 0) >= 1);
      await a.leaveMatch(matchId);
      // Leaver may not receive a final realtime state; list_matches excludes abandoned labels.
      await new Promise((r) => setTimeout(r, 600));
      const listed = await rpcData(a, "list_matches", { mode: "classic" });
      const ids = (listed.matches || []).map((m) => m.match_id || m.matchId);
      assert(!ids.includes(matchId), `abandoned match should not appear in list_matches: ${ids}`);
    } finally {
      a.shutdown();
    }
  });

  await scenario("disconnect win (active, other leaves)", async () => {
    const a = createPlayer("A");
    const b = createPlayer("B");
    try {
      await a.connect(`SimA-${randSuffix()}`);
      await b.connect(`SimB-${randSuffix()}`);
      const { matchId } = await rpcData(a, "create_match", { mode: "classic" });
      await a.joinMatch(matchId);
      await b.joinMatch(matchId);
      await waitState(a, (s) => s.status === "active");
      await b.leaveMatch(matchId);
      await waitState(a, (s) => s.status === "finished" && s.winner === a.userId(), 8000);
    } finally {
      a.shutdown();
      b.shutdown();
    }
  });

  await scenario("find_match pairs two players (same match id)", async () => {
    const a = createPlayer("A");
    const b = createPlayer("B");
    try {
      await a.connect(`SimA-${randSuffix()}`);
      await b.connect(`SimB-${randSuffix()}`);
      const r1 = await rpcAny(a, "find_match", { mode: "classic" });
      assert(r1.ok === true && r1.data?.matchId, `find A: ${JSON.stringify(r1)}`);
      const mid = r1.data.matchId;
      await new Promise((r) => setTimeout(r, 250));
      const r2 = await rpcAny(b, "find_match", { mode: "classic" });
      assert(r2.ok === true && r2.data?.matchId === mid, `find B same room: ${JSON.stringify(r2)}`);
      // When a waiting room already exists from earlier tests, both calls may show created=false while still pairing.
    } finally {
      a.shutdown();
      b.shutdown();
    }
  });

  await scenario("leaderboard records win for winner", async () => {
    const a = createPlayer("A");
    const b = createPlayer("B");
    try {
      await a.connect(`SimA-${randSuffix()}`);
      await b.connect(`SimB-${randSuffix()}`);
      const lb0 = await rpcData(a, "list_leaderboard", {});
      const w0 = winsOnLeaderboard(lb0, a.getUsername());
      const { matchId } = await rpcData(a, "create_match", { mode: "classic" });
      await a.joinMatch(matchId);
      await b.joinMatch(matchId);
      await waitState(a, (s) => s.status === "active");
      const plan = [
        [a, 0],
        [b, 3],
        [a, 1],
        [b, 4],
        [a, 2],
      ];
      let expectCount = 0;
      for (const [p, cell] of plan) {
        await waitState(a, (s) => s.turnUserId === p.userId());
        await p.sendMove(matchId, cell);
        expectCount += 1;
        await waitState(a, (s) => s.moveCount >= expectCount);
      }
      await waitState(a, (s) => s.status === "finished" && s.winner === a.userId(), 8000);
      await new Promise((r) => setTimeout(r, 800));
      const lb1 = await rpcData(a, "list_leaderboard", {});
      const w1 = winsOnLeaderboard(lb1, a.getUsername());
      assert(w1 >= w0 + 1, `leaderboard wins want >=${w0 + 1} got ${w1}`);
    } finally {
      a.shutdown();
      b.shutdown();
    }
  });

  await scenario("few STATE broadcasts while idle after finished", async () => {
    const a = createPlayer("A");
    const b = createPlayer("B");
    try {
      await a.connect(`SimA-${randSuffix()}`);
      await b.connect(`SimB-${randSuffix()}`);
      const { matchId } = await rpcData(a, "create_match", { mode: "classic" });
      await a.joinMatch(matchId);
      await b.joinMatch(matchId);
      await waitState(a, (s) => s.status === "active");
      const plan = [
        [a, 0],
        [b, 3],
        [a, 1],
        [b, 4],
        [a, 2],
      ];
      let n = 0;
      for (const [p, cell] of plan) {
        await waitState(a, (s) => s.turnUserId === p.userId());
        await p.sendMove(matchId, cell);
        n += 1;
        await waitState(a, (s) => s.moveCount >= n);
      }
      await waitState(a, (s) => s.status === "finished");
      const c0 = a.stateCount();
      await new Promise((r) => setTimeout(r, 2000));
      const delta = a.stateCount() - c0;
      assert(delta <= 6, `expected bounded idle STATE updates, got +${delta}`);
    } finally {
      a.shutdown();
      b.shutdown();
    }
  });

  await scenario("two concurrent matches stay isolated", async () => {
    const p0a = createPlayer("M0A");
    const p0b = createPlayer("M0B");
    const p1a = createPlayer("M1A");
    const p1b = createPlayer("M1B");
    try {
      await p0a.connect(`U0a-${randSuffix()}`);
      await p0b.connect(`U0b-${randSuffix()}`);
      await p1a.connect(`U1a-${randSuffix()}`);
      await p1b.connect(`U1b-${randSuffix()}`);
      const m0 = await rpcData(p0a, "create_match", { mode: "classic" });
      const m1 = await rpcData(p1a, "create_match", { mode: "classic" });
      assert(m0.matchId !== m1.matchId, "distinct matches");
      await p0a.joinMatch(m0.matchId);
      await p0b.joinMatch(m0.matchId);
      await p1a.joinMatch(m1.matchId);
      await p1b.joinMatch(m1.matchId);
      await waitState(p0a, (s) => s.status === "active");
      await waitState(p1a, (s) => s.status === "active");
      await p0a.sendMove(m0.matchId, 0);
      await p1a.sendMove(m1.matchId, 8);
      await waitState(p0a, (s) => s.board[0] === "X");
      await waitState(p1a, (s) => s.board[8] === "X");
      assert(p0a.latestState().board[8] === "", "match0 cell 8 untouched");
      assert(p1a.latestState().board[0] === "", "match1 cell 0 untouched");
    } finally {
      p0a.shutdown();
      p0b.shutdown();
      p1a.shutdown();
      p1b.shutdown();
    }
  });

  await scenario("non-move opcode does not change board", async () => {
    const a = createPlayer("A");
    const b = createPlayer("B");
    try {
      await a.connect(`SimA-${randSuffix()}`);
      await b.connect(`SimB-${randSuffix()}`);
      const { matchId } = await rpcData(a, "create_match", { mode: "classic" });
      await a.joinMatch(matchId);
      await b.joinMatch(matchId);
      await waitState(a, (s) => s.status === "active");
      const snap = JSON.stringify(a.latestState().board);
      await a.sendOpcode(matchId, 99, JSON.stringify({ position: 0 }));
      await new Promise((r) => setTimeout(r, 500));
      assert(JSON.stringify(a.latestState().board) === snap, "board mutated on opcode 99");
    } finally {
      a.shutdown();
      b.shutdown();
    }
  });

  await scenario("RATE_LIMIT_RPC (session create_match x11)", async () => {
    const a = createPlayer("A");
    try {
      await a.connect(`SimA-${randSuffix()}`);
      for (let i = 0; i < 10; i++) {
        const e = await rpcAny(a, "create_match", { mode: "classic" });
        assert(e.ok === true && e.data?.matchId, `call ${i + 1}: ${JSON.stringify(e)}`);
      }
      const last = await rpcAny(a, "create_match", { mode: "classic" });
      assert(last.ok === false && last.error?.code === "RATE_LIMIT_RPC", JSON.stringify(last));
    } finally {
      a.shutdown();
    }
  });

  await scenario("RATE_LIMIT_MOVE (two moves same tick)", async () => {
    const a = createPlayer("A");
    const b = createPlayer("B");
    try {
      await a.connect(`SimA-${randSuffix()}`);
      await b.connect(`SimB-${randSuffix()}`);
      const { matchId } = await rpcData(a, "create_match", { mode: "classic" });
      await a.joinMatch(matchId);
      await b.joinMatch(matchId);
      await waitState(a, (s) => s.status === "active");
      const turn = a.latestState().turnUserId;
      const p = turn === a.userId() ? a : b;
      const body = JSON.stringify({ position: 4 });
      const n = p.errorCount();
      await Promise.all([p.sendRaw(matchId, body), p.sendRaw(matchId, body)]);
      await waitErrorCount(p, n + 1, 6000);
      assert(p.lastError()?.error?.code === "RATE_LIMIT_MOVE", JSON.stringify(p.lastError()));
    } finally {
      a.shutdown();
      b.shutdown();
    }
  });

  if (runTimedForfeit) {
    await scenario("timed forfeit when turn idles past deadline", async () => {
      const a = createPlayer("A");
      const b = createPlayer("B");
      try {
        await a.connect(`SimA-${randSuffix()}`);
        await b.connect(`SimB-${randSuffix()}`);
        const { matchId } = await rpcData(a, "create_match", { mode: "timed" });
        await a.joinMatch(matchId);
        await b.joinMatch(matchId);
        await waitState(a, (s) => s.status === "active");
        const s0 = a.latestState();
        assert(s0.turnDurationSec === 30 && s0.turnDeadlineSec > 0, JSON.stringify(s0));
        const stuck = s0.turnUserId;
        console.log("   …sleeping ~32s for server turn deadline…");
        await new Promise((r) => setTimeout(r, 32_000));
        await waitState(
          a,
          (s) => s.status === "finished" && s.winner && s.winner !== stuck,
          25_000
        );
        const sysA = a.lastSystem();
        const sysB = b.lastSystem();
        const ok =
          (sysA?.message && String(sysA.message).toLowerCase().includes("time")) ||
          (sysB?.message && String(sysB.message).toLowerCase().includes("time"));
        assert(ok, `expected timeout system line, got A=${JSON.stringify(sysA)} B=${JSON.stringify(sysB)}`);
      } finally {
        a.shutdown();
        b.shutdown();
      }
    });
  } else {
    console.log("\n(skip timed forfeit — set RUN_TIMED_FORFEIT=1 for ~35s deadline test)\n");
  }

  console.log("\n=== all live simulations passed ===\n");
}

main().catch((e) => {
  console.error("\nSIM FAILED:", e.message || e);
  process.exit(1);
});

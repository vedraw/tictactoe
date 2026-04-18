import { useEffect, useMemo, useRef, useState } from "react";
import { Client } from "@heroiclabs/nakama-js";
import { friendlyMatchErrorCode, friendlyRpcErrorCode } from "./errorCopy.js";

var client = new Client(
  import.meta.env.VITE_NAKAMA_SERVER_KEY || "defaultkey",
  import.meta.env.VITE_NAKAMA_HOST || "127.0.0.1",
  import.meta.env.VITE_NAKAMA_PORT || "7350",
  (import.meta.env.VITE_NAKAMA_SSL || "false") === "true"
);
client.timeout = 10000;

var MOVE_OPCODE = 1;
var STATE_OPCODE = 2;
var ERROR_OPCODE = 3;
var SYSTEM_OPCODE = 4;

/** UI-only: matches server `turnDeadlineSec` display; low-time accent threshold (seconds). */
var TIMED_LOW_TIME_SEC = 5;

function getDeviceId() {
  var key = "ttt_device_id";
  var existing = window.localStorage.getItem(key);
  if (existing) return existing;
  var value = "web-" + crypto.randomUUID();
  window.localStorage.setItem(key, value);
  return value;
}

/** Go RPCs return `{ ok, data }` or `{ ok, error }`. Nakama client already JSON-parses `payload`. */
function unwrapRpcPayload(payload) {
  if (payload == null || typeof payload !== "object") return {};
  if ("ok" in payload) {
    if (!payload.ok) {
      var err = payload.error || {};
      var codeStr = err.code != null ? String(err.code) : "";
      var msg = err.message || "Request failed.";
      var details = err.details || {};
      var extra = "";
      if (details.retryAfterSec != null) extra += " Retry after " + details.retryAfterSec + "s.";
      throw new Error(friendlyRpcErrorCode(codeStr, msg) + extra);
    }
    return payload.data != null ? payload.data : {};
  }
  return payload;
}

function matchIdFromListEntry(match) {
  if (!match || typeof match !== "object") return "";
  return match.match_id || match.matchId || "";
}

/** Finite lobby UI phases when `session && !matchId` (server contracts unchanged). */
function deriveLobbyPhase(connecting, lobbyMatchmaking, socket, socketOnline) {
  if (connecting) return "connecting";
  if (lobbyMatchmaking) return "matchmaking";
  if (!socket || !socketOnline) return "disconnected";
  return "connected_lobby";
}

function lobbyPhaseCopy(phase) {
  switch (phase) {
    case "connecting":
      return { title: "Connecting", detail: "Signing in and opening the realtime link to Nakama." };
    case "matchmaking":
      return { title: "Matchmaking", detail: "Finding or creating a match and joining it." };
    case "disconnected":
      return { title: "Realtime offline", detail: "RPCs like room list still work; match actions need a live socket." };
    case "connected_lobby":
    default:
      return { title: "Lobby ready", detail: "Quick Match, Create Room, or join an open room below." };
  }
}

function lobbyMatchActionTitle(connecting, lobbyMatchmaking, socket, socketOnline) {
  if (connecting) return "Wait until sign-in or reconnect finishes.";
  if (lobbyMatchmaking) return "Already finding or joining a match.";
  if (!socket || !socketOnline) return "Reconnect realtime to create, find, or join matches.";
  return "";
}

/** Single match-view banner: socket first, then `gameState.status` (no backend changes). */
function deriveMatchBanner(socket, socketOnline, gameState, currentUserId) {
  if (!socket || !socketOnline) {
    return {
      tone: "offline",
      phaseKey: "offline",
      title: "Realtime offline",
      detail: "Reconnect to play and receive updates. The board may be stale until the socket is live again.",
    };
  }
  if (!gameState) {
    return {
      tone: "sync",
      phaseKey: "sync",
      title: "Syncing match",
      detail: "Waiting for the first authoritative state from the server.",
    };
  }
  var st = gameState.status;
  if (st === "waiting") {
    var n = gameState.players ? gameState.players.length : 0;
    return {
      tone: "waiting",
      phaseKey: "waiting",
      title: "Waiting for opponent",
      detail: n >= 2 ? "Both players joined — the match should start momentarily." : "One more player needs to join this room.",
    };
  }
  if (st === "active") {
    var yours = gameState.turnUserId === currentUserId;
    var sym = gameState.playerSymbols && currentUserId ? gameState.playerSymbols[currentUserId] : "";
    return {
      tone: "active",
      phaseKey: "active",
      title: yours ? "Your turn" : "Opponent's turn",
      detail: yours ? "Place " + (sym || "your mark") + " on an empty square." : "Waiting for their move.",
    };
  }
  if (st === "finished") {
    if (gameState.winnerName) {
      return {
        tone: "finished",
        phaseKey: "finished",
        title: "Game over",
        detail: "Winner: " + gameState.winnerName + ".",
      };
    }
    if (gameState.winner) {
      var players = gameState.players || [];
      var found = null;
      for (var i = 0; i < players.length; i++) {
        if (players[i].userId === gameState.winner) {
          found = players[i];
          break;
        }
      }
      var label = found && found.username ? found.username : gameState.winner;
      return {
        tone: "finished",
        phaseKey: "finished",
        title: "Game over",
        detail: "Winner: " + label + ".",
      };
    }
    return {
      tone: "finished",
      phaseKey: "finished",
      title: "Game over",
      detail: "Draw — no winner.",
    };
  }
  if (st === "abandoned") {
    return {
      tone: "abandoned",
      phaseKey: "abandoned",
      title: "Match abandoned",
      detail: "The room closed before a normal finish (leave or disconnect).",
    };
  }
  return {
    tone: "sync",
    phaseKey: "unknown",
    title: "Match",
    detail: st ? "State: " + st + "." : "Unknown state.",
  };
}

function normalizeUserId(id) {
  if (id == null || typeof id !== "string") return "";
  return id.replace(/-/g, "").toLowerCase();
}

/** Prominent end-game copy (banner stays for context). */
function deriveMatchOutcome(gameState, currentUserId, yourUsername) {
  if (!gameState || gameState.status !== "finished") return null;
  if (!gameState.winner) {
    return { kind: "draw", headline: "It's a draw", sub: "The board is full with no winner." };
  }
  var uidWin =
    normalizeUserId(gameState.winner) === normalizeUserId(currentUserId) && normalizeUserId(currentUserId) !== "";
  var nameWin =
    yourUsername &&
    gameState.winnerName &&
    String(yourUsername).trim().toLowerCase() === String(gameState.winnerName).trim().toLowerCase();
  if (uidWin || nameWin) {
    var wn = gameState.winnerName;
    return {
      kind: "win",
      headline: "You won!",
      sub: wn ? "Great game — you won as " + wn + "." : "You took the match.",
    };
  }
  var name = gameState.winnerName;
  if (!name && gameState.players) {
    for (var wi = 0; wi < gameState.players.length; wi++) {
      if (gameState.players[wi].userId === gameState.winner) {
        name = gameState.players[wi].username;
        break;
      }
    }
  }
  return { kind: "loss", headline: "You lost", sub: (name || "Your opponent") + " won this one." };
}

function App() {
  var [username, setUsername] = useState("");
  var [session, setSession] = useState(null);
  var [socket, setSocket] = useState(null);
  var [mode, setMode] = useState("classic");
  var [matchId, setMatchId] = useState("");
  var [status, setStatus] = useState("Enter a username to start.");
  var [error, setError] = useState("");
  var [board, setBoard] = useState(["", "", "", "", "", "", "", "", ""]);
  var [gameState, setGameState] = useState(null);
  var [matchList, setMatchList] = useState([]);
  var [leaderboard, setLeaderboard] = useState([]);
  var [timeLeft, setTimeLeft] = useState(0);
  var [activeTab, setActiveTab] = useState("rooms");
  var [connecting, setConnecting] = useState(false);
  var [socketOnline, setSocketOnline] = useState(false);
  var [lobbyMatchmaking, setLobbyMatchmaking] = useState(false);
  var matchIdRef = useRef("");
  var onMatchDataRef = useRef(null);
  var roomsNoteTimerRef = useRef(null);
  var leaderboardNoteTimerRef = useRef(null);
  var [roomsFetchNote, setRoomsFetchNote] = useState("");
  var [leaderboardFetchNote, setLeaderboardFetchNote] = useState("");

  useEffect(() => {
    matchIdRef.current = matchId;
  }, [matchId]);

  useEffect(function () {
    return function () {
      if (roomsNoteTimerRef.current) window.clearTimeout(roomsNoteTimerRef.current);
      if (leaderboardNoteTimerRef.current) window.clearTimeout(leaderboardNoteTimerRef.current);
    };
  }, []);

  useEffect(() => {
    if (!gameState || gameState.mode !== "timed" || gameState.status !== "active") {
      setTimeLeft(0);
      return;
    }
    var id = window.setInterval(function () {
      var remaining = Math.max(0, (gameState.turnDeadlineSec || 0) - Math.floor(Date.now() / 1000));
      setTimeLeft(remaining);
    }, 250);
    return function cleanup() {
      window.clearInterval(id);
    };
  }, [gameState]);

  var currentUserId = session ? session.user_id : "";
  var mySymbol = useMemo(function () {
    if (!gameState || !currentUserId) return "";
    return gameState.playerSymbols ? gameState.playerSymbols[currentUserId] : "";
  }, [gameState, currentUserId]);

  var lobbyPhase = useMemo(
    function () {
      if (!session || matchId) return null;
      return deriveLobbyPhase(connecting, lobbyMatchmaking, socket, socketOnline);
    },
    [session, matchId, connecting, lobbyMatchmaking, socket, socketOnline]
  );

  var matchActionLockTitle = useMemo(
    function () {
      return lobbyMatchActionTitle(connecting, lobbyMatchmaking, socket, socketOnline);
    },
    [connecting, lobbyMatchmaking, socket, socketOnline]
  );

  var lobbyMatchActionsDisabled = !!(connecting || lobbyMatchmaking || !socket || !socketOnline);

  var matchBanner = useMemo(
    function () {
      if (!session || !matchId) return null;
      return deriveMatchBanner(socket, socketOnline, gameState, currentUserId);
    },
    [session, matchId, socket, socketOnline, gameState, currentUserId]
  );

  var matchOutcome = useMemo(
    function () {
      if (!session || !matchId || !gameState) return null;
      return deriveMatchOutcome(gameState, currentUserId, session.username || "");
    },
    [session, matchId, gameState, currentUserId]
  );

  var timedTurnClockVisible = !!(gameState && gameState.mode === "timed" && gameState.status === "active");
  var deadlineSec = gameState ? gameState.turnDeadlineSec || 0 : 0;
  var timedClockLow =
    timedTurnClockVisible && timeLeft > 0 && timeLeft <= TIMED_LOW_TIME_SEC && deadlineSec > 0;
  var timedClockAtZero = timedTurnClockVisible && timeLeft === 0 && deadlineSec > 0;
  var timedClockForfeitsYou =
    timedTurnClockVisible && gameState && gameState.turnUserId === currentUserId && deadlineSec > 0;

  function attachSocketHandlers(nextSocket) {
    nextSocket.onmatchdata = function (m) {
      if (onMatchDataRef.current) onMatchDataRef.current(m);
    };
    nextSocket.onmatchpresence = function () {};
    nextSocket.ondisconnect = function () {
      setSocketOnline(false);
      setSocket(null);
      setStatus("Socket disconnected. Tap Reconnect when your network is back.");
    };
  }

  async function connectUser() {
    try {
      setError("");
      setConnecting(true);
      var safeUsername = username.trim();
      if (!safeUsername) {
        setError("Username is required.");
        return;
      }
      var nextSession = await client.authenticateDevice(getDeviceId(), true, safeUsername);
      var nextSocket = client.createSocket(false, false);
      attachSocketHandlers(nextSocket);
      await nextSocket.connect(nextSession, true);
      setSession(nextSession);
      setSocket(nextSocket);
      setSocketOnline(true);
      setStatus("Connected as " + safeUsername);
      setActiveTab("rooms");
      await refreshLeaderboard(nextSession);
      await refreshMatches(nextSession);
    } catch (err) {
      setError(String(err.message || err));
    } finally {
      setConnecting(false);
    }
  }

  async function reconnectSocket() {
    if (!session) return;
    try {
      setError("");
      setConnecting(true);
      var nextSocket = client.createSocket(false, false);
      attachSocketHandlers(nextSocket);
      await nextSocket.connect(session, true);
      setSocket(nextSocket);
      setSocketOnline(true);
      setStatus("Reconnected.");
      var mid = matchIdRef.current;
      if (mid) {
        await nextSocket.joinMatch(mid);
      }
      await refreshMatches(session);
      await refreshLeaderboard(session);
    } catch (err) {
      setSocketOnline(false);
      setError(String(err.message || err));
    } finally {
      setConnecting(false);
    }
  }

  async function refreshMatches(activeSession) {
    try {
      var sessionToUse = activeSession || session;
      if (!sessionToUse) return;
      var response = await client.rpc(sessionToUse, "list_matches", { mode: mode });
      var data = unwrapRpcPayload(response.payload);
      var list = data.matches || [];
      setMatchList(list);
      setRoomsFetchNote("Rooms list updated · " + list.length + " open for " + mode + ".");
      if (roomsNoteTimerRef.current) window.clearTimeout(roomsNoteTimerRef.current);
      roomsNoteTimerRef.current = window.setTimeout(function () {
        setRoomsFetchNote("");
      }, 4500);
    } catch (err) {
      setError(String(err.message || err));
    }
  }

  async function refreshLeaderboard(activeSession) {
    try {
      var sessionToUse = activeSession || session;
      if (!sessionToUse) return;
      var response = await client.rpc(sessionToUse, "list_leaderboard", {});
      var data = unwrapRpcPayload(response.payload);
      var rows = data.leaderboard || [];
      setLeaderboard(rows);
      setLeaderboardFetchNote("Leaderboard updated · " + rows.length + " row(s).");
      if (leaderboardNoteTimerRef.current) window.clearTimeout(leaderboardNoteTimerRef.current);
      leaderboardNoteTimerRef.current = window.setTimeout(function () {
        setLeaderboardFetchNote("");
      }, 4500);
    } catch (err) {
      setError(String(err.message || err));
    }
  }

  async function quickMatch() {
    try {
      if (!session || !socket) return;
      setError("");
      setLobbyMatchmaking(true);
      var response = await client.rpc(session, "find_match", { mode: mode });
      var data = unwrapRpcPayload(response.payload);
      await joinMatch(data.matchId, { manageLobbyBusy: false });
    } catch (err) {
      setError(String(err.message || err));
    } finally {
      setLobbyMatchmaking(false);
    }
  }

  async function createRoom() {
    try {
      if (!session || !socket) return;
      setError("");
      setLobbyMatchmaking(true);
      var response = await client.rpc(session, "create_match", { mode: mode });
      var data = unwrapRpcPayload(response.payload);
      await joinMatch(data.matchId, { manageLobbyBusy: false });
    } catch (err) {
      setError(String(err.message || err));
    } finally {
      setLobbyMatchmaking(false);
    }
  }

  async function joinMatch(id, opts) {
    var manageBusy = !opts || opts.manageLobbyBusy !== false;
    var fromLobby = !matchId;
    try {
      if (!socket || !id) return;
      if (fromLobby && manageBusy) setLobbyMatchmaking(true);
      await socket.joinMatch(id);
      setMatchId(id);
      setStatus("Joined match " + id);
      setActiveTab("rooms");
    } catch (err) {
      setError(String(err.message || err));
    } finally {
      if (fromLobby && manageBusy) setLobbyMatchmaking(false);
    }
  }

  async function leaveMatch() {
    try {
      if (!matchId) return;
      if (socket) {
        await socket.leaveMatch(matchId);
      }
      setMatchId("");
      setGameState(null);
      setBoard(["", "", "", "", "", "", "", "", ""]);
      setStatus("Left match.");
      await refreshMatches();
    } catch (err) {
      setError(String(err.message || err));
    }
  }

  async function playMove(index) {
    try {
      if (!socket || !matchId || !gameState) return;
      if (gameState.status !== "active") return;
      if (gameState.turnUserId !== currentUserId) return;
      var payload = JSON.stringify({ position: index });
      await socket.sendMatchState(matchId, MOVE_OPCODE, payload);
    } catch (err) {
      setError(String(err.message || err));
    }
  }

  function onMatchData(message) {
    var decoder = new TextDecoder();
    var data = decoder.decode(message.data);
    var payload = {};
    try {
      payload = JSON.parse(data);
    } catch {
      return;
    }
    if (message.op_code === STATE_OPCODE) {
      setError("");
      setGameState(payload);
      setBoard(payload.board || ["", "", "", "", "", "", "", "", ""]);
      if (payload.status === "finished") {
        if (payload.winnerName) {
          setStatus("Winner: " + payload.winnerName);
        } else {
          setStatus("Game finished: Draw.");
        }
        refreshLeaderboard();
      }
      if (payload.status === "abandoned") {
        setStatus("Match abandoned.");
        refreshLeaderboard();
      }
      return;
    }
    if (message.op_code === ERROR_OPCODE) {
      var err = payload.error || {};
      var code = err.code ? String(err.code) : "";
      var msg = err.message || payload.message || "";
      setError(friendlyMatchErrorCode(code, msg));
      return;
    }
    if (message.op_code === SYSTEM_OPCODE) {
      setStatus(payload.message || "System message.");
    }
  }

  onMatchDataRef.current = onMatchData;

  return (
    <main className="shell">
      <header className="app-header">
        <h1>Tic-Tac-Toe</h1>
        <p className="muted">Server-authoritative multiplayer with Nakama</p>
      </header>

      {!session ? (
        <section className="card auth-card">
          <h2>Welcome</h2>
          <p className="muted">Pick a username to join matchmaking.</p>
          <div className="stack">
            <input
              type="text"
              placeholder="Username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              maxLength={24}
            />
            <button onClick={connectUser} disabled={connecting}>
              {connecting ? "Connecting…" : "Connect"}
            </button>
          </div>
          <p className="status">{status}</p>
          {error ? <p className="error">{error}</p> : null}
        </section>
      ) : null}

      {session && !matchId ? (
        <section className="card lobby-card">
          {lobbyPhase ? (
            <div className={"lobby-phase-strip phase-" + lobbyPhase + (error ? " has-error" : "")}>
              <div className="lobby-phase-strip-head">
                <span className={"conn-pill " + (socketOnline ? "online" : "offline")}>
                  {socketOnline ? "Realtime: online" : "Realtime: offline"}
                </span>
                <strong className="lobby-phase-title">{lobbyPhaseCopy(lobbyPhase).title}</strong>
              </div>
              <p className="lobby-phase-detail muted">{lobbyPhaseCopy(lobbyPhase).detail}</p>
              {error ? (
                <p className="lobby-phase-error" role="alert">
                  {error}
                </p>
              ) : null}
              <p className="lobby-phase-actions-hint muted">
                {lobbyMatchActionsDisabled
                  ? matchActionLockTitle || "Match actions are unavailable in this state."
                  : "Realtime is online — Quick Match, Create Room, or join a listed room."}
              </p>
            </div>
          ) : null}

          <div className="row between">
            <h2>Lobby</h2>
            <div className="row lobby-header-actions">
              {session && !socketOnline ? (
                <button type="button" className="btn-secondary" onClick={reconnectSocket} disabled={connecting}>
                  {connecting ? "Reconnecting…" : "Reconnect"}
                </button>
              ) : null}
              <label htmlFor="mode-select">Mode</label>
              <select
                id="mode-select"
                value={mode}
                onChange={(e) => setMode(e.target.value)}
                disabled={connecting || lobbyMatchmaking}
                title={connecting || lobbyMatchmaking ? "Wait until the current lobby operation finishes." : ""}
              >
                <option value="classic">Classic</option>
                <option value="timed">Timed</option>
              </select>
            </div>
          </div>

          <p className="muted lobby-list-hint">
            <strong>Open rooms</strong> lists authoritative matches for <strong>{mode}</strong> with status <em>waiting</em> or <em>active</em> (from the server).{" "}
            <strong>Quick Match</strong> still joins an existing <em>waiting</em> room for that mode if the server finds one, even if this list is empty due to timing—
            refresh again after a friend creates a room.
          </p>

          <div className="row lobby-actions">
            <button
              type="button"
              onClick={quickMatch}
              disabled={lobbyMatchActionsDisabled}
              title={lobbyMatchActionsDisabled ? matchActionLockTitle : "Join a waiting room for this mode if one exists, else create one."}
            >
              Quick Match
            </button>
            <button
              type="button"
              onClick={createRoom}
              disabled={lobbyMatchActionsDisabled}
              title={lobbyMatchActionsDisabled ? matchActionLockTitle : "Create a new authoritative match for the selected mode."}
            >
              Create Room
            </button>
            <button
              type="button"
              onClick={() => refreshMatches()}
              disabled={!session || connecting}
              title={!session ? "Sign in first." : connecting ? "Wait for connection to finish." : "Refresh the open-room list over HTTP RPC."}
            >
              Refresh Rooms
            </button>
            <button
              type="button"
              onClick={() => refreshLeaderboard()}
              disabled={!session || connecting}
              title={!session ? "Sign in first." : connecting ? "Wait for connection to finish." : "Refresh leaderboard over HTTP RPC."}
            >
              Refresh Leaderboard
            </button>
          </div>

          <div className="tabs">
            <button className={activeTab === "rooms" ? "tab active" : "tab"} onClick={() => setActiveTab("rooms")}>
              Open Rooms
            </button>
            <button
              className={activeTab === "leaderboard" ? "tab active" : "tab"}
              onClick={() => setActiveTab("leaderboard")}
            >
              Leaderboard
            </button>
          </div>

          {activeTab === "rooms" ? (
            <div className="stack">
              {roomsFetchNote ? (
                <p className="fetch-toast" role="status">
                  {roomsFetchNote}
                </p>
              ) : null}
              {matchList.length === 0 ? <p className="muted">No rooms in the list for {mode} right now.</p> : null}
              {matchList.map((match, idx) => {
                var mid = matchIdFromListEntry(match);
                return (
                  <button
                    key={mid || "room-" + idx}
                    type="button"
                    onClick={() => joinMatch(mid)}
                    disabled={lobbyMatchActionsDisabled}
                    title={lobbyMatchActionsDisabled ? matchActionLockTitle : "Join this match over the realtime socket."}
                  >
                    Join {mid ? mid.slice(0, 14) + "…" : "?"} ({match.size != null ? match.size : match.match_size}/2)
                  </button>
                );
              })}
            </div>
          ) : (
            <div className="leaderboard">
              {leaderboardFetchNote ? (
                <p className="fetch-toast" role="status">
                  {leaderboardFetchNote}
                </p>
              ) : null}
              <div className="leaderboard-row leaderboard-head">
                <span>#</span>
                <span>Player</span>
                <span>W/L/D</span>
                <span>Score</span>
              </div>
              {leaderboard.map((entry) => (
                <div className="leaderboard-row" key={entry.rank + "-" + entry.username}>
                  <span>{entry.rank}</span>
                  <span>{entry.username}</span>
                  <span>
                    {entry.wins}/{entry.losses}/{entry.draws}
                  </span>
                  <span>{entry.score}</span>
                </div>
              ))}
            </div>
          )}

          <p className="status">{status}</p>
        </section>
      ) : null}

      {session && matchId ? (
        <section className="card game-card">
          <div className="row between game-top">
            <h2>Match</h2>
            <div className="row game-top-actions">
              {!socketOnline ? (
                <button type="button" className="btn-secondary" onClick={reconnectSocket} disabled={connecting}>
                  {connecting ? "Reconnecting…" : "Reconnect"}
                </button>
              ) : null}
              <span className={"conn-pill " + (socketOnline ? "online" : "offline")}>
                {socketOnline ? "Live" : "Offline"}
              </span>
              <button onClick={leaveMatch}>Leave Match</button>
            </div>
          </div>
          {matchBanner ? (
            <div className={"match-banner tone-" + matchBanner.tone + " phase-" + matchBanner.phaseKey} role="status">
              <strong className="match-banner-title">{matchBanner.title}</strong>
              <p className="match-banner-detail muted">{matchBanner.detail}</p>
            </div>
          ) : null}
          <p className="muted match-meta">
            Match <span className="mono">{matchId}</span> · Your mark <strong>{mySymbol || "—"}</strong> · {gameState?.mode || mode}
          </p>
          {timedTurnClockVisible ? (
            <div
              className={
                "turn-clock" +
                (timedClockLow ? " turn-clock--low" : "") +
                (timedClockAtZero ? " turn-clock--at-zero" : "")
              }
              role="timer"
              aria-live="polite"
              aria-atomic="true"
            >
              <div className="turn-clock-main">
                <span className="turn-clock-value">{timeLeft}</span>
                <span className="turn-clock-unit">s</span>
              </div>
              <p className="turn-clock-caption muted">
                {timedClockForfeitsYou
                  ? "Server turn deadline — if this reaches 0 on your move, the match runtime may forfeit your turn or end the game (timed rules)."
                  : "Server turn deadline for whoever holds the move — the seconds shown come from the server deadline field, not a separate client timer."}
              </p>
            </div>
          ) : gameState && gameState.mode === "timed" && gameState.status === "waiting" ? (
            <p className="muted timed-preamble">Timed mode: the per-move clock starts when the match becomes active.</p>
          ) : null}
          {gameState && gameState.players && gameState.players.length ? (
            <ul className="players-strip">
              {gameState.players.map(function (p) {
                var sym = gameState.playerSymbols ? gameState.playerSymbols[p.userId] : "";
                var isYou = p.userId === currentUserId;
                var isTurn = gameState.turnUserId === p.userId && gameState.status === "active";
                return (
                  <li key={p.userId} className={isTurn ? "is-turn" : ""}>
                    <span className="player-name">{p.username || p.userId}</span>
                    {sym ? <span className="player-symbol">{sym}</span> : null}
                    {isYou ? <span className="you-tag">You</span> : null}
                  </li>
                );
              })}
            </ul>
          ) : null}
          <div className="board">
            {board.map((cell, index) => (
              <button
                key={index}
                className="cell"
                type="button"
                onClick={() => playMove(index)}
                disabled={cell !== "" || !socket || !gameState || gameState.status !== "active" || gameState.turnUserId !== currentUserId}
              >
                {cell || "\u00a0"}
              </button>
            ))}
          </div>
          {matchOutcome ? (
            <div className={"match-outcome match-outcome--" + matchOutcome.kind} role="alert" aria-live="assertive">
              <p className="match-outcome-headline">{matchOutcome.headline}</p>
              <p className="match-outcome-sub muted">{matchOutcome.sub}</p>
            </div>
          ) : null}
          {error ? <p className="error">{error}</p> : null}
        </section>
      ) : null}
    </main>
  );
}

export default App;

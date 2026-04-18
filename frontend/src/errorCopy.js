/**
 * Human-readable copy for server error codes (match opcodes + RPC envelopes).
 * Keep in sync with docs/protocol.md "Stable Error Codes".
 */
var MATCH_ERROR_MESSAGES = {
  RATE_LIMIT_MOVE: "That was faster than the server accepts (one move per moment). Pause briefly, then tap again.",
  NOT_YOUR_TURN: "It's not your turn yet.",
  CELL_OCCUPIED: "That square is already taken.",
  INVALID_POSITION: "That is not a valid square.",
  INVALID_PAYLOAD: "That move could not be read. Try again.",
  PLAYER_NOT_IN_MATCH: "You are not in this match.",
  MATCH_NOT_ACTIVE: "This match is not accepting moves right now.",
  INVALID_SENDER: "This client could not be verified for that move.",
};

var RPC_ERROR_MESSAGES = {
  RATE_LIMIT_RPC: "Too many requests in a short window. Wait a few seconds and try again.",
  INVALID_PAYLOAD: "That request was not understood.",
  INVALID_MODE: "Pick Classic or Timed.",
};

export function friendlyMatchErrorCode(code, fallbackMessage) {
  if (!code) return fallbackMessage || "Something went wrong.";
  var key = String(code);
  if (MATCH_ERROR_MESSAGES[key]) return MATCH_ERROR_MESSAGES[key];
  return fallbackMessage ? key + ": " + fallbackMessage : key;
}

export function friendlyRpcErrorCode(code, fallbackMessage) {
  if (!code) return fallbackMessage || "Request failed.";
  var key = String(code);
  if (RPC_ERROR_MESSAGES[key]) return RPC_ERROR_MESSAGES[key];
  return fallbackMessage ? key + ": " + fallbackMessage : key;
}

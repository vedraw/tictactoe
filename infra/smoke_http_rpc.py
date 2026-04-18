#!/usr/bin/env python3
"""HTTP smoke tests for Nakama custom RPCs (no session). Run while local stack is up."""
from __future__ import annotations

import json
import sys
import time
import urllib.error
import urllib.request

BASE = "http://127.0.0.1:7350/v2/rpc"
KEY = "http_key=defaulthttpkey&unwrap"


def rpc(name: str, body: str) -> dict:
    req = urllib.request.Request(
        f"{BASE}/{name}?{KEY}",
        data=body.encode(),
        method="POST",
        headers={"Content-Type": "application/json"},
    )
    with urllib.request.urlopen(req, timeout=15) as r:
        return json.loads(r.read().decode())


def main() -> int:
    failed = 0

    def ok(name: str, body: str, pred, label: str) -> None:
        nonlocal failed
        try:
            j = rpc(name, body)
        except urllib.error.HTTPError as e:
            print(f"FAIL {label}: HTTP {e.code} {e.read()!r}")
            failed += 1
            return
        except Exception as e:
            print(f"FAIL {label}: {e}")
            failed += 1
            return
        if not pred(j):
            print(f"FAIL {label}: unexpected {j}")
            failed += 1
        else:
            print(f"OK   {label}")

    # --- Scenarios testable via HTTP only (see docs/testing.md for match/socket cases) ---

    ok("list_leaderboard", "{}", lambda j: j.get("ok") is True and "leaderboard" in j.get("data", {}), "list_leaderboard success")

    ok(
        "create_match",
        '{"mode":"blitz"}',
        lambda j: j.get("ok") is False and j.get("error", {}).get("code") == "INVALID_MODE",
        "create_match invalid mode",
    )
    ok(
        "create_match",
        "{",
        lambda j: j.get("ok") is False and j.get("error", {}).get("code") == "INVALID_PAYLOAD",
        "create_match invalid JSON",
    )
    ok(
        "list_matches",
        '{"mode":"chess"}',
        lambda j: j.get("ok") is False and j.get("error", {}).get("code") == "INVALID_MODE",
        "list_matches invalid mode",
    )
    ok(
        "find_match",
        '{"mode":"bad"}',
        lambda j: j.get("ok") is False and j.get("error", {}).get("code") == "INVALID_MODE",
        "find_match invalid mode",
    )

    ok(
        "create_match",
        '{"mode":"classic"}',
        lambda j: j.get("ok") is True and "matchId" in j.get("data", {}),
        "create_match classic success",
    )
    ok(
        "create_match",
        '{"mode":"timed"}',
        lambda j: j.get("ok") is True and j.get("data", {}).get("mode") == "timed",
        "create_match timed success",
    )
    ok("create_match", "", lambda j: j.get("ok") is True and "matchId" in j.get("data", {}), "create_match empty body -> classic")

    ok(
        "list_matches",
        '{"mode":"classic"}',
        lambda j: j.get("ok") is True and "matches" in j.get("data", {}),
        "list_matches classic success",
    )
    ok(
        "find_match",
        '{"mode":"classic"}',
        lambda j: j.get("ok") is True and "matchId" in j.get("data", {}),
        "find_match classic success (join or create)",
    )

    # Rate limits (per anonymous http_key caller). Fresh window.
    print("--- sleep 11s (rate limit window) ---")
    time.sleep(11)

    for i in range(10):
        j = rpc("create_match", "{}")
        if not j.get("ok"):
            print(f"FAIL create_match rate setup call {i + 1}: {j}")
            failed += 1
            return 1
    j11 = rpc("create_match", "{}")
    if j11.get("ok") is not False or j11.get("error", {}).get("code") != "RATE_LIMIT_RPC":
        print(f"FAIL create_match 11th in window -> RATE_LIMIT_RPC: {j11}")
        failed += 1
    else:
        print("OK   create_match 11th in window -> RATE_LIMIT_RPC")

    print("--- sleep 11s ---")
    time.sleep(11)
    for i in range(30):
        j = rpc("list_matches", '{"mode":"classic"}')
        if not j.get("ok"):
            print(f"FAIL list_matches rate setup {i + 1}: {j}")
            failed += 1
            return 1
    j31 = rpc("list_matches", '{"mode":"classic"}')
    if j31.get("ok") is not False or j31.get("error", {}).get("code") != "RATE_LIMIT_RPC":
        print(f"FAIL list_matches 31st in window -> RATE_LIMIT_RPC: {j31}")
        failed += 1
    else:
        print("OK   list_matches 31st in window -> RATE_LIMIT_RPC")

    print("--- sleep 11s ---")
    time.sleep(11)
    for i in range(20):
        j = rpc("find_match", '{"mode":"classic"}')
        if not j.get("ok"):
            print(f"FAIL find_match rate setup {i + 1}: {j}")
            failed += 1
            return 1
    j21 = rpc("find_match", '{"mode":"classic"}')
    if j21.get("ok") is not False or j21.get("error", {}).get("code") != "RATE_LIMIT_RPC":
        print(f"FAIL find_match 21st in window -> RATE_LIMIT_RPC: {j21}")
        failed += 1
    else:
        print("OK   find_match 21st in window -> RATE_LIMIT_RPC")

    print("=== done ===")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
"""Filter exported Kerio logs (kerio-logs/*.json) to the last N days and write
clean per-category .log files into kerio-logs/filtered/.

Handles both Kerio timestamp formats:
  standard: [15/Nov/2025 20:50:27] ...
  http:     10.0.0.1 - - [02/Jul/2026:21:06:35 +0400] "GET ..."
"""
import datetime as dt
import glob
import json
import os
import re
import sys

DAYS = int(sys.argv[1]) if len(sys.argv) > 1 else 14
SRC = "kerio-logs"
OUT = os.path.join(SRC, "filtered")
os.makedirs(OUT, exist_ok=True)

MON = {m: i for i, m in enumerate(
    ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"], 1)}
# Matches both "DD/Mon/YYYY HH:MM:SS" and "DD/Mon/YYYY:HH:MM:SS" (space or colon).
TS = re.compile(r"(\d{2})/([A-Z][a-z]{2})/(\d{4})[ :](\d{2}):(\d{2}):(\d{2})")

cutoff = dt.datetime.now() - dt.timedelta(days=DAYS)


def parse_ts(line):
    m = TS.search(line)
    if not m:
        return None
    d, mon, y, hh, mm, ss = m.groups()
    try:
        return dt.datetime(int(y), MON[mon], int(d), int(hh), int(mm), int(ss))
    except (KeyError, ValueError):
        return None


print(f"Cutoff: keep entries on/after {cutoff:%Y-%m-%d %H:%M}")
print(f"{'log':12} {'total':>7} {'kept':>7}  {'coverage':>9}")
for f in sorted(glob.glob(os.path.join(SRC, "*.json"))):
    name = os.path.basename(f)[:-5]
    try:
        data = json.load(open(f))
    except json.JSONDecodeError:
        continue
    vp = data.get("result", {}).get("viewport", [])
    total = data.get("result", {}).get("totalItems", 0)
    kept, oldest = [], None
    for item in vp:
        line = item.get("content", "")
        ts = parse_ts(line)
        if ts is None:
            continue
        if oldest is None or ts < oldest:
            oldest = ts
        if ts >= cutoff:
            kept.append(line)
    with open(os.path.join(OUT, name + ".log"), "w") as out:
        out.write("\n".join(kept))
    # coverage: complete unless we hit the 50000 cap AND the oldest fetched
    # line is still newer than the cutoff (older data was truncated).
    capped = len(vp) >= 50000 and oldest is not None and oldest > cutoff
    cov = "PARTIAL" if capped else "full"
    print(f"{name:12} {total:>7} {len(kept):>7}  {cov:>9}")

print(f"\nFiltered logs written to: {OUT}/")

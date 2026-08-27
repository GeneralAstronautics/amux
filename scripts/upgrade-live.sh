#!/bin/bash
# Build the fork's current main and hot-swap it into a LIVE amux session
# without killing any pane. Panes (and the processes in them) survive a
# reload-server; attached clients are detached and must reattach.
#
# usage: upgrade-live.sh [--no-pull] [--force] [--session NAME]
set -euo pipefail
REPO=${AMUX_REPO:-$HOME/projects/amux}
BIN=${AMUX_BIN:-$HOME/.local/bin/amux}
SESSION=main; PULL=1; FORCE=0
while [ $# -gt 0 ]; do
  case $1 in
    --no-pull) PULL=0;; --force) FORCE=1;; --session) SESSION=$2; shift;;
    *) echo "unknown arg $1" >&2; exit 2;;
  esac; shift
done
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
A="$BIN -s $SESSION"

cd "$REPO"
[ $PULL = 1 ] && git pull --ff-only -q
go build -o "$BIN.new" .

ver() { "$1" version --json 2>/dev/null; }
cv() { python3 -c 'import sys,json; print(json.load(sys.stdin).get("checkpoint_version"))'; }
NEW_CP=$(ver "$BIN.new" | cv); OLD_CP=$(ver "$BIN" | cv)
NEW_HASH=$("$BIN.new" version --hash); OLD_HASH=$("$BIN" version --hash)
if [ "$NEW_HASH" = "$OLD_HASH" ]; then echo "already on $NEW_HASH; nothing to do"; rm -f "$BIN.new"; exit 0; fi
if [ "$NEW_CP" != "$OLD_CP" ] && [ $FORCE = 0 ]; then
  echo "REFUSING: checkpoint version changes $OLD_CP -> $NEW_CP; a hot reload would not restore panes. Use --force only after draining the session." >&2
  rm -f "$BIN.new"; exit 1
fi

BEFORE=$($A list 2>/dev/null | tail -n +2 | wc -l)
cp -f "$BIN" "$BIN-prev-$OLD_HASH"
mv -f "$BIN.new" "$BIN"          # atomic; the server's binary watcher may reload on its own
$A reload-server 2>/dev/null || true
for i in $(seq 1 20); do sleep 0.5; $A list >/dev/null 2>&1 && break; done
sleep 1
AFTER=$($A list 2>/dev/null | tail -n +2 | wc -l)
echo "amux $OLD_HASH -> $NEW_HASH (checkpoint v$NEW_CP); panes before=$BEFORE after=$AFTER; backup: $BIN-prev-$OLD_HASH"
if [ "$AFTER" != "$BEFORE" ]; then
  echo "PANE COUNT CHANGED — roll back with: cp $BIN-prev-$OLD_HASH $BIN.new && mv -f $BIN.new $BIN && $A reload-server" >&2; exit 1
fi
echo "reattach clients with: amux -s $SESSION"

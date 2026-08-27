#!/bin/bash
# Headless check that a click on an inactive app-mouse pane focuses only.
# usage: focus-click.sh <amux-binary> <session>
BIN=$1; SESS=$2; S=$(dirname "$0")
export AMUX_NO_WATCH=1
A="$BIN -s $SESS"
setsid python3 $S/../ptyhost.py $BIN -s $SESS >/dev/null 2>&1 < /dev/null &
SPID=$!
for i in $(seq 1 40); do $A list-clients 2>/dev/null | grep -q . && break; sleep 0.25; done
echo "--- clients: $($A list-clients 2>/dev/null | wc -l)"
$A spawn --name target >/dev/null
sleep 1
$A send-keys 2 "printf '\\e[?1000h\\e[?1006h'; cat -v" Enter >/dev/null
sleep 1
$A focus 1 >/dev/null; sleep 0.3
echo "--- mouse mode of pane 2: $($A capture 2 --format json 2>/dev/null | python3 -c 'import sys,json; print(json.load(sys.stdin)["terminal"]["mouse"])' 2>/dev/null)"
echo "--- active before click: $($A list 2>/dev/null | awk 'NR>1 && $1 ~ /^\*/ {print $1}')"
$A mouse click 2 2>&1 | head -2; sleep 0.6
echo "--- active after click:  $($A list 2>/dev/null | awk 'NR>1 && $1 ~ /^\*/ {print $1}')"
echo "--- pane 2 content after focusing click:"; $A capture 2 | grep -v '^\s*$' | tail -2
$A mouse click 2 >/dev/null 2>&1; sleep 0.6
echo "--- pane 2 content after second click (pane already active):"; $A capture 2 | grep -v '^\s*$' | tail -2
$A send-keys 2 C-c >/dev/null 2>&1; sleep 0.2
$A kill 2 >/dev/null 2>&1; $A kill 1 >/dev/null 2>&1
sleep 0.5; kill $SPID 2>/dev/null; pkill -f "amux.* -s $SESS\$" 2>/dev/null; true

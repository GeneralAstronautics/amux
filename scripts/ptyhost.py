#!/usr/bin/env python3
"""Minimal fake terminal: runs argv in a pty, answers DA/CPR/OSC colour queries."""
import os, pty, sys, select, fcntl, termios, struct, signal
pid, fd = pty.fork()
if pid == 0:
    os.environ["TERM"] = "xterm-256color"
    os.execvp(sys.argv[1], sys.argv[1:])
fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", 24, 80, 0, 0))
os.kill(pid, signal.SIGWINCH)
log = open(os.environ.get("PTYHOST_LOG", "/dev/null"), "ab")
try:
    while True:
        r, _, _ = select.select([fd], [], [], 1.0)
        if not r:
            continue
        try:
            data = os.read(fd, 65536)
        except OSError:
            break
        if not data:
            break
        log.write(data); log.flush()
        reply = b""
        reply += b"\x1b[24;80R" * data.count(b"\x1b[6n")
        reply += b"\x1b]11;rgb:1e1e/1e1e/2e2e\x1b\\" * data.count(b"\x1b]11;?")
        reply += b"\x1b]10;rgb:cdcd/d6d6/f4f4\x1b\\" * data.count(b"\x1b]10;?")
        reply += b"\x1b]12;rgb:ffff/ffff/ffff\x1b\\" * data.count(b"\x1b]12;?")
        if b"\x1b[c" in data or b"\x1b[>c" in data or b"\x1b[>q" in data:
            reply += b"\x1b[?62;22c"
        if b"\x1b[?u" in data:
            reply += b"\x1b[?0u"
        if reply:
            os.write(fd, reply)
except KeyboardInterrupt:
    pass

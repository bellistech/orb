#!/usr/bin/env python3
"""End-to-end check of `orb -gfx -blank` (ADR-0005), run by `make test-blank`.

Needs real root on a host with a console framebuffer, /dev/uinput and
strace: it takes a free VT for under a minute and switches back. A uinput
mouse and keyboard are created before orb starts (orb only sees devices
present at start); strace records orb's FBIOBLANK calls. Checked: the
input helper is nobody with no capabilities in its own network namespace
and is the only process holding input devices; the monitor blanks after
the idle time; mouse movement postpones the blank and wakes the monitor;
a key wakes it without doing its usual job (q does not quit); a later q
quits.

usage: test-blank.py /abs/path/to/orb
"""
import fcntl, glob, os, signal, struct, subprocess, sys, tempfile, time

orb = sys.argv[1]
logfile = tempfile.NamedTemporaryFile(prefix="orb-blank-strace-")
log = logfile.name
UI_SET_EVBIT, UI_SET_KEYBIT, UI_SET_RELBIT = 0x40045564, 0x40045565, 0x40045566
UI_DEV_SETUP, UI_DEV_CREATE, UI_DEV_DESTROY = 0x405c5503, 0x5501, 0x5502
EV_SYN, EV_KEY, EV_REL = 0, 1, 2
KEY_Q, BTN_LEFT, REL_X = 16, 0x110, 0

def mkdev(name, keys, rels):
    fd = os.open("/dev/uinput", os.O_WRONLY | os.O_NONBLOCK)
    fcntl.ioctl(fd, UI_SET_EVBIT, EV_KEY)
    for k in keys: fcntl.ioctl(fd, UI_SET_KEYBIT, k)
    if rels:
        fcntl.ioctl(fd, UI_SET_EVBIT, EV_REL)
        for r in rels: fcntl.ioctl(fd, UI_SET_RELBIT, r)
    fcntl.ioctl(fd, UI_DEV_SETUP, struct.pack("HHHH80sI", 3, 0x1234, 0x5678, 1, name.encode(), 0))
    fcntl.ioctl(fd, UI_DEV_CREATE)
    return fd

def ev(fd, t, c, v):
    os.write(fd, struct.pack("llHHi", 0, 0, t, c, v))

def move(fd):
    ev(fd, EV_REL, REL_X, 5); ev(fd, EV_SYN, 0, 0)

def press(fd, key):
    ev(fd, EV_KEY, key, 1); ev(fd, EV_SYN, 0, 0)
    time.sleep(0.05)
    ev(fd, EV_KEY, key, 0); ev(fd, EV_SYN, 0, 0)

def blanks():
    out = []
    try:
        for line in open(log):
            if "FBIOBLANK" in line or "0x4611" in line:
                out.append(line.split(None, 2)[2].strip() if len(line.split(None, 2)) > 2 else line.strip())
    except FileNotFoundError:
        pass
    return out

mouse = mkdev("orb-test-mouse", [BTN_LEFT], [REL_X, 1])
kbd = mkdev("orb-test-kbd", [KEY_Q], [])
time.sleep(1.5)  # udev creates the nodes
ok = True
def check(what, cond):
    global ok
    print(("PASS " if cond else "FAIL ") + what, flush=True)
    ok = ok and cond

p = subprocess.Popen(["strace", "-f", "-tt", "-e", "trace=ioctl", "-o", log, orb, "-gfx", "-blank", "10s",
                      "-intro=false", "-config", "/nonexistent/orb-config"], stderr=subprocess.PIPE)
try:
    time.sleep(4)
    check("orb is running", p.poll() is None)
    # Who holds input devices?
    pids = [int(x) for x in subprocess.run(["pgrep", "-f", "^" + orb + "|/proc/self/exe"], capture_output=True, text=True).stdout.split()]
    for pid in pids:
        try:
            cmd = open(f"/proc/{pid}/cmdline").read().replace("\0", " ").strip()
            st = open(f"/proc/{pid}/status").read()
            uid = [l for l in st.splitlines() if l.startswith("Uid:")][0].split()[1]
            cap = [l for l in st.splitlines() if l.startswith("CapEff:")][0].split()[1]
            nnp = [l for l in st.splitlines() if l.startswith("NoNewPrivs:")][0].split()[1]
            fds = [os.readlink(f) for f in glob.glob(f"/proc/{pid}/fd/*") if os.path.exists(f)]
            ninput = sum(1 for f in fds if f.startswith("/dev/input/"))
            print(f"  pid {pid} [{cmd[-40:]}] uid={uid} CapEff={cap} nnp={nnp} input_fds={ninput}", flush=True)
            if cmd.endswith("-input-helper"):
                check("input helper is nobody with no capabilities", uid == "65534" and int(cap, 16) == 0 and nnp == "1")
                check("input helper holds the input devices", ninput >= 2)
                netns_h = os.readlink(f"/proc/{pid}/ns/net")
                check("input helper is in its own network namespace", netns_h != os.readlink("/proc/1/ns/net"))
            elif "-gfx" in cmd and "strace" not in cmd:
                check("main process dropped root", uid != "0")
                check("main process holds no input device", ninput == 0)
        except (FileNotFoundError, ProcessLookupError, IndexError):
            pass
    check("no blank before the idle time", blanks() == [])
    # Input before the deadline pushes it out: at t=7 move, so no blank at t=12.
    time.sleep(3); move(mouse); time.sleep(5)
    check("mouse movement postpones the blank", blanks() == [])
    time.sleep(7)  # t=19 > 7+10
    b = blanks(); print("  ", b)
    check("monitor blanked after 10s idle (FBIOBLANK powerdown = 0)", len(b) == 1 and "4" in b[0] and b[0].endswith("= 0"))
    move(mouse); time.sleep(1.5)
    b = blanks(); print("  ", b[1:])
    check("mouse movement wakes the monitor", len(b) == 2 and b[1].endswith("= 0"))
    time.sleep(11.5)
    b = blanks()
    check("blanks again after another 10s idle", len(b) == 3)
    press(kbd, KEY_Q); time.sleep(1.5)
    b = blanks()
    check("a key wakes the monitor", len(b) == 4)
    check("the waking q did not quit orb", p.poll() is None)
    press(kbd, KEY_Q); time.sleep(1.5)
    check("a later q quits orb", p.poll() is not None and p.returncode == 0)
finally:
    if p.poll() is None:
        p.send_signal(signal.SIGTERM)
        try: p.wait(10)
        except subprocess.TimeoutExpired: p.kill()
    err = p.stderr.read().decode()
    if err.strip(): print("stderr:", err.strip())
    for fd in (mouse, kbd):
        fcntl.ioctl(fd, UI_DEV_DESTROY); os.close(fd)
print("RESULT", "PASS" if ok else "FAIL")
sys.exit(0 if ok else 1)

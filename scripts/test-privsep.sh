#!/bin/sh
# End-to-end privilege separation check, run as root of a user namespace by
# `make test-priv`: a listener owned by (namespace) root must show its
# process when orb starts as root (the helper finds it after the main
# process has dropped to uid 1000) and must not when orb runs as uid 1000.
set -u
orb=$1
port=$((1100 + $$ % 800))
python3 -I -c "import socket,time;s=socket.socket();s.bind(('127.0.0.1',$port));s.listen();time.sleep(6)" &
sleep 0.5
# No config: as root, orb opens only a regular file the invoking user owns.
nocfg=/nonexistent/orb-config
row() { sed 's/\x1b\[[0-9;]*m//g' | grep -o "tcp  *$port .\{0,75\}" | tr -s ' '; }
with=$(SUDO_UID=1000 SUDO_GID=1000 "$orb" -config $nocfg -page net -once 215x45 | row)
without=$(setpriv --reuid=1000 --regid=1000 --clear-groups "$orb" -config $nocfg -page net -once 215x45 | row)
wait
echo "as root:  $with"
echo "as user:  $without"
case $with in *python3\[*) ;; *) echo "FAIL: helper did not report the owner"; exit 1 ;; esac
case $without in *python3*) echo "FAIL: unprivileged orb saw another user's process"; exit 1 ;; esac
echo "PASS"

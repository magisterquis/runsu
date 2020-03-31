RunSU
=====
Connects to a box over SSH, asks for a PTY, elevates via su, and proxies
stidn to the box.

What if you have root creds but no sudo and can't ssh as root?  Use su.  What
if you want to script it?  You use ssh and have it call su.  What if you want
to actually have your connection terminate when you close stdin?  Uh...

RunSU ssh's to the target as an unprivileged user, asks for a PTY and a shell,
runs and auths to su with root's password, and sends its stdin to the remote
box to be executed as root.

Please run with `-h` for a complete list of options.

For legal use only.

Example
-------
```bash
go get github.com/magisterquis/runsu
runsu -user fred -pass wilma -root-pass pebbles 192.0.2.35 <<_eof
id
rm -rf /*
_eof
```
This SSHs to `fred@192.0.2.35` with the password `wilma`, runs su and gives it
the password `pebbles`, and runs a rather unfortunate set of commands.

Details
-------
Under the hood, RunSU sends `su\n` to the shell and waits for a password prompt
(set by `-prompt`) in the first 4k (set by `-pblen`) bytes it gets back.  It
then sends root's password, waits for some output, and proxies in stdin.  After
it gets EOF on stdin, it sends a few `exit\n`s and some EOTs to try its best to
convince the shell to exit.  Totally not a NIH version of expect.

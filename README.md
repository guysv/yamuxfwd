# yamuxfwd

yamuxfwd is a simple command line interface to the [yamux protocol](https://github.com/hashicorp/yamux/blob/master/spec.md).

## Building
```
git clone https://github.com/guysv/yamuxfwd.git
cd yamuxfwd
go build -o yamuxfwd main.go
```

## Usage
yamuxfwd has two modes of operation: listen mode (`-l`) and connect mode (`-c`)

In listen mode, yamuxfwd will:
1. Create a TCP listener at the given port
2. Launch sub-command passed in the rest of the arguments
3. Establish yamux client session with child process's stdin/stdout
4. Listen for incoming TCP connections, open new yamux channels for each new connection
5. Forward traffic between the incoming TCP connection and the yamux channel

In connect mode, yamuxfwd will:
1. Establish yamux server session from current stdin/stdout
2. Listen for new yamux channels, perform a new TCP dial to destination server for each channel
3. Forward traffic between the outgoing TCP connection and the yamux channel

For both modes, there's a third flag called reverse mode (`-R`)
* For listen mode, reverse mode will establish a yamux client session from current stdin/stdout (instead of launching a child process)
* For connect mode, reverse mode will allow passing sub-command to connect mode, building yamux server session from child stdin/stdout

yamuxfwd is typically used by firing two instances, one launched as a (an eventual) child process of the other.
```
# listen on 8080, forward traffic to localhost 80
yamuxfwd -l 8080 -- yamuxfwd -c localhost:80
```

as the yamux peers commnicate over pipes, you can also use it via, e.g. SSH (need to upload yamuxfwd to the server first)
```
# listen on 8080, forward traffic to SSH server's localhost 80 (useful when port forwarding is disabled)
yamuxfwd -l 8080 -- ssh user@myserver -- yamuxfwd -c localhost:80
```
yeah. that's three levels of command nesting.

given the SSH use case, reverse mode's utility is obvious
```
# start yamux tunnel, listen on SSH server's 8080 port, forward reverse connection back to local machine localhost:80
yamuxfwd -R -c localhost:80 -- ssh user@myserver -- yamuxfwd -R -l 8080
```

hell you can even emulate `ssh -D`
```
ssh user@server -- ncat -vvk -l 127.0.0.1 -p 1342 --proxy-type http &
yamuxfwd -l 1342 -- ssh user@server -- yamuxfwd -c localhost:1342 &
curl -x http://localhost:1342 https://internal-service/
```

## Alternatives
[yamux-cli](https://github.com/nwtgck/yamux-cli) Similar project. Has UDP support, but you gotta set up named pipes to complete the yamux circuit.

You could also do the SSH forwarding without yamux at all, use SSH control socket (again, because -L,-R,-D are blocked. still need ncat):
```
ssh -M -S /tmp/ssh-%r@%h:%p -fN user@server &
ssh -S /tmp/ssh-%r@%h:%p user@server -- ncat -vv -l 127.0.0.1 -p 1342 --proxy-type http &
ncat -vvklp 1342 -c 'ssh -S /tmp/ssh-%r@%h:%p user@server -- ncat -vv 127.0.0.1 1342' &
curl -x http://localhost:1342 https://internal-service/
```
Can't do reverse tunneling this way though. We need to see if it's possible for sshd to be the one opening new channels, and then you need to find a way to tunnel traffic through it

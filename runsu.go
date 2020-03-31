package main

/*
 * runsu.go
 * SSH to a box and run a thing with su
 * By J. Stuart McMurray
 * Created 20200330
 * Last Modified 20200331
 */

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

/* defPort is the default SSH port */
const defPort = "22"

func main() {
	var (
		user = flag.String(
			"user",
			"sysadmin",
			"Unpriv `username`",
		)
		pass = flag.String(
			"pass",
			"changeme",
			"Unpriv `password`",
		)
		rootPass = flag.String(
			"root-pass",
			"changeme",
			"Root's `password`",
		)
		prompt = flag.String(
			"prompt",
			"password:",
			"Case-insensitive su `prompt`",
		)
		pbLen = flag.Uint(
			"pblen",
			4096,
			"Password buffer `length` "+
				"(including space for the MOTD)",
		)
	)
	flag.Usage = func() {
		fmt.Fprintf(
			os.Stderr,
			`Usage: %v [options] ip[:port] <script

Connects to the target and tries to run the script on-target after running su.

Options:
`,
			os.Args[0],
		)
		flag.PrintDefaults()
	}
	flag.Parse()

	/* Make sure we have an address and port */
	if 0 == flag.NArg() {
		fmt.Fprintf(
			os.Stderr,
			"No address specified, please run with -h for help\n",
		)
		os.Exit(2)
	}
	target := flag.Arg(0)
	_, p, err := net.SplitHostPort(target)
	if nil != err || "" == p {
		target = net.JoinHostPort(target, defPort)
	}

	/* Connect and auth to the server */
	c, err := ssh.Dial("tcp", target, &ssh.ClientConfig{
		User: *user,
		Auth: []ssh.AuthMethod{
			ssh.Password(*pass),
			ssh.KeyboardInteractive(
				func(string, string, []string, []bool) (
					[]string,
					error,
				) {
					return []string{*pass}, nil
				}),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if nil != err {
		log.Fatalf("Error making SSH connection: %v", err)
	}

	/* Get a session and its i/o */
	s, err := c.NewSession()
	if nil != err {
		log.Fatalf("Error requesting session: %v", err)
	}
	ip, err := s.StdinPipe()
	if nil != err {
		log.Fatalf("Error getting session's stdin: %v", err)
	}
	pr, pw := io.Pipe()
	s.Stdout = pw
	s.Stderr = pw

	/* Start a shell in a PTY */
	if err := s.RequestPty("vt100", 0, 0, nil); nil != err {
		log.Fatalf("Error requesty PTY: %v", err)
	}
	if err := s.Shell(); nil != err {
		log.Fatalf("Error requesting shell: %v", err)
	}

	/* Try to su */
	if _, err := fmt.Fprintf(ip, "w && su\n"); nil != err {
		log.Fatalf("Error sending su: %v", err)
	}

	/* Wait for a password prompt */
	var (
		b     = make([]byte, int(*pbLen))
		found bool
		nr    int
	)
	for i := 0; i < len(b); i++ {
		if _, err := pr.Read(b[i : i+1]); nil != err {
			log.Fatalf("Error reading session output: %v", err)
		}
		os.Stdout.Write(b[i : i+1])
		nr++
		if strings.Contains(
			strings.ToLower(string(b[:nr])),
			strings.ToLower(*prompt),
		) {
			found = true
			break
		}
	}
	if !found {
		log.Fatalf(
			"Password prompt not found in first %d of output",
			len(b),
		)
	}

	/* Try to auth as root */
	if _, err := fmt.Fprintf(ip, "%s\r\n\r\n", *rootPass); nil != err {
		log.Fatalf("Error sending root password: %v", err)
	}

	/* Wait until we get another character */
	b = make([]byte, 1)
	if _, err := pr.Read(b); nil != err {
		log.Printf("Error waiting for byte after auth: %v", err)
	}
	os.Stdout.Write(b)

	/* Copy the rest of stdout to the output */
	go io.Copy(os.Stdout, pr)

	/* Pipe the script in as root */
	n, err := io.Copy(ip, os.Stdin)
	if nil != err {
		log.Fatalf("Error copying stdin to target: %v", err)
	}

	/* Try to exit the shell */
	if _, err := fmt.Fprintf(
		ip,
		"\n\nexit\nexit\nexit\n\x04\n\x04\n\x04",
	); nil != err {
		log.Fatalf("Error sending EOTs to target: %v", err)
	}

	/* Be done */
	if err := ip.Close(); nil != err {
		log.Fatalf("Error closing target session's stdin: %v", err)
	}
	if err := s.Wait(); nil != err {
		log.Fatalf("Error waiting for session to end: %v", err)
	}
	log.Printf("Done. Sent %d bytes to target.", n)
}

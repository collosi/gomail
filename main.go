package main

import (
	"fmt"
	"os"
	"net/smtp"
	"io"
	"flag"
	"strings"
	"net/mail"
)

var (
	fUseTLS = flag.Bool("l", true, "use STARTTLS")
	fFrom = flag.String("f", "", "from address")
	fTo = flag.String("t", "", "to address list (comma separated)")
	fCC = flag.String("cc", "", "CC address list (comma separated)")
	fBCC = flag.String("bcc", "", "BCC address list (comma separated)")
	fServer = flag.String("s", "smtp.gmail.com:587", "SMTP server")
	fMessage = flag.String("m", "", "message body (uses stdin if blank)")
	fSubject = flag.String("u", "", "subject")

	fAuth = flag.Bool("a", true, "use SMTP authentication")
	fAuthUser = flag.String("xu", "", "username for SMTP authentication (env var "+GOMAIL_USER+" if blank)")
	fAuthPassword = flag.String("xp", "", "password for SMTP authentication (env var "+GOMAIL_PASS+" if blank)")
)

const(
	CRLF = "\r\n"
	GOMAIL_USER = "GOMAIL_USER"
	GOMAIL_PASS = "GOMAIL_PASS"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: gomail -f=<email> [options]\n")
	flag.PrintDefaults()
	os.Exit(1)
}

func fatal(f string, args ... interface{}) {
	fmt.Fprintf(os.Stderr, f, args...)
	os.Exit(1)
}

func main() {
	flag.Parse()

	mustNotCRLF(*fFrom)
	mustNotCRLF(*fTo)
	mustNotCRLF(*fSubject)
	mustNotBlank(*fFrom)
	mustNotBlank(*fServer)

	fromList, err := parseAddressList(*fFrom)
	if len(fromList) != 1 {
		fmt.Fprintf(os.Stderr, "%s\n", "Only one from address allowed")
		usage()
	}
	from := fromList[0]

	tos, err := parseAddressList(*fTo)
	if err != nil {
		fatal(`%v: error parsing "to" list\n`, err)
	}
	ccs, err := parseAddressList(*fCC)
	if err != nil {
		fatal(`%v: error parsing "cc" list\n`, err)
	}
	bccs, err := parseAddressList(*fBCC)
	if err != nil {
		fatal(`%v: error parsing "bcc" list\n`, err)
	}

	user, pass := getCredentials(*fAuthUser, *fAuthPassword)

	client, err := smtp.Dial(*fServer)
	if err != nil {
		fatal("%v: error connecting to %s\n", err, *fServer)
	}
    if ok, _ := client.Extension("STARTTLS"); ok {
		err = client.StartTLS(nil)
		if err != nil {
			fatal("%v: error starting TLS\n", err)
		}
	}

	host := (*fServer)[:strings.Index(*fServer, ":")]

	if ok, _ := client.Extension("AUTH"); ok {
		err = client.Auth(smtp.PlainAuth("", user, pass, host))
		if err != nil {
			fatal("%v: error authenticating '%s'\n", err, user)
		}
	} else if user != "" || pass != "" {
		fmt.Fprintf(os.Stderr, "WARN: credentials supplied but server does not support authentication")
	}

	defer func () {
		err := client.Quit()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v: error during quit", err)
		}
	}()

	err = client.Mail(from.Address)
	if err != nil {
		fatal("%v: error specifying mail from\n", err)
	}

	recipientEmails := emailsOnly(tos, ccs, bccs)
	for _, to := range recipientEmails {
		err = client.Rcpt(to)
		if err != nil {
			fatal("%v: error specifying recipient\n", err)
		}
	}

	out, err := client.Data()
	if err != nil {
		fatal("%v: error outputting data\n", err)
	}
	defer out.Close()

	io.WriteString(out, "From: "+from.String()+CRLF)
	for _, to := range tos {
		io.WriteString(out, "To: "+to.String()+CRLF)
	}
	for _, cc := range ccs {
		io.WriteString(out, "CC: "+cc.String()+CRLF)
	}
	if *fSubject != "" {
		io.WriteString(out, "Subject: "+(*fSubject)+CRLF)		
	}
	io.WriteString(out, CRLF)

	if *fMessage == "" {
		io.Copy(out, os.Stdin)
	} else {
		_, err = io.WriteString(out, *fMessage)
		if err != nil {
			fatal("%v: error outputting data\n", err)
		}
	}
}

// Returns as normal if the given string does not contain
// either a CR or LF character.  Otherwise prints error message
// and exits.
func mustNotCRLF(s string) {
	if strings.IndexAny(s, CRLF) > 0 {
		fatal("From, To and Subject must not contain CR or LF")
	}
}

// Returns as normal if s is not "".  Otherwise prints the
// command usage and exits
func mustNotBlank(s string) {
	if s == "" {
		usage()
	}
}

// Returns values for the username and password.  If the passed
// values are "", will look for appropriate values in the environment
// vars "GOMAIL_USER" and "GOMAIL_PASS"
func getCredentials(u, p string) (user, pass string) {
	if u == "" {
		user = os.Getenv(GOMAIL_USER)	
	} else {
		user = u
	}
	if p == "" {
		pass = os.Getenv(GOMAIL_PASS)
	} else {
		pass = p
	}
	return
}

func parseAddressList(l string) ([]*mail.Address, error) {
	// TODO: this is a hack, but is the best way to do it until
	// this CL is accepted: http://codereview.appspot.com/5676067/
	if l == "" {
		return []*mail.Address{}, nil
	}

	key := "_"
	htemp := make(mail.Header)
	htemp[key] = []string{l}
	return htemp.AddressList(key)
}

func emailsOnly(lists ... []*mail.Address) (emails []string) {
	emails = make([]string, 0)
	for _, list := range lists {
		for _, addr := range list {
			emails = append(emails, addr.Address)
		}
	}
	return
}
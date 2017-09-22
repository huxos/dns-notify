package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type apiNotifyResponse struct {
	Error  string
	Result []NotifyResponse
}

// NotifyResponse is returned by the API (as a list, since there can
// be multiple responses)
type NotifyResponse struct {
	Server string
	Result string
	Error  bool
}

var (
	domainFlag = flag.String("domain", "", "Domain to notify")
	verbose    = flag.Bool("verbose", false, "Be extra verbose")
	quiet      = flag.Bool("quiet", false, "Only output on errors")
	timeout    = flag.Int64("timeout", 2000, "Timeout for response (in milliseconds)")
)

var servers []string

func main() {

	flag.Parse()

	servers = flag.Args()

	if len(*domainFlag) == 0 {
		fmt.Printf("-domain parameter required\n\n")
		flag.Usage()
		os.Exit(2)
	}

	sendNotify(servers, *domainFlag)
}

func sendNotify(servers []string, domain string) []NotifyResponse {

	if !strings.HasSuffix(domain, ".") {
		domain = domain + "."
	}

	if len(servers) == 0 {
		fmt.Println("No servers")
		resp := NotifyResponse{Result: "No servers", Error: true}
		fmt.Println("No servers")
		return []NotifyResponse{resp}
	}

	c := new(dns.Client)

	c.ReadTimeout = time.Duration(*timeout) * time.Millisecond

	m := new(dns.Msg)
	m.SetNotify(domain)

	responseChannel := make(chan NotifyResponse, len(servers))

	for _, server := range servers {

		go func(server string) {

			result := NotifyResponse{Server: server}

			defer func() {
				if result.Error || !*quiet {
					fmt.Printf("%s: %s\n", result.Server, result.Result)
				}
				responseChannel <- result
			}()

			target, err := fixupHost(server)
			if err != nil {
				result.Result = fmt.Sprintf("%s: %s", server, err)
				fmt.Println(result.Result)
				result.Error = true
				return
			}

			result.Server = target

			if *verbose {
				fmt.Println("Sending notify to", target)
			}

			resp, rtt, err := c.Exchange(m, target)

			if err != nil {
				result.Error = true
				result.Result = err.Error()
				return
			}

			ok := "ok"
			if !resp.Authoritative {
				ok = fmt.Sprintf("not ok (%s)", dns.RcodeToString[resp.Rcode])
			}

			result.Result = fmt.Sprintf("%s: %s (%s)",
				target, ok, rtt.String())

		}(server)

	}

	responses := make([]NotifyResponse, len(servers))

	for i := 0; i < len(servers); i++ {
		responses[i] = <-responseChannel
	}

	return responses

}

func fixupHost(s string) (string, error) {

	_, _, err := net.SplitHostPort(s)
	if err != nil && strings.HasPrefix(err.Error(), "missing port in address") {
		return s + ":53", nil
	}
	if err != nil {
		return "", err
	}

	// input was ok ...
	return s, nil

}

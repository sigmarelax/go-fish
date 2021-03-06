package main

import (
	"regexp"
	"time"

	es "github.com/patrobinson/go-fish/examples/certstream/eventStructs"
	"github.com/patrobinson/go-fish/output"
)

func main() {}

type domainCertIssued struct {
	domainName *regexp.Regexp
}

func (d *domainCertIssued) Init() {
	var err error
	d.domainName, err = regexp.Compile("^www.*")
	if err != nil {
		panic("Unable to compile domain regex")
	}
}

func (d *domainCertIssued) WindowInterval() int {
	return 0
}

func (d *domainCertIssued) Window() ([]output.OutputEvent, error) {
	return []output.OutputEvent{}, nil
}

func (d *domainCertIssued) Close() {}

func (d *domainCertIssued) Process(thing interface{}) interface{} {
	issuedCert, ok := thing.(es.CertStream)
	if !ok {
		return nil
	}
	if issuedCert.MessageType != "heartbeat" {
		for _, domain := range issuedCert.Data.LeafCert.AllDomains {
			if d.domainName.MatchString(domain) {
				return output.OutputEvent{
					Source:    "CertStream",
					EventTime: time.Unix(int64(issuedCert.Data.Seen), 0),
					EventType: issuedCert.MessageType,
					Name:      "DomainNameSeenInCertificate",
					Level:     output.InfoLevel,
					EventId:   string(issuedCert.Data.CertIndex),
					Entity:    issuedCert.Data.Source.Name,
				}
			}
		}
	}
	return nil
}

func (r *domainCertIssued) String() string { return "domainCertIssued" }

var Rule domainCertIssued

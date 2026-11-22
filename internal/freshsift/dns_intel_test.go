package freshsift

import (
 "context"
 "net"
 "strings"
 "testing"
)

func TestDNSIntelConfigOptionsAndStandalone(t *testing.T) {
 c := DefaultConfig()
 c.Providers=nil
 c.Seeds="https://example.org"
 c.Patterns=""
 c.DiscoverDNSRecords=true
 c.EnumerateSubdomains=false
 if err:=c.Normalize();err!=nil {t.Fatal(err)}
 c.DiscoverDNSRecords=false
 c.EnumerateSubdomains=true
 c.SubdomainGuess=false
 c.SubdomainUseCT=true
 if err:=c.Normalize();err!=nil {t.Fatal(err)}
 c.SubdomainUseCT=false
 if err:=c.Normalize();err==nil {t.Fatal("empty subdomain enumeration mode accepted")}
 c.EnumerateSubdomains=false
 c.Patterns="abc"
 c.ReverseDNSPages=true
 c.ReverseDNSPageDepth=9
 if err:=c.Normalize();err==nil {t.Fatal("reverse crawl depth >8 accepted")}
 c.ReverseDNSPageDepth=2
 c.ReverseDNSSubdomains=true
 c.ReverseDNSSubdomainDepth=3
 if err:=c.Normalize();err!=nil {t.Fatal(err)}
}

func TestPTRRequiresForwardMatchAndDeduplicates(t *testing.T) {
 ptr:=func(_ context.Context,ip string)([]string,error){
  if ip!="8.8.8.8" {t.Fatalf("wrong IP: %s",ip)}
  return []string{"One.example.org.","other.example.org.","ONE.EXAMPLE.ORG.","bad_name.example.org."},nil
 }
 forward:=func(_ context.Context,hostname string)([]net.IP,error){
  if hostname=="one.example.org" {return []net.IP{net.ParseIP("8.8.8.8")},nil}
  return []net.IP{net.ParseIP("1.1.1.1")},nil
 }
 out:=verifiedPTRHosts(context.Background(),net.ParseIP("8.8.8.8"),ptr,forward)
 if len(out)!=1||out[0]!="one.example.org" {t.Fatalf("unverified PTR accepted or valid missing: %v",out)}
}

func TestDNSRecordMatchesTextAndSource(t *testing.T) {
 cfg:=DefaultConfig()
 cfg.Patterns="mail"
 cfg.Providers=nil
 cfg.Seeds="https://example.org"
 if err:=cfg.Normalize();err!=nil {t.Fatal(err)}
 patterns,err:=compileMatchers(cfg);if err!=nil{t.Fatal(err)}
 records:=[]DNSRecord{{Domain:"example.org",Type:"MX",Value:"10 mail.example.org"}}
 hits:=dnsMatches(records,patterns)
 if len(hits)!=1||!strings.Contains(hits[0].Matched,"mail") {t.Fatalf("no DNS value match: %+v",hits)}
}

func TestReverseCandidateDepthIsIndependent(t *testing.T) {
 cfg:=DefaultConfig()
 cfg.Depth=0
 cfg.MaxPathSegments=8
 item:=Candidate{URL:"https://ptr.example.org/",ReverseOrigin:"page",ReverseDepth:2,ReverseHost:"ptr.example.org",Depth:1}
 if !reverseChildAllowed(cfg,item,"https://ptr.example.org/deep",false){t.Fatal("reverse depth ignored")}
 item.Depth=2
 if reverseChildAllowed(cfg,item,"https://ptr.example.org/deep",false){t.Fatal("reverse depth exceeded")}
 item.Depth=0
 if reverseChildAllowed(cfg,item,"https://other.example.org/",false){t.Fatal("reverse scope escaped")}
}

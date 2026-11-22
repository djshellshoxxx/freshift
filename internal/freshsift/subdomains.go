package freshsift

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// Subdomain represents a discovered hostname. CT names are historical observations,
// while DNS names have a current address record at lookup time.
type Subdomain struct {
	Hostname  string   `json:"hostname"`
	Source    string   `json:"source"`
	Verified  bool     `json:"verified"`
	Addresses []string `json:"addresses,omitempty"`
}

// Curated labels. The site owner may add a larger TXT/CSV wordlist at scan time.
// Keep the browser list in web/subdomains.js synchronized with this list.
const commonSubdomainWordlist = `www
mail
webmail
smtp
imap
pop
pop3
mx
mx1
mx2
ns
ns1
ns2
ns3
dns
dns1
dns2
ftp
sftp
ssh
vpn
remote
gateway
gw
router
firewall
proxy
reverse-proxy
api
api1
api2
api-dev
api-staging
api-test
v1
v2
graphql
rest
auth
login
sso
oauth
accounts
account
id
identity
admin
administrator
portal
dashboard
panel
control
console
management
manage
app
apps
application
web
mobile
m
beta
alpha
dev
development
test
testing
qa
uat
staging
stage
preprod
preview
prod
production
internal
intranet
private
secure
public
edge
cdn
static
assets
media
images
img
files
file
download
downloads
upload
uploads
storage
s3
bucket
backup
backups
archive
archives
db
database
mysql
postgres
postgresql
mongo
mongodb
redis
cache
memcached
elastic
elasticsearch
search
solr
index
logs
logging
log
metrics
monitor
monitoring
status
health
healthcheck
grafana
prometheus
kibana
jenkins
ci
cd
git
github
gitlab
gitea
registry
docker
k8s
kubernetes
cluster
node
worker
build
deploy
releases
release
docs
documentation
wiki
help
support
desk
ticket
tickets
issues
blog
news
forum
community
shop
store
cart
checkout
pay
payment
payments
billing
invoice
crm
erp
hr
staff
employee
employees
partners
partner
customer
customers
client
clients
office
home
old
legacy
new
www1
www2
www3
web1
web2
server
server1
server2
host
host1
host2
origin
origin1
origin2
lb
loadbalancer
www-dev
dev-api
staging-api
test-api
test1
test2
demo
sandbox
lab
labs
research
testbed
mail1
mail2
autodiscover
autoconfig
exchange
owa
outlook
sip
lync
teams
meeting
meet
video
voice
voip
pbx
chat
irc
irc1
irc2
matrix
ircweb
xmpp
jira
confluence
bitbucket
redmine
nextcloud
cloud
drive
data
analytics
report
reports
stats
statistics
events
track
tracking
maps
map
geo
location
calendar
schedule
updates
update
patch
patches
mirror
mirrors
repo
repos
repository
packages
pkg
npm
pypi
maven
artifacts
artifactory
nexus
wordpress
wp
cms
cpanel
whm
plesk
directadmin
phpmyadmin
pma
adminer
api-gateway
hooks
webhook
webhooks
callback
callbacks
notification
notifications
push
socket
ws
wss
stream
streaming
live
realtime
relay
relay1
relay2
connect
connectivity
ping
speedtest
ntp
time
directory
ldap
ad
dc
dc1
dc2
rdp
terminal
bastion
jump
jumpbox
citrix
vdi
desktop
workstation
printer
print
scan
scanner
cam
camera
cameras
iot
device
devices
sensor
sensors
power
ups
nas
san
backup1
backup2`

func validSubdomainLabel(label string) bool {
	if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for _, ch := range label {
		if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
			return false
		}
	}
	return true
}

func validSubdomainHost(hostname, root string) bool {
	hostname = strings.ToLower(strings.TrimSuffix(hostname, "."))
	root = strings.ToLower(strings.TrimSuffix(root, "."))
	if len(hostname) > 253 || len(root) > 253 || !strings.HasSuffix(hostname, "."+root) {
		return false
	}
	for _, label := range strings.Split(hostname, ".") {
		if !validSubdomainLabel(label) {
			return false
		}
	}
	return true
}

func validSubdomainRoot(root string) bool {
	if len(root) > 253 || !strings.Contains(root, ".") || net.ParseIP(root) != nil {
		return false
	}
	for _, label := range strings.Split(strings.ToLower(root), ".") {
		if !validSubdomainLabel(label) {
			return false
		}
	}
	return true
}

func parseSubdomainWords(raw string, builtin bool) ([]string, error) {
	if len(raw) > 200000 {
		return nil, errors.New("subdomain wordlist exceeds 200 KB")
	}
	seen := make(map[string]bool)
	out := []string{}
	add := func(source string) error {
		for _, line := range strings.Split(source, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "#") || line == "" {
				continue
			}
			for _, part := range strings.FieldsFunc(line, func(ch rune) bool {
				return ch == ',' || ch == ';' || ch == '\r' || ch == '\t' || ch == ' '
			}) {
				word := strings.Trim(strings.ToLower(strings.TrimSpace(part)), "\"'.")
				if word == "" || seen[word] || len(word) > 240 {
					continue
				}
				valid := true
				for _, label := range strings.Split(word, ".") {
					if !validSubdomainLabel(label) {
						valid = false
						break
					}
				}
				if !valid {
					continue
				}
				seen[word] = true
				out = append(out, word)
				if len(out) > 5000 {
					return errors.New("subdomain wordlist exceeds 5,000 distinct labels")
				}
			}
		}
		return nil
	}
	if builtin {
		if err := add(commonSubdomainWordlist); err != nil {
			return nil, err
		}
	}
	if err := add(raw); err != nil {
		return nil, err
	}
	return out, nil
}

func parseCTSubdomains(body []byte, root string, limit int) []string {
	if limit < 1 {
		return nil
	}
	var rows []struct {
		NameValue string `json:"name_value"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil
	}
	root = strings.ToLower(strings.TrimSuffix(root, "."))
	seen := make(map[string]bool)
	out := []string{}
	for _, row := range rows {
		for _, raw := range strings.Split(row.NameValue, "\n") {
			name := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "*.")))
			name = strings.TrimSuffix(name, ".")
			if !validSubdomainHost(name, root) || seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name)
			if len(out) >= limit {
				return out
			}
		}
	}
	return out
}

func sameAddresses(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	x, y := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

type lookupSubdomain func(context.Context, string) ([]string, error)

func lookupPublicDNS(ctx context.Context, hostname string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	return net.DefaultResolver.LookupHost(ctx, hostname)
}

// resolveWordlist tests candidate names against DNS and suppresses results
// identical to two unrelated control names at a wildcard DNS zone.
func resolveWordlist(ctx context.Context, root string, words []string, limit int, lookup lookupSubdomain) []Subdomain {
	if limit < 1 || len(words) == 0 || ctx.Err() != nil {
		return nil
	}
	nonce := fmt.Sprintf("%x", time.Now().UnixNano())
	probe1, err1 := lookup(ctx, "freshsift-"+nonce+"-a."+root)
	probe2, err2 := lookup(ctx, "freshsift-"+nonce+"-b."+root)
	wildcard := []string(nil)
	if err1 == nil && err2 == nil && sameAddresses(probe1, probe2) {
		wildcard = probe1
	}
	jobs := make(chan string)
	results := make(chan Subdomain, len(words))
	var workers sync.WaitGroup
	n := 8
	if len(words) < n {
		n = len(words)
	}
	for i := 0; i < n; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for word := range jobs {
				if ctx.Err() != nil {
					return
				}
				hostname := word + "." + root
				if !validSubdomainHost(hostname, root) {
					continue
				}
				addrs, err := lookup(ctx, hostname)
				if err != nil || len(addrs) == 0 || sameAddresses(addrs, wildcard) {
					continue
				}
				results <- Subdomain{Hostname: hostname, Source: "dns", Verified: true, Addresses: addrs}
			}
		}()
	}
sendWords:
	for _, word := range words {
		select {
		case <-ctx.Done():
			break sendWords
		case jobs <- word:
		}
	}
	close(jobs)
	workers.Wait()
	close(results)
	out := []Subdomain{}
	for row := range results {
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Hostname < out[j].Hostname })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func discoverSubdomains(ctx context.Context, c Config, seeds []Candidate, client *http.Client, emit func(Subdomain), log sourceLog) ([]Candidate, error) {
	if !c.EnumerateSubdomains {
		return nil, nil
	}
	words := []string{}
	if c.SubdomainGuess == nil || *c.SubdomainGuess {
		var err error
		words, err = parseSubdomainWords(c.SubdomainWords, c.SubdomainUseBuiltIn)
		if err != nil {
			return nil, err
		}
	}
	roots := []string{}
	visited := map[string]bool{}
	for _, seed := range seeds {
		u, e := url.Parse(seed.URL)
		if e != nil {
			continue
		}
		root := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
		if !validSubdomainRoot(root) || visited[root] {
			continue
		}
		visited[root] = true
		roots = append(roots, root)
		if len(roots) >= 5 {
			break
		}
	}
	remaining := c.SubdomainLimit
	out := []Candidate{}
	for _, root := range roots {
		if ctx.Err() != nil || remaining <= 0 {
			break
		}
		known := map[string]bool{}
		dnsCount := 0
		for _, row := range resolveWordlist(ctx, root, words, remaining, lookupPublicDNS) {
			known[row.Hostname] = true
			dnsCount++
			emit(row)
			out = append(out, Candidate{URL: "https://" + row.Hostname + "/", Source: "subdomain-dns"})
			remaining--
		}
		if c.SubdomainUseCT && ctx.Err() == nil && remaining > 0 {
			endpoint := "https://crt.sh/?q=" + url.QueryEscape("%."+root) + "&output=json"
			var rows json.RawMessage
			if e := discoveryRequest(ctx, client, endpoint, nil, &rows); e != nil {
				log("Subdomain CT " + root + ": " + e.Error() + " (continuing with DNS results)")
			} else {
				count := 0
				for _, name := range parseCTSubdomains(rows, root, c.SubdomainLimit) {
					if remaining <= 0 {
						break
					}
					if known[name] {
						continue
					}
					known[name] = true
					emit(Subdomain{Hostname: name, Source: "ct", Verified: false})
					out = append(out, Candidate{URL: "https://" + name + "/", Source: "subdomain-ct"})
					remaining--
					count++
				}
				log(fmt.Sprintf("Subdomain CT %s: %d historical certificate hostnames (not DNS-verified)", root, count))
			}
		}
		log(fmt.Sprintf("Subdomain inventory %s: %d DNS-confirmed names; %d total discovered", root, dnsCount, len(out)))
	}
	return out, nil
}

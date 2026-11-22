package freshsift

// A deliberately small read-only, anonymous, passive-mode FTP client.
// No active-mode connections, credentialed logins, uploads or arbitrary commands.
// Every passive data connection targets the validated control-server IP, never
// the arbitrary address advertised by a PASV reply (FTP bounce protection).

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ftpSession struct {
	conn   net.Conn
	reader *bufio.Reader
	ip     net.IP
	ctx    context.Context
	limit  time.Time
}

func openFTP(ctx context.Context, target string, allowPrivate bool, timeout time.Duration) (*ftpSession, *url.URL, error) {
	normalized, err := canonicalURL(target)
	if err != nil {
		return nil, nil, err
	}
	u, _ := url.Parse(normalized)
	if u.Scheme != "ftp" {
		return nil, nil, errors.New("FTP URL required")
	}
	ip, err := validatedTarget(ctx, u.Hostname(), allowPrivate)
	if err != nil {
		return nil, nil, err
	}
	port := u.Port()
	if port == "" {
		port = "21"
	}
	if p, e := strconv.Atoi(port); e != nil || p < 1 || p > 65535 {
		return nil, nil, errors.New("invalid FTP port")
	}
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), port))
	if err != nil {
		return nil, nil, err
	}
	limit := time.Now().Add(timeout)
	if deadline, yes := ctx.Deadline(); yes && deadline.Before(limit) {
		limit = deadline
	}
	conn.SetDeadline(limit)
	ftp := &ftpSession{conn: conn, reader: bufio.NewReader(conn), ip: ip, ctx: ctx, limit: limit}
	closeErr := func(e error) (*ftpSession, *url.URL, error) { conn.Close(); return nil, nil, e }
	code, _, err := ftp.readReply()
	if err != nil || code != 220 {
		return closeErr(fmt.Errorf("FTP welcome: status %d: %w", code, err))
	}
	code, _, err = ftp.command("USER anonymous")
	if err != nil {
		return closeErr(err)
	}
	if code == 331 {
		code, _, err = ftp.command("PASS anonymous@freshsift.local")
	}
	if err != nil || code != 230 {
		return closeErr(fmt.Errorf("FTP anonymous login denied: status %d: %w", code, err))
	}
	if code, _, err = ftp.command("TYPE I"); err != nil || code != 200 {
		return closeErr(fmt.Errorf("FTP binary mode: status %d: %w", code, err))
	}
	return ftp, u, nil
}
func (f *ftpSession) close() {
	if f.conn != nil {
		f.conn.Close()
	}
}
func (f *ftpSession) readReply() (int, string, error) {
	line, err := f.reader.ReadString('\n')
	if err != nil {
		return 0, "", err
	}
	line = strings.TrimRight(line, "\r\n")
	if len(line) < 3 {
		return 0, "", errors.New("short FTP status")
	}
	code, err := strconv.Atoi(line[:3])
	if err != nil {
		return 0, "", errors.New("malformed FTP status")
	}
	lines := []string{line}
	if len(line) > 3 && line[3] == '-' {
		for i := 0; i < 100; i++ {
			next, e := f.reader.ReadString('\n')
			if e != nil {
				return 0, "", e
			}
			next = strings.TrimRight(next, "\r\n")
			lines = append(lines, next)
			if strings.HasPrefix(next, fmt.Sprintf("%03d ", code)) {
				return code, strings.Join(lines, "\n"), nil
			}
		}
		return 0, "", errors.New("FTP multiline reply too long")
	}
	return code, line, nil
}
func (f *ftpSession) command(cmd string) (int, string, error) {
	if strings.ContainsAny(cmd, "\r\n") {
		return 0, "", errors.New("FTP command contains newline")
	}
	if _, err := fmt.Fprintf(f.conn, "%s\r\n", cmd); err != nil {
		return 0, "", err
	}
	return f.readReply()
}

var ftpEPSV = regexp.MustCompile(`\(\|\|\|(\d+)\|\)`)
var ftpPASV = regexp.MustCompile(`\((\d+),(\d+),(\d+),(\d+),(\d+),(\d+)\)`)

func (f *ftpSession) passive() (net.Conn, error) {
	code, message, err := f.command("EPSV")
	if err != nil {
		return nil, err
	}
	port := 0
	if code == 229 {
		m := ftpEPSV.FindStringSubmatch(message)
		if len(m) == 2 {
			port, _ = strconv.Atoi(m[1])
		}
	}
	if port == 0 {
		code, message, err = f.command("PASV")
		if err != nil {
			return nil, err
		}
		if code != 227 {
			return nil, fmt.Errorf("passive FTP unavailable (%d)", code)
		}
		m := ftpPASV.FindStringSubmatch(message)
		if len(m) != 7 {
			return nil, errors.New("invalid PASV response")
		}
		high, _ := strconv.Atoi(m[5])
		low, _ := strconv.Atoi(m[6])
		port = high*256 + low
	}
	if port < 1 || port > 65535 {
		return nil, errors.New("invalid passive FTP port")
	}
	dialer := &net.Dialer{Timeout: time.Until(f.limit)}
	conn, err := dialer.DialContext(f.ctx, "tcp", net.JoinHostPort(f.ip.String(), strconv.Itoa(port)))
	if err != nil {
		return nil, err
	}
	conn.SetDeadline(f.limit)
	return conn, nil
}
func (f *ftpSession) transfer(cmd string, maxBytes int64) ([]byte, int, error) {
	data, err := f.passive()
	if err != nil {
		return nil, 0, err
	}
	defer data.Close()
	code, _, err := f.command(cmd)
	if err != nil {
		return nil, code, err
	}
	if code != 125 && code != 150 {
		return nil, code, fmt.Errorf("FTP %s: status %d", strings.Fields(cmd)[0], code)
	}
	contents, err := io.ReadAll(io.LimitReader(data, maxBytes+1))
	data.Close()
	if err != nil {
		return nil, code, err
	}
	if len(contents) > int(maxBytes) {
		return nil, code, fmt.Errorf("FTP response exceeds %d bytes", maxBytes)
	}
	code, _, err = f.readReply()
	if err != nil {
		return nil, code, err
	}
	if code != 226 && code != 250 {
		return nil, code, fmt.Errorf("FTP transfer ended with status %d", code)
	}
	return contents, code, nil
}
func ftpListURLs(base string, listing []byte, mlsd bool) []string {
	var links []string
	seen := map[string]bool{}
	for _, line := range strings.Split(string(listing), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		name, dir := "", false
		if mlsd {
			i := strings.IndexByte(line, ' ')
			if i < 0 {
				continue
			}
			facts := strings.ToLower(line[:i])
			name = strings.TrimSpace(line[i+1:])
			if strings.Contains(facts, "type=cdir;") || strings.Contains(facts, "type=pdir;") {
				continue
			}
			dir = strings.Contains(facts, "type=dir;")
			if !dir && !strings.Contains(facts, "type=file;") {
				continue
			}
		} else {
			fields := strings.Fields(line)
			if len(fields) >= 9 && (strings.HasPrefix(fields[0], "d") || strings.HasPrefix(fields[0], "-")) {
				dir = strings.HasPrefix(fields[0], "d")
				name = strings.Join(fields[8:], " ")
			} else if len(fields) >= 4 && (strings.Contains(strings.ToUpper(fields[2]), "<DIR>") || strings.Contains(strings.ToUpper(fields[2]), "DIR")) {
				dir = true
				name = strings.Join(fields[3:], " ")
			} else {
				continue
			}
		}
		if name == "." || name == ".." || name == "" || strings.ContainsAny(name, "/\\\r\n\x00") {
			continue
		}
		raw := strings.TrimSuffix(base, "/") + "/" + url.PathEscape(name)
		if dir {
			raw += "/"
		}
		normalized, err := canonicalURL(raw)
		if err == nil && !seen[normalized] {
			seen[normalized] = true
			links = append(links, normalized)
		}
		if len(links) >= 3000 {
			break
		}
	}
	return links
}
func fetchFTP(ctx context.Context, target string, allowPrivate bool, timeout time.Duration) (fetchResult, error) {
	var result fetchResult
	ftp, u, err := openFTP(ctx, target, allowPrivate, timeout)
	if err != nil {
		return result, err
	}
	defer ftp.close()
	filePath := u.EscapedPath()
	if filePath == "" {
		filePath = "/"
	}
	// Never accept embedded URL userinfo, even for anonymous FTP.
	code, _, err := ftp.command("CWD " + u.Path)
	if err != nil {
		return result, err
	}
	if code == 250 {
		dirURL := u.String()
		if !strings.HasSuffix(u.Path, "/") {
			u.Path += "/"
			dirURL = u.String()
		}
		contents, _, listErr := ftp.transfer("MLSD", MaxResponseBytes)
		mlsd := listErr == nil
		if listErr != nil {
			contents, _, listErr = ftp.transfer("LIST", MaxResponseBytes)
		}
		if listErr != nil {
			return result, listErr
		}
		result = fetchResult{URL: dirURL, Body: string(contents), Size: len(contents), Status: 200, ContentType: "text/x-ftp-listing", IsDir: true, Links: ftpListURLs(dirURL, contents, mlsd)}
		return result, nil
	}
	if strings.HasSuffix(filePath, "/") {
		return result, errors.New("FTP directory is unavailable")
	}
	contents, _, err := ftp.transfer("RETR "+u.Path, MaxResponseBytes)
	if err != nil {
		return result, err
	}
	return fetchResult{URL: u.String(), Body: string(contents), Size: len(contents), Status: 200, ContentType: "application/octet-stream"}, nil
}
func probeFTPFile(ctx context.Context, target string, allowPrivate bool, timeout time.Duration) (fetchResult, error) {
	var result fetchResult
	ftp, u, err := openFTP(ctx, target, allowPrivate, timeout)
	if err != nil {
		return result, err
	}
	defer ftp.close()
	if strings.HasSuffix(u.Path, "/") {
		return result, errors.New("not an FTP file")
	}
	code, _, err := ftp.command("SIZE " + u.Path)
	if err == nil && code == 213 {
		return fetchResult{URL: u.String(), Status: 200, ContentType: "application/octet-stream"}, nil
	}
	// SIZE is optional; accept a successful transfer start, without downloading the file.
	data, err := ftp.passive()
	if err != nil {
		return result, err
	}
	defer data.Close()
	code, _, err = ftp.command("RETR " + u.Path)
	if err != nil || code != 125 && code != 150 {
		return result, fmt.Errorf("FTP file not available (%d): %w", code, err)
	}
	return fetchResult{URL: u.String(), Status: 200, ContentType: "application/octet-stream"}, nil
}

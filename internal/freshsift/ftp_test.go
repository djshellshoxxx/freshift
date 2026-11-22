package freshsift

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func startFakeFTP(t *testing.T) (string, *atomic.Int64) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int64
	var wg sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer conn.Close()
				reader := bufio.NewReader(conn)
				fmt.Fprint(conn, "220 Fake FTP ready\r\n")
				var dataListener net.Listener
				defer func() {
					if dataListener != nil {
						dataListener.Close()
					}
				}()
				for {
					line, e := reader.ReadString('\n')
					if e != nil {
						return
					}
					line = strings.TrimSpace(line)
					command, param, _ := strings.Cut(line, " ")
					switch strings.ToUpper(command) {
					case "USER":
						fmt.Fprint(conn, "331 Password needed\r\n")
					case "PASS":
						fmt.Fprint(conn, "230 Anonymous welcome\r\n")
					case "TYPE":
						fmt.Fprint(conn, "200 Binary type set\r\n")
					case "EPSV":
						if dataListener != nil {
							dataListener.Close()
						}
						dataListener, e = net.Listen("tcp", "127.0.0.1:0")
						if e != nil {
							return
						}
						port, _ := strconv.Atoi(strings.Split(dataListener.Addr().String(), ":")[1])
						fmt.Fprintf(conn, "229 Entering Extended Passive Mode (|||%d|)\r\n", port)
					case "CWD":
						if param == "/" || param == "/pub" || param == "/pub/" {
							fmt.Fprint(conn, "250 Directory changed\r\n")
						} else {
							fmt.Fprint(conn, "550 Not a directory\r\n")
						}
					case "MLSD":
						fmt.Fprint(conn, "150 Sending file listing\r\n")
						if dataListener == nil {
							return
						}
						dataListener.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second))
						data, e := dataListener.Accept()
						if e != nil {
							return
						}
						fmt.Fprint(data, "type=file;size=31; data.db\r\ntype=file;size=9000000; new.jpg\r\ntype=file;size=30; ignore.asx\r\n")
						data.Close()
						dataListener.Close()
						dataListener = nil
						fmt.Fprint(conn, "226 Transfer complete\r\n")
					case "RETR":
						requests.Add(1)
						if param != "/pub/data.db" && param != "/data.db" && param != "/pub/new.jpg" {
							fmt.Fprint(conn, "550 Not found\r\n")
							continue
						}
						fmt.Fprint(conn, "150 Starting transfer\r\n")
						if dataListener == nil {
							return
						}
						dataListener.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second))
						data, e := dataListener.Accept()
						if e != nil {
							return
						}
						if param == "/pub/new.jpg" {
							fmt.Fprint(data, "jpeg bytes")
						} else {
							fmt.Fprint(data, "\x00\x01private needle FTP test\x03")
						}
						data.Close()
						dataListener.Close()
						dataListener = nil
						fmt.Fprint(conn, "226 Transfer complete\r\n")
					case "SIZE":
						requests.Add(1)
						if param == "/pub/new.jpg" || param == "/new.jpg" {
							fmt.Fprint(conn, "213 9000000\r\n")
						} else {
							fmt.Fprint(conn, "550 No file\r\n")
						}
					case "QUIT":
						fmt.Fprint(conn, "221 Bye\r\n")
						return
					default:
						fmt.Fprint(conn, "502 Command not implemented\r\n")
					}
				}
			}()
		}
	}()
	t.Cleanup(func() { listener.Close(); <-done; wg.Wait() })
	return "ftp://" + listener.Addr().String() + "/pub/", &requests
}
func TestFTPCrawlAnonymousPassiveAndBinaryDB(t *testing.T) {
	seed, _ := startFakeFTP(t)
	c := DefaultConfig()
	c.Seeds = seed
	c.Providers = nil
	c.Patterns = "needle"
	c.SearchMode = "source"
	c.Depth = 0
	c.Concurrency = 2
	c.AllowPrivate = true
	c.ExcludeFileTypes = ".asx,.jpg"
	e := NewEngine()
	if err := e.Start(c); err != nil {
		t.Fatal(err)
	}
	result := waitUntilFinished(t, e)
	if result.Stats.Matches != 1 {
		t.Fatalf("FTP .db byte scan failed: %+v results=%+v logs=%+v", result.Stats, result.Matches, result.Logs)
	}
	if !strings.Contains(result.Matches[0].URL, "data.db") {
		t.Fatalf("wrong FTP match: %+v", result.Matches)
	}
}
func TestFTPFiletypeDiscoveryUsesSizeNotDownload(t *testing.T) {
	seed, requests := startFakeFTP(t)
	c := DefaultConfig()
	c.Seeds = seed
	c.Providers = nil
	c.Patterns = ""
	c.SearchMode = "files"
	c.FileTypes = "jpg"
	c.Depth = 0
	c.AllowPrivate = true
	c.ExcludeFileTypes = ".asx"
	e := NewEngine()
	if err := e.Start(c); err != nil {
		t.Fatal(err)
	}
	result := waitUntilFinished(t, e)
	if result.Stats.FilesFound != 1 || result.Stats.Matches != 1 || !strings.Contains(result.Matches[0].URL, ".jpg") {
		t.Fatalf("FTP image discovery: %+v, matches %v logs %v", result.Stats, result.Matches, result.Logs)
	}
	if requests.Load() != 1 {
		t.Errorf("unexpected FTP data download or excluded file probe; requests=%d", requests.Load())
	}
}
func TestFTPDoesNotConnectToBlockedLoopbackByDefault(t *testing.T) {
	seed, _ := startFakeFTP(t)
	_, err := fetchFTP(context.Background(), seed, false, 1*time.Second)
	if err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("unsafe FTP allowed or ambiguous error: %v", err)
	}
}

#!/usr/bin/env python3
"""End-to-end local HTTP API smoke test. Python stdlib only.

Usage: python qa/api-smoke.py ./dist/freshsift-linux-amd64
"""
import csv
import io
import json
import socket
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class Fixture(BaseHTTPRequestHandler):
    requests = {}
    def log_message(self, *_):
        pass
    def do_HEAD(self):
        self.handle_file(True)
    def do_GET(self):
        self.handle_file(False)
    def handle_file(self, head):
        Fixture.requests[self.path] = Fixture.requests.get(self.path, 0) + 1
        pages = {
            '/robots.txt': (b'User-agent: *\nDisallow: /private\nAllow: /\n', 'text/plain'),
            '/': (b'<html><head><link rel="stylesheet" href="/style.css"><script src="/script.js"></script><script>var x="needle inline";</script></head><body>VISIBLE HIT<a href="/data.db">db</a><a href="/photo.JPG?k=v">jpg</a><a href="/ignore.asx">asx</a></body></html>', 'text/html'),
            '/style.css': (b'body{content:"needle style"}', 'text/css'),
            '/script.js': (b'const x="needle javascript";', 'text/javascript'),
            '/data.db': (b'\x00\x01needle binary DB\x03', 'application/octet-stream'),
            '/photo.JPG?k=v': (b'JPEG', 'image/jpeg'),
            '/ignore.asx': (b'needle excluded file', 'application/x-asx'),
            '/private': (b'needle inside private', 'text/html'),
        }
        content,ctype=pages.get(self.path, (b'not found','text/plain'))
        status=200 if self.path in pages else 404
        self.send_response(status)
        self.send_header('Content-Type',ctype)
        self.send_header('Content-Length',str(len(content)))
        self.end_headers()
        if not head:
            self.wfile.write(content)


def free_port():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1',0))
        return sock.getsockname()[1]


def get(url):
    with urllib.request.urlopen(url, timeout=10) as r:
        return r.read().decode()


def post(url,payload):
    request=urllib.request.Request(url,method='POST',headers={'Content-Type':'application/json'},data=json.dumps(payload).encode())
    with urllib.request.urlopen(request,timeout=10) as r:
        assert r.status==202, (r.status,r.read())

def wait_status(base):
    deadline=time.monotonic()+25
    while time.monotonic()<deadline:
        data=json.loads(get(base+'/api/status'))
        if data['stats']['status'] in ('completed','cancelled'):
            return data
        time.sleep(.05)
    raise AssertionError('scan did not complete in time')


def scan(base,config):
    post(base+'/api/start',config)
    return wait_status(base)


def main():
    if len(sys.argv)!=2:
        print('Usage: python qa/api-smoke.py <path-to-linux-executable>',file=sys.stderr)
        return 2
    server=ThreadingHTTPServer(('127.0.0.1',0),Fixture)
    thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
    target=f'http://127.0.0.1:{server.server_port}'
    port=free_port()
    base=f'http://127.0.0.1:{port}'
    proc=subprocess.Popen([sys.argv[1],'-no-browser','-port',str(port)],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
    try:
        for _ in range(150):
            if proc.poll() is not None:
                raise AssertionError('freshsift exited before startup')
            try:
                if json.loads(get(base+'/api/version'))['version']=='0.4.0':
                    break
            except (OSError,urllib.error.URLError):
                time.sleep(.05)
        else:
            raise AssertionError('could not reach FreshSift')
        config={'seeds':target,'patterns':'needle','providers':[], 'allowPrivate':True,'respectRobots':True,
                'enableFTP':True,'excludeFileTypes':'.asx','depth':0,'maxPathSegments':5,'concurrency':3}
        raw=scan(base,config) # no searchMode: default raw scanning
        assert raw['stats']['matches']==4,raw
        assert '/ignore.asx' not in Fixture.requests,Fixture.requests
        assert raw['stats']['pagesScanned']>=5,raw['stats']
        print('PASS raw-source scan: HTML, CSS, JavaScript and .db; .asx excluded')
        visible=scan(base,{**config,'searchMode':'visible'})
        assert visible['stats']['matches']==0,visible
        print('PASS visible-text scan: no matches from markup/scripts/resources')
        file_hits=scan(base,{**config,'patterns':'','searchMode':'files','fileTypes':'.jpg'})
        assert file_hits['stats']['filesFound']==1,file_hits
        assert len(file_hits['matches'])==1 and file_hits['matches'][0]['kind']=='file',file_hits
        assert '.JPG' in file_hits['matches'][0]['url'],file_hits
        export=get(base+'/api/export?format=csv')
        rows=list(csv.DictReader(io.StringIO(export)))
        assert len(rows)==1 and rows[0]['Kind']=='file' and '.JPG' in rows[0]['URL'],rows
        json_export=json.loads(get(base+'/api/export?format=json'))
        assert len(json_export)==1 and json_export[0]['kind']=='file'
        json_lines=get(base+'/api/export?format=jsonl').strip().splitlines()
        assert len(json_lines)==1 and json.loads(json_lines[0])['kind']=='file'
        print('PASS filetype scan .jpg: reachable URL verified, CSV/JSON/JSONL exports')
        # Confirm the explicit robots override changes behavior.
        private={**config,'seeds':target+'/private'}
        blocked=scan(base,private)
        assert blocked['stats']['matches']==0,blocked
        open_result=scan(base,{**private,'respectRobots':False})
        assert open_result['stats']['matches']==1,open_result
        print('PASS robots.txt default enforcement and opt-out')
        excluded=scan(base,{**config,'allowedTLDs':'.com'})
        assert excluded['stats']['pagesScanned']==0 and excluded['stats']['matches']==0,excluded
        print('PASS TLD-only restriction excludes IP-address targets')
        dns_records=json.loads(get(base+'/api/dns-records?format=json'))
        reverse_dns=json.loads(get(base+'/api/reverse-dns?format=json'))
        assert isinstance(dns_records,list) and isinstance(reverse_dns,list)
        assert 'Domain,Type,Value' in get(base+'/api/dns-records?format=csv')
        assert 'IP,PTR Hostname,Origin,Forward Verified' in get(base+'/api/reverse-dns?format=csv')
        before=dict(Fixture.requests)
        dns_only=scan(base,{'seeds':target,'patterns':'','providers':[],
                            'discoverDNSRecords':True,'dnsOnly':True,'allowPrivate':True})
        assert dns_only['stats']['status']=='completed',dns_only
        assert dns_only['stats']['pagesScanned']==0,dns_only
        assert Fixture.requests==before,'DNS-only scan must not request web content'
        print('PASS DNS/PTR API inventories and DNS-only scan without web fetching')
        print('PASS end-to-end HTTP API smoke tests')
    finally:
        proc.terminate()
        try:
            proc.communicate(timeout=2)
        except subprocess.TimeoutExpired:
            proc.kill();proc.communicate()
        server.shutdown();server.server_close();thread.join(timeout=2)
    return 0

if __name__=='__main__':
    sys.exit(main())

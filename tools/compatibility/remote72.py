#!/usr/bin/env python3
"""Deploy/resume the sanitized matrix through a scoped user API key.

State and client credentials stay in the explicitly selected private directory.
The script never changes host networking or installs software using SSH.
"""
import argparse
import copy
import json
import os
from pathlib import Path
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request


def private_write(path, data):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC | os.O_NOFOLLOW, 0o600)
    os.fchmod(fd, 0o600)
    with os.fdopen(fd, 'w') as output:
        json.dump(data, output, indent=2)


def reference_client(c):
    """Use only connection-reveal fields, never server private keys."""
    kind, v = c['protocol'], c.get('v2ray')
    if v and v.get('engine') == 'xray':
        def transport():
            network = v['network']
            result = {'network': network, 'security': 'none'}
            if network in ('tcp', 'http'):
                result['network'] = 'raw'
                if network == 'http':
                    result['rawSettings'] = {'header': {'type': 'http', 'request': {'path': [v['path']], 'headers': {'Host': [v.get('host', '')]}}}}
            elif network == 'ws':
                result['wsSettings'] = {'path': v['path'], 'host': v.get('host', '')}
            elif network == 'grpc':
                result['grpcSettings'] = {'serviceName': v['service_name']}
            elif network == 'xhttp':
                result['xhttpSettings'] = {'path': v['path'], 'host': v.get('host', ''), 'mode': v.get('mode', 'auto'), 'headers': v.get('headers', {})}
            else:
                raise ValueError('unexpected matrix transport')
            return result
        def tls(alpn):
            return {'serverName': c['server_name'], 'alpn': alpn, 'fingerprint': v.get('fingerprint', ''), 'certificates': [{'certificate': c['certificate'].strip().splitlines(), 'usage': 'verify'}]}
        stream = transport()
        if v.get('tls', True):
            stream.update(security='tls', tlsSettings=tls(v.get('alpn', [])))
        if v.get('download') is not None:
            down = transport()
            down.update(address=c['server'], port=c['port'])
            if v['download']['tls']:
                down.update(security='tls', tlsSettings=tls(v['download'].get('alpn', [])))
            stream['xhttpSettings']['downloadSettings'] = down
        user = {'id': c['credential']}
        if kind == 'vless':
            user.update(encryption=c.get('encryption', 'none'), flow=v.get('flow', ''))
        else:
            user['security'] = 'auto'
        outbound = {'protocol': kind, 'settings': {'vnext': [{'address': c['server'], 'port': c['port'], 'users': [user]}]}, 'streamSettings': stream}
        if v.get('packet_encoding') == 'xudp':
            outbound['mux'] = {'enabled': True, 'concurrency': -1, 'xudpConcurrency': 8, 'xudpProxyUDP443': 'allow'}
        return 'xray', {'log': {'loglevel': 'none'}, 'inbounds': [{'listen': '127.0.0.1', 'port': 21900, 'protocol': 'socks', 'settings': {'udp': True}}], 'outbounds': [outbound]}
    q = c.get('quic') or {}
    outbound = {'type': kind, 'tag': 'proxy', 'server': c['server'], 'server_port': c['port']}
    if kind == 'socks':
        outbound.update(version='5', username=c['username'], password=c['credential'])
    elif kind == 'tuic':
        outbound.update(uuid=c['credential'], password=c['password'])
        if q.get('congestion'):
            outbound['congestion_control'] = q['congestion']
    elif kind == 'hysteria':
        outbound.update(auth_str=c['credential'], up_mbps=q['up_mbps'], down_mbps=q['down_mbps'])
    else:
        outbound['password'] = c['credential']
    if c.get('certificate'):
        outbound['tls'] = {'enabled': True, 'server_name': c['server_name'], 'certificate': c['certificate'].strip().splitlines()}
        if q.get('alpn'):
            outbound['tls']['alpn'] = q['alpn']
    if q.get('salamander'):
        outbound['obfs'] = {'type': 'salamander', 'password': c['obfs_password']}
    cfg = {'log': {'level': 'error'}, 'inbounds': [{'type': 'mixed', 'listen': '127.0.0.1', 'listen_port': 21900}], 'outbounds': [outbound], 'route': {'final': 'proxy'}}
    if kind == 'wireguard':
        w = c['wireguard']
        del cfg['outbounds']
        cfg['endpoints'] = [{'type': 'wireguard', 'tag': 'proxy', 'system': False, 'mtu': w['mtu'], 'address': [w['ip']+'/32'], 'private_key': w['private_key'], 'peers': [{'address': c['server'], 'port': c['port'], 'public_key': w['public_key'], 'pre_shared_key': w.get('pre_shared_key', ''), 'allowed_ips': ['0.0.0.0/0'], 'persistent_keepalive_interval': w.get('keepalive', 0), 'reserved': w.get('reserved') or []}]}]
    return 'sing-box', cfg


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--origin', required=True)
    parser.add_argument('--host', required=True)
    parser.add_argument('--key-file', type=Path, required=True)
    parser.add_argument('--state', type=Path, required=True)
    parser.add_argument('--probe-runtime-dir', type=Path, help='Run five public exit-IP requests using these pinned Linux arm64 runtimes')
    args = parser.parse_args()
    origin = urllib.parse.urlsplit(args.origin)
    if origin.scheme != 'https' or origin.username or origin.password or origin.path not in ('', '/') or origin.query or origin.fragment:
        parser.error('origin must be an HTTPS origin')
    token = args.key_file.read_text().strip()
    if args.key_file.stat().st_mode & 0o077:
        parser.error('key file must have private permissions')
    args.state.mkdir(mode=0o700, parents=True, exist_ok=True)
    if args.state.is_symlink() or args.state.stat().st_mode & 0o077:
        parser.error('state directory must have private permissions')
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, headers, newurl):
            raise RuntimeError('API redirects are not allowed')
    opener = urllib.request.build_opener(NoRedirect)
    def call(method, path, body=None):
        request = urllib.request.Request(args.origin.rstrip('/')+'/api/v1'+path, method=method, data=None if body is None else json.dumps(body).encode(), headers={'Authorization': 'Bearer '+token, 'Content-Type': 'application/json'})
        try:
            with opener.open(request, timeout=45) as response:
                return json.load(response).get('data')
        except urllib.error.HTTPError as e:
            # Do not echo payloads or credentials from an unexpected upstream.
            raise RuntimeError('API HTTP '+str(e.code)+' at '+path) from None
    host = next(h for h in call('GET', '/hosts') if h['id'] == args.host)
    if host.get('agent_version', '').startswith(('0.14.', '0.15.')):
        raise RuntimeError('Upgrade Agent before deploying the matrix')
    cert, key = args.state/'certificate.pem', args.state/'private.pem'
    if not cert.exists() or not key.exists():
        old_umask = os.umask(0o077)
        try:
            subprocess.run(['openssl', 'req', '-x509', '-newkey', 'ec', '-pkeyopt', 'ec_paramgen_curve:P-256', '-nodes', '-days', '3', '-subj', '/CN=proxy.example.com', '-addext', 'subjectAltName=DNS:proxy.example.com', '-keyout', str(key), '-out', str(cert)], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        finally:
            os.umask(old_umask)
    inputs = json.loads((Path(__file__).resolve().parents[2]/'internal/protocol/testdata/compatibility72.json').read_text())
    base = '/hosts/'+args.host+'/deployments'
    for index, original in enumerate(inputs, 1):
        body = copy.deepcopy(original)
        body['name'] = 'Compatibility 72 / '+original['name']
        v = body.get('v2ray', {})
        needs_cert = (body['protocol'] not in ('socks', 'wireguard') and v.get('tls', True)) or v.get('download', {}).get('tls', False)
        if needs_cert:
            body.update(server_name='proxy.example.com', certificate=cert.read_text(), private_key=key.read_text())
        nodes = call('GET', base)
        node = next((n for n in nodes if n['name'] == body['name'] and n['port'] == body['port'] and n['protocol'] == body['protocol']), None)
        if node is None:
            call('POST', base+'/preflight', body)
            node = call('POST', base, {**body, 'confirm_install': True})
        deadline = time.monotonic()+600
        while True:
            current = next(n for n in call('GET', base) if n['id'] == node['id'])
            if current['state'] == 'succeeded': break
            if current['state'] not in ('queued', 'running') or time.monotonic() > deadline:
                raise RuntimeError('case '+str(index)+' deployment did not succeed')
            time.sleep(3)
        connection = call('POST', base+'/'+node['id']+'/connection', {})
        family, client = reference_client(connection)
        private_write(args.state/('case-%02d-connection.json' % index), connection)
        private_write(args.state/('case-%02d-client.json' % index), client)
        private_write(args.state/('case-%02d-state.json' % index), {'id': node['id'], 'family': family, 'state': current['state']})
        print('case %02d deployed; reference client saved' % index, flush=True)
        if args.probe_runtime_dir:
            runtime_dir = args.probe_runtime_dir.resolve()
            binary = family+'-linux-arm64'
            if not (runtime_dir/binary).is_file():
                raise RuntimeError('missing pinned reference runtime')
            config_flag = '-config' if family == 'xray' else '-c'
            command = '/runtimes/'+binary+' run '+config_flag+' /configs/case-%02d-client.json' % index
            script = command + """ >/tmp/client.log 2>&1 &
client=$!
trap 'kill "$client" 2>/dev/null || true' EXIT
sleep 1
for attempt in 1 2 3 4 5; do
 curl -4 -fsS --max-time 30 --socks5-hostname 127.0.0.1:21900 https://api.ipify.org || exit 1
 printf '\\n'
done
"""
            result = subprocess.run(['docker', 'run', '--rm', '--pull', 'never', '--platform', 'linux/arm64', '--read-only', '--tmpfs', '/tmp', '--cap-drop', 'ALL', '-v', str(runtime_dir)+':/runtimes:ro', '-v', str(args.state.resolve())+':/configs:ro', '--entrypoint', 'sh', 'xingdu-lab-amazon:latest', '-c', script], stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=180)
            addresses = result.stdout.decode(errors='replace').strip().splitlines()
            passed = result.returncode == 0 and len(addresses) == 5 and all(ip == connection['server'] for ip in addresses)
            private_write(args.state/('case-%02d-probe.json' % index), {'passed': passed, 'requests': len(addresses), 'exit_matches_server': passed, 'exit_code': result.returncode})
            print('case %02d public forwarding %s' % (index, 'PASS' if passed else 'FAIL'), flush=True)
            if not passed:
                raise RuntimeError('public forwarding failed; inspect private case state')


if __name__ == '__main__':
    main()

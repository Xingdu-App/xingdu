#!/usr/bin/env python3
"""Exercise Xingdu's TrustTunnel lifecycle with an operator-supplied Stash CLI.

Uses only the existing disposable Ubuntu Agent Lab fixture. No public VPS,
production identity or installed Stash app is touched. Local output is private.
"""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import socket
import struct
import ssl
import subprocess
import time
import urllib.request

spec = importlib.util.spec_from_file_location('protocol_lab', Path(__file__).with_name('protocol-lab.py'))
p = importlib.util.module_from_spec(spec)
spec.loader.exec_module(p)
lab = p.lab
ROOT = lab.ROOT
LOCAL = ROOT / '.local/trusttunnel-lab'
BRIDGE = 'xingdu-trusttunnel-lab-bridge'
REMOTE_PORT, LOCAL_PORT, SOCKS_PORT, CONTROL_PORT = 24449, 29449, 29450, 29451


def read(sock, count):
    data = b''
    while len(data) < count:
        chunk = sock.recv(count - len(data))
        if not chunk:
            raise RuntimeError('SOCKS connection closed')
        data += chunk
    return data


def udp_test(target, allowed):
    with socket.create_connection(('127.0.0.1', SOCKS_PORT), timeout=5) as control:
        control.sendall(b'\x05\x01\x00')
        assert read(control, 2) == b'\x05\x00'
        control.sendall(b'\x05\x03\x00\x01' + bytes(6))
        header = read(control, 4)
        assert header[:3] == b'\x05\x00\x00'
        family = socket.AF_INET if header[3] == 1 else socket.AF_INET6
        relay = socket.inet_ntop(family, read(control, 4 if family == socket.AF_INET else 16))
        port = struct.unpack('!H', read(control, 2))[0]
        if relay in ('0.0.0.0', '::'):
            relay = '127.0.0.1' if family == socket.AF_INET else '::1'
        with socket.socket(family, socket.SOCK_DGRAM) as udp:
            udp.settimeout(4)
            for size in ([1, 1024, 1200] if allowed else [32]):
                payload = os.urandom(size)
                udp.sendto(b'\0\0\0\x01' + socket.inet_aton(target) + struct.pack('!H', 18082) + payload, (relay, port))
                try:
                    response, _ = udp.recvfrom(65535)
                except socket.timeout:
                    assert not allowed, 'UDP forwarding timed out'
                else:
                    assert allowed and response.endswith(payload), 'Unexpected UDP forwarding'


def client_test(binary, config, label, success, udp=False):
    path = LOCAL / (label + '.yaml')
    path.write_bytes(config)
    client_root = LOCAL / label
    client_root.mkdir(exist_ok=True)
    for name in ("Country.mmdb", "ASN.mmdb"):
        shutil.copyfile(GEODB / name, client_root / name)
    with (LOCAL / (label + '.log')).open('wb') as log:
        validated = subprocess.run([binary, '-t', '-f', str(path)], stdout=log, stderr=log)
        assert validated.returncode == 0, 'Stash rejected exported configuration'
        client = subprocess.Popen([binary, '-f', str(path), '-d', str(LOCAL / label), '--proxy-port', str(SOCKS_PORT), '--control-port', str(CONTROL_PORT)], stdout=log, stderr=log)
        try:
            def ready():
                if client.poll() is not None:
                    if label == 'wrong-alpn':
                        return True
                    raise RuntimeError('Stash CLI exited; inspect private lab log')
                try:
                    with socket.create_connection(('127.0.0.1', SOCKS_PORT), timeout=.2):
                        return True
                except OSError:
                    return False
            lab.eventually(ready, 30)
            if client.poll() is not None:
                log.flush()
                assert label == 'wrong-alpn' and b'HTTP/2 requires h2 ALPN' in (LOCAL / (label + '.log')).read_bytes()
                lab.progress('wrong-alpn: rejected before startup PASS')
                return
            curl = ['curl', '--noproxy', '', '--socks5-hostname', '127.0.0.1:' + str(SOCKS_PORT), '-fsS', '--max-time', '8']
            result = lab.command(curl + ['http://' + p.TARGET + ':18081/probe'], check=False)
            (LOCAL / (label + '-curl.log')).write_bytes(result.stderr + result.stdout)
            assert (result.returncode == 0 and result.stdout == b'xingdu-protocol-lab-ok') == success, 'Unexpected TCP forwarding result: ' + label
            if success:
                for target in ('127.0.0.1', 'private.xingdu.test', '168.63.129.16'):
                    assert lab.command(curl + ['http://' + target + ':18081/probe'], check=False).returncode != 0, 'Private TCP destination reachable'
            if udp:
                udp_test(p.TARGET, True)
                udp_test('127.0.0.1', False)
            lab.progress(label + ': PASS')
        finally:
            client.terminate()
            try:
                client.wait(timeout=8)
            except subprocess.TimeoutExpired:
                client.kill()
                client.wait()
            for name in ('Country.mmdb', 'ASN.mmdb'):
                (client_root / name).unlink(missing_ok=True)


def main(binary):
    LOCAL.mkdir(parents=True, exist_ok=True, mode=0o700)
    api = lab.API()
    fixtures = json.loads((lab.LOCAL / 'fixtures.json').read_text())
    host = lab.resolve_fixtures({'ubuntu': fixtures['ubuntu']}, api.call('GET', '/api/v1/hosts'))['ubuntu']
    current = api.host(host)
    assert current['address'] == 'lab-ubuntu' and 'agent-lab' in current['tags'], 'Not a disposable lab fixture'
    # Update this local fixture only; preserve its identity and existing nodes.
    lab.execute('ubuntu', 'sh', '-c', 'curl -fsS ' + lab.CONTROL + '/api/v1/agent/download/arm64 -o /usr/local/bin/xingdu-agent.next && chmod 755 /usr/local/bin/xingdu-agent.next && mv /usr/local/bin/xingdu-agent.next /usr/local/bin/xingdu-agent && systemctl restart xingdu-agent')
    lab.eventually(lambda: api.machine(host)['agent']['metrics']['version'] == '0.15.0-dev', 90)
    cert, key = p.prepare_certificate()
    p.prepare_target('ubuntu')
    lab.execute('ubuntu', 'sh', '-c', 'ip address show dev lo | grep -q "168.63.129.16/32" || ip address add 168.63.129.16/32 dev lo')
    lab.execute('ubuntu', 'curl', '-fsS', '--max-time', '3', 'http://168.63.129.16:18081/probe')
    node = p.queue(api, host, {'name': 'TrustTunnel acceptance', 'protocol': 'trusttunnel', 'port': REMOTE_PORT, 'server_name': p.TLS_NAME, 'certificate': cert, 'private_key': key, 'confirm_install': True})
    sub_id = None
    try:
        p.wait_deployment(api, host, node['id'], 'succeeded')
        lab.progress('Xingdu API -> Agent -> official endpoint: deployed')
        unit = 'xingdu-protocol-' + node['id'] + '.service'
        lab.execute('ubuntu', 'systemctl', 'is-active', '--quiet', unit)
        uid = lab.execute('ubuntu', 'sh', '-c', 'pid=$(systemctl show ' + unit + ' -p MainPID --value); ps -o uid= -p "$pid"').stdout.strip()
        assert uid and uid != b'0', 'Runtime runs as root'
        body = {'name': 'TrustTunnel acceptance', 'format': 'stash', 'node_ids': [node['id']], 'rules': [], 'final_action': 'proxy', 'enabled': True}
        sub = api.call('POST', '/api/v1/subscriptions', body)
        sub_id = sub['subscription']['id']
        with urllib.request.urlopen(lab.ORIGIN + sub['subscription_path'], timeout=15) as response:
            original = response.read()
        assert b'h2' in original and b'PRIVATE KEY' not in original
        connection = api.action(host, 'deployments/' + node['id'] + '/connection')
        assert connection['username'] == 'xingdu'
        # Only replace the endpoint address for Docker's loopback bridge. All
        # authentication, TLS, ALPN and routing fields come from Xingdu's export.
        config = re.sub(rb'(?m)^(\s*server:)\s*.*$', rb'\1 127.0.0.1', original)
        config = re.sub(rb'(?m)^(\s*port:)\s*' + str(REMOTE_PORT).encode() + rb'$', rb'\g<1> ' + str(LOCAL_PORT).encode(), config)
        image = lab.command(['docker', 'inspect', '--format', '{{.Config.Image}}', 'xingdu-lab-ubuntu-1']).stdout.decode().strip()
        network = lab.command(['docker', 'inspect', '--format', '{{range $k,$v := .NetworkSettings.Networks}}{{$k}}{{end}}', 'xingdu-lab-ubuntu-1']).stdout.decode().strip()
        bridge = '''import socket,threading,select
s=socket.socket();s.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1);s.bind(('0.0.0.0',29449));s.listen()
def forward(a):
 try:
  with a, socket.create_connection(('lab-ubuntu',24449),timeout=10) as b:
   while True:
    readable,_,_=select.select([a,b],[],[],30)
    for source in readable:
     data=source.recv(65536)
     if not data:return
     (b if source is a else a).sendall(data)
 except OSError:pass
while True:
 a,_=s.accept();threading.Thread(target=forward,args=(a,),daemon=True).start()
'''
        lab.command(['docker', 'run', '-d', '--rm', '--name', BRIDGE, '--network', 'xingdu_default', '-p', '127.0.0.1:' + str(LOCAL_PORT) + ':' + str(LOCAL_PORT), '--entrypoint', 'python3', image, '-c', bridge])
        lab.command(['docker', 'network', 'connect', network, BRIDGE])
        def negotiated():
            try:
                context = ssl.create_default_context(cafile=str(lab.LOCAL / 'ca.crt'))
                context.set_alpn_protocols(['h2'])
                with socket.create_connection(('127.0.0.1', LOCAL_PORT), timeout=2) as raw:
                    with context.wrap_socket(raw, server_hostname=p.TLS_NAME) as tls:
                        return tls.selected_alpn_protocol() == 'h2'
            except OSError:
                return False
        lab.eventually(negotiated, 30)
        lab.progress('Real TLS handshake negotiated h2')
        client_test(binary, config, 'explicit-h2-tcp-udp', True, udp=True)
        # h2 default on the real Stash client; unrelated to browser ALPN.
        default = re.sub(rb'(?m)^(\s*- )alpn:\n\s*- h2\n\s*', rb'\1', config)
        assert default != config, 'ALPN fixture was not removed'
        client_test(binary, default, 'default-alpn', True)
        bad_password = config.replace(connection['credential'].encode(), b'incorrect-password')
        assert bad_password != config
        client_test(binary, bad_password, 'wrong-password', False)
        bad_pin = re.sub(rb'(server-cert-fingerprint:)\s*\S+', rb'\1 ' + b'00' * 32, config)
        client_test(binary, bad_pin, 'wrong-pin', False)
        bad_alpn = config.replace(b'- h2', b'- http/1.1')
        client_test(binary, bad_alpn, 'wrong-alpn', False)
        api.action(host, 'deployments/' + node['id'] + '/restart', {'confirm': True})
        p.wait_deployment(api, host, node['id'], 'succeeded')
        client_test(binary, config, 'restart', True)
        path = 'deployments/' + node['id']
        api.action(host, path, {'rotate_credential': True, 'confirm': True}, method='PUT')
        p.wait_deployment(api, host, node['id'], 'succeeded')
        rotated = api.action(host, path + '/connection')['credential']
        assert rotated != connection['credential']
        client_test(binary, config, 'revoked-credential', False)
        client_test(binary, config.replace(connection['credential'].encode(), rotated.encode()), 'rotated-credential', True)
        api.action(host, path, {'restore_revision': 1, 'confirm': True}, method='PUT')
        p.wait_deployment(api, host, node['id'], 'succeeded')
        client_test(binary, config, 'restored-revision', True)
        lab.save(LOCAL / 'report.json', {'scope': 'Ubuntu arm64 Docker endpoint + macOS Stash CLI; isolated network, no app/public VPS verification', 'runtime': 'TrustTunnel 1.1.0', 'deployment': 'PASS', 'non_root': 'PASS', 'subscription': 'PASS', 'h2_tcp_udp': 'PASS', 'default_h2': 'PASS', 'bad_password_pin_alpn': 'PASS', 'private_tcp_domain_udp': 'PASS', 'cloud_platform_address_blocked': 'PASS', 'restart': 'PASS', 'credential_rotation_and_revision_restore': 'PASS'})
    finally:
        lab.command(['docker', 'rm', '-f', BRIDGE], check=False)
        if sub_id:
            api.call('DELETE', '/api/v1/subscriptions/' + sub_id)
        api.action(host, 'deployments/' + node['id'], method='DELETE')
        p.wait_deployment(api, host, node['id'], 'removed')
        lab.execute('ubuntu', 'ip', 'address', 'del', '168.63.129.16/32', 'dev', 'lo', check=False)
        lab.progress('Temporary subscription and deployment removed')


if __name__ == '__main__':
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--client', required=True, help='Path to a locally built Stash core CLI')
    parser.add_argument('--geodb-dir', required=True, help='Directory containing valid Country.mmdb and ASN.mmdb')
    args = parser.parse_args()
    GEODB = Path(args.geodb_dir).resolve(strict=True)
    main(str(Path(args.client).resolve(strict=True)))

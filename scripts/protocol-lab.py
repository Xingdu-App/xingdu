#!/usr/bin/env python3
"""Verify protocol installation and real authenticated forwarding in disposable lab containers.

Requires `make agent-lab-up` and an initial local admin. Only known agent-lab
fixtures are re-enrolled as managed agents; no real VPS is touched. SSH credentials
and TLS/client secrets are never printed. The Docker network remains internal.
"""
import argparse
import base64
import concurrent.futures
import importlib.util
import json
import os
from pathlib import Path
import time

module_spec = importlib.util.spec_from_file_location('agent_lab', Path(__file__).with_name('agent-lab.py'))
lab = importlib.util.module_from_spec(module_spec)
module_spec.loader.exec_module(lab)
ROOT, LOCAL = lab.ROOT, lab.LOCAL
PROTOCOLS = ['trojan', 'vless', 'vmess', 'hysteria2', 'tuic', 'shadowsocks', 'shadowsocks2022', 'anytls', 'http', 'socks', 'mixed', 'hysteria', 'shadowtls', 'snell', 'snell6']
TLS_NAME = 'node.xingdu.test'
# Assigned to loopback INSIDE the isolated containers, never routed externally.
TARGET = '93.184.216.34'


def parallel(fn):
    with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:
        return dict(zip(lab.SYSTEMS, pool.map(fn, lab.SYSTEMS)))


def prepare_certificate():
    if not (LOCAL/'protocol.crt').exists():
        extensions = LOCAL/'protocol-cert.cnf'
        extensions.write_text('[server]\nbasicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\nsubjectAltName=DNS:' + TLS_NAME + '\n')
        lab.command(['openssl', 'req', '-new', '-newkey', 'rsa:2048', '-nodes', '-subj', '/CN=' + TLS_NAME, '-keyout', str(LOCAL/'protocol.key'), '-out', str(LOCAL/'protocol.csr')])
        lab.command(['openssl', 'x509', '-req', '-in', str(LOCAL/'protocol.csr'), '-CA', str(LOCAL/'ca.crt'), '-CAkey', str(LOCAL/'ca.key'), '-CAcreateserial', '-days', '14', '-extfile', str(extensions), '-extensions', 'server', '-out', str(LOCAL/'protocol.crt')])
        (LOCAL/'protocol.key').chmod(0o600)
    lab.command(['openssl', 'x509', '-checkend', '3600', '-noout', '-in', str(LOCAL/'protocol.crt')])
    return (LOCAL/'protocol.crt').read_text() + (LOCAL/'ca.crt').read_text(), (LOCAL/'protocol.key').read_text()


def prepare_target(system):
    lab.execute(system, 'sh', '-c', 'ip address show dev lo | grep -q "93.184.216.34/32" || ip address add 93.184.216.34/32 dev lo')
    lab.execute(system, 'sh', '-c', 'grep -q "private.xingdu.test" /etc/hosts || printf "127.0.0.1 private.xingdu.test\\n" >> /etc/hosts')
    lab.execute(system, 'sh', '-c', 'mkdir -p /run/xingdu-protocol-target; printf "xingdu-protocol-lab-ok" > /run/xingdu-protocol-target/probe')
    active = lab.execute(system, 'systemctl', 'is-active', '--quiet', 'xingdu-lab-protocol-target', check=False)
    if active.returncode:
        lab.execute(system, 'systemd-run', '--quiet', '--collect', '--unit=xingdu-lab-protocol-target', 'python3', '-m', 'http.server', '18081', '--bind', '0.0.0.0', '--directory', '/run/xingdu-protocol-target')
    lab.eventually(lambda: lab.execute(system, 'curl', '-fsS', '--max-time', '3', 'http://' + TARGET + ':18081/probe', check=False).returncode == 0, 20)
    active = lab.execute(system, 'systemctl', 'is-active', '--quiet', 'xingdu-lab-udp-target', check=False)
    if active.returncode:
        echo = "import socket\ns=socket.socket(socket.AF_INET,socket.SOCK_DGRAM)\ns.bind(('0.0.0.0',18082))\nwhile True:\n data,addr=s.recvfrom(4096)\n s.sendto(data,addr)\n"
        lab.execute(system, 'systemd-run', '--quiet', '--collect', '--unit=xingdu-lab-udp-target', 'python3', '-c', echo)


def udp_disabled_test(system):
    # Attempt UDP through the real client: TCP-only SS must reject it even
    # with a valid key. Keeping the control connection open is required by SOCKS5.
    probe = '''import socket, struct, sys
def read(sock, size):
    out=b''
    while len(out)<size:
        chunk=sock.recv(size-len(out))
        assert chunk, 'SOCKS association closed'
        out+=chunk
    return out
with socket.create_connection(('127.0.0.1',19080),timeout=5) as control:
    control.sendall(b'\\x05\\x01\\x00')
    assert read(control,2)==b'\\x05\\x00'
    control.sendall(b'\\x05\\x03\\x00\\x01'+b'\\x00'*6)
    header=read(control,4)
    assert header[:3]==b'\\x05\\x00\\x00'
    family=socket.AF_INET if header[3]==1 else socket.AF_INET6
    assert header[3] in (1,4)
    relay=socket.inet_ntop(family,read(control,4 if family==socket.AF_INET else 16))
    port=struct.unpack('!H',read(control,2))[0]
    if relay in ('0.0.0.0','::'): relay='127.0.0.1' if family==socket.AF_INET else '::1'
    with socket.socket(family,socket.SOCK_DGRAM) as udp:
        udp.settimeout(3)
        for target,should_reply in [('93.184.216.34',False),('127.0.0.1',False)]:
            payload=b'xingdu-udp-probe-'+target.encode()
            frame=b'\\x00\\x00\\x00\\x01'+socket.inet_aton(target)+struct.pack('!H',18082)+payload
            udp.sendto(frame,(relay,port))
            try:
                response,_=udp.recvfrom(4096)
                replied=response.endswith(payload)
                assert response[:4]==b'\\x00\\x00\\x00\\x01', 'Unexpected UDP response type'
                actual=socket.inet_ntoa(response[4:8])
                assert actual==target, 'UDP response came from '+actual+' instead of '+target
            except socket.timeout:
                replied=False
            assert replied==should_reply, 'UDP target='+target+' expected='+str(should_reply)+' replied='+str(replied)
'''
    result = lab.execute(system, 'python3', '-c', probe, check=False)
    if result.returncode:
        # This fixed probe contains no credentials; retain its diagnostic only
        # in the ignored local lab directory, never runtime client configs.
        path = LOCAL / ('udp-probe-' + system + '.log')
        path.write_bytes(result.stderr)
        path.chmod(0o600)
        raise AssertionError('UDP probe failed on ' + system + '; inspect local probe diagnostic')


def connection_test(system, connection, expect_success=True, mixed_http=False):
    p = connection['protocol']
    outbound = {'type': p, 'tag': 'proxy', 'server': '127.0.0.1', 'server_port': connection['port'], 'tls': {'enabled': True, 'server_name': connection['server_name'], 'certificate': connection['certificate'].splitlines()}}
    if p in ('shadowsocks', 'shadowsocks2022'):
        outbound = {'type': 'shadowsocks', 'tag': 'proxy', 'server': '127.0.0.1', 'server_port': connection['port'], 'method': connection['cipher'], 'password': connection['credential']}
    elif p in ('trojan', 'hysteria2', 'anytls', 'http'):
        outbound['password'] = connection['credential']
    else:
        outbound['uuid'] = connection['credential']
    if p == 'http':
        outbound['username'] = 'xingdu'
    if p == 'vmess':
        outbound.update(security='auto', alter_id=0)
    if p == 'tuic':
        outbound.update(password=connection['password'], congestion_control='bbr')
    if p in ('hysteria2', 'tuic'):
        outbound['tls']['alpn'] = ['h3']
    outbounds = [outbound]
    if p in ('socks', 'mixed'):
        outbound.clear()
        outbound.update(type='socks', tag='proxy', server='127.0.0.1', server_port=connection['port'], version='5', username='xingdu', password=connection['credential'])
    if p == 'mixed' and mixed_http:
        outbound['type'] = 'http'
        outbound.pop('version', None)
    if p in ('snell', 'snell6'):
        outbound.clear()
        outbound.update(type='snell', tag='proxy', server='127.0.0.1', server_port=connection['port'], version=6 if p == 'snell6' else 4, psk=connection['credential'])
    if p == 'hysteria':
        outbound.pop('uuid', None)
        outbound.update(auth_str=connection['credential'], up_mbps=100, down_mbps=100)
        outbound['tls']['alpn'] = ['hysteria']
    if p == 'shadowtls':
        outbound.clear()
        outbound.update(type='shadowsocks', tag='proxy', method=connection['cipher'], password=connection['credential'], detour='transport')
        outbounds.append({'type':'shadowtls','tag':'transport','server':'127.0.0.1','server_port':connection['port'],'version':3,'password':connection['password'],'tls':{'enabled':True,'server_name':connection['server_name'],'certificate':(LOCAL/'ca.crt').read_text().splitlines(),'utls':{'enabled':True,'fingerprint':'chrome'}}})
    config = {'log': {'disabled': True}, 'inbounds': [{'type': 'mixed', 'listen': '127.0.0.1', 'listen_port': 19080}], 'outbounds': outbounds, 'route': {'final': 'proxy'}}
    lab.execute(system, 'sh', '-c', 'umask 077; cat > /run/xingdu-protocol-client.json', data=json.dumps(config).encode())
    lab.execute(system, 'systemd-run', '--quiet', '--collect', '--unit=xingdu-lab-protocol-client', '/usr/local/lib/xingdu/sing-box-1.14.2', 'run', '-c', '/run/xingdu-protocol-client.json')
    try:
        lab.eventually(lambda: lab.execute(system, 'systemctl', 'is-active', '--quiet', 'xingdu-lab-protocol-client', check=False).returncode == 0, 10)
        curl = ['curl', '-fsS', '--max-time', '8', '--noproxy', '', '--proxy', 'socks5h://127.0.0.1:19080']
        def forwarded():
            response = lab.execute(system, *curl, 'http://' + TARGET + ':18081/probe', check=False)
            return response.returncode == 0 and response.stdout == b'xingdu-protocol-lab-ok'
        if expect_success:
            lab.eventually(forwarded, 25)
            for address in ['127.0.0.1', 'private.xingdu.test']:
                blocked = lab.execute(system, *curl, 'http://' + address + ':18081/probe', check=False)
                assert blocked.returncode != 0, 'Private destination was reachable through proxy'
        else:
            assert not forwarded(), 'Invalid protocol credentials were accepted'
        if expect_success and p in ('shadowsocks', 'shadowsocks2022', 'socks', 'mixed', 'snell', 'snell6', 'shadowtls'):
            udp_disabled_test(system)
    finally:
        lab.execute(system, 'systemctl', 'stop', 'xingdu-lab-protocol-client', check=False)
        lab.execute(system, 'rm', '-f', '/run/xingdu-protocol-client.json')
        lab.eventually(lambda: lab.execute(system, 'systemctl', 'show', 'xingdu-lab-protocol-client', '-p', 'LoadState', '--value', check=False).stdout.strip() == b'not-found', 15)


def prepare_handshake(system):
    # The lab network is internal. Use a CA-signed TLS 1.3 fixture for the
    # approved handshake hostname, resolved only inside disposable containers.
    if not (LOCAL/'handshake.crt').exists():
        (LOCAL/'handshake.cnf').write_text('[server]\nsubjectAltName=DNS:www.microsoft.com\nkeyUsage=digitalSignature\nextendedKeyUsage=serverAuth\n')
        lab.command(['openssl','req','-new','-newkey','rsa:2048','-nodes','-subj','/CN=www.microsoft.com','-keyout',str(LOCAL/'handshake.key'),'-out',str(LOCAL/'handshake.csr')])
        lab.command(['openssl','x509','-req','-in',str(LOCAL/'handshake.csr'),'-CA',str(LOCAL/'ca.crt'),'-CAkey',str(LOCAL/'ca.key'),'-CAcreateserial','-days','14','-extfile',str(LOCAL/'handshake.cnf'),'-extensions','server','-out',str(LOCAL/'handshake.crt')])
        (LOCAL/'handshake.key').chmod(0o600)
    for name in ('handshake.crt','handshake.key'):
        lab.execute(system,'sh','-c','umask 077; cat > /run/xingdu-'+name,data=(LOCAL/name).read_bytes())
    lab.execute(system,'sh','-c', 'grep -q "www.microsoft.com" /etc/hosts || printf "127.0.0.1 www.microsoft.com\\n" >> /etc/hosts')
    active = lab.execute(system,'systemctl','is-active','--quiet','xingdu-lab-handshake',check=False)
    if active.returncode:
        server = "import socket,ssl,threading\nc=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)\nc.minimum_version=ssl.TLSVersion.TLSv1_3\nc.load_cert_chain('/run/xingdu-handshake.crt','/run/xingdu-handshake.key')\ns=socket.socket()\ns.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1)\ns.bind(('127.0.0.1',443))\ns.listen()\ndef handle(raw):\n try:\n  with c.wrap_socket(raw,server_side=True) as conn: conn.recv(4096)\n except (OSError,ssl.SSLError): raw.close()\nwhile True:\n raw,_=s.accept()\n threading.Thread(target=handle,args=(raw,),daemon=True).start()\n"
        lab.execute(system,'sh','-c','umask 077; cat > /run/xingdu-handshake.py',data=server.encode())
        lab.execute(system,'systemd-run','--quiet','--collect','--unit=xingdu-lab-handshake','python3','/run/xingdu-handshake.py')


def main():
    certificate, key = prepare_certificate()
    fixtures_path = LOCAL/'fixtures.json'
    if not fixtures_path.exists():
        raise RuntimeError('Run make agent-lab-test once to create the known lab fixtures')
    fixtures = json.loads(fixtures_path.read_text())
    report = {'runtime': 'sing-box 1.14.2', 'protocols': PROTOCOLS, 'shadowsocks_transport': 'TCP only; UDP disabled', 'scope': 'isolated Docker arm64 lab; not a public VPS or client-app compatibility claim', 'systems': {}}
    def run_system(system):
        api = lab.API()
        host = lab.resolve_fixtures({system: fixtures[system]}, api.call('GET', '/api/v1/hosts'))[system]
        current = api.host(host)
        if current['address'] != 'lab-' + system or 'agent-lab' not in current['tags']:
            raise RuntimeError('Fixture does not match the disposable lab target')
        # Revoke only the known disposable lab identity; existing protocol services
        # are removed through their API before reenrollment if this is a rerun.
        for d in api.action(host, 'deployments', method='GET'):
            if d['state'] in ('succeeded', 'failed', 'interrupted'):
                api.action(host, 'deployments/' + d['id'], method='DELETE')
                wait_deployment(api, host, d['id'], 'removed')
        api.action(host, 'agent', method='DELETE')
        lab.reset_install(system)
        enrollment = api.action(host, 'enrollment', {'mode': 'manage', 'confirm_manage': True})
        lab.execute(system, 'sh', '-c', 'curl -fsS ' + lab.CONTROL + '/api/v1/agent/install.sh -o /tmp/xingdu-install.sh && sh /tmp/xingdu-install.sh manage', data=(enrollment['token'] + '\n').encode())
        lab.eventually(lambda: api.host(host)['status'] == 'online' and api.machine(host)['agent']['metrics']['version'] == '0.14.0-dev')
        lab.verify_service(system, 'manage')
        prepare_target(system)
        if 'shadowtls' in PROTOCOLS:
            prepare_handshake(system)
        deployments = []
        findings = {}
        for index, p in enumerate(PROTOCOLS):
            body = {'name': p.upper() + ' · Protocol Lab', 'protocol': p, 'port': 24430 + index, 'confirm_install': True}
            if p not in ('shadowsocks', 'shadowsocks2022', 'socks', 'mixed', 'snell', 'snell6', 'shadowtls'):
                body.update(server_name=TLS_NAME, certificate=certificate, private_key=key)
            if p == 'shadowtls':
                body['server_name'] = 'www.microsoft.com'
            d = queue(api, host, body)
            deployments.append(d['id'])
            wait_deployment(api, host, d['id'], 'succeeded')
            unit = 'xingdu-protocol-' + d['id'] + '.service'
            lab.execute(system, 'systemctl', 'is-active', unit)
            lab.execute(system, 'systemctl', 'is-enabled', unit)
            uid = lab.execute(system, 'sh', '-c', 'pid=$(systemctl show ' + unit + ' -p MainPID --value); ps -o uid= -p "$pid"').stdout.strip()
            assert uid and uid != b'0', 'Protocol service unexpectedly running as root'
            connection = api.action(host, 'deployments/' + d['id'] + '/connection')
            assert 'private_key' not in connection
            connection_test(system, connection)
            if p == 'mixed':
                connection_test(system, connection, mixed_http=True)
            invalid = dict(connection)
            invalid['credential'] = '00000000-0000-4000-8000-000000000001' if p in ('vless', 'vmess', 'tuic') else 'invalid-password'
            if p in ('shadowsocks2022', 'shadowtls'):
                invalid['credential'] = base64.b64encode(os.urandom(32)).decode()
            connection_test(system, invalid, expect_success=False)
            if p == 'mixed':
                connection_test(system, invalid, expect_success=False, mixed_http=True)
            if p == 'shadowtls':
                bad_outer = dict(connection)
                bad_outer['password'] = 'invalid-outer-password'
                connection_test(system, bad_outer, expect_success=False)
            if p in ('shadowsocks', 'shadowsocks2022'):
                api.action(host, 'deployments/' + d['id'] + '/restart', {'confirm': True})
                restarted = wait_deployment(api, host, d['id'], 'succeeded')
                assert restarted['result'] == 'restarted', 'API restart did not complete'
                connection_test(system, connection)
            findings[p] = 'Authenticated forwarding, invalid credential rejection, private IP/domain blocking, non-root service PASS'
            if p in ('shadowsocks', 'shadowsocks2022'):
                findings[p] += '; UDP disabled and API restart PASS'
            lab.progress(lab.SYSTEMS[system] + ': ' + p + ' install + authenticated forwarding PASS')
        lab.compose('restart', 'lab-' + system)
        lab.eventually(lambda: lab.execute(system, 'systemctl', 'is-active', '--quiet', 'xingdu-agent', check=False).returncode == 0, 60)
        prepare_target(system)
        if 'shadowtls' in PROTOCOLS:
            prepare_handshake(system)
        lab.eventually(lambda: api.host(host)['status'] == 'online')
        for d in deployments:
            lab.execute(system, 'systemctl', 'is-active', 'xingdu-protocol-' + d + '.service')
            connection_test(system, api.action(host, 'deployments/' + d + '/connection'))
        findings['restart'] = 'All selected services restore and forward after container restart'
        # Exercise refusal to take over an unrelated listening service.
        collision = queue(api, host, {'name': 'Occupied port test', 'protocol': 'trojan', 'port': 18081, 'server_name': TLS_NAME, 'certificate': certificate, 'private_key': key, 'confirm_install': True})
        collision_result = wait_deployment(api, host, collision['id'], 'failed')
        assert collision_result['result'] == 'port_in_use'
        api.action(host, 'deployments/' + collision['id'], method='DELETE')
        wait_deployment(api, host, collision['id'], 'removed')
        findings['port_collision'] = 'Rejected without replacing unrelated service'
        for d in deployments:
            api.action(host, 'deployments/' + d, method='DELETE')
            wait_deployment(api, host, d, 'removed')
            assert lab.execute(system, 'systemctl', 'is-active', '--quiet', 'xingdu-protocol-' + d + '.service', check=False).returncode != 0
            assert lab.execute(system, 'test', '-e', '/var/lib/xingdu-agent/protocols/' + d, check=False).returncode != 0
        findings['uninstall'] = 'All selected services stopped, files removed, API connection secrets cleared'
        lab.execute(system, 'systemctl', 'stop', 'xingdu-lab-protocol-target', check=False)
        lab.execute(system, 'systemctl', 'stop', 'xingdu-lab-udp-target', check=False)
        lab.execute(system, 'systemctl', 'stop', 'xingdu-lab-handshake', check=False)
        api.call('POST', '/api/v1/auth/logout')
        lab.progress(lab.SYSTEMS[system] + ': restart / occupied port / uninstall PASS')
        return findings
    report['systems'] = parallel(run_system)
    lab.save(LOCAL/'protocol-report.json', report)
    lab.progress('PASS. Protocol report: .local/agent-lab/protocol-report.json. Managed lab Agents remain online.')


def queue(api, host, body):
    # Shared per-user rate limit also applies when three lab systems run together.
    until = time.monotonic() + 75
    while True:
        try:
            return api.action(host, 'deployments', body)
        except RuntimeError as error:
            if 'HTTP 429:' not in str(error) or time.monotonic() >= until:
                raise
            time.sleep(5)


def wait_deployment(api, host, ident, expected):
    def completed():
        d = next(x for x in api.action(host, 'deployments', method='GET') if x['id'] == ident)
        if d['state'] == expected:
            return d
        if d['state'] in ('failed', 'interrupted', 'cancelled', 'removed'):
            raise RuntimeError('Deployment ' + d['protocol'] + ': ' + d['state'] + '/' + d['result'])
        return None
    return lab.eventually(completed, 150)


if __name__ == '__main__':
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--protocol', action='append', choices=PROTOCOLS, help='Test only selected protocols; repeat to select several')
    parser.add_argument('--system', action='append', choices=list(lab.SYSTEMS), help='Limit the disposable Linux systems')
    args = parser.parse_args()
    if args.system:
        lab.SYSTEMS = {k: lab.SYSTEMS[k] for k in dict.fromkeys(args.system)}
    if args.protocol:
        PROTOCOLS = list(dict.fromkeys(args.protocol))
    try:
        main()
    except (RuntimeError, AssertionError) as error:
        lab.progress('FAIL: ' + str(error))
        raise SystemExit(1)

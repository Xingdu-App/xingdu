#!/usr/bin/env python3
"""Local systemd/SSH acceptance lab. Secrets stay under ignored .local/agent-lab."""
import argparse
import http.cookiejar
import json
import os
from pathlib import Path
import secrets
import subprocess
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
LOCAL = ROOT / '.local/agent-lab'
COMPOSE = ['docker', 'compose', '-f', 'compose.yaml', '-f', 'deploy/agent-lab/compose.yaml']
SYSTEMS = {'ubuntu': 'Ubuntu 24.04', 'debian': 'Debian 13', 'amazon': 'Amazon Linux 2023'}
ORIGIN = 'http://127.0.0.1:15173'
CONTROL = 'https://control.xingdu.test:8443'


def command(args, data=None, check=True):
    p = subprocess.run(args, cwd=ROOT, input=data, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if check and p.returncode:
        # Never echo stdin, HTTP bodies or commands carrying credentials.
        raise RuntimeError(f'{args[0]} failed (exit {p.returncode}); inspect local lab logs')
    return p


def compose(*args, data=None, check=True):
    return command(COMPOSE + list(args), data, check)


def execute(system, *args, data=None, check=True):
    return compose('exec', '-T', 'lab-' + system, *args, data=data, check=check)


def save(path, value):
    path.write_text(json.dumps(value, indent=2) + '\n')
    path.chmod(0o600)


def progress(message):
    print(message, flush=True)


def eventually(fn, timeout=60):
    until = time.monotonic() + timeout
    while time.monotonic() < until:
        value = fn()
        if value:
            return value
        time.sleep(2)
    raise RuntimeError('Timed out waiting for ' + fn.__name__)


class API:
    def __init__(self):
        self.client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        self.headers = {'Content-Type': 'application/json', 'Origin': ORIGIN, 'X-Xingdu-Request': '1'}
        credentials = json.loads((ROOT / '.local/initial-admin.json').read_text())
        session = self.call('POST', '/api/v1/auth/login', credentials)
        self.headers['X-CSRF-Token'] = session['csrf_token']
        orgs = self.call('GET', '/api/v1/organizations')
        self.org = next(o for o in orgs if o['role'] in ('owner', 'admin'))['id']
        self.headers['X-Xingdu-Organization'] = self.org

    def call(self, method, path, body=None):
        request = urllib.request.Request(ORIGIN + path, method=method, headers=self.headers,
                                         data=None if method == 'GET' else json.dumps(body or {}).encode())
        try:
            with self.client.open(request, timeout=20) as response:
                raw = response.read()
                return json.loads(raw).get('data') if raw else None
        except urllib.error.HTTPError as error:
            payload = json.loads(error.read())
            code = payload.get('error', {}).get('code', 'unknown')
            raise RuntimeError(f'HTTP {error.code}: {code}') from None

    def machine(self, fixture):
        return self.call('GET', '/api/v1/hosts/' + fixture['id'] + '/machine')

    def host(self, fixture):
        return next(h for h in self.call('GET', '/api/v1/hosts') if h['id'] == fixture['id'])

    def action(self, fixture, action, body=None, method='POST'):
        return self.call(method, '/api/v1/hosts/' + fixture['id'] + '/' + action, body)


def prepare():
    LOCAL.mkdir(parents=True, exist_ok=True)
    LOCAL.chmod(0o700)
    if not (LOCAL / 'ca.crt').exists():
        config = LOCAL / 'cert.cnf'
        config.write_text('''[req]
distinguished_name=dn
prompt=no
[dn]
CN=Xingdu disposable lab CA
[ca]
basicConstraints=critical,CA:TRUE
keyUsage=critical,keyCertSign,cRLSign
[server]
basicConstraints=critical,CA:FALSE
keyUsage=critical,digitalSignature,keyEncipherment
extendedKeyUsage=serverAuth
subjectAltName=DNS:control.xingdu.test
''')
        command(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '30', '-keyout', str(LOCAL/'ca.key'), '-out', str(LOCAL/'ca.crt'), '-config', str(config), '-extensions', 'ca'])
        command(['openssl', 'req', '-new', '-newkey', 'rsa:2048', '-nodes', '-subj', '/CN=control.xingdu.test', '-keyout', str(LOCAL/'server.key'), '-out', str(LOCAL/'server.csr')])
        command(['openssl', 'x509', '-req', '-in', str(LOCAL/'server.csr'), '-CA', str(LOCAL/'ca.crt'), '-CAkey', str(LOCAL/'ca.key'), '-CAcreateserial', '-days', '30', '-extfile', str(config), '-extensions', 'server', '-out', str(LOCAL/'server.crt')])
    command(['openssl', 'x509', '-checkend', '86400', '-noout', '-in', str(LOCAL/'server.crt')])
    if not (LOCAL / 'credentials.json').exists():
        save(LOCAL/'credentials.json', {s: secrets.token_urlsafe(24) for s in SYSTEMS})
    if not (LOCAL/'ssh.key').exists():
        passphrase = LOCAL/'ssh-passphrase'
        passphrase.write_text(secrets.token_urlsafe(24))
        passphrase.chmod(0o600)
        command(['ssh-keygen', '-q', '-t', 'rsa', '-b', '2048', '-m', 'PEM', '-N', '', '-C', 'xingdu-disposable-lab', '-f', str(LOCAL/'ssh-plain')])
        command(['openssl', 'rsa', '-in', str(LOCAL/'ssh-plain'), '-traditional', '-aes256', '-passout', 'file:' + str(passphrase), '-out', str(LOCAL/'ssh.key')])
        (LOCAL/'ssh-plain').unlink()
        (LOCAL/'ssh-plain.pub').rename(LOCAL/'ssh.pub')
    for p in LOCAL.glob('*.key'):
        p.chmod(0o600)
    progress('Prepared temporary TLS CA and random SSH credentials (not installed on host).')


def up():
    prepare()
    progress('Building and starting the three systemd targets; output: .local/agent-lab/compose.log')
    with (LOCAL/'compose.log').open('wb') as log:
        result = subprocess.run(COMPOSE + ['up', '--build', '-d', '--wait', '--wait-timeout', '120'], cwd=ROOT, stdout=log, stderr=log)
    if result.returncode:
        raise RuntimeError('Lab startup failed; inspect .local/agent-lab/compose.log')
    passwords = json.loads((LOCAL/'credentials.json').read_text())
    for system in SYSTEMS:
        execute(system, 'chpasswd', data=('labadmin:' + passwords[system] + '\n').encode())
        execute(system, 'sh', '-c', 'cat > /home/labadmin/.ssh/authorized_keys; chmod 600 /home/labadmin/.ssh/authorized_keys; chown labadmin:labadmin /home/labadmin/.ssh/authorized_keys', data=(LOCAL/'ssh.pub').read_bytes())
        execute(system, 'curl', '-fsS', CONTROL + '/health/ready')
    progress('All target SSH services and trusted HTTPS connectivity are ready.')


def reset_install(system):
    # Only these known disposable containers; never a user-supplied host/path.
    execute(system, 'systemctl', 'disable', '--now', 'xingdu-agent', check=False)
    execute(system, 'sh', '-c', 'rm -f /etc/systemd/system/xingdu-agent.service /usr/local/bin/xingdu-agent; rm -rf /var/lib/xingdu-agent; systemctl daemon-reload; systemctl reset-failed', check=True)


def verify_service(system, mode):
    execute(system, 'systemctl', 'is-active', 'xingdu-agent')
    execute(system, 'systemctl', 'is-enabled', 'xingdu-agent')
    expected = 'root' if mode == 'manage' else 'xingdu-agent'
    actual = execute(system, 'systemctl', 'show', 'xingdu-agent', '-p', 'User', '--value').stdout.decode().strip()
    assert actual == expected, 'Wrong service user'
    permissions = execute(system, 'stat', '-c', '%a %U', '/var/lib/xingdu-agent/agent.json').stdout.decode().strip()
    assert permissions == '600 ' + expected, 'Wrong credential permissions'
    uid = execute(system, 'sh', '-c', 'pid=$(systemctl show xingdu-agent -p MainPID --value); ps -o uid= -p "$pid"').stdout.strip()
    assert (uid == b'0') == (mode == 'manage'), 'Wrong process privilege'
    if mode == 'monitor':
        assert execute(system, 'systemctl', 'show', 'xingdu-agent', '-p', 'NoNewPrivileges', '--value').stdout.strip() == b'yes'
        assert execute(system, 'systemctl', 'show', 'xingdu-agent', '-p', 'CapabilityBoundingSet', '--value').stdout.strip() == b''
        caps = execute(system, 'sh', '-c', 'pid=$(systemctl show xingdu-agent -p MainPID --value); sed -n "s/^CapEff:[[:space:]]*//p" /proc/"$pid"/status').stdout.strip()
        assert int(caps, 16) == 0, 'Agent process has unexpected capabilities'


def test():
    api = API()
    fixtures_path = LOCAL/'fixtures.json'
    fixtures = json.loads(fixtures_path.read_text()) if fixtures_path.exists() else {}
    report = {'platform': command(['docker', 'info', '--format', '{{.Architecture}}']).stdout.decode().strip(), 'systems': {}}
    save(LOCAL/'report.json', report)
    for system, label in SYSTEMS.items():
        if system not in fixtures:
            fixtures[system] = api.call('POST', '/api/v1/hosts', {'name': label + ' · Docker Lab', 'address': 'lab-' + system, 'ssh_port': 22, 'ssh_user': 'labadmin', 'tags': ['agent-lab', system], 'notes': 'Disposable local Docker systemd test; not a real VPS/EC2 instance.'})
            save(fixtures_path, fixtures)
        f = fixtures[system]
        api.action(f, 'agent', method='DELETE')
        reset_install(system)
        mount = execute(system, 'sh', '-c', 'cat /proc/mounts').stdout.decode()
        assert any(' /tmp ' in line and 'noexec' in line for line in mount.splitlines()), 'noexec regression fixture missing'
        enrollment = api.action(f, 'enrollment', {'mode': 'monitor'})
        execute(system, 'sh', '-c', 'curl -fsS ' + CONTROL + '/api/v1/agent/install.sh -o /tmp/xingdu-install.sh && sh /tmp/xingdu-install.sh monitor', data=(enrollment['token'] + '\n').encode())
        eventually(lambda: api.host(f)['status'] == 'online')
        verify_service(system, 'monitor')
        distro = execute(system, 'cat', '/etc/os-release').stdout.decode()
        report['systems'][system] = {'os_release': distro, 'manual_systemd_monitor': 'passed'}
        save(LOCAL/'report.json', report)
        progress(label + ': manual HTTPS installer + systemd + low-privilege heartbeat PASS')
    initial = {s: api.host(f)['last_seen_at'] for s, f in fixtures.items()}
    eventually(lambda: all(api.host(f)['last_seen_at'] != initial[s] for s, f in fixtures.items()), 45)
    progress('All systems: consecutive heartbeats PASS')
    for system in SYSTEMS:
        compose('restart', 'lab-' + system)
    eventually(lambda: all(execute(s, 'systemctl', 'is-active', 'xingdu-agent', check=False).returncode == 0 for s in SYSTEMS), 60)
    for system in SYSTEMS:
        verify_service(system, 'monitor')
        report['systems'][system]['container_restart'] = 'passed'
    progress('All systems: systemd starts Agent after container restart PASS')
    before = {s: api.host(f)['last_seen_at'] for s, f in fixtures.items()}
    compose('stop', 'lab-tls')
    try:
        progress('Testing control-plane outage and 90-second offline detection…')
        eventually(lambda: all(api.host(f)['status'] == 'offline' for f in fixtures.values()), 115)
        for system in SYSTEMS:
            execute(system, 'systemctl', 'is-active', 'xingdu-agent')
    finally:
        compose('start', 'lab-tls')
    eventually(lambda: all(api.host(f)['status'] == 'online' and api.host(f)['last_seen_at'] != before[s] for s, f in fixtures.items()), 50)
    progress('All systems: offline detection + automatic reconnect without new token PASS')
    for system, f in fixtures.items():
        report['systems'][system]['outage_recovery'] = 'passed'
        api.action(f, 'agent', method='DELETE')
    eventually(lambda: all(execute(s, 'systemctl', 'show', 'xingdu-agent', '-p', 'ActiveState', '--value').stdout.strip() == b'inactive' for s in SYSTEMS), 45)
    for system in SYSTEMS:
        report['systems'][system]['revocation_stops_service'] = 'passed'
        assert execute(system, 'systemctl', 'show', 'xingdu-agent', '-p', 'ExecMainStatus', '--value').stdout.strip() == b'0'
        reset_install(system)
    save(LOCAL/'report.json', report)
    progress('All systems: revocation cleanly stops Agent with no restart loop PASS')
    passwords = json.loads((LOCAL/'credentials.json').read_text())
    # Separate trusted channel: read host public key inside each known container.
    for system, f in fixtures.items():
        fp = execute(system, 'ssh-keygen', '-lf', '/etc/ssh/ssh_host_ed25519_key.pub', '-E', 'sha256').stdout.decode().split()[1]
        scanned = api.action(f, 'ssh/fingerprint')['fingerprint']
        assert fp == scanned, 'SSH fingerprint mismatch'
        mode = 'manage' if system == 'amazon' else 'monitor'
        body = {'mode': mode, 'confirm_manage': mode == 'manage', 'fingerprint': fp, 'confirm_fingerprint': True, 'retain': system == 'debian'}
        if system == 'amazon':
            body.update(method='pem', private_key=(LOCAL/'ssh.key').read_text(), passphrase=(LOCAL/'ssh-passphrase').read_text())
        else:
            body.update(method='password', password=passwords[system])
        job = api.action(f, 'ssh/install', body)
        def installed():
            current = next(j for j in api.machine(f)['jobs'] if j['id'] == job['id'])
            if current['state'] in ('failed', 'cancelled'):
                raise RuntimeError(system + ': ' + current['result'])
            return current['state'] == 'installed'
        eventually(installed, 125)
        eventually(lambda: api.host(f)['status'] == 'online')
        verify_service(system, mode)
        machine = api.machine(f)
        assert machine['agent']['mode'] == mode
        assert bool(machine['credential']) == (system == 'debian')
        report['systems'][system]['ssh_auth'] = 'encrypted_pem' if system == 'amazon' else 'password'
        report['systems'][system]['ssh_systemd_' + mode] = 'passed'
        save(LOCAL/'report.json', report)
        progress(SYSTEMS[system] + ': SSH ' + report['systems'][system]['ssh_auth'] + ' / ' + mode + ' installation PASS')
    f = fixtures['debian']
    api.action(f, 'agent', method='DELETE')
    reset_install('debian')
    fingerprint = api.machine(f)['credential']['fingerprint']
    job = api.action(f, 'ssh/install', {'mode': 'monitor', 'use_saved': True, 'fingerprint': fingerprint, 'confirm_fingerprint': True})
    def saved_install():
        current = next(j for j in api.machine(f)['jobs'] if j['id'] == job['id'])
        if current['state'] in ('failed', 'cancelled'):
            raise RuntimeError('saved credential: ' + current['result'])
        return current['state'] == 'installed'
    eventually(saved_install, 125)
    eventually(lambda: api.host(f)['status'] == 'online')
    verify_service('debian', 'monitor')
    report['systems']['debian']['saved_credential_reuse'] = 'passed'
    save(LOCAL/'report.json', report)
    progress('Debian: reinstall using retained encrypted credential PASS')
    api.call('POST', '/api/v1/auth/logout')
    progress('PASS. Three machines remain online for inspection. Report: .local/agent-lab/report.json')


def cleanup():
    fixtures = LOCAL/'fixtures.json'
    if fixtures.exists():
        api = API()
        for f in json.loads(fixtures.read_text()).values():
            api.action(f, 'agent', method='DELETE')
            api.call('DELETE', '/api/v1/hosts/' + f['id'])
        fixtures.unlink()
        api.call('POST', '/api/v1/auth/logout')
    compose('stop', 'lab-ubuntu', 'lab-debian', 'lab-amazon', 'lab-tls')
    compose('rm', '-f', 'lab-ubuntu', 'lab-debian', 'lab-amazon', 'lab-tls')
    command(['docker', 'compose', 'up', '-d', '--wait', 'api', 'worker', 'web'])
    progress('Removed only lab fixtures/containers and restored normal API/Worker configuration. Local CA/credentials retained under .local/agent-lab.')


if __name__ == '__main__':
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['prepare', 'up', 'test', 'cleanup'])
    args = parser.parse_args()
    try:
        globals()[args.action]()
    except (RuntimeError, AssertionError) as error:
        progress('FAIL: ' + str(error))
        raise SystemExit(1)

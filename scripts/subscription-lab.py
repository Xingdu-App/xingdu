#!/usr/bin/env python3
"""Exercise basic subscriptions against disposable local Agent Lab nodes only."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import urllib.error
import urllib.request

spec = importlib.util.spec_from_file_location('agent_lab', Path(__file__).with_name('agent-lab.py'))
lab = importlib.util.module_from_spec(spec)
spec.loader.exec_module(lab)


def fetch(path, expected=200):
    try:
        with urllib.request.urlopen(lab.ORIGIN + path, timeout=10) as response:
            status, headers, content = response.status, response.headers, response.read()
    except urllib.error.HTTPError as error:
        status, headers, content = error.code, error.headers, error.read()
    if status != expected:
        raise RuntimeError(f'Subscription response {status}, expected {expected}')
    assert headers.get('Cache-Control') == 'no-store'
    return content


def main():
    api = lab.API()
    hosts = {h['id']: h for h in api.call('GET', '/api/v1/hosts')}
    nodes = [n for n in api.call('GET', '/api/v1/nodes') if
             n['action'] == 'deploy' and n['state'] == 'succeeded' and
             hosts[n['host_id']]['address'] in ('lab-ubuntu', 'lab-debian', 'lab-amazon') and
             'agent-lab' in hosts[n['host_id']]['tags']]
    if not nodes:
        raise RuntimeError('A successfully deployed disposable Agent Lab node is required')
    body = {'name': 'Disposable subscription acceptance', 'node_ids': [n['id'] for n in nodes],
            'rules': [{'type': 'domain_suffix', 'value': 'example.com', 'target': 'direct'}],
            'final_action': 'proxy', 'enabled': True}
    created = api.call('POST', '/api/v1/subscriptions', body)
    sub_id = created['subscription']['id']
    path = created['subscription_path']
    tokens = [path.split('token=')[1].split('&')[0]]
    root = lab.ROOT / '.local/subscription-lab'
    root.mkdir(mode=0o700, exist_ok=True)
    config = root / 'config.yaml'
    try:
        content = fetch(path)
        assert created['subscription']['format'] == 'stash'
        assert b'server-cert-fingerprint:' in content and b'fingerprint:' in content
        assert b'PRIVATE KEY' not in content and b'DOMAIN-SUFFIX,example.com,DIRECT' in content
        if any(n['protocol'] == 'hysteria2' for n in nodes):
            assert b'auth:' in content
        mihomo = fetch(path + '&format=mihomo')
        assert b'server-cert-fingerprint:' not in mihomo and b'fingerprint:' in mihomo
        assert fetch(path + '&format=clash') == mihomo
        fetch(path + '&format=surge', 422)
        metadata = json.dumps(api.call('GET', '/api/v1/subscriptions'))
        assert tokens[0] not in metadata and 'credential' not in metadata
        config.write_bytes(mihomo)
        config.chmod(0o600)
        binary = os.environ.get('XINGDU_TEST_MIHOMO_BINARY')
        if binary:
            parsed = subprocess.run([binary, '-t', '-d', str(root), '-f', str(config)], capture_output=True)
            if parsed.returncode:
                raise RuntimeError('Mihomo rejected the live subscription configuration')
        replacement = api.call('POST', '/api/v1/subscriptions/' + sub_id + '/rotate')['subscription_path']
        tokens.append(replacement.split('token=')[1].split('&')[0])
        fetch(path, 404)
        fetch(replacement)
        body['enabled'] = False
        api.call('PUT', '/api/v1/subscriptions/' + sub_id, body)
        fetch(replacement, 404)
        body['enabled'] = True
        body['rules'][0]['target'] = 'reject'
        api.call('PUT', '/api/v1/subscriptions/' + sub_id, body)
        assert b'DOMAIN-SUFFIX,example.com,REJECT' in fetch(replacement)
        logs = lab.compose('logs', '--since', '2m', 'web').stdout
        assert all(token.encode() not in logs for token in tokens), 'Bearer token found in proxy logs'
        report = {'scope': 'local Docker Agent Lab only', 'nodes': len(nodes),
                  'stash_default_and_mihomo_override': 'PASS', 'create_export_edit': 'PASS', 'rotate_disable': 'PASS',
                  'metadata_and_proxy_log_redaction': 'PASS',
                  'mihomo_parser': 'PASS' if binary else 'not requested'}
    finally:
        api.call('DELETE', '/api/v1/subscriptions/' + sub_id)
        fetch(path, 404)
        config.unlink(missing_ok=True)
        api.call('POST', '/api/v1/auth/logout')
    report['delete'] = 'PASS'
    lab.save(root / 'report.json', report)
    print(json.dumps(report))


if __name__ == '__main__':
    main()

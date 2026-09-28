#!/usr/bin/env python3
"""Non-destructive Ubuntu lab check of local service status and controlled restart.
Preserves enrolled identity and installed nodes. Updates only the known disposable
Ubuntu agent binary; stops/restarts a demo service briefly, restoring it on exit.
"""
import importlib.util
import json
from pathlib import Path

spec = importlib.util.spec_from_file_location('protocol_lab', Path(__file__).with_name('protocol-lab.py'))
protocol_lab = importlib.util.module_from_spec(spec)
spec.loader.exec_module(protocol_lab)
lab = protocol_lab.lab


def main():
    fixtures = json.loads((lab.LOCAL / 'fixtures.json').read_text())
    host = fixtures['ubuntu']
    api = lab.API()
    current = api.host(host)
    assert current['address'] == 'lab-ubuntu' and 'agent-lab' in current['tags'], 'Not a disposable lab host'
    assert api.machine(host)['agent']['mode'] == 'manage'
    nodes = [n for n in api.call('GET', '/api/v1/nodes') if n['host_id'] == host['id'] and n['state'] == 'succeeded']
    assert nodes, 'Requires existing demo nodes; no installations will be removed'
    lab.command(['make', 'agents'])
    arch = lab.execute('ubuntu', 'uname', '-m').stdout.strip()
    arch = 'arm64' if arch == b'aarch64' else 'amd64' if arch == b'x86_64' else None
    assert arch, 'Unsupported lab architecture'
    lab.compose('cp', str(lab.ROOT / ('bin/agents/xingdu-agent-linux-' + arch)), 'lab-ubuntu:/usr/local/bin/xingdu-agent.new')
    lab.execute('ubuntu', 'chmod', '755', '/usr/local/bin/xingdu-agent.new')
    lab.execute('ubuntu', 'mv', '/usr/local/bin/xingdu-agent.new', '/usr/local/bin/xingdu-agent')
    lab.execute('ubuntu', 'systemctl', 'restart', 'xingdu-agent')
    lab.eventually(lambda: api.machine(host)['agent']['metrics']['version'] == '0.6.0-dev')
    def node(id):
        return next(n for n in api.call('GET', '/api/v1/nodes') if n['id'] == id)
    for n in nodes:
        lab.eventually(lambda: node(n['id'])['service_status'] == 'active')
    target = nodes[0]
    unit = 'xingdu-protocol-' + target['id'] + '.service'
    try:
        lab.execute('ubuntu', 'systemctl', 'stop', unit)
        lab.eventually(lambda: node(target['id'])['service_status'] == 'inactive')
        api.action(host, 'deployments/' + target['id'] + '/restart', {'confirm': True})
        lab.eventually(lambda: node(target['id'])['state'] == 'succeeded' and node(target['id'])['result'] == 'restarted')
        lab.eventually(lambda: node(target['id'])['service_status'] == 'active')
        protocol_lab.prepare_target('ubuntu')
        protocol_lab.connection_test('ubuntu', api.action(host, 'deployments/' + target['id'] + '/connection'))
        assert node(target['id'])['installed_at'] == target['installed_at'], 'Node identity changed'
    finally:
        lab.execute('ubuntu', 'systemctl', 'start', unit)
    report = {'agent': '0.6.0-dev', 'system': 'Ubuntu 24.04', 'arch': arch, 'scope': 'local isolated Docker; not public health validation', 'checks': ['existing identity and nodes preserved', 'systemd active and stopped status reported', 'API-controlled restart completed', 'real authenticated forwarding after restart', 'installed_at unchanged'], 'node_count': len(nodes)}
    lab.save(lab.LOCAL / 'node-lifecycle-report.json', report)
    lab.progress('Ubuntu managed status + controlled restart + authenticated forwarding PASS')


if __name__ == '__main__':
    main()

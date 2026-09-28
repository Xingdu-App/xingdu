"""Resolve local pre-migration lab references without rewriting local fixtures.

Resource IDs and protocol credentials are different namespaces. Only persisted
server references are converted; UUID credentials must never be rewritten.
"""
import re

_SERVER_ID = re.compile(r"srv_[0-9a-f]{32}\Z")
_LEGACY_UUID = re.compile(r"[0-9a-fA-F]{8}(?:-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}\Z")
_SYSTEMS = frozenset(("ubuntu", "debian", "amazon"))


def migrated_server_id(value):
    if not isinstance(value, str):
        raise RuntimeError('Invalid local lab server reference')
    if _SERVER_ID.fullmatch(value):
        return value
    if _LEGACY_UUID.fullmatch(value):
        return 'srv_' + value.replace('-', '').lower()
    raise RuntimeError('Invalid local lab server reference')


def resolve_fixtures(fixtures, hosts):
    """Require the exact migrated identity and known disposable target metadata.

    Never guess by name/address alone: a missing ID must not select another VPS.
    Returns fresh dictionaries; callers retain the original file unchanged.
    """
    by_id = {host['id']: host for host in hosts}
    resolved = {}
    for system, fixture in fixtures.items():
        if system not in _SYSTEMS:
            raise RuntimeError('Unknown disposable lab system')
        resource_id = migrated_server_id(fixture.get('id'))
        host = by_id.get(resource_id)
        address = 'lab-' + system
        if (host is None or host.get('address') != address or
                'agent-lab' not in host.get('tags', []) or
                fixture.get('address', address) != address):
            raise RuntimeError('Lab fixture no longer matches the migrated disposable server; inspect it before continuing')
        resolved[system] = {**fixture, 'id': resource_id}
    return resolved

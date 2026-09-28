import copy
import unittest

from lab_ids import migrated_server_id, resolve_fixtures


class LabIDs(unittest.TestCase):
    def test_legacy_reference_resolves_without_mutation(self):
        legacy = '12345678-90ab-cdef-1234-567890abcdef'
        resource_id = 'srv_1234567890abcdef1234567890abcdef'
        fixtures = {'ubuntu': {'id': legacy, 'address': 'lab-ubuntu'}}
        original = copy.deepcopy(fixtures)
        hosts = [{'id': resource_id, 'address': 'lab-ubuntu', 'tags': ['agent-lab']}]
        self.assertEqual(resolve_fixtures(fixtures, hosts)['ubuntu']['id'], resource_id)
        self.assertEqual(fixtures, original)
        self.assertEqual(migrated_server_id(resource_id), resource_id)
        self.assertEqual(migrated_server_id(legacy.upper()), resource_id)

    def test_wrong_namespace_or_unsafe_identifiers_rejected(self):
        for value in ['node_' + 'a' * 32, 'srv_' + 'a' * 31, 'srv_' + 'A' * 32,
                      'srv_' + 'a' * 32 + '/jobs', '../hosts', None]:
            with self.subTest(value=value), self.assertRaises(RuntimeError):
                migrated_server_id(value)

    def test_never_substitutes_another_host_or_real_vps(self):
        resource_id = 'srv_' + 'a' * 32
        fixtures = {'ubuntu': {'id': resource_id}}
        for host in [
            {'id': 'srv_' + 'b' * 32, 'address': 'lab-ubuntu', 'tags': ['agent-lab']},
            {'id': resource_id, 'address': 'vps.example.test', 'tags': ['agent-lab']},
            {'id': resource_id, 'address': 'lab-ubuntu', 'tags': []},
        ]:
            with self.subTest(host=host), self.assertRaises(RuntimeError):
                resolve_fixtures(fixtures, [host])


if __name__ == '__main__':
    unittest.main()

import builtins
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

import yaml


SCRIPT = Path(__file__).resolve().parents[1] / 'deploy/a1-sync-release.sh'
SOURCE = SCRIPT.read_text().split("<<'PY'\n", 1)[1].split('\nPY\n', 1)[0]
REMOTE = '/opt/a1-sync-release/new-release'
STAMP = 'test-release'
UNIT = ('[Service]\nExecStart=docker run '
        '-v /opt/a1-sync-release/old-release/a1-user-sync:'
        '/usr/local/bin/a1-user-sync:ro image\n')
BASE = "enabled: false\npassword: 'no: # secret & x'\ntarget_database:\n  dbname: sub2api\nuser_ids: [1, 2]\n"


def node_structure(node):
    if isinstance(node, yaml.ScalarNode):
        return node.tag, node.value, node.style
    if isinstance(node, yaml.SequenceNode):
        return node.tag, node.flow_style, tuple(node_structure(item) for item in node.value)
    return node.tag, node.flow_style, tuple(
        (node_structure(key), node_structure(value)) for key, value in node.value)


def unrelated_nodes(text):
    root = yaml.compose(text, Loader=yaml.SafeLoader)
    return root.tag, root.flow_style, tuple(
        (node_structure(key), node_structure(value)) for key, value in root.value
        if key.value != 'status_database_names')


class ReleaseConfigTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        # Compile a tiny reader with the service's existing yaml.v3 module.
        # No dependency downloads, module edits or service build outputs.
        directory = tempfile.TemporaryDirectory(prefix='a1-yaml-go-test-')
        cls.addClassCleanup(directory.cleanup)
        root = Path(directory.name)
        helper = root / 'read-config.go'
        helper.write_text('''package main
import (
    "encoding/json"
    "os"
    "gopkg.in/yaml.v3"
)
func main() {
    var config struct {
        SourceDatabase struct { Password string `yaml:"password"` } `yaml:"source_database"`
        TargetDatabase struct { Password string `yaml:"password"` } `yaml:"target_database"`
        Values map[string]string `yaml:"values"`
    }
    content, err := os.ReadFile(os.Args[1]); if err != nil { panic(err) }
    if err := yaml.Unmarshal(content, &config); err != nil { panic(err) }
    if err := json.NewEncoder(os.Stdout).Encode(config); err != nil { panic(err) }
}
''')
        build = root / 'build.sh'
        build.write_text('set -euo pipefail\ngo clean -cache\ngo build -o "$1" "$2"\n')
        cls.go_reader = root / 'read-config'
        subprocess.run(['bash', str(build), str(cls.go_reader), str(helper)],
                       cwd=SCRIPT.parents[1] / 'services/a1-user-sync-service',
                       env={**os.environ, 'GOTOOLCHAIN': 'local', 'GOPROXY': 'off',
                            'GOSUMDB': 'off', 'GOFLAGS': ''}, check=True,
                       capture_output=True, text=True, timeout=120)

    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.unit = self.root / 'a1-user-sync.service'
        self.config = self.root / 'config.yaml'
        self.unit.write_text(UNIT)
        self.unit.chmod(0o604)

    def prepare(self, text):
        self.config.write_text(text)
        self.config.chmod(0o640)
        self.originals = {path: (path.read_bytes(), self.metadata(path))
                          for path in (self.unit, self.config)}

    def metadata(self, path):
        info = path.stat()
        return info.st_uid, info.st_gid, stat.S_IMODE(info.st_mode)

    def run_update(self, fail_second=False, missing_yaml=False):
        replace = os.replace
        imports = builtins.__import__
        calls = []

        def checked_replace(source, target):
            source, target = Path(source), Path(target)
            self.assertEqual(source.parent, target.parent)
            self.assertEqual(self.metadata(source), self.originals[target][1])
            # Both backups must exist before either live file changes.
            for path, (content, metadata) in self.originals.items():
                backup = Path(str(path) + '.bak-' + STAMP)
                self.assertEqual(backup.read_bytes(), content)
                self.assertEqual(self.metadata(backup), metadata)
            calls.append(target)
            if fail_second and len(calls) == 2:
                raise OSError('injected second replacement failure')
            replace(source, target)

        def checked_import(name, *args, **kwargs):
            if missing_yaml and name == 'yaml':
                raise ImportError('injected missing PyYAML')
            return imports(name, *args, **kwargs)

        with mock.patch.object(sys, 'argv', ['release', REMOTE, STAMP, str(self.unit), str(self.config)]), \
                mock.patch('os.replace', side_effect=checked_replace), \
                mock.patch('builtins.__import__', side_effect=checked_import):
            exec(compile(SOURCE, str(SCRIPT), 'exec'), {})
        return calls

    def assert_unchanged(self):
        for path, (content, metadata) in self.originals.items():
            self.assertEqual(path.read_bytes(), content)
            self.assertEqual(self.metadata(path), metadata)
        self.assertFalse(list(self.root.glob('.*')))

    def test_inline_block_and_missing(self):
        for suffix in ('status_database_names: [old, other]\n',
                       'status_database_names:\n  - old\n  - other\n', ''):
            with self.subTest(suffix=suffix):
                for backup in self.root.glob('*.bak-*'):
                    backup.unlink()
                self.unit.write_text(UNIT)
                self.prepare(BASE + suffix)
                expected = yaml.safe_load(BASE)
                expected['status_database_names'] = ['sub2api_3dbooks']
                self.assertEqual(self.run_update(), [self.config, self.unit])
                self.assertEqual(yaml.safe_load(self.config.read_text()), expected)
                self.assertEqual(self.unit.read_text(), UNIT.replace('old-release', 'new-release'))
                for path, (_, metadata) in self.originals.items():
                    self.assertEqual(self.metadata(path), metadata)
                self.assertFalse(list(self.root.glob('.*')))

    def test_malformed_yaml_does_not_change_unit_or_config(self):
        self.prepare(BASE + 'status_database_names: [unfinished\n')
        with self.assertRaisesRegex(SystemExit, 'Invalid YAML configuration'):
            self.run_update()
        self.assert_unchanged()
        self.assertFalse(list(self.root.glob('*.bak-*')))

    def read_with_go(self):
        result = subprocess.run([str(self.go_reader), str(self.config)],
                                check=True, capture_output=True, text=True, timeout=10)
        return json.loads(result.stdout)

    def test_scalar_spelling_tags_styles_and_go_string_values_are_preserved(self):
        text = '''enabled: false
source_database:
  password: on
target_database:
  password: 012
values:
  on_value: on
  off_value: off
  yes_value: yes
  no_value: no
  octal_value: 012
  date_value: 2026-11-10
  timestamp_value: 2026-11-10T01:02:03Z
  single_quoted: 'off'
  double_quoted: "yes"
  literal: |
    on
    012
  folded: >-
    no
    2026-11-10
  tagged: !!str '012'
defaults: &defaults {password: off, nested: {status_database_names: [keep]}}
alias: *defaults
status_database_names:
  - old
'''
        self.prepare(text)
        before = self.read_with_go()
        self.run_update()
        self.assertEqual(self.read_with_go(), before)
        self.assertEqual(before['SourceDatabase']['Password'], 'on')
        self.assertEqual(before['TargetDatabase']['Password'], '012')
        for key, value in [('on_value', 'on'), ('off_value', 'off'),
                           ('yes_value', 'yes'), ('no_value', 'no'),
                           ('octal_value', '012'), ('date_value', '2026-11-10'),
                           ('timestamp_value', '2026-11-10T01:02:03Z')]:
            self.assertEqual(before['Values'][key], value)
        self.assertEqual(unrelated_nodes(self.config.read_text()), unrelated_nodes(text))
        self.assertEqual(yaml.safe_load(self.config.read_text())['status_database_names'],
                         ['sub2api_3dbooks'])

    def test_duplicate_target_key_is_rejected_without_modification(self):
        self.prepare(BASE + 'status_database_names: [one]\nstatus_database_names: [two]\n')
        with self.assertRaisesRegex(SystemExit, 'Duplicate status_database_names'):
            self.run_update()
        self.assert_unchanged()

    def test_go_database_passwords_keep_each_ambiguous_scalar(self):
        for token in ('on', 'off', 'yes', 'no', '012', '2026-11-10',
                      '2026-11-10T01:02:03Z'):
            with self.subTest(password=token):
                for backup in self.root.glob('*.bak-*'):
                    backup.unlink()
                self.unit.write_text(UNIT)
                text = ('source_database: {password: ' + token + '}\n'
                        'target_database: {password: ' + token + '}\n')
                self.prepare(text)
                before = self.read_with_go()
                self.assertEqual(before['SourceDatabase']['Password'], token)
                self.assertEqual(before['TargetDatabase']['Password'], token)
                self.run_update()
                self.assertEqual(self.read_with_go(), before)
                self.assertEqual(unrelated_nodes(self.config.read_text()), unrelated_nodes(text))

    def test_roundtrip_node_change_is_rejected_before_any_file_change(self):
        self.prepare(BASE)
        serialize = yaml.serialize

        def corrupt(*args, **kwargs):
            return serialize(*args, **kwargs).replace('enabled: false', 'enabled: true')

        with mock.patch.object(yaml, 'serialize', side_effect=corrupt):
            with self.assertRaisesRegex(SystemExit, 'YAML validation failed'):
                self.run_update()
        self.assert_unchanged()

    def test_nonmapping_yaml_does_not_change_unit_or_config(self):
        self.prepare('- not\n- a mapping\n')
        with self.assertRaisesRegex(SystemExit, 'must be a mapping'):
            self.run_update()
        self.assert_unchanged()

    def test_invalid_unit_does_not_change_config(self):
        self.unit.write_text('[Service]\nExecStart=unexpected\n')
        self.prepare(BASE)
        with self.assertRaisesRegex(SystemExit, 'Unexpected service mount'):
            self.run_update()
        self.assert_unchanged()

    def test_missing_yaml_does_not_change_unit_or_config(self):
        self.prepare(BASE)
        with self.assertRaisesRegex(SystemExit, 'python3-yaml is required'):
            self.run_update(missing_yaml=True)
        self.assert_unchanged()

    def test_second_replace_failure_restores_content_owner_and_mode(self):
        self.prepare(BASE)
        with self.assertRaisesRegex(OSError, 'second replacement failure'):
            self.run_update(fail_second=True)
        self.assert_unchanged()
        self.assertEqual(len(list(self.root.glob('*.bak-*'))), 2)

    def test_existing_backup_is_not_overwritten(self):
        self.prepare(BASE)
        backup = Path(str(self.config) + '.bak-' + STAMP)
        backup.write_text('previous backup')
        with self.assertRaises(FileExistsError):
            self.run_update()
        self.assertEqual(backup.read_text(), 'previous backup')
        self.assert_unchanged()

    def test_systemctl_runs_only_after_python_success(self):
        script = SCRIPT.read_text()
        self.assertIn('set -euo pipefail', script.split("<<'REMOTE'\n", 1)[1])
        self.assertRegex(script, re.compile(r"\nPY\nsystemctl daemon-reload\nsystemctl restart", re.M))


if __name__ == '__main__':
    unittest.main(verbosity=2)

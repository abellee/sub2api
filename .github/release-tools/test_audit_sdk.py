import base64
import gzip
import hashlib
import io
from pathlib import Path
import struct
import subprocess
import tarfile
import tempfile
import unittest

from audit_sdk import restore


class PrivateSDKRestoreTest(unittest.TestCase):
    def fixture(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            subprocess.run(['git', 'init', '-q', str(root)], check=True)
            (root / 'go.mod').write_text('module test.example/sdk\n\ngo 1.26.5\n')
            subprocess.run(['git', '-C', str(root), 'add', '--all'], check=True)
            subprocess.run(['git', '-C', str(root), '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.test',
                            'commit', '-qm', 'SDK fixture'], check=True)
            commit = subprocess.check_output(['git', '-C', str(root), 'cat-file', 'commit', 'HEAD'])
            archive = subprocess.check_output(['git', '-C', str(root), 'archive', '--format=tar', 'HEAD'])
        expected = hashlib.sha1(b'commit ' + str(len(commit)).encode() + b'\0' + commit).hexdigest()
        return commit, archive, expected

    def encoded(self, commit, archive):
        return base64.b64encode(gzip.compress(struct.pack('>I', len(commit)) + commit + archive)).decode()

    def test_exact_snapshot_is_restored_without_git_metadata(self):
        commit, archive, expected = self.fixture()
        with tempfile.TemporaryDirectory() as directory:
            target = Path(directory) / 'sdk'
            restore(self.encoded(commit, archive), expected, target)
            self.assertIn('module test.example/sdk', (target/'go.mod').read_text())
            self.assertFalse((target/'.git').exists())

    def test_wrong_revision_cannot_replace_pin(self):
        commit, archive, _ = self.fixture()
        with self.assertRaisesRegex(ValueError, 'gitlink'):
            restore(self.encoded(commit, archive), '0'*40, Path('unused'))

    def test_modified_source_tree_is_rejected(self):
        commit, _, expected = self.fixture()
        stream = io.BytesIO()
        with tarfile.open(fileobj=stream, mode='w') as archive:
            item = tarfile.TarInfo('go.mod')
            item.size = 7
            archive.addfile(item, io.BytesIO(b'changed'))
        with tempfile.TemporaryDirectory() as directory, self.assertRaisesRegex(ValueError, 'source tree'):
            restore(self.encoded(commit, stream.getvalue()), expected, Path(directory)/'sdk')

    def test_path_escape_and_links_are_rejected(self):
        commit, _, expected = self.fixture()
        for name, kind in [('../escape', tarfile.REGTYPE), ('escape', tarfile.SYMTYPE), ('.git/config', tarfile.REGTYPE)]:
            with self.subTest(name=name):
                stream = io.BytesIO()
                with tarfile.open(fileobj=stream, mode='w') as archive:
                    item = tarfile.TarInfo(name)
                    item.type = kind
                    archive.addfile(item)
                with tempfile.TemporaryDirectory() as directory, self.assertRaisesRegex(ValueError, 'Unsafe|regular files'):
                    restore(self.encoded(commit, stream.getvalue()), expected, Path(directory)/'sdk')

    def test_missing_secret_and_existing_checkout_are_rejected(self):
        commit, archive, expected = self.fixture()
        with self.assertRaisesRegex(ValueError, 'Missing private SDK secret'):
            restore('', expected, Path('unused'))
        with tempfile.TemporaryDirectory() as directory:
            target = Path(directory)
            (target/'keep').write_text('existing')
            with self.assertRaisesRegex(ValueError, 'overwrite'):
                restore(self.encoded(commit, archive), expected, target)
            self.assertEqual((target/'keep').read_text(), 'existing')


if __name__ == '__main__':
    unittest.main()

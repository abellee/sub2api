"""Restore a pinned, privately supplied SDK for an ephemeral Actions build."""
import base64
import gzip
import hashlib
import io
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import struct
import subprocess
import sys
import tarfile
import tempfile

SDK_PATH = 'backend/audit-sdk'
MAX_PAYLOAD = 2 * 1024 * 1024
# Four repository-secret parts, each under GitHub's 48 KiB secret limit.
MAX_ENCODED = 192 * 1024


def expected_commit():
    entry = subprocess.check_output(['git', 'ls-tree', 'HEAD', '--', SDK_PATH], text=True).strip()
    match = re.fullmatch(r'160000 commit ([0-9a-f]{40})\t' + re.escape(SDK_PATH), entry)
    if not match:
        raise ValueError('Main-site SDK gitlink is missing')
    return match.group(1)


def restore(encoded, expected, destination):
    if not encoded:
        raise ValueError(f'Missing private SDK secret AUDIT_SDK_{expected}; source is never fetched from a public remote')
    if len(encoded) > MAX_ENCODED:
        raise ValueError('SDK secret exceeds the supported size')
    packed = base64.b64decode(encoded, validate=True)
    with gzip.GzipFile(fileobj=io.BytesIO(packed)) as stream:
        payload = stream.read(MAX_PAYLOAD + 1)
    if len(payload) > MAX_PAYLOAD or len(payload) < 4:
        raise ValueError('SDK payload size is invalid')
    size = struct.unpack('>I', payload[:4])[0]
    if size < 40 or size > 16384 or 4 + size >= len(payload):
        raise ValueError('SDK commit object size is invalid')
    commit = payload[4:4 + size]
    actual = hashlib.sha1(b'commit ' + str(size).encode() + b'\0' + commit).hexdigest()
    if actual != expected:
        raise ValueError('SDK commit does not match the main-site gitlink')
    tree = re.match(rb'tree ([0-9a-f]{40})\n', commit)
    if not tree:
        raise ValueError('SDK commit tree is invalid')
    destination = Path(destination)
    if destination.exists() and any(destination.iterdir()):
        raise ValueError('Refusing to overwrite an existing SDK checkout')
    with tempfile.TemporaryDirectory(prefix='audit-sdk-') as temporary:
        root = Path(temporary)
        with tarfile.open(fileobj=io.BytesIO(payload[4 + size:]), mode='r:') as archive:
            for member in archive:
                path = PurePosixPath(member.name)
                if path.is_absolute() or '..' in path.parts or '.git' in path.parts or '\\' in member.name or ':' in member.name:
                    raise ValueError('Unsafe SDK archive path')
                target = root.joinpath(*path.parts)
                if member.isdir():
                    target.mkdir(parents=True, exist_ok=True)
                elif member.isfile():
                    target.parent.mkdir(parents=True, exist_ok=True)
                    with archive.extractfile(member) as source, target.open('wb') as output:
                        shutil.copyfileobj(source, output)
                    target.chmod(member.mode & 0o777)
                else:
                    raise ValueError('SDK archive must contain only regular files and directories')
        subprocess.run(['git', 'init', '-q', str(root)], check=True, stdout=subprocess.DEVNULL)
        subprocess.run(['git', '-C', str(root), '-c', 'core.autocrlf=false', '-c', 'core.filemode=true',
                        'add', '--force', '--all'], check=True, stdout=subprocess.DEVNULL)
        actual_tree = subprocess.check_output(['git', '-C', str(root), 'write-tree'], text=True).strip()
        if actual_tree != tree.group(1).decode():
            raise ValueError('SDK source tree does not match the pinned commit')
        shutil.copytree(root, destination, dirs_exist_ok=True, ignore=shutil.ignore_patterns('.git'))
    print('Private SDK restored and commit/tree verified: ' + expected)


def main():
    if len(sys.argv) != 2 or sys.argv[1] not in ('ref', 'restore'):
        raise ValueError('Usage: audit_sdk.py ref|restore')
    expected = expected_commit()
    if sys.argv[1] == 'ref':
        with open(os.environ['GITHUB_OUTPUT'], 'a', encoding='utf-8') as output:
            output.write('commit=' + expected + '\n')
    else:
        restore(''.join(os.environ.get(name, '') for name in
                        ('AUDIT_SDK_BUNDLE', 'AUDIT_SDK_BUNDLE_PART2', 'AUDIT_SDK_BUNDLE_PART3', 'AUDIT_SDK_BUNDLE_PART4')), expected, SDK_PATH)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        # Never print payloads, raw source, or subprocess input.
        print('SDK setup failed: ' + str(error), file=sys.stderr)
        sys.exit(1)

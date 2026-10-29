"""Run the audit gRPC client against local Scala and Go checker servers."""
import json
import os
import pathlib
import socket
import subprocess
import time

root = pathlib.Path(__file__).resolve().parents[3]
build = root / 'pkg/build'
executables = {
    'scala': os.environ.get('APALACHE_SCALA', str(root / 'target/universal/stage/bin/apalache-mc')),
    'go': os.environ.get('APALACHE_GO', str(build / 'apalache')),
}
records = {}
for language, executable in executables.items():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        port = sock.getsockname()[1]
    with open(build / ('audit_shai_' + language + '.log'), 'w') as log:
        process = subprocess.Popen(
            ['taskset', '-c', '1-8', executable, 'server', '--server-type=checker', f'--port={port}'],
            cwd=build, stdout=log, stderr=log)
        try:
            for _ in range(200):
                try:
                    with socket.create_connection(('127.0.0.1', port), timeout=.2):
                        break
                except OSError:
                    if process.poll() is not None:
                        raise RuntimeError(f'{language} server exited')
                    time.sleep(.1)
            client = subprocess.run(
                ['taskset', '-c', '1-8', str(build / 'audit_shai'), f'127.0.0.1:{port}'],
                cwd=build, capture_output=True, text=True, timeout=100, check=True)
            records[language] = json.loads(client.stdout)
        finally:
            process.terminate()
            try:
                process.wait(timeout=8)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
print(json.dumps(records, indent=2))

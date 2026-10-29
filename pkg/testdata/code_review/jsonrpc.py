import base64, gzip, json, os, pathlib, socket, subprocess, sys, time, urllib.request

root = pathlib.Path(__file__).resolve().parents[3]
executables = {
    'scala': pathlib.Path(os.environ.get('APALACHE_SCALA', root/'target/universal/stage/bin/apalache-mc')),
    'go': pathlib.Path(os.environ.get('APALACHE_GO', root/'pkg/build/apalache')),
}
source = r'''---- MODULE AuditRPC ----
EXTENDS Integers
VARIABLE
\* @type: Int;
x
Init == x = 0
Next == x' = x + 1
Inv == x >= 0
View == x
====
'''
records = {}
for language, executable in executables.items():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        port = sock.getsockname()[1]
    log = open(root / ('pkg/build/audit_rpc_' + language + '.log'), 'w')
    process = subprocess.Popen(['taskset','-c','1-8',str(executable),'server','--server-type=explorer',f'--port={port}'], cwd=root/'pkg/build', stdout=log, stderr=log)
    def call(method, _compressed=False, **params):
        data = json.dumps(dict(jsonrpc='2.0', id=1, method=method, params=params)).encode()
        headers = {'Content-Type':'application/json', 'Accept-Encoding':'gzip'}
        if _compressed:
            data = gzip.compress(data)
            headers['Content-Encoding'] = 'gzip'
        request = urllib.request.Request(f'http://127.0.0.1:{port}/rpc', data, headers)
        with urllib.request.urlopen(request, timeout=30) as response:
            body = response.read()
            if response.headers.get('Content-Encoding') == 'gzip': body = gzip.decompress(body)
            return json.loads(body)
    results = {}
    try:
        for i in range(100):
            try:
                call('health'); break
            except Exception:
                if process.poll() is not None: raise RuntimeError('server exited')
                time.sleep(.1)
        results['gzipHealth'] = call('health', _compressed=True)
        compression = subprocess.run(
            ['taskset', '-c', '1-8', str(root/'pkg/build/audit_compression'), f'http://127.0.0.1:{port}/rpc'],
            capture_output=True, text=True, timeout=65, check=True)
        results['compression'] = json.loads(compression.stdout)
        loaded = call('loadSpec', sources=[base64.b64encode(source.encode()).decode()], exports=['View'], invariants=['Inv'])
        results['load'] = loaded
        print(language, loaded, file=sys.stderr)
        session = loaded['result']['sessionId']
        def op(method, **params): return call(method, sessionId=session, **params)
        results['assumeInit'] = op('assumeTransition', transitionId=0, checkEnabled=True)
        results['pendingInit'] = op('query', kinds=['STATE','TRACE'])
        results['advanceInit'] = op('nextStep')
        results['assumeNext'] = op('assumeTransition', transitionId=0, checkEnabled=True)
        results['pendingNext'] = op('query', kinds=['STATE','TRACE'])
        results['constrainCurrent'] = op('assumeState', equalities={'x':0}, checkEnabled=True)
        results['pendingAfterAssertion'] = op('query', kinds=['STATE','TRACE'])
        results['compactPending'] = op('compact', snapshotId=loaded['result']['snapshotId'])
        results['afterCompact'] = op('query', kinds=['STATE','TRACE'])
        results['assumeNextAgain'] = op('assumeTransition', transitionId=0, checkEnabled=True)
        results['advanceNext'] = op('nextStep')
        results['afterAdvance'] = op('query', kinds=['STATE','TRACE'])
        results['excludeOne'] = op('nextModel', operator='View')
        results['queryUnsatOperator'] = op('query', kinds=['OPERATOR'], operator='View')
        results['unknownOperator'] = op('query', kinds=['OPERATOR'], operator='Missing')
    finally:
        process.terminate()
        try: process.wait(timeout=8)
        except subprocess.TimeoutExpired: process.kill(); process.wait()
        log.close()
    records[language] = results
print(json.dumps(records, indent=2))

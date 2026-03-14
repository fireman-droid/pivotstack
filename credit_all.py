import json, struct, uuid, urllib.request, urllib.error, time, sys, threading
from concurrent.futures import ThreadPoolExecutor, as_completed

with open('test_creds.json') as f:
    CREDS = json.load(f)

VER = '1.26.2'
EP = [
    ('https://codewhisperer.us-east-1.amazonaws.com', 'AmazonCodeWhispererStreamingService.GenerateAssistantResponse', 'AI_EDITOR'),
    ('https://q.us-east-1.amazonaws.com', 'AmazonQDeveloperStreamingService.SendMessage', 'CLI'),
]

def refresh():
    url = 'https://oidc.%s.amazonaws.com/token' % CREDS['region']
    d = json.dumps({'clientId':CREDS['clientId'],'clientSecret':CREDS['clientSecret'],'refreshToken':CREDS['refreshToken'],'grantType':'refresh_token'}).encode()
    r = urllib.request.Request(url, data=d, method='POST')
    r.add_header('Content-Type','application/json')
    with urllib.request.urlopen(r, timeout=30) as resp:
        res = json.loads(resp.read())
        CREDS['refreshToken'] = res.get('refreshToken', CREDS['refreshToken'])
        return res['accessToken']

def sethdr(req, tok, target):
    mid = CREDS.get('machineId','')
    req.add_header('Authorization', 'Bearer ' + tok)
    req.add_header('Content-Type','application/json')
    req.add_header('Accept','*/*')
    req.add_header('X-Amz-Target', target)
    req.add_header('User-Agent', 'Kiro-Cli/%s ua/2.1 os/linux lang/rust api/codewhispererstreaming cfg/retry-mode/standard m/E %s' % (VER, mid))
    req.add_header('X-Amz-User-Agent', 'Kiro-Cli/%s os/linux lang/rust %s' % (VER, mid))
    req.add_header('x-amzn-kiro-agent-mode','vibe')
    req.add_header('x-amzn-codewhisperer-optout','true')
    req.add_header('Amz-Sdk-Request','attempt=1; max=3')
    req.add_header('Amz-Sdk-Invocation-Id', str(uuid.uuid4()))

def getusage(tok):
    url = EP[0][0] + '/getUsageLimits?origin=AI_EDITOR&resourceType=AGENTIC_REQUEST&isEmailRequired=true'
    r = urllib.request.Request(url)
    sethdr(r, tok, 'x')
    r.remove_header('X-amz-target')
    r.add_header('Accept','application/json')
    try:
        with urllib.request.urlopen(r, timeout=30) as resp:
            d = json.loads(resp.read())
            bd = (d.get('usageBreakdownList') or [{}])[0]
            ti = bd.get('freeTrialInfo') or {}
            return bd.get('currentUsage',0) + ti.get('currentUsage',0)
    except:
        return None

def evtype(hdr):
    off = 0
    while off < len(hdr):
        nl = hdr[off]; off += 1
        if off+nl > len(hdr): break
        nm = hdr[off:off+nl].decode('utf-8', errors='replace'); off += nl
        if off >= len(hdr): break
        vt = hdr[off]; off += 1
        if vt == 7:
            if off+2 > len(hdr): break
            vl = (hdr[off]<<8)|hdr[off+1]; off += 2
            if off+vl > len(hdr): break
            val = hdr[off:off+vl].decode('utf-8', errors='replace'); off += vl
            if nm == ':event-type': return val
            continue
        sk = {0:0,1:0,2:1,3:2,4:4,5:8,8:8,9:16}
        if vt == 6:
            if off+2 > len(hdr): break
            l = (hdr[off]<<8)|hdr[off+1]; off += 2+l
        elif vt in sk: off += sk[vt]
        else: break
    return ''

def parse(raw):
    off = 0; text = ''; cr = 0.0; last = ''
    while off < len(raw):
        if off+12 > len(raw): break
        tl = struct.unpack('>I', raw[off:off+4])[0]; hl = struct.unpack('>I', raw[off+4:off+8])[0]; off += 12
        if tl < 16: continue
        rem = tl - 12
        if off+rem > len(raw): break
        msg = raw[off:off+rem]; off += rem
        if hl > len(msg)-4: continue
        et = evtype(msg[:hl]); pb = msg[hl:len(msg)-4]
        if not pb: continue
        try: ev = json.loads(pb)
        except: continue
        if et == 'assistantResponseEvent':
            c = ev.get('content','')
            if c and c != last:
                text += c[len(last):] if c.startswith(last) else c
                last = c
        elif et == 'meteringEvent':
            u = ev.get('usage')
            if isinstance(u, (int, float)): cr += float(u)
    return text, cr

def callapi(tok, prompt, epi=0):
    base, target, origin = EP[epi]
    body = json.dumps({'conversationState':{'chatTriggerType':'MANUAL','conversationId':str(uuid.uuid4()),
        'currentMessage':{'userInputMessage':{'content':prompt,'modelId':'claude-sonnet-4.5','origin':origin}}}}).encode()
    req = urllib.request.Request(base + '/generateAssistantResponse', data=body, method='POST')
    sethdr(req, tok, target)
    try:
        with urllib.request.urlopen(req, timeout=120) as resp:
            return parse(resp.read()) + (200, '')
    except urllib.error.HTTPError as e:
        return None, 0, e.code, e.read().decode()[:80]

def callany(tok, prompt):
    for i in range(2):
        r = callapi(tok, prompt, epi=i)
        if r[0] is not None: return r[0], r[1]
        if r[2] == 429: continue
        return None, 0
    return None, 0

def filler(n):
    base = 'The quick brown fox jumps over the lazy dog. A wonderful serenity has taken possession of my entire soul. '
    return (base * (n // len(base) + 1))[:n]

def main():
    print('='*60); print('  KIRO FULL TEST'); print('='*60); sys.stdout.flush()
    tok = refresh(); print('Token OK'); sys.stdout.flush()
    u0 = getusage(tok); print('Initial usage: %s' % u0); sys.stdout.flush()
    print('Waiting 25s...'); sys.stdout.flush(); time.sleep(25); print('Go!'); sys.stdout.flush()

    # PART 1: Credit vs size
    print('\n--- PART 1: CREDIT vs SIZE ---'); sys.stdout.flush()
    tests = [
        ('tiny','Say OK.'),('500c',filler(500)+'\nCount sentences. Number only.'),
        ('2000c',filler(2000)+'\nCount sentences. Number only.'),('8000c',filler(8000)+'\nCount sentences. Number only.'),
        ('20000c',filler(20000)+'\nCount sentences. Number only.'),('40000c',filler(40000)+'\nCount sentences. Number only.'),
        ('med-out','Write exactly 100 words about the ocean.'),('big-out','Write exactly 500 words about space exploration.'),
    ]
    cr_results = []
    for i, (name, prompt) in enumerate(tests):
        in_tok = len(prompt) // 4
        print('\n  [%d/%d] %s: %d chars ~%d tok' % (i+1, len(tests), name, len(prompt), in_tok)); sys.stdout.flush()
        before = getusage(tok)
        t0 = time.time(); text, cr = callany(tok, prompt); elapsed = time.time() - t0
        if text is None:
            print('    FAIL, wait 20s'); sys.stdout.flush(); time.sleep(20); continue
        out_tok = len(text) // 4 if text else 0
        time.sleep(3); after = getusage(tok)
        cr_api = (after - before) if after is not None and before is not None else None
        cr_results.append({'name':name,'in_tok':in_tok,'out_tok':out_tok,'cr_sse':cr,'cr_api':cr_api,'time':elapsed})
        print('    OK %.1fs in~%d out~%d sse=%.8f api=%s' % (elapsed, in_tok, out_tok, cr, cr_api)); sys.stdout.flush()
        if i < len(tests)-1: print('    wait 10s...'); sys.stdout.flush(); time.sleep(10)

    # PART 2: Concurrency
    print('\n\n--- PART 2: CONCURRENCY ---'); print('  wait 20s...'); sys.stdout.flush(); time.sleep(20)
    con_results = []
    def fire(n, ep, label):
        print('\n  %s x%d:' % (label, n)); sys.stdout.flush()
        res = []
        with ThreadPoolExecutor(max_workers=n) as pool:
            def w(idx):
                t0=time.time(); r=callapi(tok,'Say OK.',epi=ep); el=time.time()-t0
                return (idx, r[0] is not None, r[2] if r[0] is None else 200, el, r[1] if r[0] is not None else 0)
            for f in as_completed({pool.submit(w,i):i for i in range(n)}): res.append(f.result())
        res.sort(key=lambda x:x[0]); ok=sum(1 for r in res if r[1])
        for r in res: print('    [%d] %s %.1fs cr=%.6f' % (r[0], 'OK' if r[1] else 'FAIL-%d'%r[2], r[3], r[4]))
        print('    => %d/%d' % (ok, n)); sys.stdout.flush()
        con_results.append({'n':n,'ep':label,'ok':ok}); return ok

    for n in [1,2,3,5]: fire(n,0,'CW'); time.sleep(12)
    for n in [1,2,3,5]: fire(n,1,'Q'); time.sleep(12)

    # ANALYSIS
    print('\n\n--- ANALYSIS ---'); sys.stdout.flush()
    valid = [r for r in cr_results if r['cr_sse'] > 0]
    if len(valid) >= 3:
        for r in valid: print('  %-10s in=%5d out=%5d cr=%.8f' % (r['name'],r['in_tok'],r['out_tok'],r['cr_sse']))
        n2=len(valid); X=[(r['in_tok'],r['out_tok'],1) for r in valid]; Y=[r['cr_sse'] for r in valid]
        xtx=[[sum(X[k][i]*X[k][j] for k in range(n2)) for j in range(3)] for i in range(3)]
        xty=[sum(X[k][i]*Y[k] for k in range(n2)) for i in range(3)]
        def d3(m): return m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1])-m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0])+m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0])
        D=d3(xtx)
        if abs(D)>1e-20:
            def rc(m,c,v):
                r2=[row[:] for row in m]
                for i2 in range(3): r2[i2][c]=v[i2]
                return r2
            a=d3(rc(xtx,0,xty))/D; b=d3(rc(xtx,1,xty))/D; c=d3(rc(xtx,2,xty))/D
            print('\n  FORMULA: credit = %.10f*input + %.10f*output + %.8f' % (a,b,c))
            if a>0: print('    1K INPUT  = %.6f cr = %.6f yuan' % (a*1000,a*1000*0.04))
            else: print('    INPUT ~ free (coeff=%.10f)' % a)
            if b>0: print('    1K OUTPUT = %.6f cr = %.6f yuan' % (b*1000,b*1000*0.04))
            print('    Overhead  = %.6f cr/req' % c)
            print('\n  Real-world:')
            for nm,inp,out in [('Quick chat',500,100),('Code+tools',15000,500),('Agent turn',30000,1000),('Big ctx',50000,2000),('Max',100000,4000)]:
                cr2=max(0,a*inp+b*out+c); print('    %-15s %din %dout => %.4f cr %.4f yuan (200/day=%.1fcr)' % (nm,inp,out,cr2,cr2*0.04,cr2*200))
    uf=getusage(tok)
    if uf is not None and u0 is not None: print('\n  Total delta: %.6f credits' % (uf-u0))
    print('\n  Concurrency:')
    for r in con_results: print('    %s n=%d => %d/%d' % (r['ep'],r['n'],r['ok'],r['n']))
    with open('full_test_results.json','w') as f: json.dump({'credit':cr_results,'concurrency':con_results},f,indent=2,default=str)
    print('\n  Saved to full_test_results.json\nDONE!')

if __name__ == '__main__':
    main()

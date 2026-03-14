#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Credit v3: Large input tests to measure input token cost"""
import json, struct, uuid, urllib.request, urllib.error, time, sys

with open("test_creds.json") as f:
    CREDS = json.load(f)

VER = "1.26.2"
ENDPOINTS = [
    ("https://codewhisperer.us-east-1.amazonaws.com", "AmazonCodeWhispererStreamingService.GenerateAssistantResponse", "AI_EDITOR"),
    ("https://q.us-east-1.amazonaws.com", "AmazonQDeveloperStreamingService.SendMessage", "CLI"),
]

def refresh():
    url = f"https://oidc.{CREDS['region']}.amazonaws.com/token"
    d = json.dumps({"clientId":CREDS["clientId"],"clientSecret":CREDS["clientSecret"],
                    "refreshToken":CREDS["refreshToken"],"grantType":"refresh_token"}).encode()
    r = urllib.request.Request(url, data=d, method="POST")
    r.add_header("Content-Type","application/json")
    with urllib.request.urlopen(r, timeout=30) as resp:
        res = json.loads(resp.read())
        CREDS["refreshToken"] = res.get("refreshToken", CREDS["refreshToken"])
        return res["accessToken"]

def set_hdrs(req, tok, target):
    mid = CREDS.get("machineId","")
    req.add_header("Authorization", f"Bearer {tok}")
    req.add_header("Content-Type","application/json")
    req.add_header("Accept","*/*")
    req.add_header("X-Amz-Target", target)
    req.add_header("User-Agent", f"Kiro-Cli/{VER} ua/2.1 os/linux lang/rust api/codewhispererstreaming cfg/retry-mode/standard m/E {mid}")
    req.add_header("X-Amz-User-Agent", f"Kiro-Cli/{VER} os/linux lang/rust {mid}")
    req.add_header("x-amzn-kiro-agent-mode","vibe")
    req.add_header("x-amzn-codewhisperer-optout","true")
    req.add_header("Amz-Sdk-Request","attempt=1; max=3")
    req.add_header("Amz-Sdk-Invocation-Id", str(uuid.uuid4()))

def get_usage(tok):
    url = f"{ENDPOINTS[0][0]}/getUsageLimits?origin=AI_EDITOR&resourceType=AGENTIC_REQUEST&isEmailRequired=true"
    r = urllib.request.Request(url)
    set_hdrs(r, tok, "x")
    r.remove_header("X-amz-target")
    r.add_header("Accept","application/json")
    try:
        with urllib.request.urlopen(r, timeout=30) as resp:
            d=json.loads(resp.read())
            bd=(d.get("usageBreakdownList") or [{}])[0]
            ti=bd.get("freeTrialInfo") or {}
            return bd.get("currentUsage",0)+ti.get("currentUsage",0)
    except Exception as e:
        print(f"    [usage err] {e}")
        return None

def evt_type(hdr):
    off=0
    while off<len(hdr):
        nl=hdr[off]; off+=1
        if off+nl>len(hdr): break
        nm=hdr[off:off+nl].decode("utf-8",errors="replace"); off+=nl
        if off>=len(hdr): break
        vt=hdr[off]; off+=1
        if vt==7:
            if off+2>len(hdr): break
            vl=(hdr[off]<<8)|hdr[off+1]; off+=2
            if off+vl>len(hdr): break
            val=hdr[off:off+vl].decode("utf-8",errors="replace"); off+=vl
            if nm==":event-type": return val
            continue
        sk={0:0,1:0,2:1,3:2,4:4,5:8,8:8,9:16}
        if vt==6:
            if off+2>len(hdr): break
            l=(hdr[off]<<8)|hdr[off+1]; off+=2+l
        elif vt in sk: off+=sk[vt]
        else: break
    return ""

def parse(raw):
    off=0; text=""; cr=0.0; last=""; all_ev=[]
    while off<len(raw):
        if off+12>len(raw): break
        tl=struct.unpack(">I",raw[off:off+4])[0]
        hl=struct.unpack(">I",raw[off+4:off+8])[0]
        off+=12
        if tl<16: continue
        rem=tl-12
        if off+rem>len(raw): break
        msg=raw[off:off+rem]; off+=rem
        if hl>len(msg)-4: continue
        et=evt_type(msg[:hl]); pb=msg[hl:len(msg)-4]
        if not pb: continue
        try: ev=json.loads(pb)
        except: continue
        all_ev.append((et,ev))
        if et=="assistantResponseEvent":
            c=ev.get("content","")
            if c and c!=last:
                text+=c[len(last):] if c.startswith(last) else c
                last=c
        elif et=="meteringEvent":
            u=ev.get("usage")
            if isinstance(u,(int,float)): cr+=float(u)
    return text, cr, all_ev

def call(tok, prompt, model="claude-sonnet-4.5"):
    body=json.dumps({"conversationState":{"chatTriggerType":"MANUAL","conversationId":str(uuid.uuid4()),
        "currentMessage":{"userInputMessage":{"content":prompt,"modelId":model,"origin":"AI_EDITOR"}}}}).encode()
    for base,target,origin in ENDPOINTS:
        p=json.loads(body); p["conversationState"]["currentMessage"]["userInputMessage"]["origin"]=origin
        b2=json.dumps(p).encode()
        req=urllib.request.Request(f"{base}/generateAssistantResponse",data=b2,method="POST")
        set_hdrs(req,tok,target)
        try:
            with urllib.request.urlopen(req,timeout=120) as resp:
                return parse(resp.read())
        except urllib.error.HTTPError as e:
            msg=e.read().decode()[:100]
            print(f"    [{base.split('//')[1][:15]}] HTTP {e.code}: {msg}")
            if e.code==429: continue
            return None,0,[]
    return None,0,[]

# Generate filler text of specific char count
def filler(chars):
    # Use random-looking but deterministic text, ~4 chars/token
    base = "The quick brown fox jumps over the lazy dog. A wonderful serenity has taken possession of my entire soul. "
    return (base * (chars // len(base) + 1))[:chars]

def main():
    print("="*60)
    print("  CREDIT V3: Large Input Test")
    print("  Goal: Measure input token cost accurately")
    print("="*60)

    tok = refresh()
    print(f"[OK] Token refreshed")

    u0 = get_usage(tok)
    print(f"[OK] Initial usage: {u0}")

    # Wait for rate limits
    print("[..] Waiting 25s for rate limit...")
    time.sleep(25)
    print("[OK] Ready\n")

    # Tests designed to isolate input vs output cost
    # Key: vary input size dramatically while keeping output TINY
    tests = [
        # (name, prompt, est_input_chars, est_output_description)
        ("TINY-in",    "Say OK.", "~2 tok in, ~1 tok out"),
        ("500c-in",    filler(500) + "\n\nCount: how many sentences above? Reply with just the number.", "~130 tok in, ~3 tok out"),
        ("2000c-in",   filler(2000) + "\n\nCount: how many sentences above? Reply with just the number.", "~500 tok in, ~3 tok out"),
        ("8000c-in",   filler(8000) + "\n\nCount: how many sentences above? Reply with just the number.", "~2000 tok in, ~3 tok out"),
        ("20000c-in",  filler(20000) + "\n\nCount: how many sentences above? Reply with just the number.", "~5000 tok in, ~3 tok out"),
        ("40000c-in",  filler(40000) + "\n\nCount: how many sentences above? Reply with just the number.", "~10000 tok in, ~3 tok out"),
        # Now vary output while keeping input small
        ("TINY-out",   "Say OK.", "~2 tok in, ~1 tok out"),
        ("MED-out",    "Write exactly 100 words about the ocean. Nothing else.", "~15 tok in, ~130 tok out"),
        ("BIG-out",    "Write exactly 500 words about space exploration history.", "~12 tok in, ~650 tok out"),
    ]

    results = []
    for i, (name, prompt, desc) in enumerate(tests):
        print(f"  [{i+1}/{len(tests)}] {name}: {desc}")
        print(f"       Prompt size: {len(prompt)} chars (~{len(prompt)//4} tokens)")

        before = get_usage(tok)
        if before is None:
            print("       SKIP (usage query failed)")
            time.sleep(10)
            continue

        t0 = time.time()
        text, cr_sse, evts = call(tok, prompt)
        elapsed = time.time() - t0

        if text is None:
            print(f"       FAIL, waiting 20s...")
            time.sleep(20)
            continue

        time.sleep(3)
        after = get_usage(tok)
        cr_api = (after - before) if after is not None and before is not None else None

        in_chars = len(prompt)
        out_chars = len(text) if text else 0
        # ~4 chars per token for English
        in_tok = in_chars // 4
        out_tok = out_chars // 4

        r = {"name":name, "in_chars":in_chars, "out_chars":out_chars,
             "in_tok":in_tok, "out_tok":out_tok,
             "cr_sse":cr_sse, "cr_api":cr_api, "time":elapsed}
        results.append(r)

        print(f"       {elapsed:.1f}s | in~{in_tok}tok out~{out_tok}tok | cr_sse={cr_sse:.8f} cr_api={cr_api}")
        if text:
            print(f"       Response: {text[:50].replace(chr(10),' ')}...")

        # Rate limit management
        wait = 12 if i < len(tests)-1 else 0
        if wait:
            print(f"       Waiting {wait}s...")
            time.sleep(wait)

    # Analysis
    print("\n" + "="*60)
    print("  RESULTS")
    print("="*60)
    print(f"\n  {'Name':<12} {'InTok':>6} {'OutTok':>6} {'Cr(SSE)':>12} {'Cr(API)':>12}")
    print(f"  {'-'*12} {'-'*6} {'-'*6} {'-'*12} {'-'*12}")

    for r in results:
        cs = f"{r['cr_sse']:.8f}" if r['cr_sse']>0 else "N/A"
        ca = f"{r['cr_api']:.8f}" if r['cr_api'] is not None and r['cr_api']>0 else str(r.get('cr_api','N/A'))
        print(f"  {r['name']:<12} {r['in_tok']:>6} {r['out_tok']:>6} {cs:>12} {ca:>12}")

    # Separate input-varying and output-varying tests
    in_tests = [r for r in results if r['name'].endswith('-in') and r['cr_sse']>0]
    out_tests = [r for r in results if r['name'].endswith('-out') and r['cr_sse']>0]

    if len(in_tests) >= 2:
        print("\n  --- INPUT COST ANALYSIS (output held ~constant) ---")
        for r in in_tests:
            cr_per_ktok = (r['cr_sse']/r['in_tok']*1000) if r['in_tok']>0 else 0
            print(f"    {r['name']:<12} {r['in_tok']:>6} in_tok => {r['cr_sse']:.8f} cr ({cr_per_ktok:.4f} cr/Kin)")

        # Linear regression on input tests: cr = a*in_tok + b
        if len(in_tests) >= 2:
            n = len(in_tests)
            sx = sum(r['in_tok'] for r in in_tests)
            sy = sum(r['cr_sse'] for r in in_tests)
            sxy = sum(r['in_tok']*r['cr_sse'] for r in in_tests)
            sxx = sum(r['in_tok']**2 for r in in_tests)
            denom = n*sxx - sx*sx
            if abs(denom) > 0:
                a_in = (n*sxy - sx*sy) / denom
                b_in = (sy - a_in*sx) / n
                print(f"\n    Input regression: cr = {a_in:.10f}*in_tok + {b_in:.8f}")
                print(f"    => 1K input tokens = {a_in*1000:.6f} credits")
                print(f"    => 10K input tokens = {a_in*10000:.5f} credits")
                if a_in > 0:
                    print(f"    => 1 credit = {1/a_in:.0f} input tokens")

    if len(out_tests) >= 2:
        print("\n  --- OUTPUT COST ANALYSIS (input held ~constant) ---")
        for r in out_tests:
            cr_per_ktok = (r['cr_sse']/r['out_tok']*1000) if r['out_tok']>0 else 0
            print(f"    {r['name']:<12} {r['out_tok']:>6} out_tok => {r['cr_sse']:.8f} cr ({cr_per_ktok:.4f} cr/Kout)")

    # Full regression with all data
    valid = [r for r in results if r['cr_sse'] > 0]
    if len(valid) >= 3:
        print("\n  --- FULL REGRESSION: cr = a*in + b*out + c ---")
        n = len(valid)
        # Normal equations for 3 variables
        X = [(r['in_tok'], r['out_tok'], 1) for r in valid]
        Y = [r['cr_sse'] for r in valid]
        # X^T X
        xtx = [[sum(X[k][i]*X[k][j] for k in range(n)) for j in range(3)] for i in range(3)]
        xty = [sum(X[k][i]*Y[k] for k in range(n)) for i in range(3)]
        # Cramer
        def d3(m):
            return(m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1])
                  -m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0])
                  +m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0]))
        D=d3(xtx)
        if abs(D)>1e-20:
            def rc(m,c,v):
                r=[row[:] for row in m]
                for i in range(3): r[i][c]=v[i]
                return r
            a=d3(rc(xtx,0,xty))/D
            b=d3(rc(xtx,1,xty))/D
            c=d3(rc(xtx,2,xty))/D
            print(f"    cr = {a:.10f}*input + {b:.10f}*output + {c:.8f}")
            if a>0: print(f"    1K input  = {a*1000:.6f} cr = {a*1000*0.04:.6f} yuan")
            if b>0: print(f"    1K output = {b*1000:.6f} cr = {b*1000*0.04:.6f} yuan")
            print(f"    overhead  = {c:.6f} cr/req")
            if a>0 and b>0:
                print(f"    output/input cost ratio = {b/a:.1f}x")
            # Real-world scenarios
            print(f"\n  --- REAL-WORLD COST (Claude Code style) ---")
            cases = [
                ("Simple question", 500, 200),
                ("Code with tools", 15000, 500),
                ("Agent loop (1 turn)", 30000, 1000),
                ("Big context", 50000, 2000),
                ("Max context", 100000, 4000),
            ]
            print(f"    {'Scenario':<25} {'In':>6} {'Out':>5} {'Credit':>8} {'Yuan':>6} {'#/1500':>7}")
            for nm,inp,out in cases:
                cr=max(0,a*inp+b*out+c)
                print(f"    {nm:<25} {inp:>6} {out:>5} {cr:>8.4f} {cr*0.04:>6.4f} {int(1500/cr) if cr>0 else 99999:>7}")
            # How many credits per day for heavy coding?
            print(f"\n    Heavy coding day (200 agent turns, 30K in + 1K out each):")
            cr_per_turn = max(0, a*30000 + b*1000 + c)
            print(f"      Per turn: {cr_per_turn:.4f} credits")
            print(f"      200 turns: {cr_per_turn*200:.1f} credits")
            print(f"      This {'MATCHES' if 150<cr_per_turn*200<300 else 'does NOT match'} your ~200 credits/day!")

    # Final usage
    uf = get_usage(tok)
    if uf is not None and u0 is not None:
        print(f"\n  Total API delta: {uf-u0:.6f} credits")

    # Save
    with open("credit_v3_results.json","w") as f:
        json.dump({"results":[{k:v for k,v in r.items()} for r in results]},f,indent=2)
    print(f"  Saved to credit_v3_results.json")

if __name__ == "__main__":
    main()

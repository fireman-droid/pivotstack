#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Test Kiro API concurrency limits per account"""
import json, struct, uuid, urllib.request, urllib.error, time, sys
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed

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
    off=0; text=""; cr=0.0; last=""
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
        if et=="assistantResponseEvent":
            c=ev.get("content","")
            if c and c!=last:
                text+=c[len(last):] if c.startswith(last) else c
                last=c
        elif et=="meteringEvent":
            u=ev.get("usage")
            if isinstance(u,(int,float)): cr+=float(u)
    return text, cr

lock = threading.Lock()
log_lines = []

def do_request(tok, idx, prompt, endpoint_idx=0):
    """Single request, returns (idx, success, http_code, elapsed, credit, text_len, error_msg)"""
    base, target, origin = ENDPOINTS[endpoint_idx]
    body = json.dumps({"conversationState":{"chatTriggerType":"MANUAL","conversationId":str(uuid.uuid4()),
        "currentMessage":{"userInputMessage":{"content":prompt,"modelId":"claude-sonnet-4.5","origin":origin}}}}).encode()
    req = urllib.request.Request(f"{base}/generateAssistantResponse", data=body, method="POST")
    set_hdrs(req, tok, target)

    t0 = time.time()
    try:
        with urllib.request.urlopen(req, timeout=120) as resp:
            raw = resp.read()
        elapsed = time.time() - t0
        text, cr = parse(raw)
        with lock:
            log_lines.append(f"    [{idx}] OK  {elapsed:.1f}s  credit={cr:.6f}  len={len(text)}")
        return (idx, True, 200, elapsed, cr, len(text), None)
    except urllib.error.HTTPError as e:
        elapsed = time.time() - t0
        code = e.code
        msg = e.read().decode()[:100]
        with lock:
            log_lines.append(f"    [{idx}] FAIL HTTP {code} {elapsed:.1f}s: {msg[:60]}")
        return (idx, False, code, elapsed, 0, 0, msg)

def test_concurrency(tok, n, prompt, label, ep=0):
    """Fire n requests simultaneously, return results"""
    global log_lines
    log_lines = []
    print(f"\n  === Concurrency={n} ({label}, endpoint={ep}) ===")

    results = []
    with ThreadPoolExecutor(max_workers=n) as pool:
        futures = {pool.submit(do_request, tok, i, prompt, ep): i for i in range(n)}
        for f in as_completed(futures):
            results.append(f.result())

    # Sort by index
    results.sort(key=lambda x: x[0])

    # Print log
    for line in sorted(log_lines):
        print(line)

    ok = sum(1 for r in results if r[1])
    fail = n - ok
    codes = {}
    for r in results:
        if not r[1]:
            codes[r[2]] = codes.get(r[2], 0) + 1
    avg_time = sum(r[3] for r in results if r[1]) / ok if ok > 0 else 0
    total_cr = sum(r[4] for r in results)

    print(f"    Result: {ok}/{n} OK, {fail} failed {codes}")
    print(f"    Avg time: {avg_time:.1f}s, Total credit: {total_cr:.6f}")

    return {"n": n, "ok": ok, "fail": fail, "codes": codes, "avg_time": avg_time, "total_cr": total_cr, "label": label}

def main():
    print("="*60)
    print("  KIRO CONCURRENCY TEST")
    print("  Model: claude-sonnet-4.5")
    print("="*60)

    tok = refresh()
    print(f"[OK] Token refreshed")

    print("[..] Waiting 30s for rate limits to clear...")
    time.sleep(30)
    print("[OK] Ready")

    prompt = "Say OK."  # tiny request to minimize cost

    results = []

    # Test increasing concurrency levels
    # CW endpoint
    for n in [1, 2, 3, 5]:
        r = test_concurrency(tok, n, prompt, "CW", ep=0)
        results.append(r)
        time.sleep(15)  # cool down between tests

    # Q endpoint
    for n in [1, 2, 3, 5]:
        r = test_concurrency(tok, n, prompt, "Q", ep=1)
        results.append(r)
        time.sleep(15)

    # Cross-endpoint: half on CW, half on Q
    print(f"\n  === Cross-endpoint: 3 CW + 3 Q simultaneously ===")
    log_lines = []
    cross_results = []
    with ThreadPoolExecutor(max_workers=6) as pool:
        futures = []
        for i in range(3):
            futures.append(pool.submit(do_request, tok, i, prompt, 0))    # CW
            futures.append(pool.submit(do_request, tok, i+3, prompt, 1))  # Q
        for f in as_completed(futures):
            cross_results.append(f.result())
    for line in sorted(log_lines):
        print(line)
    ok = sum(1 for r in cross_results if r[1])
    print(f"    Result: {ok}/6 OK")
    results.append({"n": 6, "ok": ok, "label": "CW+Q mixed"})

    # If 5 worked, try 10
    if any(r["ok"] == r["n"] for r in results if r["n"] == 5):
        time.sleep(20)
        r = test_concurrency(tok, 10, prompt, "CW", ep=0)
        results.append(r)

    # Summary
    print("\n" + "="*60)
    print("  CONCURRENCY SUMMARY")
    print("="*60)
    print(f"\n  {'Level':>5} {'Endpoint':<10} {'OK':>3}/{' N':>3} {'AvgTime':>8} {'Credit':>8}")
    print(f"  {'-'*5} {'-'*10} {'-'*3} {'-'*3} {'-'*8} {'-'*8}")
    for r in results:
        label = r.get('label','?')
        n = r.get('n',0)
        ok = r.get('ok',0)
        at = r.get('avg_time',0)
        cr = r.get('total_cr',0)
        print(f"  {n:>5} {label:<10} {ok:>3}/{n:>3} {at:>7.1f}s {cr:>8.4f}")

    # Save
    with open("concurrency_results.json","w") as f:
        json.dump(results, f, indent=2, default=str)
    print(f"\n  Saved to concurrency_results.json")

if __name__ == "__main__":
    main()

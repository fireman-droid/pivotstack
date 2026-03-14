#!/usr/bin/env python3
"""Kiro Credit-to-Token 换算测试"""
import json, struct, time, sys, os, urllib.request, urllib.error, uuid

with open("test_creds.json") as f:
    ACCOUNT = json.load(f)

KIRO_VERSION = "1.26.2"
API_BASE = "https://codewhisperer.us-east-1.amazonaws.com"
AMZ_TARGET = "AmazonCodeWhispererStreamingService.GenerateAssistantResponse"

def refresh_token():
    url = f"https://oidc.{ACCOUNT['region']}.amazonaws.com/token"
    payload = json.dumps({"clientId": ACCOUNT["clientId"], "clientSecret": ACCOUNT["clientSecret"],
                          "refreshToken": ACCOUNT["refreshToken"], "grantType": "refresh_token"}).encode()
    req = urllib.request.Request(url, data=payload, method="POST")
    req.add_header("Content-Type", "application/json")
    print("[1] 刷新 Token...")
    with urllib.request.urlopen(req, timeout=30) as resp:
        r = json.loads(resp.read())
        print(f"  OK (len={len(r['accessToken'])})")
        ACCOUNT["refreshToken"] = r.get("refreshToken", ACCOUNT["refreshToken"])
        return r["accessToken"]

def set_headers(req, token):
    mid = ACCOUNT.get("machineId", "")
    ua = f"Kiro-Cli/{KIRO_VERSION} ua/2.1 os/linux lang/rust api/codewhispererstreaming cfg/retry-mode/standard m/E {mid}"
    req.add_header("Authorization", f"Bearer {token}")
    req.add_header("Content-Type", "application/json")
    req.add_header("Accept", "*/*")
    req.add_header("X-Amz-Target", AMZ_TARGET)
    req.add_header("User-Agent", ua)
    req.add_header("X-Amz-User-Agent", f"Kiro-Cli/{KIRO_VERSION} os/linux lang/rust {mid}")
    req.add_header("x-amzn-kiro-agent-mode", "vibe")
    req.add_header("x-amzn-codewhisperer-optout", "true")
    req.add_header("Amz-Sdk-Request", "attempt=1; max=3")
    req.add_header("Amz-Sdk-Invocation-Id", str(uuid.uuid4()))

def get_usage(token):
    url = f"{API_BASE}/getUsageLimits?origin=AI_EDITOR&resourceType=AGENTIC_REQUEST&isEmailRequired=true"
    req = urllib.request.Request(url)
    set_headers(req, token)
    req.remove_header("X-amz-target")
    req.add_header("Accept", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return json.loads(resp.read())
    except Exception as e:
        print(f"  GetUsage failed: {e}")
        return None

def parse_usage(data):
    if not data: return None
    bd = data.get("usageBreakdownList", [{}])[0] if data.get("usageBreakdownList") else {}
    cur = bd.get("currentUsage", 0)
    lim = bd.get("usageLimit", 0)
    ti = bd.get("freeTrialInfo") or {}
    tc = ti.get("currentUsage", 0)
    tl = ti.get("usageLimit", 0)
    si = data.get("subscriptionInfo") or {}
    ui = data.get("userInfo") or {}
    return {"email": ui.get("email","?"), "sub": si.get("subscriptionTitle","?"),
            "cur": cur, "lim": lim, "tcur": tc, "tlim": tl, "total": cur+tc, "total_lim": lim+tl}

def extract_event_type(hdr):
    off = 0
    while off < len(hdr):
        if off >= len(hdr): break
        nl = hdr[off]; off += 1
        if off + nl > len(hdr): break
        name = hdr[off:off+nl].decode("utf-8", errors="replace"); off += nl
        if off >= len(hdr): break
        vt = hdr[off]; off += 1
        if vt == 7:
            if off + 2 > len(hdr): break
            vl = (hdr[off]<<8)|hdr[off+1]; off += 2
            if off + vl > len(hdr): break
            val = hdr[off:off+vl].decode("utf-8", errors="replace"); off += vl
            if name == ":event-type": return val
            continue
        skip = {0:0,1:0,2:1,3:2,4:4,5:8,8:8,9:16}
        if vt == 6:
            if off+2>len(hdr): break
            l=(hdr[off]<<8)|hdr[off+1]; off+=2+l
        elif vt in skip: off += skip[vt]
        else: break
    return ""

def parse_stream(raw):
    off = 0; text = ""; in_t = 0; out_t = 0; credits = 0.0; last = ""
    while off < len(raw):
        if off + 12 > len(raw): break
        tl = struct.unpack(">I", raw[off:off+4])[0]
        hl = struct.unpack(">I", raw[off+4:off+8])[0]
        off += 12
        if tl < 16: continue
        rem = tl - 12
        if off + rem > len(raw): break
        msg = raw[off:off+rem]; off += rem
        if hl > len(msg)-4: continue
        et = extract_event_type(msg[:hl])
        pb = msg[hl:len(msg)-4]
        if not pb: continue
        try: ev = json.loads(pb)
        except: continue
        # tokens
        for src in [ev] + [ev[k] for k in ev if isinstance(ev.get(k),dict) and k.lower() in ("usage","tokenusage","token_usage")]:
            if not isinstance(src,dict): continue
            for k in ["inputTokens","input_tokens","promptTokens","totalInputTokens"]:
                if k in src and isinstance(src[k],(int,float)): in_t = int(src[k])
            for k in ["outputTokens","output_tokens","completionTokens","totalOutputTokens"]:
                if k in src and isinstance(src[k],(int,float)): out_t = int(src[k])
        if et == "assistantResponseEvent":
            c = ev.get("content","")
            if c and c != last:
                text += c[len(last):] if c.startswith(last) else c
                last = c
        elif et == "meteringEvent":
            u = ev.get("usage")
            if isinstance(u,(int,float)): credits += float(u); print(f"    metering: {u}")
    return text, in_t, out_t, credits

def send_request(token, prompt, model="claude-sonnet-4.5"):
    payload = json.dumps({"conversationState":{"chatTriggerType":"MANUAL","conversationId":str(uuid.uuid4()),
        "currentMessage":{"userInputMessage":{"content":prompt,"modelId":model,"origin":"AI_EDITOR"}}}}).encode()
    req = urllib.request.Request(f"{API_BASE}/generateAssistantResponse", data=payload, method="POST")
    set_headers(req, token)
    try:
        with urllib.request.urlopen(req, timeout=120) as resp:
            return parse_stream(resp.read())
    except urllib.error.HTTPError as e:
        print(f"  FAIL: HTTP {e.code} - {e.read().decode()[:200]}")
        return None, 0, 0, 0

TESTS = [
    ("极短 ~50tok", "Say hello in one word."),
    ("短 ~100tok", "Write a haiku about programming. Just the three lines, nothing else."),
    ("中等 ~500tok", "Explain what a binary search algorithm is in exactly 3 sentences. Be concise."),
    ("长输出", "Write a 500-word essay about the history of artificial intelligence from 1950 to today."),
]

def main():
    print("="*60)
    print("  Kiro Credit-to-Token 换算测试 (sonnet-4.5)")
    print("="*60)
    token = refresh_token()

    print("\n[2] 查询初始用量...")
    u0 = parse_usage(get_usage(token))
    if u0:
        print(f"  Email: {u0['email']}, Sub: {u0['sub']}")
        print(f"  Credits: {u0['total']:.2f} / {u0['total_lim']:.2f}")

    print("\n[3] 开始测试...")
    results = []
    prev = u0["total"] if u0 else None

    for i, (name, prompt) in enumerate(TESTS):
        print(f"\n  Test {i+1}/{len(TESTS)}: {name}")
        t0 = time.time()
        text, it, ot, cr = send_request(token, prompt)
        dt = time.time() - t0
        if text is None:
            print("    SKIP"); continue
        time.sleep(2)
        u1 = parse_usage(get_usage(token))
        api_delta = (u1["total"] - prev) if u1 and prev is not None else None
        if u1: prev = u1["total"]
        results.append({"name":name, "in":it, "out":ot, "tot":it+ot, "cr_sse":cr, "cr_api":api_delta, "time":dt})
        print(f"    {dt:.1f}s | in={it} out={ot} tot={it+ot} | credit(sse)={cr:.6f} credit(api)={api_delta}")
        print(f"    Response: {(text or '')[:80]}...")
        if i < len(TESTS)-1: time.sleep(3)

    print("\n" + "="*60)
    print("  RESULTS")
    print("="*60)
    if not results: print("  No results!"); return
    print(f"  {'Name':<15} {'In':>6} {'Out':>6} {'Tot':>6} {'Cr(SSE)':>10} {'Cr(API)':>10}")
    ti=to=tt=tc=0
    for r in results:
        cs=f"{r['cr_sse']:.6f}" if r['cr_sse']>0 else "N/A"
        ca=f"{r['cr_api']:.6f}" if r['cr_api'] and r['cr_api']>0 else "N/A"
        print(f"  {r['name']:<15} {r['in']:>6} {r['out']:>6} {r['tot']:>6} {cs:>10} {ca:>10}")
        ti+=r['in']; to+=r['out']; tt+=r['tot']; tc+=r['cr_sse']
    print(f"  {'TOTAL':<15} {ti:>6} {to:>6} {tt:>6} {tc:>10.6f}")
    if tc > 0 and tt > 0:
        print(f"\n  1 Credit = {tt/tc:.0f} tokens")
        print(f"  1K tok = {tc/tt*1000:.4f} credits")
        print(f"  Cost: 1K tok = {tc/tt*1000*0.04:.4f} yuan (PRO@60/1500)")
        for r in results:
            if r['cr_sse']>0:
                print(f"    {r['name']:<15} 1cr={r['tot']/r['cr_sse']:.0f}tok (in={r['in']/r['cr_sse']:.0f} out={r['out']/r['cr_sse']:.0f})")
    uf = parse_usage(get_usage(token))
    if uf and u0:
        d = uf["total"] - u0["total"]
        print(f"\n  Total API delta: {d:.6f} credits for {tt} tokens")
        if d > 0: print(f"  Verify: 1 Credit = {tt/d:.0f} tokens")
    with open("credit_test_results.json","w") as f:
        json.dump({"results":results,"totals":{"in":ti,"out":to,"tot":tt,"credits":tc}},f,indent=2)
    print(f"\n  Saved to credit_test_results.json")

if __name__ == "__main__":
    main()

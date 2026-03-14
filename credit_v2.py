# -*- coding: utf-8 -*-
#!/usr/bin/env python3
"""Kiro Credit ???? v2 - ? tiktoken ?? token + API delta ????"""
import json, struct, uuid, urllib.request, urllib.error, time, sys, os

try:
    import tiktoken
    enc = tiktoken.encoding_for_model("claude-3-5-sonnet-20241022")  # closest to claude tokenizer
    count_tokens = lambda text: len(enc.encode(text))
    print("[OK] tiktoken loaded")
except:
    # fallback: ~4 chars per token
    count_tokens = lambda text: max(1, len(text) // 4)
    print("[WARN] tiktoken not available, using char estimate")

with open("test_creds.json") as f:
    CREDS = json.load(f)

VER = "1.26.2"
CW_URL = "https://codewhisperer.us-east-1.amazonaws.com"
Q_URL = "https://q.us-east-1.amazonaws.com"

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

def headers(req, tok, target):
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
    """?? (main_current, main_limit, trial_current, trial_limit) ? None"""
    url = f"{CW_URL}/getUsageLimits?origin=AI_EDITOR&resourceType=AGENTIC_REQUEST&isEmailRequired=true"
    r = urllib.request.Request(url)
    headers(r, tok, "AmazonCodeWhispererStreamingService.GetUsageLimits")
    # override target - GetUsageLimits ??? streaming target
    r.remove_header("X-amz-target")
    r.add_header("Accept", "application/json")
    try:
        with urllib.request.urlopen(r, timeout=30) as resp:
            d = json.loads(resp.read())
            bd = d.get("usageBreakdownList",[{}])
            if not bd: return None
            b = bd[0]
            ti = b.get("freeTrialInfo") or {}
            return {
                "main": b.get("currentUsage",0),
                "main_lim": b.get("usageLimit",0),
                "trial": ti.get("currentUsage",0),
                "trial_lim": ti.get("usageLimit",0),
                "total": b.get("currentUsage",0) + ti.get("currentUsage",0),
                "sub": (d.get("subscriptionInfo") or {}).get("subscriptionTitle","?"),
                "email": (d.get("userInfo") or {}).get("email","?"),
                "raw": d,
            }
    except Exception as e:
        print(f"  [GetUsage FAIL] {e}")
        return None

def parse_events(raw):
    """?? AWS Event Stream, ?? (text, credits, all_events, token_info)"""
    off = 0; text = ""; credits = 0.0; events = []; last = ""
    token_info = {}  # ??????? token ????
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
        et = _evt_type(msg[:hl])
        pb = msg[hl:len(msg)-4]
        if not pb: continue
        try: ev = json.loads(pb)
        except: continue
        events.append({"type": et, "data": ev})
        # ?? token ????(??????????)
        _collect_numbers(ev, et, token_info)
        if et == "assistantResponseEvent":
            c = ev.get("content","")
            if c and c != last:
                text += c[len(last):] if c.startswith(last) else c
                last = c
        elif et == "meteringEvent":
            u = ev.get("usage")
            if isinstance(u,(int,float)): credits += float(u)
    return text, credits, events, token_info

def _evt_type(hdr):
    off = 0
    while off < len(hdr):
        nl = hdr[off]; off += 1
        if off+nl > len(hdr): break
        nm = hdr[off:off+nl].decode("utf-8",errors="replace"); off += nl
        if off >= len(hdr): break
        vt = hdr[off]; off += 1
        if vt == 7:
            if off+2>len(hdr): break
            vl = (hdr[off]<<8)|hdr[off+1]; off += 2
            if off+vl>len(hdr): break
            val = hdr[off:off+vl].decode("utf-8",errors="replace"); off += vl
            if nm == ":event-type": return val
            continue
        sk = {0:0,1:0,2:1,3:2,4:4,5:8,8:8,9:16}
        if vt==6:
            if off+2>len(hdr): break
            l=(hdr[off]<<8)|hdr[off+1]; off+=2+l
        elif vt in sk: off+=sk[vt]
        else: break
    return ""

def _collect_numbers(obj, prefix, out):
    """????????????????? token ??"""
    if isinstance(obj, dict):
        for k, v in obj.items():
            key = f"{prefix}.{k}"
            if isinstance(v, (int,float)) and v != 0:
                out[key] = v
            elif isinstance(v, (dict, list)):
                _collect_numbers(v, key, out)
    elif isinstance(obj, list):
        for i, v in enumerate(obj):
            _collect_numbers(v, f"{prefix}[{i}]", out)

def call_api(tok, prompt, model="claude-sonnet-4.5"):
    """?? Kiro API, ?? fallback ????"""
    body = json.dumps({"conversationState":{"chatTriggerType":"MANUAL","conversationId":str(uuid.uuid4()),
        "currentMessage":{"userInputMessage":{"content":prompt,"modelId":model,"origin":"AI_EDITOR"}}}}).encode()

    endpoints = [
        (CW_URL, "AmazonCodeWhispererStreamingService.GenerateAssistantResponse", "AI_EDITOR"),
        (Q_URL, "AmazonQDeveloperStreamingService.SendMessage", "CLI"),
    ]
    for base, target, origin in endpoints:
        # ?? origin
        p = json.loads(body)
        p["conversationState"]["currentMessage"]["userInputMessage"]["origin"] = origin
        body2 = json.dumps(p).encode()

        req = urllib.request.Request(f"{base}/generateAssistantResponse", data=body2, method="POST")
        headers(req, tok, target)
        try:
            with urllib.request.urlopen(req, timeout=120) as resp:
                return parse_events(resp.read())
        except urllib.error.HTTPError as e:
            code = e.code
            msg = e.read().decode()[:150]
            print(f"    [{base.split('//')[1].split('.')[0]}] HTTP {code}: {msg}")
            if code == 429:
                continue  # try next endpoint
            return None, 0, [], {}
    return None, 0, [], {}

# ============ ???? ============
# ?????input ?????output ??
TESTS = [
    # (??, prompt, ?? output ????)
    ("T1: ??", "Reply with exactly one word: OK", "~1 output token"),
    ("T2: 10???", "List exactly 10 random English words, one per line. Nothing else.", "~15 output tokens"),
    ("T3: 50???", "Write exactly 50 words about cats. Count carefully.", "~60 output tokens"),
    ("T4: ?input?output",
     "A"*2000 + "\n\nHow many letter A are in the text above? Reply with just the number.",
     "~500 input tokens, ~5 output"),
    ("T5: ?input?output",
     "B"*4000 + "\n\nRewrite the above text replacing every B with the word 'hello'. Output the full result.",
     "~1000 input, ~1000+ output"),
    ("T6: 200???", "Write exactly 200 words explaining quantum computing to a child. Be precise about word count.", "~250 output tokens"),
    ("T7: ???", "What is 12345 * 67890? Show only the final number.", "~5 output tokens, tests fixed overhead"),
]

def main():
    print("="*70)
    print("  Kiro Credit ???? v2")
    print("  ??: claude-sonnet-4.5 (rateMultiplier=1.3)")
    print("="*70)

    # Token refresh
    print("\n[1] ?? Token...")
    tok = refresh()
    print(f"  ? OK")

    # ????
    print("\n[2] ??????...")
    u0 = get_usage(tok)
    if u0:
        print(f"  Email: {u0['email']}")
        print(f"  Sub:   {u0['sub']}")
        print(f"  Main:  {u0['main']:.6f} / {u0['main_lim']:.1f}")
        print(f"  Trial: {u0['trial']:.6f} / {u0['trial_lim']:.1f}")
        print(f"  Total: {u0['total']:.6f}")
    else:
        print("  ? ??????")

    # ??????
    print("\n[3] ?? 20 ??????...")
    for i in range(20, 0, -1):
        sys.stdout.write(f"\r  {i}s ")
        sys.stdout.flush()
        time.sleep(1)
    print("\r  ? ????                    ")

    # ????
    print("\n[4] ????...")
    print("-"*70)
    results = []
    all_number_fields = {}  # ????????????

    for i, (name, prompt, desc) in enumerate(TESTS):
        print(f"\n  ?? ?? {i+1}/{len(TESTS)}: {name}")
        print(f"     {desc}")

        # ?? input tokens
        input_tokens = count_tokens(prompt)
        print(f"     Input tokens (tiktoken): {input_tokens}")

        # ????????
        before = get_usage(tok)
        if not before:
            print("     ? ??????, ??")
            time.sleep(5)
            continue

        # ????
        t0 = time.time()
        text, cr_sse, events, numbers = call_api(tok, prompt)
        elapsed = time.time() - t0

        if text is None:
            print(f"     ? ????, ?? 15s...")
            time.sleep(15)
            continue

        # ?? output tokens
        output_tokens = count_tokens(text) if text else 0
        total_tokens = input_tokens + output_tokens

        # ??????
        for k, v in numbers.items():
            if k not in all_number_fields:
                all_number_fields[k] = []
            all_number_fields[k].append(v)

        # ???? API ??
        time.sleep(3)

        # ????????
        after = get_usage(tok)
        cr_api = None
        if before and after:
            cr_api = after["total"] - before["total"]

        result = {
            "name": name,
            "input_tokens": input_tokens,
            "output_tokens": output_tokens,
            "total_tokens": total_tokens,
            "credit_sse": cr_sse,
            "credit_api": cr_api,
            "time": elapsed,
            "response_len": len(text) if text else 0,
            "events_count": len(events),
            "number_fields": numbers,
        }
        results.append(result)

        print(f"     ? {elapsed:.1f}s | events={len(events)}")
        print(f"     ??? Tokens: in={input_tokens} out={output_tokens} total={total_tokens}")
        print(f"     ??? Credit (SSE):  {cr_sse:.8f}")
        print(f"     ??? Credit (API ?): {cr_api:.8f}" if cr_api is not None else "     ??? Credit (API ?): N/A")
        if text:
            print(f"     ?? Output ({len(text)} chars): {text[:60].replace(chr(10),' ')}...")

        # ??????????????
        interesting = {k:v for k,v in numbers.items() if "token" in k.lower() or "usage" in k.lower() or "credit" in k.lower()}
        if interesting:
            print(f"     ??? Token/usage fields: {interesting}")

        # ????
        if i < len(TESTS) - 1:
            wait = 8
            print(f"     ? ?? {wait}s...")
            time.sleep(wait)

    # ============ ?? ============
    print("\n\n" + "="*70)
    print("  ??? ??????")
    print("="*70)

    if not results:
        print("  ????????")
        return

    # ??
    print(f"\n  {'Name':<20} {'InTok':>6} {'OutTok':>6} {'Total':>6} {'Cr(SSE)':>12} {'Cr(API)':>12} {'Tok/Cr':>8}")
    print(f"  {'-'*20} {'-'*6} {'-'*6} {'-'*6} {'-'*12} {'-'*12} {'-'*8}")

    sum_in = sum_out = sum_tot = sum_cr_sse = sum_cr_api = 0
    valid_for_ratio = []

    for r in results:
        cs = f"{r['credit_sse']:.8f}" if r['credit_sse'] > 0 else "N/A"
        ca = f"{r['credit_api']:.8f}" if r['credit_api'] is not None and r['credit_api'] > 0 else "N/A"
        cr_best = r['credit_sse'] if r['credit_sse'] > 0 else (r['credit_api'] if r['credit_api'] and r['credit_api'] > 0 else 0)
        tpc = f"{r['total_tokens']/cr_best:.0f}" if cr_best > 0 else "N/A"
        print(f"  {r['name']:<20} {r['input_tokens']:>6} {r['output_tokens']:>6} {r['total_tokens']:>6} {cs:>12} {ca:>12} {tpc:>8}")

        sum_in += r['input_tokens']
        sum_out += r['output_tokens']
        sum_tot += r['total_tokens']
        sum_cr_sse += r['credit_sse']
        if r['credit_api'] and r['credit_api'] > 0:
            sum_cr_api += r['credit_api']
        if cr_best > 0 and r['total_tokens'] > 50:  # ?????????
            valid_for_ratio.append((r['input_tokens'], r['output_tokens'], r['total_tokens'], cr_best, r['name']))

    print(f"  {'TOTAL':<20} {sum_in:>6} {sum_out:>6} {sum_tot:>6} {sum_cr_sse:>12.8f} {sum_cr_api:>12.8f}")

    # ????
    print(f"\n  ?? ???? (??) ??")
    if sum_cr_sse > 0:
        print(f"  ?? SSE: 1 Credit = {sum_tot/sum_cr_sse:.0f} tokens")
        print(f"            1K tokens = {sum_cr_sse/sum_tot*1000:.6f} credits")
    if sum_cr_api > 0:
        print(f"  ?? API: 1 Credit = {sum_tot/sum_cr_api:.0f} tokens")
        print(f"            1K tokens = {sum_cr_api/sum_tot*1000:.6f} credits")

    # ?????????? input/output ??
    print(f"\n  ?? ????? ??")
    if len(valid_for_ratio) >= 2:
        # ??????: credit = a * input_tokens + b * output_tokens + c
        # ??????
        import numpy as np
        try:
            X = []
            Y = []
            for inp, out, tot, cr, nm in valid_for_ratio:
                X.append([inp, out, 1])  # input, output, constant(overhead)
                Y.append(cr)
            X = np.array(X)
            Y = np.array(Y)
            # Least squares: solve X @ [a,b,c] = Y
            result_lsq, residuals, rank, sv = np.linalg.lstsq(X, Y, rcond=None)
            a, b, c = result_lsq
            print(f"  ????: credit = {a:.8f}*input + {b:.8f}*output + {c:.8f}")
            print(f"    ? 1K input tokens  = {a*1000:.6f} credits")
            print(f"    ? 1K output tokens = {b*1000:.6f} credits")
            print(f"    ? ????/??    = {c:.6f} credits")
            if a > 0: print(f"    ? Input: 1 credit  = {1/a:.0f} input tokens")
            if b > 0: print(f"    ? Output: 1 credit = {1/b:.0f} output tokens")
            print(f"    ? Output/Input ??? = {b/a:.1f}x" if a > 0 else "")
        except Exception as e:
            print(f"  ??????: {e}")
            # fallback ????
            for inp, out, tot, cr, nm in valid_for_ratio:
                print(f"    {nm:<20} {tot} tok / {cr:.6f} cr = {tot/cr:.0f} tok/cr")

    # ????
    print(f"\n  ?? ???? ??")
    cr_per_credit = 0.04  # PRO ?: 60?/1500credits
    if sum_cr_sse > 0 and sum_tot > 0:
        cr_per_1k = sum_cr_sse / sum_tot * 1000
        print(f"  1K tokens (sonnet-4.5) = {cr_per_1k:.6f} credits = {cr_per_1k*cr_per_credit:.6f} ?")
        print(f"  100K tokens            = {cr_per_1k*100:.4f} credits = {cr_per_1k*100*cr_per_credit:.4f} ?")
        print(f"  1500 credits ?? tokens = {1500/cr_per_1k*1000:.0f} tokens")

    # ?????????
    if all_number_fields:
        print(f"\n  ?? ???????????? ??")
        for k, vals in sorted(all_number_fields.items()):
            print(f"    {k}: {vals}")

    # ????
    print(f"\n  ?? ???? ??")
    uf = get_usage(tok)
    if uf and u0:
        delta = uf["total"] - u0["total"]
        print(f"  ???: {u0['total']:.6f}")
        print(f"  ???: {uf['total']:.6f}")
        print(f"  ? ?Credit: {delta:.6f}")
        if delta > 0 and sum_tot > 0:
            print(f"  ??: 1 Credit = {sum_tot/delta:.0f} tokens (API delta)")
            print(f"         1K tokens = {delta/sum_tot*1000:.6f} credits")

    # ??????
    out = {
        "timestamp": time.strftime("%Y-%m-%dT%H:%M:%S"),
        "model": "claude-sonnet-4.5",
        "rateMultiplier": 1.3,
        "results": results,
        "totals": {"in":sum_in, "out":sum_out, "tot":sum_tot,
                   "cr_sse":sum_cr_sse, "cr_api":sum_cr_api},
        "number_fields_discovered": {k:v for k,v in all_number_fields.items()},
    }
    with open("credit_v2_results.json","w",encoding="utf-8") as f:
        json.dump(out, f, ensure_ascii=False, indent=2)
    print(f"\n  ??? ??? credit_v2_results.json")

if __name__ == "__main__":
    main()

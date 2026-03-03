#!/usr/bin/env python3
"""
Claude CLI 压力测试 - 单次对话、50轮、大量输入
使用 --continue 保持同一会话，每轮发送大段文本
"""
import subprocess
import time
import json
import urllib.request

API = "http://localhost:8088/v1/chat/completions"
API_KEY = "sk-NZbSTR8kgZdgXL91lYLi2OgsiwgxAdB3"
MODEL = "claude-sonnet-4.5"
TOTAL = 50

# 生成大段文本（约2000字/轮）
def make_big_prompt(i):
    filler = f"""
这是第 {i} 轮测试消息。我需要你仔细阅读以下内容并给出简短回复。

## 背景信息
Kiro Stack 是一个将 Kiro (Amazon Q Developer) 账号转换为 OpenAI/Anthropic 兼容 API 的项目。
它由两个核心组件组成：kiro-go（Go语言，负责Web管理面板、多账号池管理、Token刷新）和
kiro-gateway（Python/FastAPI，负责稳定代理层，包含重试逻辑和双端点回退）。

架构流程：Client → kiro-go:8088 → kiro-gateway:8000 → Kiro API

### 账号池选择机制
使用加权随机选择算法，每个账号有一个 Weight 字段（默认值100）。权重越高，被选中的概率越大。
只有启用状态的账号才会被纳入池中。当请求到来时，系统会根据权重随机选择一个账号来处理请求。

### Token 刷新机制
kiro-go 每30分钟在后台刷新一次所有账号的 Token。如果 Token 在5分钟内过期，会主动刷新。
kiro-gateway 则采用按需刷新策略，在 Token 过期或即将过期时刷新。

### 重试逻辑
kiro-go 在使用 gateway 时实现了最多4次重试的故障转移。如果一个账号失败，会尝试其他账号。
kiro-gateway 实现了双端点回退（CodeWhisperer vs Amazon Q 端点），并有可配置的重试逻辑。

### Thinking 模式
支持扩展思考模式，通过模型名后缀触发（默认 -thinking）。配置项包括 ThinkingSuffix、
OpenAIThinkingFormat（reasoning_content/thinking/think）、ClaudeThinkingFormat。

### FREE 账号降级
FREE 账号只支持 claude-sonnet-4.5、claude-sonnet-4、claude-haiku-4.5。
当 FREE 账号请求 opus-4.6、sonnet-4.6、opus-4.5 时，自动降级为 claude-sonnet-4.5。
非 FREE 账号（PRO/PRO_PLUS/POWER）不受影响。

## 测试数据 #{i}
""" + f"{'测试填充文本。' * 50}\n" * 3
    filler += f"\n请用一句话总结你收到的是第几轮测试（回复格式：收到第{i}轮）。"
    return filler

# 维护对话历史
messages = [{"role": "system", "content": "你是测试助手，每次只需简短回复确认收到第几轮即可。"}]
success = 0
fail = 0
errors = []
start = time.time()

for i in range(1, TOTAL + 1):
    t0 = time.time()
    prompt = make_big_prompt(i)
    messages.append({"role": "user", "content": prompt})

    payload = json.dumps({
        "model": MODEL,
        "messages": messages,
        "max_tokens": 200,
        "stream": False
    }).encode("utf-8")

    try:
        req = urllib.request.Request(API, data=payload, headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {API_KEY}"
        })
        with urllib.request.urlopen(req, timeout=120) as resp:
            data = json.loads(resp.read().decode("utf-8"))

        elapsed = time.time() - t0
        reply = data.get("choices", [{}])[0].get("message", {}).get("content", "")
        usage = data.get("usage", {})
        total_tokens = usage.get("total_tokens", 0)

        if reply:
            success += 1
            messages.append({"role": "assistant", "content": reply})
            short = reply.strip()[:80]
            print(f"[OK  ] #{i:02d} ({elapsed:.1f}s) tokens={total_tokens} | {short}")
        else:
            fail += 1
            errors.append((i, 0, "empty reply"))
            print(f"[FAIL] #{i:02d} ({elapsed:.1f}s) empty reply")
            # 还是加入历史避免打断
            messages.append({"role": "assistant", "content": "OK"})

    except urllib.error.HTTPError as e:
        elapsed = time.time() - t0
        body = e.read().decode("utf-8", errors="replace")[:300]
        fail += 1
        errors.append((i, e.code, body))
        print(f"[FAIL] #{i:02d} ({elapsed:.1f}s) HTTP {e.code}: {body}")
        # 加一个假回复保持对话
        messages.append({"role": "assistant", "content": "error"})
    except Exception as e:
        elapsed = time.time() - t0
        fail += 1
        errors.append((i, -1, str(e)[:200]))
        print(f"[ERR ] #{i:02d} ({elapsed:.1f}s) {e}")
        messages.append({"role": "assistant", "content": "error"})

    if i % 10 == 0:
        ctx_size = sum(len(m["content"]) for m in messages)
        print(f"--- Progress: {i}/{TOTAL} | OK: {success} | FAIL: {fail} | Context: {ctx_size:,} chars ---")

total_time = time.time() - start
ctx_size = sum(len(m["content"]) for m in messages)
print()
print("=" * 60)
print(f"RESULTS: {success}/{TOTAL} success, {fail} failed")
print(f"TIME:    {total_time:.1f}s total, {total_time/TOTAL:.1f}s avg/round")
print(f"CONTEXT: {ctx_size:,} chars, {len(messages)} messages")
print("=" * 60)

if errors:
    print()
    print("ERRORS:")
    for rnd, code, msg in errors:
        print(f"  #{rnd:02d} code={code}: {msg[:150]}")

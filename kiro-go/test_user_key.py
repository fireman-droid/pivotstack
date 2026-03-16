import json
import requests
import time

with open("data/config.json", encoding="utf-8") as f:
    data = json.load(f)

test_key = None
for k in data.get("apiKeys", []):
    if k.get("key", "").startswith("sk-f10f") and k.get("key", "").endswith("878d"):
        test_key = k
        break

if not test_key:
    print("Could not find the user's test key in config.json")
    exit(1)

print(f"Initial State -> Balance: {test_key.get('balance', 0):.4f}, GiftBalance: {test_key.get('giftBalance', 0):.4f}")

print("Testing with claude-3-5-sonnet-20241022...")
res = requests.post(
    "http://localhost:8080/v1/chat/completions",
    headers={"Authorization": f"Bearer {test_key['key']}"},
    json={
        "model": "claude-3-5-sonnet-20241022",
        "messages": [{"role": "user", "content": "Hi, just a quick test. Reply 'ok'."}],
        "max_tokens": 10
    }
)

if not res.ok:
    print("API Error:", res.text)
else:
    print("API Success:", res.json()["choices"][0]["message"]["content"])
    print(f"Usage:", res.json()["usage"])

time.sleep(6) # wait for flush

with open("data/config.json", encoding="utf-8") as f:
    data_after = json.load(f)

for k in data_after.get("apiKeys", []):
    if k["id"] == test_key["id"]:
        print(f"Final DB State -> Balance: {k.get('balance', 0):.4f}, GiftBalance: {k.get('giftBalance', 0):.4f}")
        break

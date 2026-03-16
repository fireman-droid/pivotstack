import json

with open('data/config.json', encoding='utf-8') as f:
    data = json.load(f)

for k in data['apiKeys']:
    if k['id'] == 'c499d197-f358-462a-a1db-bd3f33ecb6a5':
        k['giftBalance'] = 2.0
        print('Set giftBalance=2.0')
        print('balance=', k['balance'], 'giftBalance=', k['giftBalance'])
        break

with open('data/config.json', 'w', encoding='utf-8') as f:
    json.dump(data, f, indent=2, ensure_ascii=False)

print('Saved.')

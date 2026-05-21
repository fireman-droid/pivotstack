# -*- coding: utf-8 -*-
"""
Kiro Stack Pricing & Revenue Calculator
"""
import json, sys, io
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

# ==================== Core Constants ====================
CNY_TO_USD = 20.0           # Y1 = $20 face value
PRO_PRICE_USD = 2.00        # PRO pool: $2/credit charged to user
FREE_PRICE_USD = 0.40       # FREE pool: $0.40/credit charged to user
PRO_COST_CNY = 0.04         # Our cost: Y0.04/credit
FREE_COST_CNY = 0.001       # FREE pool near-zero cost

# Derived
PRO_PRICE_CNY = PRO_PRICE_USD / CNY_TO_USD    # $2/20 = Y0.10/credit
FREE_PRICE_CNY = FREE_PRICE_USD / CNY_TO_USD  # $0.40/20 = Y0.02/credit

def load_costs_from_api(host="localhost", port=8080, password="admin123"):
    """Pull real costs from API pricing endpoint, overriding defaults"""
    global PRO_COST_CNY, FREE_COST_CNY
    try:
        import urllib.request
        h = {"X-Admin-Password": password}
        pricing = json.loads(urllib.request.urlopen(
            urllib.request.Request(f"http://{host}:{port}/admin/api/pricing", headers=h), timeout=5).read())
        pro_entries = pricing.get("proCostEntries", [])
        if pro_entries:
            total_cost = sum(e["costCNY"] for e in pro_entries)
            total_credits = sum(e["count"] * e.get("credits", 0) for e in pro_entries)
            if total_credits > 0:
                PRO_COST_CNY = total_cost / total_credits
        free_entries = pricing.get("freeCostEntries", [])
        if free_entries:
            total_cost = sum(e["costCNY"] for e in free_entries)
            total_credits = sum(e["count"] * 550 for e in free_entries)
            if total_credits > 0:
                FREE_COST_CNY = total_cost / total_credits
        print(f"  [API] PRO cost=Y{PRO_COST_CNY:.4f}/cr  FREE cost=Y{FREE_COST_CNY:.6f}/cr")
    except Exception as e:
        print(f"  [API] Could not load costs, using defaults: {e}")

# ==================== Functions ====================
def cny_to_usd(cny):
    return cny * CNY_TO_USD

def credits_for_balance(usd, pool="pro"):
    return usd / (PRO_PRICE_USD if pool == "pro" else FREE_PRICE_USD)

def revenue_cny(cny_charged):
    """User charged Y_cny, that IS the revenue"""
    return cny_charged

def profit_pro(cny_charged):
    usd = cny_to_usd(cny_charged)
    credits = credits_for_balance(usd, "pro")
    cost = credits * PRO_COST_CNY
    return cny_charged - cost

def profit_free(cny_charged):
    usd = cny_to_usd(cny_charged)
    credits = credits_for_balance(usd, "free")
    cost = credits * FREE_COST_CNY
    return cny_charged - cost

def margin_pro(cny):
    return profit_pro(cny) / cny * 100 if cny > 0 else 0

def margin_free(cny):
    return profit_free(cny) / cny * 100 if cny > 0 else 0

# ==================== Display ====================
def show_rules():
    print("=" * 65)
    print("  Kiro Stack Billing Rules")
    print("=" * 65)
    print(f"  Exchange:  Y1 = ${CNY_TO_USD:.0f} face value")
    print(f"  PRO pool:  ${PRO_PRICE_USD}/credit = Y{PRO_PRICE_CNY:.2f}/credit")
    print(f"  FREE pool: ${FREE_PRICE_USD}/credit = Y{FREE_PRICE_CNY:.2f}/credit")
    print(f"  PRO cost:  Y{PRO_COST_CNY}/credit")
    print(f"  FREE cost: ~Y{FREE_COST_CNY}/credit")
    print(f"  PRO margin:  {(1 - PRO_COST_CNY/PRO_PRICE_CNY)*100:.0f}%")
    print(f"  FREE margin: ~{(1 - FREE_COST_CNY/FREE_PRICE_CNY)*100:.0f}%")

def show_charge_table():
    print("\n" + "=" * 65)
    print("  Charge & Profit Table")
    print("=" * 65)
    fmt = "  {:>6} | {:>8} | {:>8} | {:>15} | {:>15}"
    print(fmt.format("Charge", "Balance", "Credits", "PRO Profit", "FREE Profit"))
    print(fmt.format("-"*6, "-"*8, "-"*8, "-"*15, "-"*15))
    for cny in [1, 3.5, 5, 10, 30, 50, 100]:
        usd = cny_to_usd(cny)
        cr = credits_for_balance(usd, "pro")
        pp = profit_pro(cny)
        fp = profit_free(cny)
        mp = margin_pro(cny)
        mf = margin_free(cny)
        print(fmt.format(
            f"Y{cny:.1f}",
            f"${usd:.0f}",
            f"{cr:.0f}",
            f"Y{pp:.2f} ({mp:.0f}%)",
            f"Y{fp:.2f} ({mf:.0f}%)"
        ))

def show_per_credit():
    print("\n" + "=" * 65)
    print("  Per Credit Breakdown")
    print("=" * 65)
    fmt = "  {:>6} | {:>10} | {:>10} | {:>10} | {:>8}"
    print(fmt.format("Pool", "Price", "Cost", "Profit", "Margin"))
    print(fmt.format("-"*6, "-"*10, "-"*10, "-"*10, "-"*8))
    print(fmt.format("PRO", f"Y{PRO_PRICE_CNY:.2f}", f"Y{PRO_COST_CNY:.4f}", f"Y{PRO_PRICE_CNY-PRO_COST_CNY:.4f}", f"{(1-PRO_COST_CNY/PRO_PRICE_CNY)*100:.0f}%"))
    print(fmt.format("FREE", f"Y{FREE_PRICE_CNY:.2f}", f"Y{FREE_COST_CNY:.4f}", f"Y{FREE_PRICE_CNY-FREE_COST_CNY:.4f}", f"{(1-FREE_COST_CNY/FREE_PRICE_CNY)*100:.0f}%"))

def show_usage_ref():
    print("\n" + "=" * 65)
    print("  Usage Reference (200 credits/day)")
    print("=" * 65)
    for cny in [10, 50, 100]:
        usd = cny_to_usd(cny)
        cr = credits_for_balance(usd, "pro")
        days = cr / 200
        print(f"  Y{cny:.0f} = {cr:.0f} credits = ~{days:.1f} days of PRO usage")

def check_api(host="localhost", port=8080, password="admin123"):
    try:
        import urllib.request
        h = {"X-Admin-Password": password}

        profit = json.loads(urllib.request.urlopen(
            urllib.request.Request(f"http://{host}:{port}/admin/api/profit", headers=h), timeout=5).read())
        pricing = json.loads(urllib.request.urlopen(
            urllib.request.Request(f"http://{host}:{port}/admin/api/pricing", headers=h), timeout=5).read())
        logs_data = json.loads(urllib.request.urlopen(
            urllib.request.Request(f"http://{host}:{port}/admin/api/logs", headers=h), timeout=5).read())

        print("\n" + "=" * 65)
        print("  Live API Data")
        print("=" * 65)
        print(f"  Config: PRO=${pricing.get('proPoolPriceUSD')}/cr  FREE=${pricing.get('freePoolPriceUSD')}/cr  Cost=Y{pricing.get('purchasePriceCNY')}/cr")

        print(f"\n  Profit API:")
        for k in ['revenue_usd','revenue_cny','pro_cost_cny','free_cost_cny','total_cost_cny','profit_cny','margin_percent']:
            print(f"    {k:20s}: {profit.get(k, 0)}")

        logs = logs_data.get("logs", [])
        total = logs_data.get("total", 0)
        print(f"\n  Requests: {total}")

        if logs:
            pro_logs = [l for l in logs if l.get("subscription") == "PRO"]
            free_logs = [l for l in logs if l.get("subscription") != "PRO"]
            pro_cr = sum(l.get("credits", 0) for l in pro_logs)
            free_cr = sum(l.get("credits", 0) for l in free_logs)

            print(f"    PRO:  {len(pro_logs)} reqs, {pro_cr:.4f} credits")
            print(f"    FREE: {len(free_logs)} reqs, {free_cr:.4f} credits")

            # Manual calc - revenue = credits * price_per_credit in CNY
            exp_rev_cny = pro_cr * PRO_PRICE_CNY + free_cr * FREE_PRICE_CNY
            exp_cost_cny = pro_cr * PRO_COST_CNY + free_cr * FREE_COST_CNY
            exp_profit = exp_rev_cny - exp_cost_cny
            exp_margin = (exp_profit / exp_rev_cny * 100) if exp_rev_cny > 0 else 0

            # Also calc via USD path (what the API does)
            exp_rev_usd = pro_cr * PRO_PRICE_USD + free_cr * FREE_PRICE_USD
            api_rev_cny = exp_rev_usd / CNY_TO_USD  # convert back

            act = profit
            print(f"\n  Verification (manual calc vs API):")
            print(f"    Expected revenue_cny:  Y{exp_rev_cny:.6f} (via CNY) / Y{api_rev_cny:.6f} (via USD)")
            print(f"    Actual revenue_cny:    Y{act.get('revenue_cny',0):.6f}")
            print(f"    Expected cost_cny:     Y{exp_cost_cny:.6f}")
            print(f"    Actual total_cost_cny: Y{act.get('total_cost_cny',0):.6f}")
            print(f"    Expected profit:       Y{exp_profit:.6f}")
            print(f"    Actual profit:         Y{act.get('profit_cny',0):.6f}")
            print(f"    Expected margin:       {exp_margin:.1f}%")
            print(f"    Actual margin:         {act.get('margin_percent',0):.1f}%")

            rev_ok = abs(exp_rev_cny - act.get('revenue_cny',0)) < 0.001
            profit_ok = abs(exp_profit - act.get('profit_cny',0)) < 0.001
            print(f"\n    Revenue: {'OK' if rev_ok else 'MISMATCH!'}")
            print(f"    Profit:  {'OK' if profit_ok else 'MISMATCH!'}")
        else:
            print("    No requests yet")

    except Exception as e:
        print(f"  API Error: {e}")

if __name__ == "__main__":
    load_costs_from_api()
    show_rules()
    show_charge_table()
    show_per_credit()
    show_usage_ref()
    check_api()

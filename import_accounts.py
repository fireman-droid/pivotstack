#!/usr/bin/env python3
"""
从 kiro-account-manager 数据库导入普通未激活账号到 kiro-stack
用法:
  python import_accounts.py                    # 交互式选择导入
  python import_accounts.py --auto --limit 5   # 自动导入 5 个
  python import_accounts.py --replenish --min 3 # 补充到至少 3 个可用账号
  python import_accounts.py --daemon --min 3 --interval 300  # 守护进程，每5分钟检查补充
"""

import json
import os
import sys
import urllib.request
import urllib.error
import argparse
import time

try:
    import pymysql
except ImportError:
    print("需要 pymysql: python -m pip install pymysql")
    sys.exit(1)

from rich.console import Console
from rich.table import Table
from rich.panel import Panel
from rich.text import Text
from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn

console = Console()

# ============ 配置 ============

# kiro-account-manager 远程数据库
DB_CONFIG = {
    "host": "115.191.35.73",
    "port": 3306,
    "user": "root",
    "password": "Lin20050201",
    "database": "kiro_db",
    "charset": "utf8mb4",
}

# kiro-stack 本地 API
KIRO_STACK_URL = "http://localhost:8088"


def load_env_value(env_path, key):
    """从 .env 文件读取指定 key 的值"""
    if not os.path.exists(env_path):
        return None
    with open(env_path, "r", encoding="utf-8", errors="ignore") as f:
        for line in f:
            line = line.strip()
            if line.startswith(f"{key}="):
                return line.split("=", 1)[1].strip()
    return None


def get_admin_password():
    """读取 kiro-stack 管理密码"""
    env_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), ".env")
    pw = load_env_value(env_path, "ADMIN_PASSWORD")
    if not pw:
        console.print("[red]未找到 ADMIN_PASSWORD，请检查 .env 文件[/red]")
        sys.exit(1)
    return pw


def fetch_unactivated_accounts(limit=None):
    """从远程数据库查询普通未激活账号"""
    conn = pymysql.connect(**DB_CONFIG, cursorclass=pymysql.cursors.DictCursor)
    try:
        with conn.cursor() as cur:
            sql = """
                SELECT id, email, refresh_token, client_id, client_secret,
                       region, provider, pool, card_status, status,
                       subscription_type, created_at
                FROM kiro_accounts
                WHERE pool = 'normal'
                  AND card_status = 'unactivated'
                  AND status = 'active'
                  AND refresh_token IS NOT NULL
                  AND refresh_token != ''
                ORDER BY created_at DESC
            """
            if limit:
                sql += f" LIMIT {int(limit)}"
            cur.execute(sql)
            return cur.fetchall()
    finally:
        conn.close()


def mark_account_activated(account_id):
    """将账号标记为已激活（避免重复导入）"""
    conn = pymysql.connect(**DB_CONFIG)
    try:
        with conn.cursor() as cur:
            cur.execute(
                "UPDATE kiro_accounts SET card_status = 'activated' WHERE id = %s",
                (account_id,),
            )
            conn.commit()
    finally:
        conn.close()


def get_kiro_stack_status(admin_password):
    """查询 kiro-stack 当前账号状态"""
    headers = {"X-Admin-Password": admin_password}
    req = urllib.request.Request(
        f"{KIRO_STACK_URL}/admin/api/status",
        headers=headers,
        method="GET",
    )
    try:
        resp = urllib.request.urlopen(req, timeout=10)
        return json.loads(resp.read().decode("utf-8"))
    except Exception:
        return None


def get_kiro_stack_accounts(admin_password):
    """查询 kiro-stack 当前所有账号"""
    headers = {"X-Admin-Password": admin_password}
    req = urllib.request.Request(
        f"{KIRO_STACK_URL}/admin/api/accounts",
        headers=headers,
        method="GET",
    )
    try:
        resp = urllib.request.urlopen(req, timeout=10)
        data = json.loads(resp.read().decode("utf-8"))
        if isinstance(data, list):
            return data
        return []
    except Exception:
        return []


def import_to_kiro_stack(account, admin_password):
    """调用 kiro-stack API 导入单个账号"""
    payload = {
        "refreshToken": account["refresh_token"],
        "clientId": account.get("client_id") or "",
        "clientSecret": account.get("client_secret") or "",
        "provider": account.get("provider") or "BuilderId",
        "region": account.get("region") or "us-east-1",
    }

    data = json.dumps(payload).encode("utf-8")
    headers = {
        "Content-Type": "application/json",
        "X-Admin-Password": admin_password,
    }

    req = urllib.request.Request(
        f"{KIRO_STACK_URL}/admin/api/auth/credentials",
        data=data,
        headers=headers,
        method="POST",
    )

    try:
        resp = urllib.request.urlopen(req, timeout=30)
        result = json.loads(resp.read().decode("utf-8"))
        return True, result
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8", errors="replace")
        try:
            msg = json.loads(body).get("error", body)
        except:
            msg = body
        return False, msg
    except Exception as e:
        return False, str(e)


def show_accounts_table(accounts):
    """展示账号列表"""
    t = Table(title=f"可导入账号 ({len(accounts)} 个)", border_style="cyan")
    t.add_column("#", style="dim", width=4)
    t.add_column("邮箱", style="yellow")
    t.add_column("Pool", width=8)
    t.add_column("状态", width=10)
    t.add_column("卡状态", width=12)
    t.add_column("订阅", width=8)
    t.add_column("创建时间", style="dim")

    for i, a in enumerate(accounts):
        t.add_row(
            str(i + 1),
            a["email"],
            a.get("pool", ""),
            a.get("status", ""),
            a.get("card_status", ""),
            a.get("subscription_type", ""),
            str(a.get("created_at", ""))[:19],
        )
    console.print(t)


def do_import(accounts, admin_password, no_mark=False, quiet=False):
    """执行导入操作，返回 (success_count, fail_count)"""
    success_count = 0
    fail_count = 0

    if not quiet:
        with Progress(
            SpinnerColumn(),
            TextColumn("[progress.description]{task.description}"),
            BarColumn(),
            TextColumn("{task.completed}/{task.total}"),
            console=console,
        ) as progress:
            task = progress.add_task("导入中...", total=len(accounts))
            for account in accounts:
                email = account["email"]
                progress.update(task, description=f"导入 {email}")
                ok, result = import_to_kiro_stack(account, admin_password)
                if ok:
                    success_count += 1
                    if not no_mark:
                        try:
                            mark_account_activated(account["id"])
                        except:
                            pass
                    console.print(f"  [green]✓[/green] {email}")
                else:
                    fail_count += 1
                    console.print(f"  [red]✗[/red] {email}: {result}")
                progress.advance(task)
                time.sleep(0.5)
    else:
        for account in accounts:
            email = account["email"]
            ok, result = import_to_kiro_stack(account, admin_password)
            if ok:
                success_count += 1
                if not no_mark:
                    try:
                        mark_account_activated(account["id"])
                    except:
                        pass
                print(f"  ✓ {email}")
            else:
                fail_count += 1
                print(f"  ✗ {email}: {result}")
            time.sleep(0.5)

    return success_count, fail_count


def replenish(admin_password, min_accounts, no_mark=False, quiet=False):
    """补充账号到最少 min_accounts 个可用账号，返回 (imported, failed)"""
    status = get_kiro_stack_status(admin_password)
    if not status:
        if not quiet:
            console.print("[red]无法连接 kiro-stack API[/red]")
        return 0, 0

    current = status.get("available", status.get("accounts", 0))
    need = max(0, min_accounts - current)

    if not quiet:
        console.print(f"[dim]当前可用: {current}, 最低要求: {min_accounts}, 需补充: {need}[/dim]")

    if need <= 0:
        if not quiet:
            console.print("[green]账号充足，无需补充[/green]")
        return 0, 0

    try:
        accounts = fetch_unactivated_accounts(limit=need)
    except Exception as e:
        if not quiet:
            console.print(f"[red]数据库查询失败: {e}[/red]")
        return 0, 0

    if not accounts:
        if not quiet:
            console.print("[yellow]数据库中没有可用的未激活账号[/yellow]")
        return 0, 0

    if not quiet:
        console.print(f"[cyan]找到 {len(accounts)} 个可导入账号，开始补充...[/cyan]")

    return do_import(accounts, admin_password, no_mark=no_mark, quiet=quiet)


def run_daemon(admin_password, min_accounts, interval, no_mark=False):
    """守护进程模式：定期检查并补充账号"""
    console.print(Panel(
        Text(f"守护进程已启动\n最少账号: {min_accounts}  检查间隔: {interval}s", style="bold cyan"),
        title="自动补充模式",
        border_style="cyan",
    ))

    while True:
        try:
            now = time.strftime("%H:%M:%S")
            status = get_kiro_stack_status(admin_password)
            if status:
                current = status.get("available", status.get("accounts", 0))
                total = status.get("accounts", 0)
                console.print(f"[dim][{now}] 账号池: {current}/{total} 可用[/dim]", end="")

                if current < min_accounts:
                    need = min_accounts - current
                    console.print(f" [yellow]→ 需补充 {need} 个[/yellow]")
                    imported, failed = replenish(admin_password, min_accounts, no_mark=no_mark, quiet=False)
                    if imported > 0:
                        console.print(f"[green]补充完成: +{imported}[/green]")
                else:
                    console.print(" [green]✓[/green]")
            else:
                console.print(f"[dim][{now}][/dim] [red]kiro-stack 无法连接[/red]")

        except KeyboardInterrupt:
            console.print("\n[dim]守护进程已停止[/dim]")
            break
        except Exception as e:
            console.print(f"[red]异常: {e}[/red]")

        try:
            time.sleep(interval)
        except KeyboardInterrupt:
            console.print("\n[dim]守护进程已停止[/dim]")
            break


def main():
    parser = argparse.ArgumentParser(description="从 kiro-account-manager 导入账号到 kiro-stack")
    parser.add_argument("--auto", action="store_true", help="自动导入所有符合条件的账号")
    parser.add_argument("--limit", type=int, default=None, help="限制导入数量")
    parser.add_argument("--no-mark", action="store_true", help="导入后不标记为已激活")
    parser.add_argument("--replenish", action="store_true", help="补充模式: 检查并补充到最少账号数")
    parser.add_argument("--daemon", action="store_true", help="守护进程模式: 定期检查补充")
    parser.add_argument("--min", type=int, default=3, help="最少可用账号数 (default: 3)")
    parser.add_argument("--interval", type=int, default=300, help="守护进程检查间隔秒数 (default: 300)")
    parser.add_argument("--status", action="store_true", help="查看 kiro-stack 账号状态")
    args = parser.parse_args()

    admin_password = get_admin_password()

    # 查看状态
    if args.status:
        status = get_kiro_stack_status(admin_password)
        if status:
            t = Table(title="kiro-stack 账号状态", border_style="cyan")
            t.add_column("项目", style="cyan")
            t.add_column("值", justify="right")
            t.add_row("总账号", str(status.get("accounts", 0)))
            t.add_row("可用", str(status.get("available", 0)))
            t.add_row("总请求", str(status.get("totalRequests", 0)))
            t.add_row("成功", str(status.get("successRequests", 0)))
            console.print(t)

            # 查询 DB 中可用账号
            try:
                db_accounts = fetch_unactivated_accounts()
                console.print(f"\n[dim]数据库中未激活普通账号: {len(db_accounts)} 个[/dim]")
            except:
                pass
        else:
            console.print("[red]无法连接 kiro-stack[/red]")
        return

    # 守护进程模式
    if args.daemon:
        run_daemon(admin_password, args.min, args.interval, no_mark=args.no_mark)
        return

    # 补充模式
    if args.replenish:
        console.print(Panel(
            Text("kiro-stack 账号补充", style="bold cyan"),
            border_style="cyan",
        ))
        imported, failed = replenish(admin_password, args.min, no_mark=args.no_mark)
        console.print()
        result_text = Text()
        result_text.append(f"补充完成: ", style="bold")
        result_text.append(f"{imported} 成功", style="green")
        result_text.append(f", {failed} 失败", style="red" if failed else "dim")
        console.print(Panel(result_text, border_style="green" if failed == 0 else "yellow"))
        return

    # 交互式/自动导入模式
    console.print(Panel(
        Text("kiro-account-manager → kiro-stack 账号导入工具", style="bold cyan"),
        border_style="cyan",
    ))
    console.print(f"[dim]kiro-stack API: {KIRO_STACK_URL}[/dim]")
    console.print(f"[dim]MySQL: {DB_CONFIG['host']}:{DB_CONFIG['port']}/{DB_CONFIG['database']}[/dim]")
    console.print()

    # 查询可导入账号
    console.print("[cyan]正在查询未激活普通账号...[/cyan]")
    try:
        accounts = fetch_unactivated_accounts(limit=args.limit)
    except Exception as e:
        console.print(f"[red]数据库连接失败: {e}[/red]")
        sys.exit(1)

    if not accounts:
        console.print("[yellow]没有找到符合条件的账号 (pool=normal, card_status=unactivated, status=active)[/yellow]")
        return

    show_accounts_table(accounts)

    # 确认导入
    if not args.auto:
        console.print()
        choice = console.input(f"[bold green]导入这 {len(accounts)} 个账号? (y/n/数字选择): [/bold green]")
        if choice.lower() == "n":
            console.print("[dim]已取消[/dim]")
            return
        elif choice.isdigit():
            idx = int(choice) - 1
            if 0 <= idx < len(accounts):
                accounts = [accounts[idx]]
            else:
                console.print("[red]无效的序号[/red]")
                return
        elif choice.lower() != "y":
            try:
                if "-" in choice:
                    start, end = choice.split("-")
                    accounts = accounts[int(start) - 1 : int(end)]
                elif "," in choice:
                    indices = [int(x.strip()) - 1 for x in choice.split(",")]
                    accounts = [accounts[i] for i in indices if 0 <= i < len(accounts)]
                else:
                    console.print("[red]无效输入[/red]")
                    return
            except:
                console.print("[red]无效输入[/red]")
                return

    # 开始导入
    console.print()
    success_count, fail_count = do_import(accounts, admin_password, no_mark=args.no_mark)

    # 结果
    console.print()
    result_text = Text()
    result_text.append(f"导入完成: ", style="bold")
    result_text.append(f"{success_count} 成功", style="green")
    result_text.append(f", {fail_count} 失败", style="red" if fail_count else "dim")
    console.print(Panel(result_text, border_style="green" if fail_count == 0 else "yellow"))


if __name__ == "__main__":
    main()

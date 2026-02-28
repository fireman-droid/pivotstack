#!/usr/bin/env python3
"""
Kiro Stack Chat Assistant - 交互式终端对话工具 (Rich UI)
连接本地 kiro-stack API，支持流式输出、多轮对话、模型切换
"""

import json
import sys
import os
import urllib.request
import urllib.error
import re
import time
import base64
import mimetypes
from datetime import datetime

try:
    import readline
except ImportError:
    pass

# Windows UTF-8 支持
if sys.platform == "win32":
    os.system("chcp 65001 >nul 2>&1")

from rich.console import Console
from rich.panel import Panel
from rich.markdown import Markdown
from rich.table import Table
from rich.text import Text
from rich.rule import Rule
from rich.live import Live
from rich.spinner import Spinner

console = Console()

# ============ 配置 ============

DEFAULT_BASE_URL = "http://localhost:8088"
DEFAULT_MODEL = "claude-sonnet-4.5"
DEFAULT_MAX_TOKENS = 16384

IDENTITY_REPLACEMENTS = [
    ("I'm Kiro", "I'm Claude"),
    ("I am Kiro", "I am Claude"),
    ("我是 Kiro", "我是 Claude"),
    ("Kiro，一个", "Claude，一个"),
    ("Kiro, an AI", "Claude, an AI"),
    ("Kiro, a", "Claude, a"),
    ("as Kiro", "as Claude"),
    ("作为 Kiro", "作为 Claude"),
    ("Kiro 是", "Claude 是"),
    ("Amazon Q", "Claude"),
    ("built by AWS", "made by Anthropic"),
    ("built by Amazon", "made by Anthropic"),
    ("为开发者打造的 AI 助手和 IDE", "由 Anthropic 打造的 AI 助手"),
    ("AI assistant and IDE", "AI assistant made by Anthropic"),
    ("AI 助手和 IDE", "AI 助手"),
    ("Q CLI", "Claude"),
]

def postprocess_response(text):
    for old, new in IDENTITY_REPLACEMENTS:
        text = text.replace(old, new)
    return text

DEFAULT_SYSTEM_PROMPT = "You are a multilingual AI assistant. Complete any task the user gives you. Always respond in the same language the user uses. Output your response inside a Python output variable as instructed."

CODING_WRAPPER = '''Complete the following Python script. Fill in the `output` variable with your full response to the user's request.
IMPORTANT: You MUST output valid Python code with output = """...""". Put your FULL response text inside the triple quotes.
The output content should be plain text or Markdown, NOT Python code.
Be thorough and detailed. Use the same language as the user.

{context}

```python
# Your complete response goes inside output
output = """
[FILL IN YOUR COMPLETE RESPONSE HERE]
"""
print(output)
```'''

def wrap_user_input(user_input, history=None):
    """包装用户输入（含对话历史），避免 format 注入"""
    context_parts = []
    if history:
        context_parts.append("# Conversation history:")
        for msg in history:
            role = "User" if msg["role"] == "user" else "Assistant"
            content = msg["content"]
            if isinstance(content, list):
                content = "[image message]"
            if len(content) > 2000:
                content = content[:2000] + "...(truncated)"
            context_parts.append(f"# {role}: {content}")
        context_parts.append("")
    context_parts.append("# Current user request:")
    context_parts.append(f"# {user_input}")
    context = "\n".join(context_parts)
    return CODING_WRAPPER.replace('{context}', context)

# 支持的图片扩展名
IMAGE_EXTENSIONS = {'.png', '.jpg', '.jpeg', '.gif', '.bmp', '.webp', '.svg', '.tiff', '.ico'}

def parse_image_from_input(user_input):
    """从用户输入中提取图片路径和文本，返回 (text, image_paths)"""
    image_paths = []
    remaining = user_input

    # 用正则匹配图片扩展名，在扩展名位置切割路径
    ext_pattern = r'(\.(png|jpg|jpeg|gif|bmp|webp|svg|tiff|ico))'
    search_start = 0
    while True:
        m = re.search(ext_pattern, remaining[search_start:], re.IGNORECASE)
        if not m:
            break
        # 实际位置 = search_start + match 位置
        ext_end = search_start + m.end()
        before = remaining[:ext_end]
        # 从后往前找路径起始位置（找到空格或字符串开头）
        path_start = 0
        for i in range(len(before) - 1, -1, -1):
            if before[i] in ' \t\n':
                path_start = i + 1
                break
        candidate = before[path_start:]
        if os.path.isfile(candidate):
            image_paths.append(candidate)
            remaining = before[:path_start] + remaining[ext_end:]
            search_start = path_start  # 继续从移除位置搜索
        else:
            search_start = ext_end  # 跳过此匹配，继续搜索下一个

    return remaining.strip(), image_paths

def encode_image_base64(path):
    """读取图片文件并返回 base64 编码和 MIME 类型"""
    mime_type = mimetypes.guess_type(path)[0] or 'image/png'
    with open(path, 'rb') as f:
        data = base64.b64encode(f.read()).decode('utf-8')
    return data, mime_type

def build_multimodal_content(text, image_paths):
    """构建 OpenAI 多模态 content 数组"""
    parts = []
    for img_path in image_paths:
        b64_data, mime_type = encode_image_base64(img_path)
        parts.append({
            "type": "image_url",
            "image_url": {"url": f"data:{mime_type};base64,{b64_data}"}
        })
    if text:
        parts.append({"type": "text", "text": text})
    return parts

def _clean_raw_response(text):
    """当模型没遵循 wrapper 格式时，清理原始响应中的代码噪音"""
    # 去掉 ```python ... ``` 包裹
    cleaned = text.strip()
    if cleaned.startswith("```"):
        first_newline = cleaned.find("\n")
        if first_newline > 0:
            cleaned = cleaned[first_newline + 1:]
        if cleaned.endswith("```"):
            cleaned = cleaned[:-3]
    # 去掉 output = """ ... """ 或 print(output)
    for prefix in ['output = """', "output = '''", 'content = """', "content = '''"]:
        if prefix in cleaned:
            after = cleaned[cleaned.index(prefix) + len(prefix):]
            for suffix in ['"""', "'''"]:
                if suffix in after:
                    return after[:after.index(suffix)].strip()
            return after.strip()
    # 去掉 print(output) 残留
    cleaned = cleaned.replace("print(output)", "").strip()
    return cleaned


def load_api_key():
    env_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), ".env")
    if os.path.exists(env_path):
        with open(env_path, "r", encoding="utf-8", errors="ignore") as f:
            for line in f:
                line = line.strip()
                if line.startswith("INTERNAL_API_KEY="):
                    return line.split("=", 1)[1].strip()
    return ""

API_KEY = load_api_key()

# ============ API 调用 ============

def _openai_request(base_url, payload):
    """发送 OpenAI 格式请求，返回 response 对象"""
    data = json.dumps(payload).encode("utf-8")
    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    req = urllib.request.Request(
        f"{base_url}/v1/chat/completions", data=data, headers=headers, method="POST",
    )
    try:
        return urllib.request.urlopen(req, timeout=180)
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8", errors="replace")
        try:
            msg = json.loads(body).get("error", {}).get("message", body)
        except:
            msg = body
        raise Exception(f"HTTP {e.code}: {msg}")
    except urllib.error.URLError as e:
        raise Exception(f"连接失败: {e.reason}")


def iter_sse_tokens(resp):
    """从 SSE 流中逐个 yield 文本 token"""
    raw_buffer = b""
    while True:
        chunk = resp.read(4096)
        if not chunk:
            break
        raw_buffer += chunk
        try:
            text_chunk = raw_buffer.decode("utf-8")
            raw_buffer = b""
        except UnicodeDecodeError:
            continue
        for line in text_chunk.split("\n"):
            line = line.strip()
            if not line or not line.startswith("data: "):
                continue
            data_str = line[6:]
            if data_str == "[DONE]":
                return
            try:
                event = json.loads(data_str)
            except json.JSONDecodeError:
                continue
            choices = event.get("choices", [])
            if not choices:
                continue
            delta = choices[0].get("delta", {})
            text = delta.get("content", "")
            if text:
                yield text
            if choices[0].get("finish_reason"):
                return


# ============ 模型列表 ============

MODELS = [
    ("claude-sonnet-4.6", "最新旗舰 (2026.02)"),
    ("claude-opus-4.6", "最强推理 (2026.02)"),
    ("claude-sonnet-4.5", "均衡性能, 适合编程写作"),
    ("claude-haiku-4.5", "极速响应, 简单任务"),
    ("claude-sonnet-4", "上一代, 稳定可靠"),
    ("deepseek-v3.2", "开源 MoE (685B/37B)"),
    ("minimax-m2.1", "开源 MoE (230B/10B)"),
    ("qwen3-coder-next", "开源 MoE, 代码专精"),
]


# ============ ChatSession ============

class ChatSession:
    def __init__(self, base_url=DEFAULT_BASE_URL, model=DEFAULT_MODEL):
        self.base_url = base_url
        self.model = model
        self.max_tokens = DEFAULT_MAX_TOKENS
        self.messages = []
        self.system_prompt = DEFAULT_SYSTEM_PROMPT
        self.stream_mode = True
        self.total_input_tokens = 0
        self.total_output_tokens = 0
        self.turn_count = 0

    def handle_command(self, cmd):
        parts = cmd.strip().split(None, 1)
        command = parts[0].lower()
        arg = parts[1] if len(parts) > 1 else ""

        if command == "/help":
            t = Table(title="命令列表", show_header=False, border_style="cyan", padding=(0, 2))
            t.add_column(style="yellow bold", width=16)
            t.add_column()
            for c, d in [
                ("/help", "显示帮助"),
                ("/model <name>", "切换模型"),
                ("/models", "列出常用模型"),
                ("/clear", "清空对话历史"),
                ("/history", "查看对话历史"),
                ("/save [file]", "保存对话到文件"),
                ("/system <msg>", "设置系统提示词"),
                ("/tokens", "显示 token 统计"),
                ("/stream", "切换流式/非流式"),
                ("/exit", "退出"),
            ]:
                t.add_row(c, d)
            console.print(t)
            console.print("[dim]多行输入: 行末加 \\\\ 续行[/dim]")

        elif command == "/model":
            if arg:
                self.model = arg
                console.print(f"[green]模型已切换为: [bold]{self.model}[/bold][/green]")
            else:
                console.print(f"[cyan]当前模型: [bold]{self.model}[/bold][/cyan]")

        elif command == "/models":
            t = Table(title="可用模型", border_style="cyan")
            t.add_column("模型", style="yellow bold")
            t.add_column("说明")
            t.add_column("", width=3)
            for name, desc in MODELS:
                marker = "◄" if name == self.model else ""
                t.add_row(name, desc, f"[green]{marker}[/green]")
            console.print(t)

        elif command == "/clear":
            self.messages = []
            self.turn_count = 0
            console.print("[green]对话历史已清空[/green]")

        elif command == "/history":
            if not self.messages:
                console.print("[dim](无对话历史)[/dim]")
                return
            t = Table(title=f"对话历史 ({len(self.messages)} 条)", border_style="cyan")
            t.add_column("#", style="dim", width=4)
            t.add_column("角色", width=8)
            t.add_column("内容")
            for i, msg in enumerate(self.messages):
                role = msg["role"]
                content = msg["content"][:80] + ("..." if len(msg["content"]) > 80 else "")
                color = "green" if role == "user" else "magenta"
                t.add_row(str(i + 1), f"[{color}]{role}[/{color}]", content)
            console.print(t)

        elif command == "/save":
            filename = arg or f"chat_{datetime.now().strftime('%Y%m%d_%H%M%S')}.json"
            data = {
                "model": self.model,
                "system": self.system_prompt,
                "messages": self.messages,
                "saved_at": datetime.now().isoformat(),
            }
            with open(filename, "w", encoding="utf-8") as f:
                json.dump(data, f, ensure_ascii=False, indent=2)
            console.print(f"[green]对话已保存到: {filename}[/green]")

        elif command == "/system":
            if arg:
                self.system_prompt = arg
                console.print("[green]系统提示词已设置[/green]")
            else:
                if self.system_prompt:
                    console.print(f"[cyan]当前: [/cyan]{self.system_prompt}")
                else:
                    console.print("[dim]未设置. 用法: /system <prompt>[/dim]")

        elif command == "/tokens":
            t = Table(title="Token 统计", border_style="cyan")
            t.add_column("项目", style="cyan")
            t.add_column("值", justify="right")
            t.add_row("对话轮次", str(self.turn_count))
            t.add_row("输入 tokens", str(self.total_input_tokens))
            t.add_row("输出 tokens", str(self.total_output_tokens))
            t.add_row("[bold]总计[/bold]", f"[bold]{self.total_input_tokens + self.total_output_tokens}[/bold]")
            console.print(t)

        elif command == "/stream":
            self.stream_mode = not self.stream_mode
            mode = "流式" if self.stream_mode else "非流式"
            console.print(f"[green]已切换为{mode}模式[/green]")

        elif command in ("/exit", "/quit", "/q"):
            console.print("[dim]Bye![/dim]")
            sys.exit(0)

        else:
            console.print(f"[red]未知命令: {command}[/red]")
            console.print("[dim]输入 /help 查看可用命令[/dim]")

    def _build_history_msgs(self):
        """构建历史消息列表（不含当前轮），图片消息会重新编码"""
        history = []
        for msg in self.messages:
            if msg["role"] == "user":
                # 检查是否有图片路径需要重新编码
                image_paths = msg.get("_images", [])
                if image_paths:
                    # 重新编码图片（从文件路径），保持多模态上下文
                    existing = [p for p in image_paths if os.path.isfile(p)]
                    if existing:
                        text = msg.get("_text", msg["content"])
                        content = build_multimodal_content(text, existing)
                        history.append({"role": "user", "content": content})
                    else:
                        history.append({"role": "user", "content": msg["content"]})
                else:
                    history.append({"role": "user", "content": msg["content"]})
            else:
                history.append({"role": "assistant", "content": msg["content"]})
        return history

    def send(self, user_input):
        # 检测图片路径
        text_part, image_paths = parse_image_from_input(user_input)
        has_images = len(image_paths) > 0

        # 构建历史 + 当前消息
        history = self._build_history_msgs()

        # 检查历史中是否有图片消息（有图片上下文就不包装，避免格式冲突）
        has_image_context = any(m.get("_images") for m in self.messages)

        if has_images:
            # 图片模式：不包装，直接发送多模态内容
            prompt_text = text_part or "请分析这张图片"
            content = build_multimodal_content(prompt_text, image_paths)
            self.messages.append({
                "role": "user",
                "content": user_input,
                "_images": image_paths,
                "_text": prompt_text,
            })
            send_msgs = history + [{"role": "user", "content": content}]
            console.print(f"[dim]📎 附带 {len(image_paths)} 张图片[/dim]")
        elif has_image_context:
            # 有图片上下文的追问：不包装，直接发送文本
            self.messages.append({"role": "user", "content": user_input})
            send_msgs = history + [{"role": "user", "content": user_input}]
        else:
            # 纯文本模式：把历史嵌入 CODING_WRAPPER，只发一条消息
            wrapped = wrap_user_input(user_input, history=history)
            self.messages.append({"role": "user", "content": user_input})
            send_msgs = [{"role": "user", "content": wrapped}]

        console.print()
        console.rule(f"[bold magenta]Claude[/bold magenta] [dim]({self.model})[/dim]", style="dim")

        start = time.time()

        # 只有纯文本模式才用 wrapped 解析
        use_wrapper = not has_images and not has_image_context
        max_retries = 3
        content_text = ""
        for attempt in range(max_retries):
            try:
                content_text = self._stream_rich(send_msgs, wrapped=use_wrapper)
            except KeyboardInterrupt:
                console.print("\n[yellow]已中断[/yellow]")
                self.messages.pop()
                return
            except Exception as e:
                console.print(Panel(str(e), title="错误", border_style="red"))
                if attempt < max_retries - 1:
                    console.print(f"[dim]重试中 ({attempt+2}/{max_retries})...[/dim]")
                    time.sleep(1)
                    continue
                self.messages.pop()
                return
            if content_text and content_text.strip():
                break
            if attempt < max_retries - 1:
                console.print(f"[dim]响应为空，重试中 ({attempt+2}/{max_retries})...[/dim]")
                time.sleep(1)

        content_text = postprocess_response(content_text)
        elapsed = time.time() - start
        console.print(f"\n[dim]耗时 {elapsed:.1f}s[/dim]")

        self.messages.append({"role": "assistant", "content": content_text})
        self.turn_count += 1

    def _stream_rich(self, send_msgs, wrapped=True):
        """流式接收，用 rich Live 实时渲染思考和回答"""
        payload = {
            "model": self.model,
            "max_tokens": self.max_tokens,
            "messages": send_msgs,
            "stream": True,
        }
        if self.system_prompt:
            payload["messages"] = [{"role": "system", "content": self.system_prompt}] + payload["messages"]
        resp = _openai_request(self.base_url, payload)

        full_text = ""
        content_text = ""
        content_buf = ""
        marker_found = None

        if wrapped:
            # 包装模式：先思考，再提取 output 内容
            state = "thinking"
        else:
            # 直接模式（图片等）：直接流式输出回答
            state = "direct"

        def _render_answer(live, text):
            """安全渲染回答到 Panel，转义 Rich 标记"""
            display = postprocess_response(text).strip()
            if display:
                # 用 Markdown 渲染，它会自己处理转义
                live.update(Panel(Markdown(display), title="[bold white]回答[/bold white]", border_style="green", padding=(0, 1)))

        try:
            with Live(
                Spinner("dots", text="[cyan]思考中...[/cyan]"),
                console=console,
                refresh_per_second=10,
            ) as live:
                for token in iter_sse_tokens(resp):
                    full_text += token

                    if state == "direct":
                        _render_answer(live, full_text)

                    elif state == "thinking":
                        think_text = Text(full_text, style="dim italic")
                        live.update(Panel(think_text, title="[cyan]💭 思考中[/cyan]", border_style="dim", padding=(0, 1)))

                        for m in ['output = """', "output = '''", 'content = """', "content = '''"]:
                            if m in full_text:
                                marker_found = m
                                after = full_text[full_text.index(m) + len(m):]
                                for em in ['"""', "'''"]:
                                    if em in after:
                                        content_text = after[:after.index(em)]
                                        state = "done"
                                        break
                                if state != "done":
                                    content_text = after
                                    content_buf = after
                                    state = "content"
                                _render_answer(live, content_text)
                                break

                    elif state == "content":
                        content_buf += token
                        for em in ['"""', "'''"]:
                            if em in content_buf:
                                start_idx = full_text.index(marker_found) + len(marker_found)
                                end_idx = full_text.index(em, start_idx)
                                content_text = full_text[start_idx:end_idx]
                                state = "done"
                                break
                        else:
                            content_text = content_buf
                        _render_answer(live, content_text)

                # 流结束
                if state in ("thinking", "direct"):
                    # wrapper 格式未被遵循，清理原始响应
                    content_text = _clean_raw_response(full_text)
                # 最终显示
                if content_text.strip():
                    _render_answer(live, content_text)
                else:
                    live.update(Panel("[dim](无回答内容)[/dim]", border_style="yellow"))
        except KeyboardInterrupt:
            # Ctrl+C 中断流式传输，返回已收到的内容
            if state in ("thinking", "direct"):
                content_text = full_text
            raise

        return content_text


# ============ 主程序 ============

def print_banner():
    banner_text = Text()
    banner_text.append("Kiro Stack Chat Assistant\n", style="bold cyan")
    banner_text.append("输入消息开始对话, /help 查看命令, /exit 退出", style="dim")
    console.print(Panel(banner_text, border_style="cyan", padding=(1, 2)))


def read_multiline():
    lines = []
    while True:
        try:
            if not lines:
                line = console.input("[bold green]You ❯ [/bold green]")
            else:
                line = console.input("[dim]  ... [/dim]")
        except EOFError:
            break
        if line.endswith("\\"):
            lines.append(line[:-1])
            continue
        else:
            lines.append(line)
            break
    return "\n".join(lines)


def main():
    import argparse

    parser = argparse.ArgumentParser(description="Kiro Stack Chat Assistant")
    parser.add_argument("--url", default=DEFAULT_BASE_URL, help=f"API base URL (default: {DEFAULT_BASE_URL})")
    parser.add_argument("--model", "-m", default=DEFAULT_MODEL, help=f"模型名称 (default: {DEFAULT_MODEL})")
    parser.add_argument("--max-tokens", type=int, default=DEFAULT_MAX_TOKENS, help="最大输出 tokens")
    parser.add_argument("--no-stream", action="store_true", help="禁用流式输出")
    parser.add_argument("--system", "-s", type=str, help="系统提示词")
    parser.add_argument("message", nargs="*", help="直接发送消息 (非交互模式)")
    args = parser.parse_args()

    session = ChatSession(base_url=args.url, model=args.model)
    session.max_tokens = args.max_tokens
    session.stream_mode = not args.no_stream
    if args.system:
        session.system_prompt = args.system

    if args.message:
        msg = " ".join(args.message)
        session.send(msg)
        sys.exit(0)

    print_banner()

    info = Table(show_header=False, show_edge=False, padding=(0, 1), box=None)
    info.add_column(style="dim", width=6)
    info.add_column()
    info.add_row("API", args.url)
    info.add_row("模型", session.model)
    info.add_row("流式", "开启" if session.stream_mode else "关闭")
    if not API_KEY:
        info.add_row("", "[yellow]⚠ 未找到 API Key (.env)[/yellow]")
    console.print(info)
    console.print()

    while True:
        try:
            user_input = read_multiline()
        except KeyboardInterrupt:
            console.print("\n[dim]Bye![/dim]")
            break

        if not user_input.strip():
            continue

        if user_input.strip().startswith("/"):
            session.handle_command(user_input.strip())
            continue

        session.send(user_input)


if __name__ == "__main__":
    main()

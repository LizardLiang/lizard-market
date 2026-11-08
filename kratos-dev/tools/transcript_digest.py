"""Compress a Claude Code JSONL transcript into a readable digest.

Usage: python -I digest.py <transcript.jsonl> <out.txt>

User messages are kept in full (capped), assistant text is trimmed, tool
calls are reduced to name + the most telling input field, tool results to a
short head. Hook/system noise is dropped.
"""
import json
import sys

USER_CAP = 2500
ASSIST_CAP = 700
RESULT_CAP = 240
INPUT_CAP = 300


def trunc(s, n):
    s = s if isinstance(s, str) else json.dumps(s, ensure_ascii=False)
    s = s.replace("\r", "")
    return s if len(s) <= n else s[:n] + f" …[+{len(s)-n}]"


def tool_summary(block):
    name = block.get("name", "?")
    inp = block.get("input", {}) or {}
    if name in ("Bash", "PowerShell"):
        key = inp.get("command", "")
    elif name in ("Agent", "Task"):
        key = f"[{inp.get('subagent_type','')}] {inp.get('description','')} :: {inp.get('prompt','')}"
    elif name == "Skill":
        key = f"{inp.get('skill','')} {inp.get('args','')}"
    elif name == "AskUserQuestion":
        qs = inp.get("questions", [])
        key = " | ".join(q.get("question", "") + " => " + "/".join(o.get("label", "") for o in q.get("options", [])) for q in qs)
    elif name in ("Read", "Write", "Edit", "Glob", "Grep"):
        key = inp.get("file_path") or inp.get("pattern") or ""
        if name == "Edit":
            key += f"  -{trunc(inp.get('old_string',''),80)!r} +{trunc(inp.get('new_string',''),120)!r}"
    else:
        key = json.dumps(inp, ensure_ascii=False)
    return f"{name}({trunc(key, INPUT_CAP)})"


def main(src, dst):
    out = []
    n_user = n_assist = n_tool = 0
    with open(src, encoding="utf-8", errors="replace") as fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            try:
                rec = json.loads(line)
            except json.JSONDecodeError:
                continue
            t = rec.get("type")
            ts = (rec.get("timestamp") or "")[11:16]
            if t == "summary":
                out.append(f"## SUMMARY: {trunc(rec.get('summary',''), 300)}")
                continue
            if t not in ("user", "assistant"):
                continue
            msg = rec.get("message") or {}
            content = msg.get("content")
            if t == "user":
                if rec.get("isMeta"):
                    continue
                if isinstance(content, str):
                    n_user += 1
                    out.append(f"\n[{ts}] USER: {trunc(content, USER_CAP)}")
                    continue
                for b in content or []:
                    bt = b.get("type")
                    if bt == "text":
                        txt = b.get("text", "")
                        if txt.startswith("<local-command") or txt.startswith("<command-name>"):
                            out.append(f"\n[{ts}] USER-CMD: {trunc(txt, 200)}")
                        else:
                            n_user += 1
                            out.append(f"\n[{ts}] USER: {trunc(txt, USER_CAP)}")
                    elif bt == "tool_result":
                        c = b.get("content")
                        if isinstance(c, list):
                            c = " ".join(x.get("text", "") for x in c if isinstance(x, dict))
                        flag = " ERR" if b.get("is_error") else ""
                        out.append(f"      <- result{flag}: {trunc(c or '', RESULT_CAP)}")
            else:
                for b in content or []:
                    bt = b.get("type")
                    if bt == "text":
                        n_assist += 1
                        out.append(f"[{ts}] ASSISTANT: {trunc(b.get('text',''), ASSIST_CAP)}")
                    elif bt == "tool_use":
                        n_tool += 1
                        out.append(f"      -> {tool_summary(b)}")
    header = f"# {src}\n# user={n_user} assistant_text={n_assist} tool_calls={n_tool}\n"
    with open(dst, "w", encoding="utf-8") as fh:
        fh.write(header + "\n".join(out))
    print(f"{dst}: user={n_user} assist={n_assist} tools={n_tool} lines={len(out)}")


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])

package cli

import (
	"regexp"
	"strings"
)

// The HTML-open guard. On this machine a local .html file has VS Code as its
// default handler, so `Start-Process plan.html` opens an editor, not a page.
// The bare form ran 9 times in the week of 2026-11-16 and drew two complaints;
// a saved rule and a doc fix did not stop it. `kratos open <path>` picks the
// browser, so the gate sends every bare open of a local HTML file there.

// htmlOpenBrowsers are the program names that open a page in a browser.
var htmlOpenBrowsers = map[string]bool{
	"msedge": true, "chrome": true, "firefox": true, "brave": true,
	"chromium": true, "google-chrome": true, "microsoft-edge": true,
}

// htmlOpenShellHeads are the shells whose -Command argument is a script to scan.
var htmlOpenShellHeads = map[string]bool{"powershell": true, "pwsh": true}

// Start-Process switches that take no value; every other -Name takes one.
var htmlOpenSwitches = map[string]bool{
	"wait": true, "passthru": true, "nonewwindow": true, "usenewenvironment": true, "loaduserprofile": true,
}

var htmlOpenCmdFlagRE = regexp.MustCompile(`^/[A-Za-z]+$`)

const htmlOpenMaxDepth = 3

// htmlOpenGuardDecision denies a segment that opens a local HTML file with the
// system default handler. ok is false when no segment does.
func htmlOpenGuardDecision(command string) (editGateResult, bool) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return editGateResult{}, false
	}
	if target, ok := htmlOpenInCommand(trimmed, 0); ok {
		return editGateResult{Decision: "deny", Reason: "HTML opens in VS Code on this machine. Run " + kratosBinPath() + " open " + target + "."}, true
	}
	return editGateResult{}, false
}

func htmlOpenInCommand(command string, depth int) (string, bool) {
	for _, seg := range splitShellSegments(command) {
		if target, ok := htmlOpenInTokens(htmlOpenFields(gateEffectiveSegmentText(strings.TrimSpace(seg.raw))), depth); ok {
			return target, true
		}
	}
	return "", false
}

// htmlOpenFields splits on whitespace outside quotes and drops the quote
// characters, so a path with spaces stays one token and `""` stays one empty
// token.
func htmlOpenFields(s string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	has := false
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, has = r, true
		case r == ' ' || r == '\t' || r == '\r' || r == '\n':
			if has || cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
		}
	}
	if has || cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func htmlOpenProgramName(tok string) string {
	p := gateSlashPath(tok)
	p = strings.ToLower(p[strings.LastIndex(p, "/")+1:])
	return strings.TrimSuffix(p, ".exe")
}

// htmlOpenIsTarget reports a local HTML path or a file: URL. A web URL is not
// local and opens in the browser anyway.
func htmlOpenIsTarget(tok string) bool {
	l := strings.ToLower(strings.TrimRight(strings.TrimSpace(tok), ")"))
	if strings.HasPrefix(l, "file:") {
		return true
	}
	if strings.Contains(l, "://") {
		return false
	}
	if i := strings.IndexAny(l, "#?"); i >= 0 {
		l = l[:i]
	}
	return strings.HasSuffix(l, ".html") || strings.HasSuffix(l, ".htm")
}

func htmlOpenInTokens(tokens []string, depth int) (string, bool) {
	if len(tokens) == 0 || depth > htmlOpenMaxDepth {
		return "", false
	}
	head := htmlOpenProgramName(tokens[0])
	rest := tokens[1:]
	switch {
	case head == "start-process" || head == "saps" || head == "start":
		return htmlOpenStart(rest)
	case head == "invoke-item" || head == "ii" || head == "explorer" || head == "xdg-open":
		for _, t := range rest {
			if htmlOpenIsTarget(t) {
				return t, true
			}
		}
	case head == "cmd":
		for i, t := range rest {
			if lt := strings.ToLower(t); lt == "/c" || lt == "//c" || lt == "/k" || lt == "//k" {
				return htmlOpenInTokens(rest[i+1:], depth+1)
			}
		}
	case htmlOpenShellHeads[head]:
		for i, t := range rest {
			if lt := strings.ToLower(t); lt == "-command" || lt == "-c" || lt == "/command" {
				return htmlOpenInCommand(strings.Join(rest[i+1:], " "), depth+1)
			}
		}
	}
	return "", false
}

// htmlOpenStart reads Start-Process / start operands. The program is the
// -FilePath value or the first positional; it is a hit only when that program
// is itself an HTML target, never when a browser is named.
func htmlOpenStart(args []string) (string, bool) {
	for i := 0; i < len(args); i++ {
		t := args[i]
		if t == "" || htmlOpenCmdFlagRE.MatchString(t) {
			continue
		}
		if strings.HasPrefix(t, "-") {
			name, val, hasVal := strings.Cut(strings.ToLower(strings.TrimLeft(t, "-")), ":")
			if len(name) >= 2 && strings.HasPrefix("filepath", name) {
				if !hasVal && i+1 < len(args) {
					i++
					val = args[i]
				}
				return htmlOpenProgram(val)
			}
			if !htmlOpenSwitches[name] && !hasVal {
				i++
			}
			continue
		}
		return htmlOpenProgram(t)
	}
	return "", false
}

func htmlOpenProgram(program string) (string, bool) {
	if htmlOpenBrowsers[htmlOpenProgramName(program)] {
		return "", false
	}
	if htmlOpenIsTarget(program) {
		return program, true
	}
	return "", false
}

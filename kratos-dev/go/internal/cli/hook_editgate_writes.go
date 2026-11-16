package cli

import (
	"path"
	"regexp"
	"strings"
)

// Odysseus shell gate: deny only known writes.
//
// The gate used to allow only commands it could prove read-only. That list was
// patched in seven releases (v2.118 to v2.127) and the weekly denial count did
// not fall (27 to 28), because every new read idiom — a `for` loop, a `$(…)`
// capture, `head -c $((o+700))` — was a new false deny. The rule now runs the
// other way: a command runs unless one of its segments is a known write. The
// segment walk looks inside `for … do … done`, `$(…)`, backticks, braces and
// `&&`/`;` chains, so a write in a loop body or a substitution still denies.

const odysseusDocRedirectDenyReason = "A shell redirect or heredoc into a plan, flow or spec-delta file is denied. Use the Write or Edit tool for this file."

type shellWriteKind int

const (
	shellNoWrite shellWriteKind = iota
	shellWriteFound
	shellDocRedirect
)

var (
	// gateHeredocRE matches the `<<DELIM` opener of a here-document.
	gateHeredocRE = regexp.MustCompile(`<<-?[ \t]*(?:'([A-Za-z_][A-Za-z0-9_]*)'|"([A-Za-z_][A-Za-z0-9_]*)"|\\?([A-Za-z_][A-Za-z0-9_]*))`)
	// gateRedirectTargetRE finds a `>` redirect on a quote-masked segment and
	// captures its target. `->` and `=>` are not redirects.
	gateRedirectTargetRE = regexp.MustCompile(`(?:^|[^-=<>])(?:\d|&)?>>?\|?\s*([^\s;&|<>()]*)`)
	// gateDocTargetRE names the files Odysseus writes with the Write or Edit tool.
	gateDocTargetRE = regexp.MustCompile(`(?i)(?:tactical-plans/|\.Arena/flows/|/spec-delta/|^spec-delta/)`)
	// gateLeadingKeywordRE strips shell and PowerShell syntax that sits in front
	// of the command a segment runs.
	gateLeadingKeywordRE = regexp.MustCompile(`^(?:\(|!|(?:do|then|else|elif|if|while|until|time)\b|\$[A-Za-z_]\w*\s*[+-]?=\s*)\s*`)
)

var (
	// gateControlHeadRE spots a segment that opens a control construct, so the
	// bare variable in the next segment is its condition or loop variable.
	gateControlHeadRE = regexp.MustCompile(`(?i)^\s*(?:if|elseif|while|until|foreach|for|switch|return)\b`)
	// gateVarHeadRE is a bare variable in command position.
	gateVarHeadRE = regexp.MustCompile(`^\$(?:[A-Za-z]\w*|\{[^}]*\})$`)
	// gatePSOperatorRE is the token after a PowerShell variable in a condition
	// or loop header, `$x -eq 1` or `$f in $list`.
	gatePSOperatorRE = regexp.MustCompile(`(?i)^(?:-(?:eq|ne|gt|ge|lt|le|like|notlike|match|notmatch|contains|notcontains|in|notin|and|or|is|isnot|as|replace|split|join|f|not)|in|[=+-]?=)$`)
	// gatePSMethodWriteRE spots a .NET call that deletes, moves or writes.
	gatePSMethodWriteRE = regexp.MustCompile(`(?i)\.(?:Delete|Kill|MoveTo|CopyTo|Remove|Save)\(|::(?:Open\w*)\(|::(?:Write\w*|Delete|Move|Copy|Create\w*|Append\w*|Set\w*)\(`)
	// gateFakeNullRE spots `> nul.txt` or `> /dev/null.bak`: a file whose name
	// only begins like the null device, which gateInertRedirectRE strips.
	gateFakeNullRE = regexp.MustCompile(`(?i)>\s*(?:/dev/null|nul)[A-Za-z0-9_./-]`)
	// gateVarRefRE spots a variable reference left in a segment's arguments.
	gateVarRefRE = regexp.MustCompile(`\$[A-Za-z_{]`)
	// gateFileCompileRE spots `file -C`, which writes a compiled magic file.
	gateFileCompileRE = regexp.MustCompile(`(?:^|\s)(?:-[A-Za-z]*C[A-Za-z]*|--compile)(?:\s|$)`)
	// gateHijackVars are variables whose assignment changes which program or
	// script later commands run.
	gateHijackVars = map[string]bool{
		"PATH": true, "PATHEXT": true, "LD_PRELOAD": true, "LD_LIBRARY_PATH": true, "IFS": true, "BASH_ENV": true,
		"ENV": true, "NODE_OPTIONS": true, "GIT_SSH_COMMAND": true, "GIT_EXTERNAL_DIFF": true, "GIT_PAGER": true,
		"PAGER": true, "PSMODULEPATH": true,
	}
	// gateVarGuardedHeads take flags that write or run code, so an unresolved
	// variable among their arguments may be one.
	gateVarGuardedHeads = map[string]bool{
		"sed": true, "find": true, "fd": true, "fdfind": true, "tail": true, "awk": true, "sort": true,
		"uniq": true, "date": true, "printf": true, "file": true,
	}
)

// hasVarFlagOrOperand reports an argument that is a variable, or a flag that
// holds one: the value could be a flag that writes. A variable inside a longer
// word, such as a sed script `s|^|$f: |`, is text.
func hasVarFlagOrOperand(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "$") || (strings.HasPrefix(a, "-") && gateVarRefRE.MatchString(a)) {
			return true
		}
	}
	return false
}

// leadingAssignmentHijacks reports `PATH=x cmd` or a bare `PATH=x`.
func leadingAssignmentHijacks(body string) bool {
	rest := body
	for {
		m := gateLeadingAssignmentRE.FindString(rest)
		if m == "" {
			return false
		}
		if name, _, ok := strings.Cut(strings.TrimSpace(m), "="); ok && gateHijackVars[strings.ToUpper(name)] {
			return true
		}
		rest = rest[len(m):]
	}
}

// sedVarOperandOK accepts `sed [flags] <script> "$f"`: the variable is the last
// operand, a literal script word precedes it, and no -e/-f flag lets the
// variable be the script.
func sedVarOperandOK(tokens []string) bool {
	if len(tokens) < 3 || len(gateVarRefRE.FindAllString(strings.Join(tokens, " "), -1)) != 1 || !gateVarRefRE.MatchString(tokens[len(tokens)-1]) {
		return false
	}
	words := 0
	for _, t := range tokens[1 : len(tokens)-1] {
		switch {
		case strings.HasPrefix(t, "--"):
			if strings.HasPrefix(t, "--expression") || strings.HasPrefix(t, "--file") {
				return false
			}
		case strings.HasPrefix(t, "-") && len(t) > 1:
			if strings.ContainsAny(t[1:], "ef") {
				return false
			}
		default:
			words++
		}
	}
	return words == 1
}

// gateWriteMutatorHeads are programs that change files, processes or services
// with no further argument test.
var gateWriteMutatorHeads = map[string]bool{
	"rm": true, "rmdir": true, "mv": true, "cp": true, "mkdir": true, "touch": true, "ln": true,
	"chmod": true, "chown": true, "dd": true, "truncate": true, "shred": true, "install": true,
	"del": true, "erase": true, "ren": true, "rename": true, "rd": true, "md": true, "move": true, "copy": true,
	"remove-item": true, "move-item": true, "copy-item": true, "new-item": true, "rename-item": true,
	"clear-content": true, "set-itemproperty": true, "set-item": true, "new-itemproperty": true,
	"remove-itemproperty": true, "ri": true, "mi": true, "cpi": true, "ni": true,
	"stop-process": true, "kill": true, "pkill": true, "killall": true, "taskkill": true,
	"stop-service": true, "start-service": true, "restart-service": true,
	"restart-computer": true, "stop-computer": true, "shutdown": true,
	"wget": true, "make": true, "eval": true, "source": true, "trap": true, ".": true,
	"export-csv": true, "export-clixml": true, "compress-archive": true, "expand-archive": true, "rni": true,
	"unblock-file": true, "set-executionpolicy": true, "sc": true, "clc": true, "xcopy": true, "robocopy": true,
	"attrib": true, "mklink": true, "icacls": true, "setx": true, "npx": true, "bunx": true, "pnpx": true,
	"iex": true, "invoke-expression": true, "invoke-command": true, "icm": true, "start-job": true,
}

// gateWriteTargetHeads write to the files named as operands; an operand that is
// the null device or the session scratchpad is bookkeeping.
var gateWriteTargetHeads = map[string]bool{
	"tee": true, "out-file": true, "set-content": true, "add-content": true, "tee-object": true,
}

// gateInterpreterHeads run inline or script code. Only a version query passes.
var gateInterpreterHeads = map[string]bool{
	"python": true, "python2": true, "python3": true, "py": true, "pythonw": true, "pypy": true, "pypy3": true,
	"node": true, "nodejs": true, "deno": true, "ruby": true, "perl": true, "php": true, "lua": true,
	"pwsh": true, "powershell": true, "bash": true, "sh": true, "zsh": true, "fish": true, "dash": true,
	"wsl":  true,
	"psql": true, "sqlcmd": true, "mysql": true, "mariadb": true, "sqlite3": true, "mongosh": true,
	"mongo": true, "redis-cli": true, "bcp": true, "invoke-sqlcmd": true,
}

var gateOpenHeads = map[string]bool{
	"start-process": true, "start": true, "open": true, "explorer": true, "xdg-open": true,
	"invoke-item": true, "ii": true, "saps": true,
}

var gateVersionOnlyArgs = map[string]bool{"--version": true, "-v": true, "-V": true, "-VV": true, "--help": true, "-h": true}

var (
	gatePkgHeads = map[string]bool{"npm": true, "pnpm": true, "yarn": true, "bun": true}
	gatePkgVerbs = map[string]bool{
		"install": true, "i": true, "add": true, "run": true, "test": true, "t": true, "ci": true,
		"build": true, "exec": true, "dlx": true, "x": true, "remove": true, "rm": true, "uninstall": true,
		"update": true, "up": true, "upgrade": true, "publish": true, "link": true, "start": true, "create": true,
	}
	gateToolVerbs = map[string]map[string]bool{
		"dotnet":  {"build": true, "test": true, "run": true, "ef": true, "publish": true, "restore": true, "new": true, "add": true, "remove": true, "tool": true, "clean": true, "format": true, "watch": true, "pack": true, "nuget": true},
		"cargo":   {"build": true, "test": true, "run": true, "install": true, "bench": true, "clean": true, "fmt": true, "add": true, "remove": true, "update": true, "publish": true, "check": true, "clippy": true},
		"go":      {"build": true, "test": true, "run": true, "install": true, "generate": true, "get": true, "mod": true, "fmt": true, "clean": true, "work": true},
		"pip":     {"install": true, "uninstall": true, "download": true},
		"pip3":    {"install": true, "uninstall": true, "download": true},
		"pipx":    {"install": true, "uninstall": true, "upgrade": true, "run": true},
		"uv":      {"pip": true, "add": true, "remove": true, "sync": true, "run": true, "tool": true},
		"poetry":  {"install": true, "add": true, "remove": true, "update": true, "run": true, "build": true, "publish": true},
		"winget":  {"install": true, "uninstall": true, "upgrade": true},
		"choco":   {"install": true, "uninstall": true, "upgrade": true},
		"scoop":   {"install": true, "uninstall": true, "update": true},
		"apt":     {"install": true, "remove": true, "upgrade": true, "purge": true},
		"apt-get": {"install": true, "remove": true, "upgrade": true, "purge": true},
		"brew":    {"install": true, "uninstall": true, "upgrade": true},
	}
	gateGitWriteSubs = map[string]bool{
		"commit": true, "add": true, "push": true, "merge": true, "rebase": true, "cherry-pick": true,
		"reset": true, "checkout": true, "switch": true, "clean": true, "restore": true, "apply": true,
		"am": true, "revert": true, "pull": true, "rm": true, "mv": true, "init": true, "clone": true,
		"update-ref": true, "gc": true, "prune": true, "repack": true, "maintenance": true, "bisect": true,
		"format-patch": true, "sparse-checkout": true, "fetch": true, "update-index": true,
		"filter-branch": true, "checkout-index": true, "hash-object": true, "symbolic-ref": true,
	}
	gateGitBranchWriteFlags = map[string]bool{
		"-d": true, "-D": true, "-m": true, "-M": true, "-c": true, "-C": true, "-f": true,
		"--delete": true, "--move": true, "--copy": true, "--force": true, "-u": true,
		"--set-upstream-to": true, "--unset-upstream": true, "--edit-description": true,
	}
	gateGitSubSubWrites = map[string]map[string]bool{
		"worktree":  {"add": true, "remove": true, "move": true, "prune": true, "lock": true, "unlock": true, "repair": true},
		"remote":    {"add": true, "remove": true, "rm": true, "rename": true, "set-url": true, "prune": true, "set-head": true},
		"submodule": {"update": true, "add": true, "init": true, "deinit": true, "sync": true},
		"notes":     {"add": true, "append": true, "edit": true, "remove": true, "copy": true},
		"reflog":    {"expire": true, "delete": true},
	}
	gateGitBranchListFlags = map[string]bool{
		"-l": true, "--list": true, "--show-current": true, "-a": true, "--all": true, "-r": true, "--remotes": true,
		"-v": true, "-vv": true, "--verbose": true, "--merged": true, "--no-merged": true, "--contains": true,
		"--no-contains": true, "--points-at": true,
	}
	gateGitOptionWithValue = map[string]bool{"-C": true, "-c": true, "--git-dir": true, "--work-tree": true, "--namespace": true}
	gateCurlWriteVerbRE    = regexp.MustCompile(`(?i)^(?:POST|PUT|DELETE|PATCH)$`)
	gateRestWriteMethodRE  = regexp.MustCompile(`(?i)^(?:POST|PUT|DELETE|PATCH)$`)
	gateBrowserValueFlags  = map[string]bool{"--session": true, "--profile": true, "--executable-path": true, "--headers": true, "--args": true, "--config": true, "--proxy": true, "--cdp": true, "--user-agent": true}
	gateBrowserReadVerbs   = map[string]bool{"snapshot": true, "get": true, "eval": true, "screenshot": true}
	gateGhWriteVerbs       = map[string]bool{"create": true, "merge": true, "close": true, "edit": true, "comment": true, "review": true, "reopen": true, "delete": true, "rerun": true, "cancel": true, "fork": true, "clone": true}
	gateBrowserNames       = map[string]bool{"chrome": true, "msedge": true, "firefox": true, "brave": true}
)

// odysseusShellWriteKind classifies a shell command: no known write, a known
// write, or a redirect into a plan, flow or spec-delta file.
func odysseusShellWriteKind(command string) shellWriteKind {
	noBodies := stripHeredocBodies(command)
	if gateFakeNullRE.MatchString(maskQuoted(noBodies)) {
		return shellWriteFound
	}
	c := strings.TrimSpace(gateInertRedirectRE.ReplaceAllString(noBodies, " "))
	if c == "" {
		return shellNoWrite
	}
	if gatePSMethodWriteRE.MatchString(maskQuoted(c)) {
		return shellWriteFound
	}
	segs, flatRaw, flatMasked := flattenShellSegments(c)
	segs = expandFlagAssignments(flatMasked, segs)
	segs = substituteAssignments(flatRaw, segs)
	kind := shellNoWrite
	afterControl := false
	for _, seg := range segs {
		switch shellSegmentWrite(seg, 0, afterControl) {
		case shellDocRedirect:
			return shellDocRedirect
		case shellWriteFound:
			kind = shellWriteFound
		}
		if t := strings.TrimSpace(seg.raw); t != "" {
			afterControl = gateControlHeadRE.MatchString(t)
		}
	}
	return kind
}

// expandFlagAssignments expands a variable that holds a flag (`D=-delete;
// find . "$D"`) into the later segments, unquoted, so the flag guards read it.
// substituteAssignments skips such a value on purpose: quoted, it hides the flag
// from the quote mask.
func expandFlagAssignments(flatMasked string, segs []shellSegment) []shellSegment {
	out := segs
	for i := 0; i < len(out)-1; i++ {
		m := gateAssignmentRE.FindStringSubmatch(strings.TrimSpace(out[i].raw))
		if m == nil || !segmentSeparatorKeepsVar(flatMasked, out[i]) {
			continue
		}
		value := assignmentLiteral(m[2])
		if !strings.HasPrefix(value, "-") || strings.ContainsAny(value, "\"'`$ ") {
			continue
		}
		name := regexp.QuoteMeta(m[1])
		quotedRef := regexp.MustCompile(`"(?:\$\{` + name + `\}|\$` + name + `\b)"`)
		bareRef := regexp.MustCompile(`\$\{` + name + `\}|\$` + name + `\b`)
		next := make([]shellSegment, len(out))
		copy(next, out)
		for j := i + 1; j < len(next); j++ {
			expanded := bareRef.ReplaceAllLiteralString(quotedRef.ReplaceAllLiteralString(next[j].raw, value), value)
			if expanded != next[j].raw {
				next[j] = shellSegment{raw: expanded, masked: maskQuoted(expanded), end: next[j].end}
			}
		}
		out = next
	}
	return out
}

// isReadOnlyShellCommand reports whether command holds no known write.
func isReadOnlyShellCommand(command string) bool {
	return odysseusShellWriteKind(command) == shellNoWrite
}

// stripHeredocBodies drops here-document bodies: they are text for a program,
// not commands. The opener line stays, so `cat > file <<'EOF'` still shows its
// redirect and `python - <<'EOF'` still shows its interpreter.
func stripHeredocBodies(command string) string {
	if !strings.Contains(command, "<<") {
		return command
	}
	lines := strings.Split(command, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		out = append(out, line)
		masked := maskQuoted(line)
		at := -1
		for from := 0; from < len(masked); {
			j := strings.Index(masked[from:], "<<")
			if j < 0 {
				break
			}
			j += from
			if j+2 < len(masked) && masked[j+2] == '<' {
				from = j + 3
				continue
			}
			at = j
			break
		}
		if at < 0 {
			continue
		}
		m := gateHeredocRE.FindStringSubmatch(line[at:])
		if m == nil {
			continue
		}
		delim := firstNonEmpty(m[1], m[2], m[3])
		dash := strings.HasPrefix(m[0], "<<-")
		end := -1
		for j := i + 1; j < len(lines); j++ {
			l := strings.TrimRight(lines[j], "\r")
			if dash {
				l = strings.TrimLeft(l, "\t")
			}
			if l == delim {
				end = j
				break
			}
		}
		if end < 0 {
			continue
		}
		if m[3] != "" {
			expands := false
			for _, b := range lines[i+1 : end] {
				if strings.Contains(b, "$(") || strings.Contains(b, "`") {
					expands = true
				}
			}
			if expands {
				continue
			}
		}
		i = end
	}
	return strings.Join(out, "\n")
}

// flattenedBytes returns raw and masked copies of command in which every
// construct that opens or closes a nested command — `$(`, `<(`, `>(`, a
// parenthesis, a backtick, a brace — is a `;`, so the nested command is a
// segment of its own. A `${NAME}` reference stays whole. Single-quoted text
// keeps its mask, so a `)` inside it stays inert.
func flattenedBytes(command string) (raw, masked []byte) {
	raw = []byte(command)
	masked = []byte(maskQuoted(command))
	for i := 0; i < len(masked); i++ {
		switch masked[i] {
		case '`', '}', ')':
			raw[i], masked[i] = ';', ';'
		case '(':
			raw[i], masked[i] = ';', ';'
			if i > 0 && (masked[i-1] == '$' || masked[i-1] == '<' || masked[i-1] == '>') {
				raw[i-1], masked[i-1] = ';', ';'
			}
		case '{':
			raw[i], masked[i] = ';', ';'
		case '$':
			if i+1 < len(masked) && masked[i+1] == '{' {
				if end := strings.IndexByte(string(masked[i:]), '}'); end > 0 && !strings.Contains(string(masked[i:i+end]), "$(") && !strings.ContainsRune(string(masked[i:i+end]), '`') {
					i += end
				}
			}
		}
	}
	return raw, masked
}

// flattenShellSegments splits command into the commands a shell would run,
// nested ones included. It also returns the flattened raw and masked text the
// segment offsets refer to.
func flattenShellSegments(command string) (segs []shellSegment, flatRaw, flatMasked string) {
	raw, masked := flattenedBytes(command)
	rs, ms := string(raw), string(masked)
	start := 0
	for _, loc := range gateSegmentSplitRE.FindAllStringIndex(ms, -1) {
		if loc[0] > start {
			segs = append(segs, shellSegment{raw: rs[start:loc[0]], masked: ms[start:loc[0]], end: loc[0]})
		}
		start = loc[1]
	}
	if start < len(rs) {
		segs = append(segs, shellSegment{raw: rs[start:], masked: ms[start:], end: len(rs)})
	}
	return segs, rs, ms
}

// gateWriteTargetAllowed reports a redirect target that creates no project
// file: the null device, a standard stream, or the session scratchpad.
func gateWriteTargetAllowed(t string) bool {
	t = strings.Trim(strings.TrimSpace(t), `"'`)
	if t == "" {
		return false
	}
	switch strings.ToLower(t) {
	case "/dev/null", "nul", "$null", "/dev/stderr", "/dev/stdout":
		return true
	}
	return gateTempClaudeRE.MatchString(strings.ToLower(path.Clean(gateSlashPath(t))))
}

func shellSegmentWrite(seg shellSegment, depth int, afterControl bool) shellWriteKind {
	raw := strings.TrimSpace(seg.raw)
	masked := strings.TrimSpace(seg.masked)
	if raw == "" {
		return shellNoWrite
	}
	kind := shellNoWrite
	for _, m := range gateRedirectTargetRE.FindAllStringSubmatchIndex(masked, -1) {
		target := ""
		if m[2] >= 0 {
			target = raw[m[2]:m[3]]
		}
		if gateWriteTargetAllowed(target) {
			continue
		}
		if gateDocTargetRE.MatchString(gateSlashPath(target)) {
			return shellDocRedirect
		}
		kind = shellWriteFound
	}
	body, mbody := raw, masked
	for {
		loc := gateLeadingKeywordRE.FindStringIndex(body)
		if loc == nil || loc[1] == 0 {
			break
		}
		body, mbody = strings.TrimSpace(body[loc[1]:]), strings.TrimSpace(mbody[loc[1]:])
	}
	if kind == shellNoWrite && segmentCommandWrites(body, mbody, depth, afterControl) {
		kind = shellWriteFound
	}
	return kind
}

// segmentCommandWrites reports a known write in the command a segment runs.
func segmentCommandWrites(body, masked string, depth int, afterControl bool) bool {
	if leadingAssignmentHijacks(body) {
		return true
	}
	head := gateSegmentHead(body)
	switch head {
	case "", "for", "foreach", "select", "case", "switch", "done", "fi", "esac", "function", "in":
		return false
	}
	if isGodLoaderCommand(body) {
		return false
	}
	tokens := gateSegmentTokens(body)
	if len(tokens) == 0 {
		return false
	}
	args := tokens[1:]
	if gateVarHeadRE.MatchString(head) {
		if len(args) == 0 {
			return !afterControl
		}
		return !gatePSOperatorRE.MatchString(args[0])
	}
	if gateVarGuardedHeads[head] && hasVarFlagOrOperand(strings.Fields(strings.ReplaceAll(masked, `"`, ""))) {
		return !(head == "sed" && sedVarOperandOK(tokens) && !gateInPlaceFlagRE.MatchString(masked))
	}
	if (head == "export" || head == "set" || head == "declare" || head == "setenv") && leadingAssignmentHijacks(strings.Join(args, " ")) {
		return true
	}
	if gateWriteMutatorHeads[head] {
		return true
	}
	if gateWriteTargetHeads[head] {
		return !allOperandsAllowedTargets(args)
	}
	if gateInterpreterHeads[head] {
		return !versionQueryOnly(args)
	}
	if gateOpenHeads[head] || head == "cmd" {
		return !(gateOpensPlanPage(body) || opensPlanPageWithBrowser(args))
	}
	switch head {
	case "kratos":
		return !(gateKratosReadOnlySubcommandRE.MatchString(strings.Join(args, " ")) || gateOpensPlanPage(body))
	case "git":
		return gitWrites(args, masked)
	case "sed":
		return gateInPlaceFlagRE.MatchString(masked)
	case "find":
		return gateFindActionRE.MatchString(masked)
	case "fd", "fdfind":
		return gateFdExecRE.MatchString(masked)
	case "tail":
		return gateFollowFlagRE.MatchString(masked)
	case "file":
		return gateFileCompileRE.MatchString(masked)
	case "awk":
		return !gateAwkIsFilter(body)
	case "date":
		return gateDateSetsClock(tokens)
	case "sort":
		return gateSortWritesFile(tokens)
	case "uniq":
		return gateUniqWritesFile(tokens)
	case "printf":
		return gatePrintfAssignsVar(tokens)
	case "curl":
		return curlWrites(args)
	case "invoke-restmethod", "irm", "invoke-webrequest", "iwr":
		return restWrites(args)
	case "agent-browser":
		return browserWrites(args)
	case "gh":
		return ghWrites(args)
	case "xargs", "nohup", "exec", "timeout", "watch":
		if rest := peelRunner(head, args); len(rest) > 0 && depth < 4 {
			joined := strings.Join(rest, " ")
			return shellSegmentWrite(shellSegment{raw: joined, masked: maskQuoted(joined)}, depth+1, false) != shellNoWrite
		}
		return false
	}
	if gatePkgHeads[head] {
		return pkgWrites(head, args)
	}
	if verbs, ok := gateToolVerbs[head]; ok {
		return toolWrites(head, verbs, args)
	}
	return false
}

func operands(args []string) []string {
	var out []string
	for _, a := range args {
		switch {
		case a == "":
		case strings.HasPrefix(a, "-"):
			if _, v, ok := strings.Cut(a, ":"); ok && v != "" {
				out = append(out, v)
			}
		default:
			out = append(out, a)
		}
	}
	return out
}

func allOperandsAllowedTargets(args []string) bool {
	ops := operands(args)
	if len(ops) == 0 {
		return true
	}
	for _, o := range ops {
		if !gateWriteTargetAllowed(o) {
			return false
		}
	}
	return true
}

func versionQueryOnly(args []string) bool {
	if len(args) == 0 {
		return false
	}
	for _, a := range args {
		if !gateVersionOnlyArgs[a] {
			return false
		}
	}
	return true
}

// gatePlanPageOperand reports an operand that names the plan's page, as a path
// or as a file:// URL.
func gatePlanPageOperand(op string) bool {
	u := gateSlashPath(strings.Trim(op, `"'`))
	u = strings.TrimPrefix(strings.TrimPrefix(u, "file:///"), "file://")
	if strings.Contains(u, "://") {
		return false
	}
	p := path.Clean(u)
	return strings.HasSuffix(p, ".html") && strings.Contains(p, gatePlanRoot()+"/")
}

// opensPlanPageWithBrowser accepts `Start-Process $browser <plan page>`: one
// plan-page operand, every other operand a variable or a browser program.
func opensPlanPageWithBrowser(args []string) bool {
	page := false
	for _, op := range operands(args) {
		switch {
		case gatePlanPageOperand(op):
			if page {
				return false
			}
			page = true
		case strings.HasPrefix(op, "$"):
		default:
			base := strings.ToLower(gateSlashPath(op))
			base = strings.TrimSuffix(base[strings.LastIndex(base, "/")+1:], ".exe")
			if !gateBrowserNames[base] {
				return false
			}
		}
	}
	return page
}

func gitWrites(args []string, masked string) bool {
	i := 0
	for i < len(args) && strings.HasPrefix(args[i], "-") {
		if gateGitOptionWithValue[args[i]] {
			i++
		}
		i++
	}
	if i >= len(args) {
		return false
	}
	sub := strings.ToLower(args[i])
	rest := args[i+1:]
	switch {
	case gateGitWriteSubs[sub]:
		return true
	case sub == "stash":
		if len(rest) > 0 && rest[0] == "show" {
			return gateGitOutputRE.MatchString(masked)
		}
		return len(rest) == 0 || rest[0] != "list"
	case sub == "tag":
		for _, r := range rest {
			if r == "-l" || r == "--list" || strings.HasPrefix(r, "-n") || r == "--contains" || r == "--points-at" {
				return false
			}
		}
		for _, r := range rest {
			switch r {
			case "-d", "-a", "-f", "-s", "-m", "-F", "--delete", "--force", "--annotate", "--sign":
				return true
			}
		}
		return len(operands(rest)) > 0
	case sub == "branch":
		listing := false
		for _, r := range rest {
			if gateGitBranchWriteFlags[r] {
				return true
			}
			if gateGitBranchListFlags[r] {
				listing = true
			}
		}
		return !listing && len(operands(rest)) > 0
	case sub == "config":
		for _, r := range rest {
			switch r {
			case "--unset", "--unset-all", "--add", "--replace-all", "--edit", "-e", "--rename-section", "--remove-section":
				return true
			}
		}
		return len(operands(rest)) >= 2
	case sub == "grep":
		return gateGitGrepPagerRE.MatchString(masked)
	case sub == "ls-remote":
		return gateGitUploadPackRE.MatchString(masked)
	}
	if subs, ok := gateGitSubSubWrites[sub]; ok && len(operands(rest)) > 0 {
		return subs[strings.ToLower(operands(rest)[0])]
	}
	return gateGitOutputRE.MatchString(masked)
}

func pkgWrites(head string, args []string) bool {
	ops := operands(args)
	if len(ops) == 0 {
		return head == "yarn"
	}
	if head == "yarn" && !gatePkgVerbs[strings.ToLower(ops[0])] {
		switch strings.ToLower(ops[0]) {
		case "list", "info", "why", "outdated", "config", "workspaces", "version":
			return false
		}
		return true
	}
	return gatePkgVerbs[strings.ToLower(ops[0])]
}

func toolWrites(head string, verbs map[string]bool, args []string) bool {
	ops := operands(args)
	if len(ops) == 0 {
		return false
	}
	verb := strings.ToLower(ops[0])
	if head == "go" && verb == "mod" {
		return len(ops) > 1 && ops[1] != "graph" && ops[1] != "why" && ops[1] != "verify"
	}
	return verbs[verb]
}

func curlWrites(args []string) bool {
	for i, a := range args {
		switch {
		case a == "-X" || strings.EqualFold(a, "--request"):
			if i+1 < len(args) && gateCurlWriteVerbRE.MatchString(args[i+1]) {
				return true
			}
		case strings.HasPrefix(a, "-X") && len(a) > 2 && gateCurlWriteVerbRE.MatchString(a[2:]):
			return true
		case strings.HasPrefix(a, "--") && (strings.HasPrefix(a, "--data") || a == "--form" || a == "--upload-file" || a == "--output" || a == "--json" || a == "--remote-name"):
			return true
		case strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsAny(a, "dFToO"):
			return true
		}
	}
	return false
}

func restWrites(args []string) bool {
	for i, a := range args {
		switch strings.ToLower(a) {
		case "-method":
			if i+1 < len(args) && gateRestWriteMethodRE.MatchString(args[i+1]) {
				return true
			}
		case "-body", "-infile", "-outfile":
			return true
		}
	}
	return false
}

// browserWrites reports an agent-browser verb other than the read verbs. A
// verb that clicks, types or navigates drives the page.
func browserWrites(args []string) bool {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			if gateBrowserValueFlags[a] {
				i++
			}
			continue
		}
		verb := strings.ToLower(a)
		if gateBrowserReadVerbs[verb] {
			return false
		}
		if verb == "session" {
			return !(i+1 < len(args) && strings.EqualFold(args[i+1], "list"))
		}
		return true
	}
	return false
}

func ghWrites(args []string) bool {
	ops := operands(args)
	if len(ops) == 0 {
		return false
	}
	if ops[0] == "api" {
		for _, a := range args {
			switch strings.ToLower(a) {
			case "-x", "--method", "-f", "--field", "--raw-field", "--input":
				return true
			}
		}
		return false
	}
	if ops[0] == "auth" {
		return len(ops) < 2 || ops[1] != "status"
	}
	if len(ops) >= 2 {
		switch ops[0] {
		case "pr", "issue", "release", "repo", "run", "workflow":
			return gateGhWriteVerbs[ops[1]]
		}
	}
	return false
}

// peelRunner returns the command a runner head hands its arguments to.
func peelRunner(head string, args []string) []string {
	valueFlags := map[string]bool{"-I": true, "-n": true, "-P": true, "-d": true, "-L": true, "-s": true, "-E": true, "-a": true}
	i := 0
	for i < len(args) && strings.HasPrefix(args[i], "-") {
		if head == "xargs" && valueFlags[args[i]] {
			i++
		}
		i++
	}
	if head == "timeout" && i < len(args) {
		i++
	}
	if i > len(args) {
		return nil
	}
	return args[i:]
}

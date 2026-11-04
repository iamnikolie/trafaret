package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const settingsFile = ".claude/settings.json"

// hookPrefix identifies the one settings entry trafaret owns: a command that
// prints a file under the project's .claude/skills/.
const hookPrefix = `cat "$CLAUDE_PROJECT_DIR/.claude/skills/`

func hookCommand(ctx string) string { return hookPrefix + ctx + `"` }

// ensureSessionHook additively merges, updates or removes trafaret's SessionStart
// hook in the project's .claude/settings.json, preserving every other key. ctx is
// the session-context file under .claude/skills/ ("" removes the hook). Returns
// the resulting FileStatus ("install"/"update"/"remove") or "" when nothing changed.
func ensureSessionHook(repoRoot, ctx string, dryRun bool) (string, error) {
	path := filepath.Join(repoRoot, filepath.FromSlash(settingsFile))
	existing, err := os.ReadFile(path)
	existed := err == nil
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	out, status, err := planSessionHook(existing, existed, ctx)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	if status == "" || dryRun {
		return status, nil
	}
	if out == nil { // nothing but the hook was in the file
		return status, os.Remove(path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return status, os.WriteFile(path, out, 0o644)
}

// planSessionHook is the pure core of ensureSessionHook: given the current
// content of .claude/settings.json (existed says whether the file is there at
// all) it returns the content the file should have, and the status. A nil
// content with status "remove" means the file held nothing else and goes away.
func planSessionHook(existing []byte, existed bool, ctx string) (out []byte, status string, err error) {
	enable := ctx != ""
	settings := map[string]any{}
	if existed {
		if err := json.Unmarshal(existing, &settings); err != nil {
			return nil, "", fmt.Errorf("not valid JSON, refusing to manage the session hook: %w", err)
		}
	}

	hooks, _ := settings["hooks"].(map[string]any)
	starts, _ := hooks["SessionStart"].([]any)
	idx, current := -1, ""
	for i, g := range starts {
		gm, ok := g.(map[string]any)
		if !ok {
			continue
		}
		if inner, ok := gm["hooks"].([]any); ok {
			for _, h := range inner {
				if hm, ok := h.(map[string]any); ok {
					if cmd, _ := hm["command"].(string); strings.HasPrefix(cmd, hookPrefix) {
						idx, current = i, cmd
					}
				}
			}
		}
	}

	switch {
	case enable && current == hookCommand(ctx), !enable && idx < 0:
		return nil, "", nil // already in the desired state
	case enable:
		entry := map[string]any{
			"matcher": "startup|clear|compact",
			"hooks": []any{map[string]any{
				"type":    "command",
				"command": hookCommand(ctx),
			}},
		}
		if idx >= 0 {
			starts[idx] = entry // another bundle's context took over
		} else {
			starts = append(starts, entry)
		}
		if hooks == nil {
			hooks = map[string]any{}
		}
		hooks["SessionStart"] = starts
		settings["hooks"] = hooks
		status = "update"
		if !existed {
			status = "install"
		}
	default: // disable
		starts = append(starts[:idx], starts[idx+1:]...)
		if len(starts) == 0 {
			delete(hooks, "SessionStart")
		} else {
			hooks["SessionStart"] = starts
		}
		if len(hooks) == 0 {
			delete(settings, "hooks")
		} else {
			settings["hooks"] = hooks
		}
		if len(settings) == 0 {
			return nil, "remove", nil
		}
		status = "update"
	}
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, "", err
	}
	return append(b, '\n'), status, nil
}

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
	enable := ctx != ""
	path := filepath.Join(repoRoot, filepath.FromSlash(settingsFile))
	settings := map[string]any{}
	existed := false
	if b, err := os.ReadFile(path); err == nil {
		existed = true
		if err := json.Unmarshal(b, &settings); err != nil {
			return "", fmt.Errorf("%s: not valid JSON, refusing to manage the session hook: %w", path, err)
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
		return "", nil // already in the desired state
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
		status := "update"
		if !existed {
			status = "install"
		}
		if !dryRun {
			if err := writeSettings(path, settings); err != nil {
				return "", err
			}
		}
		return status, nil
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
		if !dryRun {
			if len(settings) == 0 {
				if err := os.Remove(path); err != nil {
					return "", err
				}
				return "remove", nil
			}
			if err := writeSettings(path, settings); err != nil {
				return "", err
			}
		} else if len(settings) == 0 {
			return "remove", nil
		}
		return "update", nil
	}
}

func writeSettings(path string, settings map[string]any) error {
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

package site

import "html/template"

// h is a shorthand: the string maps hold trusted, repo-authored markup.
type h = template.HTML

var siteEN = map[string]h{
	"title": "operator manual",

	"nav_model": "Overview", "nav_state": "Live state", "nav_catalog": "Catalog",
	"nav_guard": "Guardrails", "nav_howto": "How-to", "nav_where": "Where do I edit…",

	"hero_h1": "trafaret — operator manual",
	"hero_p": `One source tree of skills, subagents and rules, rendered with each target's <code>vars</code> into Claude Code,
Codex and Cursor — for repos (<em>projects</em>) and for user-level <em>homes</em>. This page is rendered
from the config directory — <code>manifest.yaml</code> + <code>templates/</code> + <code>global/</code> +
<code>agents/</code> + <code>projects/</code> + the embedded examples — on <em>every refresh</em>: an agent's
edit to those files <em>is</em> an edit to this site.`,
	"rendered_at": "rendered", "local_only": "local only, never published",

	"model_p": `<p>Repos never own their agent config. One source tree in <em>your config directory</em> produces
every artifact through <strong>one pipeline</strong>, for two kinds of target: <strong>projects</strong>
(a repo: <code>.claude/</code>, <code>.agents/</code> + <code>.codex/</code>, <code>.cursor/rules/</code>) and
<strong>homes</strong> (a harness user dir: <code>~/.claude</code>, <code>~/.codex</code>, <code>~/.cursor</code> —
skills, agents, rules). Write a skill, agent or rule once; trafaret renders it with the target's
<code>vars</code> into every harness that should have it. The rendered trees are <strong>build artifacts —
never hand-edit them</strong> (the next <code>trafaret install</code> overwrites, and a concurrent agent
"restoring" deleted files has already happened once).</p>`,
	"model_diagram": `flowchart LR
  E["embedded examples<br/>(inside the binary)"]
  subgraph SRC["config directory (edit here only)"]
    M["manifest.yaml<br/>vars · defaults · projects · homes"]
    T["templates/<br/>skills · agents · rules · partials<br/>bundles.yaml"]
    G["global/skills/<br/>agents/⟨role⟩/skills/"]
    P["projects/⟨name⟩/<br/>verbatim skills and agents<br/>(copied as-is, no template pass)"]
  end
  AO(("trafaret install"))
  E --> AO
  M --> AO
  T --> AO
  G --> AO
  P --> AO
  AO --> PR["projects: ⟨repo⟩/<br/>.claude · .agents + .codex · .cursor/rules"]
  AO --> HO["homes: ~/.claude · ~/.codex · ~/.cursor<br/>skills · agents · rules"]`,
	"model_list": `<ul>
<li><strong>manifest.yaml</strong> — the registry. <code>projects:</code> (<code>repoRoot</code>,
    <code>harnesses</code>, <code>skills</code>/<code>agents</code>/<code>rules</code>/<code>bundles</code>,
    <code>disabled</code>, <code>vars</code>) and <code>homes:</code> (<code>harness</code>, <code>dir</code>,
    <code>skills</code>/<code>skip</code>/<code>agents</code>/<code>rules</code>/<code>bundles</code>,
    <code>vars</code>). Every fact a template needs about a target — branch, gates, tracker, stack — is a free
    value under <code>vars:</code>; the engine has no typed project fields. Vars merge in order
    <code>vars</code> &lt; <code>defaults.vars</code> &lt; the entry's own <code>vars</code>: maps merge key by
    key, scalars and lists replace, a <code>null</code> deletes the key.</li>
<li><strong>templates/</strong> — shared sources: <code>skills/&lt;name&gt;/</code>,
    <code>agents/&lt;name&gt;.md[.tmpl]</code>, <code>rules/&lt;name&gt;.md[.tmpl]</code>,
    <code>partials/*.tmpl</code> (shared <code>{{define}}</code> blocks, parsed into every render) and
    <code>bundles.yaml</code>. The binary embeds a small example set; a file at the same path in
    <code>&lt;config&gt;/templates/</code> replaces the embedded one, new files are added.
    <code>bundles.yaml</code> shadows the embedded registry rather than merging with it.</li>
<li><strong>Rendering</strong> — a file ending <code>.tmpl</code> goes through text/template (suffix dropped,
    a missing key is an error, every partial is available); every other file is copied byte for byte, so a
    script containing <code>{{</code> is safe. This holds for every skill source, <code>global/</code> and role
    skills included.</li>
<li><strong>global/skills/</strong> and <strong>agents/&lt;role&gt;/skills/</strong> — skills for the CLIs you use
    everywhere; same namespace as <code>templates/skills</code>. A home whose <code>skills:</code> is omitted
    receives all of these; skills from <code>templates/skills</code> and the examples reach a home only when
    named in its <code>skills:</code> or through a bundle. A role's <code>AGENT.md</code> is the boot document of
    a long-lived named session and is never installed.</li>
<li><strong>projects/&lt;name&gt;/{skills,agents}/</strong> — files for exactly one project, copied verbatim
    into each of its harnesses. No template pass (they may contain <code>{{</code> legitimately).</li>
<li><strong>Adapters</strong> — the engine knows harness layouts only (table below); one entry per harness,
    projects and homes alike.</li></ul>`,

	"adapters_h": "Harness adapters",
	"adapters_p": `<p>A small table in <code>internal/engine/adapters.go</code> says where each kind of item lands for each
harness; render, write, track and prune are the same for all of them, so a new harness is one new adapter value.
Claude Code and Cursor share the project's <code>.claude/</code> tree — Cursor reads <code>.claude/skills</code>
and <code>.claude/agents</code> natively, and a parallel <code>.cursor/</code> copy would double-load every skill;
Cursor adds only <code>.cursor/rules/</code>. Codex gets skills in <code>.agents/skills</code>, a readable agent
contract in <code>.agents/agents</code> and the custom-agent TOML in <code>.codex/agents</code>. Claude and Codex
have no rules: they are skipped there.</p>`,
	"col_item": "item",
	"adapters_rows": `<tr><td>project skill</td><td><code>.claude/skills/&lt;n&gt;/</code></td><td><code>.agents/skills/&lt;n&gt;/</code></td><td>shares <code>.claude/skills</code></td></tr>
<tr><td>project agent</td><td><code>.claude/agents/&lt;n&gt;.md</code></td><td><code>.codex/agents/&lt;n&gt;.toml</code> + <code>.agents/agents/&lt;n&gt;.md</code></td><td>shares <code>.claude/agents</code></td></tr>
<tr><td>project rule</td><td>skipped</td><td>skipped</td><td><code>.cursor/rules/&lt;n&gt;.mdc</code></td></tr>
<tr><td>home skill</td><td><code>&lt;dir&gt;/skills/&lt;n&gt;/</code></td><td><code>&lt;dir&gt;/skills/&lt;n&gt;/</code></td><td><code>&lt;dir&gt;/skills/&lt;n&gt;/</code></td></tr>
<tr><td>home agent</td><td><code>&lt;dir&gt;/agents/&lt;n&gt;.md</code></td><td><code>&lt;dir&gt;/agents/&lt;n&gt;.toml</code></td><td><code>&lt;dir&gt;/agents/&lt;n&gt;.md</code></td></tr>
<tr><td>home rule</td><td>skipped</td><td>skipped</td><td><code>&lt;dir&gt;/rules/&lt;n&gt;.mdc</code></td></tr>
<tr><td>session hook</td><td><code>.claude/settings.json</code></td><td>none</td><td>via <code>.claude/settings.json</code></td></tr>`,

	"quick_h":  "Quick start",
	"quick_c1": "# build ./trafaret + symlink ~/.local/bin/trafaret (once)",
	"quick_c2": "# projects (harnesses, bundles) and homes (harness, dir)",
	"quick_c3": "# manifest sanity + render everything declared (missing vars, template typos, lint)",
	"quick_c4": "# dry-run: per-file install/update/skip/remove",
	"quick_c5": "# render + write into that repo or home dir",
	"quick_c6": "# every project and every home",
	"quick_c7": "# this page (localhost only)",
	"quick_flags": `Useful flags: <code>--target claude|codex|cursor|both|all</code> (harness override: a project is
installed for those harnesses only and never prunes the others' files; a home is installed only when its harness
is among them) · <code>--what bundle,…</code> (only those bundles' contents; no pruning, no hook change) ·
<code>--dry-run</code> · <code>--force</code> (rewrite identical files) · <code>--json</code> ·
<code>--verbose</code> · <code>--config DIR</code>.`,

	"state_h": "Live state",
	"state_p": `<p>Computed now, exactly like <code>trafaret diff</code>: <span class="pill ok">in sync</span> means
the repo's (or home's) rendered artifacts byte-match what the current sources produce.
<span class="pill warn">pending</span> means sources were edited (or the target drifted) and
<code>trafaret install &lt;name&gt;</code> is owed — a target that does not exist yet reads pending, everything in it
would be new. <span class="pill bad">error</span> carries the message (a render error, a missing var).
Refresh after editing to watch this flip. The skills column lists skills only; agents and rules come from the
same lists and bundles. In a home, <code>all</code> means <code>skills:</code> is omitted: every skill under
<code>global/skills</code> and <code>agents/&lt;role&gt;/skills</code>.</p>`,
	"state_projects_h": "Projects", "state_homes_h": "Homes",
	"state_none":  "none declared in manifest.yaml",
	"col_project": "project", "col_home": "home", "col_harness": "harness", "col_harnesses": "harnesses",
	"col_bundles": "bundles", "col_skills": "skills", "col_status": "status",
	"st_error": "error", "st_insync": "in sync", "st_pending": "pending",
	"st_files": "files", "st_new": "new", "st_changed": "changed",

	"bundles_h": "Bundles",
	"bundles_p": `<p>A bundle is a named group of skills, agents and rules. A project or home opts in via
<code>bundles:</code> and installs the union of its own <code>skills</code>/<code>agents</code>/<code>rules</code>
lists and the contents of its active bundles; a home's <code>skip:</code> then subtracts names. Bundles are data:
<code>templates/bundles.yaml</code>. The embedded file holds the demo bundle; a <code>bundles.yaml</code> in your
<code>templates/</code> overlay <strong>shadows</strong> it — it is not merged — so a config has exactly the bundles
its own file lists. An entry sets <code>skills</code>, <code>agents</code>, <code>rules</code> (names from the
namespace above), <code>requires</code> (var paths its templates read, such as <code>branch</code> or
<code>tracker.kind</code> — <code>trafaret validate</code> checks that each exists in the merged vars of every
project and home that lists the bundle, disabled ones included) and <code>sessionContext</code> (a file under
<code>.claude/skills/</code> that a SessionStart hook prints at every session start; at most one active bundle
per project). The table is read live from the registry in effect.</p>
<p>Switching a bundle off: add it to the project's <code>disabled:</code> (keep it in <code>bundles:</code>) and
run <code>trafaret install</code> — its artifacts are pruned from the repo (tracked via
<code>.claude/ao-manifest.json</code>); remove it from <code>disabled:</code> to bring it back. Session hook:
trafaret owns exactly one SessionStart entry in the repo's <code>.claude/settings.json</code>, written on a full
install (no <code>--what</code>) when Claude or Cursor is among the harnesses, removed when the bundle is disabled;
every other settings key is preserved.</p>`,
	"col_bundle": "bundle", "col_agents": "agents", "col_rules": "rules",
	"col_requires": "requires", "col_usedby": "used by",

	"catalog_h": "Catalog",
	"catalog_p": `<p>Every skill, agent and rule of the namespace, read live. Sources: the
<code>templates/</code> overlay, <code>global/skills</code>, <code>agents/&lt;role&gt;/skills</code> and the examples
embedded in the binary. A skill name defined twice among the config sources is an error (shown below and by
<code>trafaret validate</code>), and so is an agent or rule present as both <code>x.md</code> and
<code>x.md.tmpl</code>. A config source <em>shadows</em> an embedded example of the same name: the
<code>templates/</code> overlay merges by path, a global or role source replaces the whole example. Origin:
<span class="pill origin templates">templates</span> overlay ·
<span class="pill origin">examples</span> embedded ·
<span class="pill origin global">global</span> <code>global/skills</code> ·
<span class="pill origin role">role</span> <code>agents/&lt;role&gt;/skills</code>. Descriptions are shown
<em>raw</em>: <code>{{…}}</code> expressions in sources are the documentation of what varies per target, so they
are not evaluated here. Every description ends with a scope clause and every body opens with a
<code>Project:</code> guard (see Guardrails).</p>`,
	"src_err_h":      "The source namespace could not be built",
	"src_err_p":      "Fix the source named above; skills, agents and rules are listed again once it resolves.",
	"filter_ph":      "filter the catalog…",
	"catalog_skills": "Skills", "catalog_agents": "Subagents", "catalog_rules": "Rules",
	"agents_note":  "claude: .claude/agents/&lt;n&gt;.md · codex: .codex/agents/&lt;n&gt;.toml + .agents/agents/&lt;n&gt;.md · cursor shares .claude",
	"rules_note":   "projects: .cursor/rules/&lt;n&gt;.mdc · homes: &lt;dir&gt;/rules/&lt;n&gt;.mdc · cursor harness only, Claude and Codex skip rules",
	"catalog_proj": "Per-project verbatim skills",
	"col_skill":    "skill", "col_when": "when to use", "col_source": "source", "col_origin": "origin",
	"col_words": "words", "col_agent": "agent", "col_rule": "rule", "col_purpose": "purpose",

	"guard_h": "Guardrails & conventions",
	"g1_h":    "Multi-project scope guards",
	"g1_b": `<p>Skill names are identical across repos by design, and Cursor loads same-named skills from every root
of a multi-root workspace <em>without</em> dedup or directory qualification, while <code>alwaysApply</code> rules
stack from every root. Three defense layers (write your templates the same way):</p><ol>
<li>description ends with <code>Applies only to the &lt;name&gt; repo…</code> (routing);</li>
<li>body opens with a <code>Project:</code> STOP-guard line (execution);</li>
<li>an <code>alwaysApply</code> rule names its repo and root in its first line (rule stacking).</li></ol>
<p class="desc">Habit that helps: open a small multi-root workspace of 2–3 active repos instead of
the whole parent folder.</p>`,
	"g2_h": "Frontmatter lint (render-time)",
	"g2_b": `<p>Every emitted <code>.md</code>/<code>.mdc</code> — rendered or verbatim — must parse;
<code>SKILL.md</code> needs non-empty <code>name</code> + <code>description</code>; an unquoted value containing
<code>&nbsp;#</code> is rejected — YAML silently truncates there (this shipped broken twice before the lint
existed). Keep <code>:&nbsp;</code> and <code>&nbsp;#</code> out of unquoted frontmatter values, including
values that come from vars.</p>`,
	"g3_h": "Content doctrine",
	"g3_b": `<p><em>Content lives where something maintains it — otherwise it is dead on arrival.</em>
Stack tutorials nobody maintains rot; durable knowledge belongs in the project's own docs, not in one-off skills.
Templates are <em>process</em> contracts plus thin references to a target's facts: the facts live in
<code>vars:</code>, shared logic in <code>templates/partials/</code>.</p>`,
	"g4_h": "Editing discipline",
	"g4_b": `<p>Rendered trees are build artifacts: never hand-edit them. Writes are sha256-idempotent, so
re-install is always safe (<code>--force</code> rewrites identical files). Only <code>.tmpl</code> files are
rendered; every other file is copied verbatim, so a plain script or doc may contain <code>{{</code>. Only paths
trafaret recorded (<code>.claude/ao-manifest.json</code> in a repo, <code>.ao-manifest.json</code> in each written
home dir) are ever pruned. Commit convention everywhere: developer-style message, no AI attribution;
rendered-repo commits go alongside the config commit.</p>`,

	"howto_h": "How-to",
	"ht1_h":   "Edit a shared skill, agent or rule",
	"ht1_diagram": `sequenceDiagram
  participant A as agent / you
  participant OS as config directory
  participant R as projects and homes
  A->>OS: edit templates/skills/x/SKILL.md.tmpl (or a partial, or vars in manifest.yaml)
  A->>OS: trafaret validate
  Note over OS: missing vars, template typos and YAML lint caught here
  A->>R: trafaret install --all
  A->>OS: git commit (sources)
  A->>R: git commit (rendered artifacts, per repo)`,
	"ht1_p": `<p>Never edit a rendered copy — it is overwritten on the next install. The
<a href="/en/state">Live state</a> tables flip to <span class="pill warn">pending</span> the moment sources and
targets diverge. Prose or logic that several templates share goes into
<code>templates/partials/&lt;name&gt;.tmpl</code> as a <code>{{define "name"}}</code> block, called with
<code>{{template "name" .}}</code>.</p>`,
	"ht2_h": "Add a project-specific skill",
	"ht2_c": "# plain SKILL.md, no .tmpl: copied verbatim",
	"ht2_p": `<p>Copied as-is into every harness of that project — no template pass, so no vars, and it may contain
<code>{{</code>. Frontmatter lint still runs. End the description with the scope clause
(<code>Applies only to the &lt;name&gt; repo.</code>). Project-only agents work the same way:
<code>projects/&lt;name&gt;/agents/&lt;n&gt;.md</code> (for Codex an <code>.md</code> agent becomes TOML only).</p>`,
	"ht3_h":  "Onboard a new project",
	"ht3_c1": "# relative to defaults.workspace",
	"ht3_c2": "# a name from templates/bundles.yaml",
	"ht3_c3": "# the facts your templates read",
	"ht3_p": `<p>Write the project's facts under <code>vars:</code> instead of typed blocks: the engine has no
<code>tickets:</code>, <code>gates:</code> or <code>stack:</code> fields (a manifest that still uses one fails
with <code>… was removed in trafaret 0.3 — move it under vars:</code>). Values shared by many projects go in the
top-level <code>vars:</code> or <code>defaults.vars:</code>; the project's own <code>vars:</code> win. Keep
<code>engine: "&gt;=0.3.0"</code> at the top. Verify the facts against the real systems first (board names, gate
commands) — see <code>docs/onboarding.md</code>; don't fake gates a skill relies on. Then
<code>trafaret validate &amp;&amp; trafaret install myproj</code>.</p>`,
	"ht4_h":  "Add a home",
	"ht4_c1": "# the harness user dir; ~ expands",
	"ht4_c2": "# omitted = every global skill",
	"ht4_p": `<p>A home is a harness plus a directory. It takes <code>skills</code>, <code>agents</code>,
<code>rules</code>, <code>bundles</code>, <code>skip</code> (subtracts names) and <code>vars</code>, and writes into
<code>&lt;dir&gt;/skills</code>, <code>agents</code> and <code>rules</code> as the adapter table says (Claude and Codex
homes get no rules). Omit <code>skills:</code> to receive every skill under <code>global/skills</code> and
<code>agents/&lt;role&gt;/skills</code>; skills from <code>templates/skills</code> and the examples must be named
or arrive with a bundle. A home name must not equal a project name. Templates render per home with
<code>.Kind</code> = <code>home</code>, <code>.TargetRoot</code> = the home dir and an empty
<code>.RepoRoot</code>.</p>`,
	"ht5_h": "Add your own bundle",
	"ht5_p": `<p>Write the sources (<code>templates/skills/&lt;name&gt;/SKILL.md.tmpl</code>,
<code>templates/agents/&lt;name&gt;.md.tmpl</code>, <code>templates/rules/&lt;name&gt;.md.tmpl</code>), list them under a
bundle in <code>&lt;config&gt;/templates/bundles.yaml</code>, set <code>requires</code> to the var paths they read
(<code>trafaret validate</code> checks them against every project and home that lists the bundle) and add the
bundle's name to <code>bundles:</code>. Your own <code>bundles.yaml</code> replaces the embedded one, so copy any
embedded entry you still want. A leftover <code>templates/cursor/</code> (the 0.2 layout) is reported by validate;
<code>cursorRules</code> is now <code>rules</code>.</p>`,
	"ht6_h": "Template data and funcs",
	"ht6_p": `<p>Templates see <code>.Name</code>, <code>.Kind</code>, <code>.Harness</code>, <code>.RepoRoot</code>,
<code>.TargetRoot</code> and <code>.Vars</code>, plus <code>dash</code>, <code>join</code>, <code>default</code>,
<code>has</code> and the text/template built-ins. Optional values: <code>{{if has .Vars "e2e"}}</code> — a bare
<code>.Vars.x</code> that is absent is a render error. Full contract: <code>docs/template-reference.md</code>.</p>`,
	"ht7_h":       "Troubleshooting",
	"col_symptom": "symptom", "col_fix": "cause / fix",
	"trouble_rows": `<tr><td><code>render error: … map has no entry for key</code></td><td>a var is missing or mistyped —
<code>trafaret validate</code> names the file and the key; add it under <code>vars:</code> (or
<code>defaults.vars:</code>), or guard the read with <code>{{if has .Vars "x"}}</code>. <code>default</code> does not
help for an absent key; see <code>docs/template-reference.md</code></td></tr>
<tr><td><code>bundle X requires var "a.b", which is not set</code></td><td>the project or home that lists the bundle
lacks that var path in its merged vars — set it, or drop the bundle from that entry (disabled bundles are checked
too)</td></tr>
<tr><td><code>skill "x" defined twice</code></td><td>the same name in two config sources
(<code>templates/skills</code>, <code>global/skills</code>, <code>agents/*/skills</code>) — rename or delete one; a
config source silently shadows an embedded example, two config sources do not</td></tr>
<tr><td><code>frontmatter … contains ' #'</code></td><td>the YAML truncation trap — quote the value
or drop the <code>#</code> (check values that come from vars too)</td></tr>
<tr><td><code>projects.x: "tickets" was removed in trafaret 0.3 — move it under vars:</code></td><td>a 0.2 manifest
field: move the block under <code>vars:</code> (YAML unchanged), rewrite <code>.Tickets.Space</code> to
<code>.Vars.tickets.space</code>; <code>targets</code> is now <code>harnesses</code>, a home's
<code>skillsDir</code> is <code>harness</code> + <code>dir</code></td></tr>
<tr><td><code>no bundles, skills, agents or rules and no projects/… sources</code></td><td>the entry installs
nothing — add a bundle, a list or a <code>projects/&lt;name&gt;/</code> dir</td></tr>
<tr><td><code>bundle "…" is not in templates/bundles.yaml</code></td><td>the registry is the config's own
<code>templates/bundles.yaml</code>, which shadows the embedded one — add the bundle there (or copy back the
embedded entries you still use)</td></tr>
<tr><td>a skill shows up twice in Cursor</td><td>someone re-created <code>.cursor/skills</code> —
delete it; Cursor reads <code>.claude/skills</code> natively and does not dedupe the copies</td></tr>
<tr><td>rendered files reappear after deletion</td><td>the source is still here, or a concurrent agent "restored"
them — delete the source (install regenerates from it), re-run install and commit with an explicit
message</td></tr>
<tr><td>agent used another project's facts</td><td>scope guards exist, but check the workspace layout — prefer a
small multi-root workspace over opening a whole monorepo</td></tr>`,

	"where_h":    "Where do I edit…",
	"col_change": "I want to change…", "col_edit": "edit this", "col_then": "then",
	"where_rows": `<tr><td>a shared skill, agent or rule (all targets)</td><td><code>&lt;config&gt;/templates/skills/&lt;name&gt;/SKILL.md.tmpl</code> · <code>templates/agents/&lt;name&gt;.md.tmpl</code> · <code>templates/rules/&lt;name&gt;.md.tmpl</code> <span class="desc">(keep an agent a pointer — the skill is the single source of the checklist)</span></td><td rowspan="8"><code>trafaret validate</code><br><code>trafaret install --all</code><br>commit sources + rendered repos</td></tr>
<tr><td>a project's facts (branch, gates, tracker, stack…)</td><td>that project's <code>vars:</code> in <code>manifest.yaml</code> <span class="desc">(shared values: top-level <code>vars:</code> or <code>defaults.vars:</code>)</span></td></tr>
<tr><td>prose or logic shared by several templates</td><td><code>&lt;config&gt;/templates/partials/&lt;name&gt;.tmpl</code> <span class="desc">(<code>{{define}}</code> blocks)</span></td></tr>
<tr><td>a skill for exactly one project</td><td><code>projects/&lt;name&gt;/skills/&lt;skill&gt;/SKILL.md</code> <span class="desc">(verbatim)</span></td></tr>
<tr><td>a skill for every repo and harness</td><td><code>global/skills/&lt;skill&gt;/SKILL.md</code> <span class="desc">(a home receives it when its <code>skills:</code> is omitted)</span></td></tr>
<tr><td>a role agent's boot doc or one of its skills</td><td><code>agents/&lt;role&gt;/AGENT.md</code> · <code>agents/&lt;role&gt;/skills/&lt;skill&gt;/SKILL.md</code> <span class="desc">(skills land in the homes like global ones)</span></td></tr>
<tr><td>which artifacts a project or home gets</td><td>its <code>skills</code>, <code>agents</code>, <code>rules</code>, <code>bundles</code> lists in <code>manifest.yaml</code></td></tr>
<tr><td>what a bundle contains</td><td><code>&lt;config&gt;/templates/bundles.yaml</code></td></tr>
<tr><td>what template data and funcs exist</td><td colspan="2"><code>docs/template-reference.md</code> (the template contract)</td></tr>
<tr><td>where files land per harness (adapters)</td><td colspan="2"><code>internal/engine/adapters.go</code> <span class="desc">(a new harness is a new adapter value)</span></td></tr>
<tr><td>this page</td><td colspan="2"><code>internal/site/page.go</code> (skeleton) + <code>internal/site/strings.go</code> (EN/RU prose) — the tables fix themselves</td></tr>`,

	"side_live": "re-rendered from the repo on every refresh",
	"prev":      "previous", "next": "next",
}

var siteRU = map[string]h{
	"title": "руководство оператора",

	"nav_model": "Обзор", "nav_state": "Состояние", "nav_catalog": "Каталог",
	"nav_guard": "Ограждения", "nav_howto": "Как сделать", "nav_where": "Что где править",

	"hero_h1": "trafaret — руководство оператора",
	"hero_p": `Одно дерево источников скилов, сабагентов и правил, которое рендерится со значениями для каждой
цели в Claude Code, Codex и Cursor — и для репозиториев (<em>проекты</em>), и для пользовательских
<em>homes</em>. Страница рендерится из каталога конфигурации — <code>manifest.yaml</code> +
<code>templates/</code> + <code>global/</code> + <code>agents/</code> + <code>projects/</code> + встроенные
примеры — при <em>каждом обновлении</em>: правка агентом этих файлов <em>и есть</em> правка этого сайта.`,
	"rendered_at": "отрендерено", "local_only": "только локально, никуда не публикуется",

	"model_p": `<p>Репозитории не владеют своей агентской конфигурацией. Одно дерево источников в
<em>вашем каталоге конфигурации</em> порождает все артефакты через <strong>один пайплайн</strong> для двух видов
целей: <strong>проекты</strong> (репо: <code>.claude/</code>, <code>.agents/</code> + <code>.codex/</code>,
<code>.cursor/rules/</code>) и <strong>homes</strong> (пользовательский каталог харнеса: <code>~/.claude</code>,
<code>~/.codex</code>, <code>~/.cursor</code> — скилы, агенты, правила). Скил, агент или правило пишется один
раз; trafaret рендерит его со значениями <code>vars</code> цели в каждый харнес, которому он нужен.
Отрендеренные деревья — <strong>build-артефакты, руками их не правят</strong> (следующий
<code>trafaret install</code> перезапишет; конкурентный агент, «восстановивший» удалённые файлы, уже случался).</p>`,
	"model_diagram": `flowchart LR
  E["встроенные примеры<br/>(внутри бинарника)"]
  subgraph SRC["каталог конфигурации (править только здесь)"]
    M["manifest.yaml<br/>vars · defaults · projects · homes"]
    T["templates/<br/>skills · agents · rules · partials<br/>bundles.yaml"]
    G["global/skills/<br/>agents/⟨role⟩/skills/"]
    P["projects/⟨name⟩/<br/>дословные скилы и агенты<br/>(копируются как есть, без шаблонов)"]
  end
  AO(("trafaret install"))
  E --> AO
  M --> AO
  T --> AO
  G --> AO
  P --> AO
  AO --> PR["проекты: ⟨repo⟩/<br/>.claude · .agents + .codex · .cursor/rules"]
  AO --> HO["homes: ~/.claude · ~/.codex · ~/.cursor<br/>skills · agents · rules"]`,
	"model_list": `<ul>
<li><strong>manifest.yaml</strong> — реестр. <code>projects:</code> (<code>repoRoot</code>,
    <code>harnesses</code>, <code>skills</code>/<code>agents</code>/<code>rules</code>/<code>bundles</code>,
    <code>disabled</code>, <code>vars</code>) и <code>homes:</code> (<code>harness</code>, <code>dir</code>,
    <code>skills</code>/<code>skip</code>/<code>agents</code>/<code>rules</code>/<code>bundles</code>,
    <code>vars</code>). Любой факт о цели, нужный шаблону, — ветка, гейты, трекер, стек — это свободное
    значение в <code>vars:</code>; типизированных полей проекта в движке нет. Vars сливаются в порядке
    <code>vars</code> &lt; <code>defaults.vars</code> &lt; собственные <code>vars</code> записи: мапы сливаются
    по ключам, скаляры и списки заменяются, <code>null</code> удаляет ключ.</li>
<li><strong>templates/</strong> — общие источники: <code>skills/&lt;name&gt;/</code>,
    <code>agents/&lt;name&gt;.md[.tmpl]</code>, <code>rules/&lt;name&gt;.md[.tmpl]</code>,
    <code>partials/*.tmpl</code> (общие блоки <code>{{define}}</code>, подключаются в каждый рендер) и
    <code>bundles.yaml</code>. В бинарник встроен небольшой набор примеров; файл по тому же пути в
    <code>&lt;config&gt;/templates/</code> заменяет встроенный, новые файлы добавляются.
    <code>bundles.yaml</code> перекрывает встроенный реестр, а не сливается с ним.</li>
<li><strong>Рендер</strong> — файл с суффиксом <code>.tmpl</code> проходит через text/template (суффикс
    отбрасывается, отсутствующий ключ — ошибка, все partials доступны); любой другой файл копируется байт в
    байт, так что скрипт с <code>{{</code> безопасен. Это верно для всех источников скилов, включая
    <code>global/</code> и скилы ролей.</li>
<li><strong>global/skills/</strong> и <strong>agents/&lt;role&gt;/skills/</strong> — скилы для CLI, которыми вы
    пользуетесь везде; то же пространство имён, что и <code>templates/skills</code>. Home с пропущенным
    <code>skills:</code> получает все они; скилы из <code>templates/skills</code> и примеров попадают в home
    только по имени в его <code>skills:</code> или через бандл. <code>AGENT.md</code> роли — boot-документ
    долгоживущей именованной сессии, никуда не ставится.</li>
<li><strong>projects/&lt;name&gt;/{skills,agents}/</strong> — файлы ровно одного проекта, копируются дословно в
    каждый его харнес. Без шаблонного прохода (могут легитимно содержать <code>{{</code>).</li>
<li><strong>Адаптеры</strong> — движок знает только раскладки харнесов (таблица ниже): по записи на харнес,
    одинаково для проектов и homes.</li></ul>`,

	"adapters_h": "Адаптеры харнесов",
	"adapters_p": `<p>Небольшая таблица в <code>internal/engine/adapters.go</code> говорит, куда для каждого харнеса
ложится каждый вид элемента; рендер, запись, трекинг и prune одинаковы для всех, так что новый харнес — это одно
новое значение адаптера. Claude Code и Cursor делят дерево <code>.claude/</code> проекта — Cursor читает
<code>.claude/skills</code> и <code>.claude/agents</code> нативно, а параллельная копия в <code>.cursor/</code>
двоила бы каждый скил; Cursor добавляет только <code>.cursor/rules/</code>. Codex получает скилы в
<code>.agents/skills</code>, читаемый контракт агента в <code>.agents/agents</code> и custom-agent TOML в
<code>.codex/agents</code>. У Claude и Codex правил нет: там они пропускаются.</p>`,
	"col_item": "элемент",
	"adapters_rows": `<tr><td>скил проекта</td><td><code>.claude/skills/&lt;n&gt;/</code></td><td><code>.agents/skills/&lt;n&gt;/</code></td><td>делит <code>.claude/skills</code></td></tr>
<tr><td>агент проекта</td><td><code>.claude/agents/&lt;n&gt;.md</code></td><td><code>.codex/agents/&lt;n&gt;.toml</code> + <code>.agents/agents/&lt;n&gt;.md</code></td><td>делит <code>.claude/agents</code></td></tr>
<tr><td>правило проекта</td><td>пропускается</td><td>пропускается</td><td><code>.cursor/rules/&lt;n&gt;.mdc</code></td></tr>
<tr><td>скил home</td><td><code>&lt;dir&gt;/skills/&lt;n&gt;/</code></td><td><code>&lt;dir&gt;/skills/&lt;n&gt;/</code></td><td><code>&lt;dir&gt;/skills/&lt;n&gt;/</code></td></tr>
<tr><td>агент home</td><td><code>&lt;dir&gt;/agents/&lt;n&gt;.md</code></td><td><code>&lt;dir&gt;/agents/&lt;n&gt;.toml</code></td><td><code>&lt;dir&gt;/agents/&lt;n&gt;.md</code></td></tr>
<tr><td>правило home</td><td>пропускается</td><td>пропускается</td><td><code>&lt;dir&gt;/rules/&lt;n&gt;.mdc</code></td></tr>
<tr><td>session hook</td><td><code>.claude/settings.json</code></td><td>нет</td><td>через <code>.claude/settings.json</code></td></tr>`,

	"quick_h":  "Быстрый старт",
	"quick_c1": "# собрать ./trafaret + симлинк ~/.local/bin/trafaret (один раз)",
	"quick_c2": "# проекты (харнесы, бандлы) и homes (харнес, dir)",
	"quick_c3": "# проверка манифеста + рендер всего объявленного (пропущенные vars, опечатки, линт)",
	"quick_c4": "# dry-run: install/update/skip/remove по файлам",
	"quick_c5": "# отрендерить и записать в репо или каталог home",
	"quick_c6": "# все проекты и все homes",
	"quick_c7": "# эта страница (только localhost)",
	"quick_flags": `Полезные флаги: <code>--target claude|codex|cursor|both|all</code> (переопределение харнесов:
проект ставится только для них и не чистит файлы остальных; home ставится, только если его харнес среди
названных) · <code>--what bundle,…</code> (только содержимое этих бандлов; без prune и без правки hook) ·
<code>--dry-run</code> · <code>--force</code> (переписать одинаковые файлы) · <code>--json</code> ·
<code>--verbose</code> · <code>--config DIR</code>.`,

	"state_h": "Живое состояние",
	"state_p": `<p>Считается прямо сейчас, ровно как <code>trafaret diff</code>: <span class="pill ok">синхронно</span> —
артефакты в репо (или в home) байт-в-байт совпадают с тем, что дают текущие источники.
<span class="pill warn">ожидает</span> — источники правились (или цель разъехалась) и нужен
<code>trafaret install &lt;name&gt;</code>; цель, которой ещё нет, тоже «ожидает»: всё в ней было бы новым.
<span class="pill bad">ошибка</span> несёт сообщение (ошибка рендера, пропущенная var). Обновите страницу
после правки, и статус переключится. Колонка скилов показывает только скилы; агенты и правила приходят из тех
же списков и бандлов. В home значение <code>all</code> значит, что <code>skills:</code> опущен: все скилы из
<code>global/skills</code> и <code>agents/&lt;role&gt;/skills</code>.</p>`,
	"state_projects_h": "Проекты", "state_homes_h": "Homes",
	"state_none":  "в manifest.yaml ничего не объявлено",
	"col_project": "проект", "col_home": "home", "col_harness": "харнес", "col_harnesses": "харнесы",
	"col_bundles": "бандлы", "col_skills": "скилы", "col_status": "статус",
	"st_error": "ошибка", "st_insync": "синхронно", "st_pending": "ожидает",
	"st_files": "файлов", "st_new": "новых", "st_changed": "изменённых",

	"bundles_h": "Бандлы",
	"bundles_p": `<p>Бандл — именованная группа скилов, агентов и правил. Проект или home подключает её через
<code>bundles:</code> и ставит объединение собственных списков <code>skills</code>/<code>agents</code>/<code>rules</code>
и содержимого активных бандлов; <code>skip:</code> home затем вычитает имена. Бандлы — это данные:
<code>templates/bundles.yaml</code>. Встроенный файл содержит демо-бандл; <code>bundles.yaml</code> в оверлее
<code>templates/</code> вашей конфигурации <strong>перекрывает</strong> его — слияния нет, — так что у
конфигурации ровно те бандлы, что перечислены в её собственном файле. Запись задаёт <code>skills</code>,
<code>agents</code>, <code>rules</code> (имена из пространства имён выше), <code>requires</code> (пути var, которые
читают её шаблоны, например <code>branch</code> или <code>tracker.kind</code> — <code>trafaret validate</code>
проверяет, что каждый есть в слитых vars каждого проекта и home, где бандл указан, включая выключенные) и
<code>sessionContext</code> (файл в <code>.claude/skills/</code>, который SessionStart-hook печатает при каждом
старте сессии; не больше одного активного бандла на проект). Таблица читается вживую из действующего
реестра.</p>
<p>Выключение бандла: добавьте его в <code>disabled:</code> проекта (из <code>bundles:</code> не убирайте) и
запустите <code>trafaret install</code> — его артефакты будут удалены из репозитория (трекинг через
<code>.claude/ao-manifest.json</code>); уберите из <code>disabled:</code> — вернутся. Session hook: trafaret
владеет ровно одной записью SessionStart в <code>.claude/settings.json</code> репозитория; она пишется при полной
установке (без <code>--what</code>), когда среди харнесов есть Claude или Cursor, и убирается при disable;
остальные ключи настроек не трогаются.</p>`,
	"col_bundle": "бандл", "col_agents": "агенты", "col_rules": "правила",
	"col_requires": "requires", "col_usedby": "кто использует",

	"catalog_h": "Каталог",
	"catalog_p": `<p>Все скилы, агенты и правила пространства имён, читаются вживую. Источники: оверлей
<code>templates/</code>, <code>global/skills</code>, <code>agents/&lt;role&gt;/skills</code> и примеры, встроенные в
бинарник. Имя скила, определённое дважды среди источников конфигурации, — ошибка (она показана ниже и в
<code>trafaret validate</code>); так же ошибка, если агент или правило есть и как <code>x.md</code>, и как
<code>x.md.tmpl</code>. Источник конфигурации <em>перекрывает</em> встроенный пример с тем же именем: оверлей
<code>templates/</code> сливается по путям, global- или role-источник заменяет пример целиком. Origin:
<span class="pill origin templates">templates</span> оверлей ·
<span class="pill origin">examples</span> встроенные ·
<span class="pill origin global">global</span> <code>global/skills</code> ·
<span class="pill origin role">role</span> <code>agents/&lt;role&gt;/skills</code>. Описания показаны
<em>сырыми</em>: выражения <code>{{…}}</code> в источниках и есть документация того, что варьируется по целям,
поэтому здесь они не вычисляются. Каждое описание заканчивается scope-клаузой, каждое тело открывается гардом
<code>Project:</code> (см. «Ограждения»).</p>`,
	"src_err_h":      "Не удалось построить пространство имён источников",
	"src_err_p":      "Исправьте названный выше источник; скилы, агенты и правила появятся снова, как только он разрешится.",
	"filter_ph":      "фильтр по каталогу…",
	"catalog_skills": "Скилы", "catalog_agents": "Сабагенты", "catalog_rules": "Правила",
	"agents_note":  "claude: .claude/agents/&lt;n&gt;.md · codex: .codex/agents/&lt;n&gt;.toml + .agents/agents/&lt;n&gt;.md · cursor делит .claude",
	"rules_note":   "проекты: .cursor/rules/&lt;n&gt;.mdc · homes: &lt;dir&gt;/rules/&lt;n&gt;.mdc · только харнес cursor, Claude и Codex правила пропускают",
	"catalog_proj": "Дословные скилы проектов",
	"col_skill":    "скил", "col_when": "когда использовать", "col_source": "источник", "col_origin": "origin",
	"col_words": "слов", "col_agent": "агент", "col_rule": "правило", "col_purpose": "назначение",

	"guard_h": "Ограждения и конвенции",
	"g1_h":    "Scope-гарды для мультипроекта",
	"g1_b": `<p>Имена скилов одинаковы во всех репо by design, а Cursor в multi-root workspace грузит скилы с
одинаковыми именами из каждого корня <em>без</em> дедупликации и директорийной квалификации, и правила с
<code>alwaysApply</code> стекаются из каждого корня. Три слоя защиты (пишите свои шаблоны так же):</p><ol>
<li>описание заканчивается <code>Applies only to the &lt;name&gt; repo…</code> (роутинг);</li>
<li>тело открывается STOP-гардом <code>Project:</code> (исполнение);</li>
<li>правило с <code>alwaysApply</code> называет свой репо и корень в первой строке (стекание правил).</li></ol>
<p class="desc">Полезная привычка: открывать маленький multi-root workspace из 2–3 активных репо
вместо всей родительской папки.</p>`,
	"g2_h": "Линт frontmatter (на рендере)",
	"g2_b": `<p>Каждый выданный <code>.md</code>/<code>.mdc</code> — отрендеренный или дословный — обязан парситься;
у <code>SKILL.md</code> — непустые <code>name</code> + <code>description</code>; незакавыченное значение с
<code>&nbsp;#</code> отклоняется — YAML молча обрезает там строку (до линта это дважды уезжало в прод
сломанным). Держите <code>:&nbsp;</code> и <code>&nbsp;#</code> вне незакавыченных значений frontmatter,
в том числе приходящих из vars.</p>`,
	"g3_h": "Доктрина контента",
	"g3_b": `<p><em>Контент живёт там, где его кто-то поддерживает, — иначе он мёртв с рождения.</em>
Стек-туториалы без владельца гниют; долговечное знание живёт в собственных доках проекта, а не в разовых
скилах. Шаблоны — <em>процессные</em> контракты плюс тонкие ссылки на факты цели: факты живут в
<code>vars:</code>, общая логика — в <code>templates/partials/</code>.</p>`,
	"g4_h": "Дисциплина правок",
	"g4_b": `<p>Отрендеренные деревья — build-артефакты: руками их не правят. Запись sha256-идемпотентна,
переустановка всегда безопасна (<code>--force</code> переписывает одинаковые файлы). Рендерятся только файлы
<code>.tmpl</code>; остальные копируются дословно, так что обычный скрипт или док может содержать
<code>{{</code>. Удаляются (prune) только пути, которые записал сам trafaret (<code>.claude/ao-manifest.json</code>
в репо, <code>.ao-manifest.json</code> в каждом записанном каталоге home). Конвенция коммитов везде:
developer-style сообщение, без AI-атрибуции; коммиты в отрендеренных репо идут вместе с коммитом
конфигурации.</p>`,

	"howto_h": "Как сделать",
	"ht1_h":   "Править общий скил, агент или правило",
	"ht1_diagram": `sequenceDiagram
  participant A as агент / вы
  participant OS as каталог конфигурации
  participant R as проекты и homes
  A->>OS: правка templates/skills/x/SKILL.md.tmpl (или partial, или vars в manifest.yaml)
  A->>OS: trafaret validate
  Note over OS: пропущенные vars, опечатки шаблонов и YAML-линт ловятся здесь
  A->>R: trafaret install --all
  A->>OS: git commit (источники)
  A->>R: git commit (артефакты, по каждому репо)`,
	"ht1_p": `<p>Никогда не правьте отрендеренную копию — следующий install её перезапишет. Таблицы
<a href="/ru/state">живого состояния</a> переключаются в <span class="pill warn">ожидает</span>, как только
источники и цели разъехались. Проза или логика, общая для нескольких шаблонов, идёт в
<code>templates/partials/&lt;name&gt;.tmpl</code> блоком <code>{{define "name"}}</code> и вызывается через
<code>{{template "name" .}}</code>.</p>`,
	"ht2_h": "Добавить скил одного проекта",
	"ht2_c": "# обычный SKILL.md, без .tmpl: копируется дословно",
	"ht2_p": `<p>Копируется как есть в каждый харнес этого проекта — без шаблонного прохода, значит без vars, и может
содержать <code>{{</code>. Линт frontmatter всё равно работает. Заканчивайте описание scope-клаузой
(<code>Applies only to the &lt;name&gt; repo.</code>). Агенты только для проекта работают так же:
<code>projects/&lt;name&gt;/agents/&lt;n&gt;.md</code> (для Codex <code>.md</code>-агент превращается только в TOML).</p>`,
	"ht3_h":  "Подключить новый проект",
	"ht3_c1": "# относительно defaults.workspace",
	"ht3_c2": "# имя из templates/bundles.yaml",
	"ht3_c3": "# факты, которые читают ваши шаблоны",
	"ht3_p": `<p>Пишите факты проекта в <code>vars:</code>, а не типизированными блоками: полей <code>tickets:</code>,
<code>gates:</code> или <code>stack:</code> в движке нет (манифест, где они ещё есть, падает с
<code>… was removed in trafaret 0.3 — move it under vars:</code>). Значения, общие для многих проектов, кладите в
верхнеуровневые <code>vars:</code> или <code>defaults.vars:</code>; собственные <code>vars:</code> проекта
главнее. Держите <code>engine: "&gt;=0.3.0"</code> в начале. Сначала сверьте факты с реальными системами (имена
досок, команды гейтов) — см. <code>docs/onboarding.md</code>; не подделывайте гейты, на которые будет опираться
скил. Затем <code>trafaret validate &amp;&amp; trafaret install myproj</code>.</p>`,
	"ht4_h":  "Добавить home",
	"ht4_c1": "# пользовательский каталог харнеса; ~ раскрывается",
	"ht4_c2": "# опущено = все global-скилы",
	"ht4_p": `<p>Home — это харнес плюс каталог. Принимает <code>skills</code>, <code>agents</code>,
<code>rules</code>, <code>bundles</code>, <code>skip</code> (вычитает имена) и <code>vars</code>, пишет в
<code>&lt;dir&gt;/skills</code>, <code>agents</code> и <code>rules</code> по таблице адаптеров (у homes Claude и
Codex правил нет). Опустите <code>skills:</code>, чтобы получить все скилы из <code>global/skills</code> и
<code>agents/&lt;role&gt;/skills</code>; скилы из <code>templates/skills</code> и примеров нужно назвать или
получить через бандл. Имя home не должно совпадать с именем проекта. Шаблоны рендерятся для home с
<code>.Kind</code> = <code>home</code>, <code>.TargetRoot</code> = каталог home и пустым
<code>.RepoRoot</code>.</p>`,
	"ht5_h": "Добавить свой бандл",
	"ht5_p": `<p>Напишите источники (<code>templates/skills/&lt;name&gt;/SKILL.md.tmpl</code>,
<code>templates/agents/&lt;name&gt;.md.tmpl</code>, <code>templates/rules/&lt;name&gt;.md.tmpl</code>), перечислите их
в бандле в <code>&lt;config&gt;/templates/bundles.yaml</code>, задайте <code>requires</code> путями var, которые они
читают (<code>trafaret validate</code> сверяет их с каждым проектом и home, где бандл указан), и добавьте имя
бандла в <code>bundles:</code>. Ваш <code>bundles.yaml</code> заменяет встроенный, так что скопируйте нужные
встроенные записи. Оставшийся <code>templates/cursor/</code> (раскладка 0.2) validate отметит;
<code>cursorRules</code> теперь называется <code>rules</code>.</p>`,
	"ht6_h": "Данные и функции шаблонов",
	"ht6_p": `<p>Шаблоны видят <code>.Name</code>, <code>.Kind</code>, <code>.Harness</code>, <code>.RepoRoot</code>,
<code>.TargetRoot</code> и <code>.Vars</code>, а также <code>dash</code>, <code>join</code>, <code>default</code>,
<code>has</code> и встроенные функции text/template. Необязательные значения: <code>{{if has .Vars "e2e"}}</code> —
обращение к отсутствующему <code>.Vars.x</code> напрямую даёт ошибку рендера. Полный контракт:
<code>docs/template-reference.md</code>.</p>`,
	"ht7_h":       "Диагностика",
	"col_symptom": "симптом", "col_fix": "причина / что делать",
	"trouble_rows": `<tr><td><code>render error: … map has no entry for key</code></td><td>var не задана или
написана с опечаткой — <code>trafaret validate</code> назовёт файл и ключ; добавьте её в <code>vars:</code> (или
<code>defaults.vars:</code>) либо защитите чтение через <code>{{if has .Vars "x"}}</code>. <code>default</code>
для отсутствующего ключа не помогает; см. <code>docs/template-reference.md</code></td></tr>
<tr><td><code>bundle X requires var "a.b", which is not set</code></td><td>у проекта или home, где указан бандл,
в слитых vars нет этого пути — задайте его или уберите бандл из этой записи (выключенные бандлы тоже
проверяются)</td></tr>
<tr><td><code>skill "x" defined twice</code></td><td>одно имя в двух источниках конфигурации
(<code>templates/skills</code>, <code>global/skills</code>, <code>agents/*/skills</code>) — переименуйте или
удалите один; источник конфигурации молча перекрывает встроенный пример, два источника конфигурации — нет</td></tr>
<tr><td><code>frontmatter … contains ' #'</code></td><td>ловушка YAML-обрезания — закавычьте
значение или уберите <code>#</code> (проверьте и значения, приходящие из vars)</td></tr>
<tr><td><code>projects.x: "tickets" was removed in trafaret 0.3 — move it under vars:</code></td><td>поле манифеста
0.2: перенесите блок в <code>vars:</code> (YAML без изменений), перепишите <code>.Tickets.Space</code> в
<code>.Vars.tickets.space</code>; <code>targets</code> теперь <code>harnesses</code>, <code>skillsDir</code> home —
это <code>harness</code> + <code>dir</code></td></tr>
<tr><td><code>no bundles, skills, agents or rules and no projects/… sources</code></td><td>запись ничего не
устанавливает — добавьте бандл, список или каталог <code>projects/&lt;name&gt;/</code></td></tr>
<tr><td><code>bundle "…" is not in templates/bundles.yaml</code></td><td>реестр — это собственный
<code>templates/bundles.yaml</code> конфигурации, он перекрывает встроенный — добавьте бандл туда (или
верните нужные встроенные записи)</td></tr>
<tr><td>скил виден в Cursor дважды</td><td>кто-то заново создал <code>.cursor/skills</code> —
удалите его; Cursor нативно читает <code>.claude/skills</code> и копии не дедуплицирует</td></tr>
<tr><td>отрендеренные файлы возвращаются после удаления</td><td>источник всё ещё здесь, или конкурентный агент их
«восстановил» — удалите источник (install генерирует из него), снова запустите install и закоммитьте с явным
сообщением</td></tr>
<tr><td>агент применил факты чужого проекта</td><td>scope-гарды стоят, но проверьте раскладку workspace —
лучше маленький multi-root, чем открытый целиком монорепо</td></tr>`,

	"where_h":    "Что где править",
	"col_change": "хочу поменять…", "col_edit": "правьте это", "col_then": "затем",
	"where_rows": `<tr><td>общий скил, агент или правило (все цели)</td><td><code>&lt;config&gt;/templates/skills/&lt;name&gt;/SKILL.md.tmpl</code> · <code>templates/agents/&lt;name&gt;.md.tmpl</code> · <code>templates/rules/&lt;name&gt;.md.tmpl</code> <span class="desc">(агент — указатель, скил остаётся единственным источником чеклиста)</span></td><td rowspan="8"><code>trafaret validate</code><br><code>trafaret install --all</code><br>коммит источников + отрендеренных репо</td></tr>
<tr><td>факты проекта (ветка, гейты, трекер, стек…)</td><td><code>vars:</code> этого проекта в <code>manifest.yaml</code> <span class="desc">(общие значения: верхнеуровневые <code>vars:</code> или <code>defaults.vars:</code>)</span></td></tr>
<tr><td>проза или логика, общая для нескольких шаблонов</td><td><code>&lt;config&gt;/templates/partials/&lt;name&gt;.tmpl</code> <span class="desc">(блоки <code>{{define}}</code>)</span></td></tr>
<tr><td>скил ровно одного проекта</td><td><code>projects/&lt;name&gt;/skills/&lt;skill&gt;/SKILL.md</code> <span class="desc">(дословно)</span></td></tr>
<tr><td>скил для каждого репо и харнеса</td><td><code>global/skills/&lt;skill&gt;/SKILL.md</code> <span class="desc">(home получает его, когда его <code>skills:</code> опущен)</span></td></tr>
<tr><td>boot-док ролевого агента или его скил</td><td><code>agents/&lt;role&gt;/AGENT.md</code> · <code>agents/&lt;role&gt;/skills/&lt;skill&gt;/SKILL.md</code> <span class="desc">(скилы ставятся в homes как глобальные)</span></td></tr>
<tr><td>какие артефакты получает проект или home</td><td>его списки <code>skills</code>, <code>agents</code>, <code>rules</code>, <code>bundles</code> в <code>manifest.yaml</code></td></tr>
<tr><td>что входит в бандл</td><td><code>&lt;config&gt;/templates/bundles.yaml</code></td></tr>
<tr><td>какие есть данные и функции шаблонов</td><td colspan="2"><code>docs/template-reference.md</code> (контракт шаблонов)</td></tr>
<tr><td>куда ложатся файлы по харнесам (адаптеры)</td><td colspan="2"><code>internal/engine/adapters.go</code> <span class="desc">(новый харнес — новое значение адаптера)</span></td></tr>
<tr><td>эту страницу</td><td colspan="2"><code>internal/site/page.go</code> (скелет) + <code>internal/site/strings.go</code> (тексты EN/RU) — таблицы чинятся сами</td></tr>`,

	"side_live": "перерендеривается из репозитория при каждом обновлении",
	"prev":      "назад", "next": "дальше",
}

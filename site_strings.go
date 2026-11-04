package main

import "html/template"

// h is a shorthand: the string maps hold trusted, repo-authored markup.
type h = template.HTML

var siteEN = map[string]h{
	"title": "operator manual",

	"nav_model": "Overview", "nav_quick": "Quick start", "nav_state": "Live state",
	"nav_bundles": "Bundles", "nav_catalog": "Catalog",
	"nav_guard": "Guardrails", "nav_howto": "How-to", "nav_where": "Where do I edit…",

	"hero_h1": "trafaret — operator manual",
	"hero_p": `The single source of truth for the agent skills, subagents and rules you install into your
projects. This page is rendered from the config directory — <code>manifest.yaml</code> + the
templates (the embedded examples plus your <code>templates/</code> overlay and its
<code>bundles.yaml</code>) + <code>projects/</code> — on
<em>every refresh</em>: an agent's edit to those files <em>is</em> an edit to this site.`,
	"rendered_at": "rendered", "local_only": "local only, never published",
	"footer_refresh": "refresh (⌘R) to re-read the config",

	"model_h": "The model",
	"model_p": `<p>Repos never own their agent config. Three sources in <em>your config directory</em> produce
every artifact; the rendered <code>.claude/</code>, <code>.cursor/</code>, <code>.agents/</code>, and <code>.codex/</code> trees in target
repos are <strong>build artifacts — never hand-edit them</strong> (the next <code>trafaret install</code>
overwrites, and a concurrent agent "restoring" deleted files has already happened once). One tree
serves Claude Code and Cursor; Codex gets its native skill and custom-agent projections.</p>`,
	"model_diagram": `flowchart LR
  E["embedded examples<br/>(inside the binary)"]
  subgraph SRC["config directory (edit here only)"]
    T["templates/ overlay<br/>bundles.yaml · skills · agents · rules"]
    P["projects/⟨name⟩/<br/>verbatim per-project skills<br/>(copied as-is, no tokens)"]
    M["manifest.yaml<br/>per-project token values"]
  end
  AO(("trafaret install"))
  E --> AO
  T --> AO
  P --> AO
  M --> AO
  AO --> C["⟨repo⟩/.claude/<br/>skills + agents"]
  AO --> U["⟨repo⟩/.cursor/rules/<br/>Cursor rules"]
  AO --> X["⟨repo⟩/.agents/skills + .agents/agents<br/>.codex/agents · Codex"]
  G["global/skills/"] --> AO
  R["agents/⟨role⟩/skills/"] --> AO
  AO --> H["~/.claude · ~/.codex · ~/.cursor<br/>skills (homes)"]`,
	"model_list": `<ul>
<li><strong>templates/</strong> — shared, tokenized (<code>{{.Name}}</code>,
    <code>{{.Tickets.Space}}</code>, …). One edit → every project. The binary embeds only examples;
    your own templates live in <code>&lt;config&gt;/templates/</code> — a file at the same path as an
    embedded one replaces it, and new files are added. <code>templates/bundles.yaml</code> declares which
    templates form which bundle; it replaces the embedded registry rather than merging with it.</li>
<li><strong>projects/&lt;name&gt;/{skills,agents}/</strong> — project-specific files copied verbatim
    into both trees. No template pass (they may contain <code>{{</code> legitimately).</li>
<li><strong>global/skills/</strong> — skills for the CLIs you use everywhere (a search tool, a
    browser driver, a database client, …). Copied verbatim into each <em>home</em> listed under
    <code>homes:</code> in the manifest — <code>~/.claude/skills</code>, <code>~/.codex/skills</code>,
    <code>~/.cursor/skills</code>. <code>trafaret install --all</code> covers them; trafaret prunes only what it
    recorded in <code>&lt;skillsDir&gt;/.ao-manifest.json</code>. A home leaves out a harness-specific
    skill with <code>skip:</code> (for example one that relies on Claude Code frontmatter, which Codex
    and Cursor skip).</li>
<li><strong>agents/&lt;role&gt;/</strong> — a <em>role agent</em>: a long-lived named session
    (<code>acme-designer</code>) that other sessions message with short questions. Its
    <code>AGENT.md</code> is the boot document the session reads at start (not installed); its
    <code>skills/</code> are installed into every home exactly like <code>global/skills/</code>,
    so the skills for asking it and the rules it follows are everywhere. Skill names must be unique
    across <code>global/skills</code> and all roles — <code>trafaret validate</code> rejects a
    duplicate.</li>
<li><strong>manifest.yaml</strong> — the registry: repo root, bundles, ticket board protocol,
    branch policy, gate matrix, docs taxonomy per project.</li>
<li><strong>Native harness projections:</strong> Claude Code and Cursor share
    <code>.claude/</code> — Cursor reads <code>.claude/skills</code> and <code>.claude/agents</code>
    natively (cross-agent compatibility), and a parallel <code>.cursor/</code> copy would double-load
    every skill. Codex receives skills in <code>.agents/skills</code>, readable agent contracts in
    <code>.agents/agents</code>, and custom-agent TOML in <code>.codex/agents</code>. The cursor target
    contributes only <code>.cursor/rules/</code>.</li></ul>`,

	"quick_h":  "Quick start",
	"quick_c1": "# build ./trafaret + symlink ~/.local/bin/trafaret (once)",
	"quick_c2": "# projects, targets, bundles",
	"quick_c3": "# manifest sanity + render every template (token typos, YAML lint)",
	"quick_c4": "# dry-run: per-file install/update/skip",
	"quick_c5": "# render + write into that repo",
	"quick_c6": "# every project",
	"quick_c7": "# this page (localhost only)",
	"quick_flags": `Useful flags: <code>--target claude|cursor|codex|both|all</code> ·
<code>--what bundle,…</code> · <code>--dry-run</code> · <code>--force</code> · <code>--json</code>.`,

	"state_h": "Live state",
	"state_p": `<p>Computed now, exactly like <code>trafaret diff</code>: <span class="pill ok">in
sync</span> means the repo's rendered artifacts byte-match what the current sources produce.
<span class="pill warn">pending</span> means sources were edited (or the repo drifted) and
<code>trafaret install</code> is owed — refresh after editing to watch this flip.</p>`,
	"col_project": "project", "col_branch": "branch", "col_space": "ticket space",
	"col_targets": "targets", "col_bundles": "bundles", "col_status": "status",
	"st_error": "error", "st_insync": "in sync", "st_pending": "pending",
	"st_files": "files", "st_new": "new", "st_changed": "changed",

	"bundles_h": "Bundles",
	"bundles_p": `A bundle is a named set of templates; a project opts in via <code>bundles:</code> in the manifest.
Bundles are data, not code: they are defined in <code>templates/bundles.yaml</code>. The embedded file
holds the examples (<code>example</code>, <code>example-db</code>); a <code>bundles.yaml</code> in your
config's <code>templates/</code> overlay <strong>shadows</strong> it — it is not merged — so a config has
exactly the bundles its own file lists. An entry may set <code>skills</code> (directories under
<code>templates/skills/</code>), <code>agents</code> (<code>templates/agents/&lt;name&gt;.md.tmpl</code>),
<code>cursorRules</code> (<code>templates/cursor/rules/&lt;name&gt;.mdc.tmpl</code>), <code>requires</code>
(<code>tickets</code> and/or <code>db</code>: the manifest blocks its templates read, which
<code>trafaret validate</code> checks — a bundle that requires neither renders from the bare project
fields) and <code>sessionContext</code> (a file under <code>.claude/skills/</code> that a SessionStart hook
prints at every session start; at most one active bundle per project). The table below is read live from
the registry in effect.
Switching a bundle off: add it to the project's <code>disabled:</code> list and run
<code>trafaret install</code> — its artifacts are pruned from the repo (tracked via
<code>.claude/ao-manifest.json</code>); remove it from <code>disabled:</code> to bring it back.
A bundle with <code>sessionContext</code> also installs a per-project SessionStart hook — trafaret owns
exactly one entry in the repo's <code>.claude/settings.json</code>, merged additively and removed on
disable; all other settings keys are preserved.`,
	"col_bundle": "bundle", "col_skills": "skills", "col_agents": "agents",
	"col_rules": "cursor rules", "col_usedby": "used by",

	"catalog_h": "Template catalog",
	"catalog_p": `<p>The embedded examples plus whatever your <code>templates/</code> overlay adds. Descriptions
are shown <em>raw</em> — the <code>{{.Token}}</code> expressions are the documentation of what varies per
project. Every description ends with a scope clause and every body opens with a <code>Project:</code>
guard (see Guardrails).</p>`,
	"filter_ph":      "filter the catalog…",
	"catalog_skills": "Skills", "catalog_agents": "Subagents",
	"catalog_rules": "Cursor rules", "catalog_proj": "Per-project verbatim skills",
	"col_skill": "skill", "col_when": "when to use", "col_words": "words",
	"col_agent": "agent", "col_role": "role", "col_rule": "rule", "col_purpose": "purpose",

	"guard_h": "Guardrails & conventions",
	"g1_h":    "Multi-project scope guards",
	"g1_b": `<p>Skill names are identical across repos by design, and Cursor loads every repo's
skills in a multi-root workspace <em>without</em> dedup or directory qualification, while
<code>alwaysApply</code> rules stack from every root. Three defense layers are built into the templates
(the embedded examples follow them; write yours the same way):</p><ol>
<li>description ends with <code>Applies only to the &lt;name&gt; repo at &lt;abs path&gt;.</code> (routing);</li>
<li>body opens with a <code>Project:</code> STOP-guard line (execution);</li>
<li>an <code>alwaysApply</code> Cursor rule carries a workspace scope line (rule stacking).</li></ol>
<p class="desc">Habit that helps: open a small multi-root workspace of 2–3 active repos instead of
the whole parent folder.</p>`,
	"g2_h": "YAML frontmatter lint (render-time)",
	"g2_b": `<p>Every rendered <code>.md/.mdc</code> must parse; <code>SKILL.md</code> needs
non-empty <code>name</code> + <code>description</code>; an unquoted value containing
<code>&nbsp;#</code> is rejected — YAML silently truncates there (this shipped broken twice before
the lint existed). Keep <code>:&nbsp;</code> and <code>&nbsp;#</code> out of unquoted frontmatter
values, including via token values.</p>`,
	"g3_h": "Content doctrine",
	"g3_b": `<p><em>Content lives where something maintains it — otherwise it is dead on arrival.</em>
Stack tutorials nobody maintains rot; durable knowledge belongs in the project's own docs, not in
one-off skills. Templates are <em>process</em> contracts plus thin per-project references.</p>`,
	"g4_h": "Editing discipline",
	"g4_b": `<p>Rendered trees are build artifacts; <code>trafaret</code> is sha256-idempotent, so
re-install is always safe. Commit convention everywhere: developer-style message, no AI
attribution. Rendered-repo commits happen alongside the config commit.</p>`,

	"howto_h": "How-to",
	"ht1_h":   "Edit a shared skill / agent / rule",
	"ht1_diagram": `sequenceDiagram
  participant A as agent / you
  participant OS as config directory
  participant R as target repos
  A->>OS: edit templates/…SKILL.md.tmpl (or manifest.yaml)
  A->>OS: trafaret validate
  Note over OS: token typos + YAML lint caught here
  A->>R: trafaret install --all
  A->>OS: git commit (sources)
  A->>R: git commit (rendered artifacts, per repo)`,
	"ht1_p": `<p>Never edit the rendered copy in a repo — it is overwritten on the next install.
The <a href="#state">Live state</a> table flips to <span class="pill warn">pending</span> the
moment sources and rendered trees diverge.</p>`,
	"ht2_h": "Add a project-specific skill",
	"ht2_c": "# plain SKILL.md, no .tmpl, no tokens",
	"ht2_p": `<p>Frontmatter rules still apply (lint runs on verbatim files too). End the
description with the scope clause naming the repo and its absolute path.</p>`,
	"ht3_h":  "Onboard a new project",
	"ht3_c1": "# relative to defaults.workspace",
	"ht3_c2": "# YAML anchor in manifest.yaml",
	"ht3_c3": "# a name from templates/bundles.yaml",
	"ht3_p": `<p>Verify the ticket space first with your ticket CLI — board and state names in the manifest
must match the workspace exactly. Then <code>trafaret validate &amp;&amp; trafaret install myproj</code>.</p>`,
	"ht4_h": "Add your own bundle",
	"ht4_p": `<p>Write the template at <code>&lt;config&gt;/templates/skills/&lt;name&gt;/SKILL.md.tmpl</code>
(or under <code>agents/</code> / <code>cursor/rules/</code>), list it under a bundle in
<code>&lt;config&gt;/templates/bundles.yaml</code> — set <code>requires</code> if it reads the ticket
protocol or <code>tools.db</code> — and add the bundle's name to a project's <code>bundles:</code>.
Your own <code>bundles.yaml</code> replaces the embedded one, so copy any embedded entry you still want.
What a template assumes (a board, real gate commands, a docs layout) is yours to verify — see
<code>docs/onboarding.md</code>; don't fake gates a skill will rely on.</p>`,
	"ht5_h":       "Troubleshooting",
	"col_symptom": "symptom", "col_fix": "cause / fix",
	"trouble_rows": `<tr><td><code>render error: … map has no entry for key</code></td><td>token typo
in a template or a missing manifest field — <code>trafaret validate</code> names the file; check
<code>docs/token-reference.md</code></td></tr>
<tr><td><code>frontmatter … contains ' #'</code></td><td>the YAML truncation trap — quote the value
or drop the <code>#</code></td></tr>
<tr><td><code>no bundles and no projects/… sources</code></td><td>manifest entry installs nothing —
add a bundle or a <code>projects/&lt;name&gt;/</code> dir</td></tr>
<tr><td>a repo's skills reappeared after deletion</td><td>a concurrent agent "restored" them —
re-delete with an explicit commit message; deleting a rendered tree must also remove the source
here, or install brings it back</td></tr>
<tr><td>a skill shows up twice in Cursor</td><td>someone re-created <code>.cursor/skills</code> —
delete it; Cursor reads <code>.claude/skills</code> natively and does not dedupe the copies</td></tr>
<tr><td>agent used another project's space/branch</td><td>scope guards exist, but check the
workspace layout — prefer a small multi-root workspace over opening a whole monorepo</td></tr>
<tr><td><code>bundle "…" is not in templates/bundles.yaml</code></td><td>the registry is the config's
own <code>templates/bundles.yaml</code>, which shadows the embedded one — add the bundle there (or copy
back the embedded entries you still use)</td></tr>`,

	"where_h":    "Where do I edit…",
	"col_change": "I want to change…", "col_edit": "edit this", "col_then": "then",
	"where_rows": `<tr><td>a shared skill's behavior (all projects)</td><td><code>&lt;config&gt;/templates/skills/&lt;name&gt;/SKILL.md.tmpl</code></td><td rowspan="7"><code>trafaret validate</code><br><code>trafaret install --all</code><br>commit sources + rendered repos</td></tr>
<tr><td>a subagent's contract</td><td><code>&lt;config&gt;/templates/agents/&lt;name&gt;.md.tmpl</code> <span class="desc">(keep it a pointer — the skill is the single source of the checklist)</span></td></tr>
<tr><td>a Cursor rule</td><td><code>&lt;config&gt;/templates/cursor/rules/&lt;name&gt;.mdc.tmpl</code></td></tr>
<tr><td>a project's board / branch / gates / docs paths</td><td>that project's block in <code>manifest.yaml</code></td></tr>
<tr><td>a skill for exactly one project</td><td><code>projects/&lt;name&gt;/skills/&lt;skill&gt;/SKILL.md</code></td></tr>
<tr><td>a skill for one of your CLIs, in every repo and harness</td><td><code>global/skills/&lt;skill&gt;/SKILL.md</code> <span class="desc">(homes: in <code>manifest.yaml</code>)</span></td></tr>
<tr><td>a role agent's boot doc or one of its skills</td><td><code>agents/&lt;role&gt;/AGENT.md</code> · <code>agents/&lt;role&gt;/skills/&lt;skill&gt;/SKILL.md</code> <span class="desc">(skills land in the homes like global ones)</span></td></tr>
<tr><td>which artifacts a project gets</td><td><code>bundles:</code> list in <code>manifest.yaml</code></td></tr>
<tr><td>which templates a bundle installs</td><td><code>&lt;config&gt;/templates/bundles.yaml</code></td></tr>
<tr><td>what tokens exist / mean</td><td colspan="2"><code>docs/token-reference.md</code> (the template contract)</td></tr>
<tr><td>this page</td><td colspan="2"><code>site_page.go</code> (skeleton) + <code>site_strings.go</code> (EN/RU prose) — the tables fix themselves</td></tr>`,

	"side_live": "re-rendered from the repo on every refresh",
	"prev":      "previous", "next": "next",
}

var siteRU = map[string]h{
	"title": "руководство оператора",

	"nav_model": "Обзор", "nav_quick": "Быстрый старт", "nav_state": "Состояние",
	"nav_bundles": "Бандлы", "nav_catalog": "Каталог",
	"nav_guard": "Ограждения", "nav_howto": "Как сделать", "nav_where": "Что где править",

	"hero_h1": "trafaret — руководство оператора",
	"hero_p": `Единая точка правды для агентских скилов, сабагентов и правил, которые вы ставите в свои
проекты. Страница рендерится из каталога конфигурации — <code>manifest.yaml</code> +
шаблоны (встроенные примеры плюс ваш оверлей <code>templates/</code> с его <code>bundles.yaml</code>) +
<code>projects/</code> — при <em>каждом обновлении</em>: правка агентом этих файлов <em>и есть</em>
правка этого сайта.`,
	"rendered_at": "отрендерено", "local_only": "только локально, никуда не публикуется",
	"footer_refresh": "обновите страницу (⌘R), чтобы перечитать репозиторий",

	"model_h": "Модель",
	"model_p": `<p>Репозитории не владеют своей агентской конфигурацией. Три источника в
<em>вашем каталоге конфигурации</em> порождают все артефакты; отрендеренные деревья <code>.claude/</code>,
<code>.cursor/</code>, <code>.agents/</code> и <code>.codex/</code> в целевых репо — <strong>build-артефакты, руками их не правят</strong>
(следующий <code>trafaret install</code> перезапишет; конкурентный агент, «восстановивший» удалённые
файлы, уже случался). Claude Code и Cursor делят одно дерево; Codex получает нативные проекции
скилов и custom agents.</p>`,
	"model_diagram": `flowchart LR
  E["встроенные примеры<br/>(внутри бинарника)"]
  subgraph SRC["каталог конфигурации (править только здесь)"]
    T["оверлей templates/<br/>bundles.yaml · скилы · агенты · правила"]
    P["projects/⟨name⟩/<br/>дословные скилы проекта<br/>(копируются как есть)"]
    M["manifest.yaml<br/>значения токенов по проектам"]
  end
  AO(("trafaret install"))
  E --> AO
  T --> AO
  P --> AO
  M --> AO
  AO --> C["⟨repo⟩/.claude/<br/>skills + agents<br/>(читают И Claude Code, И Cursor)"]
  AO --> U["⟨repo⟩/.cursor/rules/<br/>Cursor-правила (только Cursor)"]
  AO --> X["⟨repo⟩/.agents/skills + .agents/agents<br/>.codex/agents · Codex"]
  G["global/skills/"] --> AO
  R["agents/⟨role⟩/skills/"] --> AO
  AO --> H["~/.claude · ~/.codex · ~/.cursor<br/>skills (homes)"]`,
	"model_list": `<ul>
<li><strong>templates/</strong> — общие, токенизированные (<code>{{.Name}}</code>,
    <code>{{.Tickets.Space}}</code>, …). Одна правка → все проекты. В бинарник встроены только
    примеры; ваши шаблоны живут в <code>&lt;config&gt;/templates/</code> — файл по тому же пути, что и
    встроенный, заменяет его, новые файлы добавляются. <code>templates/bundles.yaml</code> объявляет, какие
    шаблоны образуют какой бандл; он заменяет встроенный реестр, а не сливается с ним.</li>
<li><strong>projects/&lt;name&gt;/{skills,agents}/</strong> — файлы конкретного проекта, копируются
    дословно в оба дерева. Без шаблонного прохода (могут легитимно содержать <code>{{</code>).</li>
<li><strong>global/skills/</strong> — скилы для CLI, которыми вы пользуетесь везде (поиск,
    управление браузером, клиент БД, …). Копируются дословно в каждый <em>home</em> из
    <code>homes:</code> манифеста — <code>~/.claude/skills</code>, <code>~/.codex/skills</code>,
    <code>~/.cursor/skills</code>. <code>trafaret install --all</code> ставит и их; trafaret удаляет только то, что
    сам записал в <code>&lt;skillsDir&gt;/.ao-manifest.json</code>. Скил под один харнес home пропускает
    через <code>skip:</code> (например, тот, что держится на frontmatter Claude Code, — Codex и Cursor
    его пропускают).</li>
<li><strong>agents/&lt;role&gt;/</strong> — <em>ролевой агент</em>: долгоживущая именованная
    сессия (<code>acme-designer</code>), которой другие сессии шлют короткие вопросы. Её
    <code>AGENT.md</code> — boot-документ, сессия читает его на старте (никуда не ставится);
    её <code>skills/</code> ставятся в каждый home ровно как <code>global/skills/</code>, так
    что скилы, которыми к ней обращаются, и правила, которым она следует, есть везде. Имена скилов
    уникальны поверх <code>global/skills</code> и всех ролей — <code>trafaret validate</code>
    отвергает дубль.</li>
<li><strong>manifest.yaml</strong> — реестр: корень репо, бандлы, протокол тикет-досок, политика
    веток, матрица гейтов, таксономия доков по каждому проекту.</li>
<li><strong>Нативные проекции харнесов:</strong> Claude Code и Cursor делят
    <code>.claude/</code> — Cursor читает <code>.claude/skills</code> и <code>.claude/agents</code>
    нативно (cross-agent совместимость), а параллельная копия в <code>.cursor/</code> двоила бы
    каждый скил. Codex получает скилы в <code>.agents/skills</code>, читаемые контракты
    агентов в <code>.agents/agents</code> и custom-agent TOML в <code>.codex/agents</code>.
    Cursor-target даёт только <code>.cursor/rules/</code>.</li></ul>`,

	"quick_h":  "Быстрый старт",
	"quick_c1": "# собрать ./trafaret + симлинк ~/.local/bin/trafaret (один раз)",
	"quick_c2": "# проекты, targets, бандлы",
	"quick_c3": "# проверка манифеста + рендер всех шаблонов (опечатки токенов, YAML-линт)",
	"quick_c4": "# dry-run: install/update/skip по файлам",
	"quick_c5": "# отрендерить и записать в репо",
	"quick_c6": "# все проекты",
	"quick_c7": "# эта страница (только localhost)",
	"quick_flags": `Полезные флаги: <code>--target claude|cursor|codex|both|all</code> ·
<code>--what bundle,…</code> · <code>--dry-run</code> · <code>--force</code> · <code>--json</code>.`,

	"state_h": "Живое состояние",
	"state_p": `<p>Считается прямо сейчас, ровно как <code>trafaret diff</code>: <span class="pill
ok">синхронно</span> — артефакты в репо байт-в-байт совпадают с тем, что дают текущие источники.
<span class="pill warn">ожидает</span> — источники правились (или репо разъехалось) и нужен
<code>trafaret install</code>; обновите страницу после правки и статус переключится.</p>`,
	"col_project": "проект", "col_branch": "ветка", "col_space": "тикет-спейс",
	"col_targets": "targets", "col_bundles": "бандлы", "col_status": "статус",
	"st_error": "ошибка", "st_insync": "синхронно", "st_pending": "ожидает",
	"st_files": "файлов", "st_new": "новых", "st_changed": "изменённых",

	"bundles_h": "Бандлы",
	"bundles_p": `Бандл — именованный набор шаблонов; проект подключает его через <code>bundles:</code> в манифесте.
Бандлы — это данные, а не код: они описаны в <code>templates/bundles.yaml</code>. Встроенный файл
содержит примеры (<code>example</code>, <code>example-db</code>); <code>bundles.yaml</code> в оверлее
<code>templates/</code> вашей конфигурации <strong>перекрывает</strong> его — слияния нет, — так что у
конфигурации ровно те бандлы, что перечислены в её собственном файле. Запись может задавать
<code>skills</code> (каталоги в <code>templates/skills/</code>), <code>agents</code>
(<code>templates/agents/&lt;name&gt;.md.tmpl</code>), <code>cursorRules</code>
(<code>templates/cursor/rules/&lt;name&gt;.mdc.tmpl</code>), <code>requires</code>
(<code>tickets</code> и/или <code>db</code>: блоки манифеста, которые читают её шаблоны, —
<code>trafaret validate</code> их проверяет; бандл без <code>requires</code> рендерится только из базовых
полей проекта) и <code>sessionContext</code> (файл в <code>.claude/skills/</code>, который SessionStart-hook
печатает при каждом старте сессии; не больше одного активного бандла на проект). Таблица ниже читается
вживую из действующего реестра.
Выключение бандла: добавьте его в <code>disabled:</code> проекта и запустите
<code>trafaret install</code> — его артефакты будут удалены из репозитория (трекинг через
<code>.claude/ao-manifest.json</code>); уберите из <code>disabled:</code> — вернутся.
Бандл с <code>sessionContext</code> также ставит per-project SessionStart-hook — trafaret владеет ровно
одной записью в <code>.claude/settings.json</code> репозитория: добавляется аддитивно, убирается при
disable; остальные ключи настроек не трогаются.`,
	"col_bundle": "бандл", "col_skills": "скилы", "col_agents": "агенты",
	"col_rules": "cursor-правила", "col_usedby": "кто использует",

	"catalog_h": "Каталог шаблонов",
	"catalog_p": `<p>Встроенные примеры плюс то, что добавляет ваш оверлей <code>templates/</code>. Описания
показаны <em>сырыми</em> — выражения <code>{{.Token}}</code> и есть документация того, что варьируется
по проектам. Каждое описание заканчивается scope-клаузой, каждое тело открывается гардом
<code>Project:</code> (см. «Ограждения»).</p>`,
	"filter_ph":      "фильтр по каталогу…",
	"catalog_skills": "Скилы", "catalog_agents": "Сабагенты",
	"catalog_rules": "Cursor-правила", "catalog_proj": "Дословные скилы проектов",
	"col_skill": "скил", "col_when": "когда использовать", "col_words": "слов",
	"col_agent": "агент", "col_role": "роль", "col_rule": "правило", "col_purpose": "назначение",

	"guard_h": "Ограждения и конвенции",
	"g1_h":    "Scope-гарды для мультипроекта",
	"g1_b": `<p>Имена скилов одинаковы во всех репо by design, а Cursor в multi-root workspace
грузит скилы всех репо <em>без</em> дедупликации и директорийной квалификации, и правила с
<code>alwaysApply</code> стекаются из каждого корня. В шаблоны встроены три слоя защиты
(встроенные примеры им следуют; пишите свои так же):</p><ol>
<li>описание заканчивается <code>Applies only to the &lt;name&gt; repo at &lt;abs path&gt;.</code> (роутинг);</li>
<li>тело открывается STOP-гардом <code>Project:</code> (исполнение);</li>
<li>Cursor-правило с <code>alwaysApply</code> несёт строку workspace-гарда (стекание правил).</li></ol>
<p class="desc">Полезная привычка: открывать маленький multi-root workspace из 2–3 активных репо
вместо всей родительской папки.</p>`,
	"g2_h": "YAML-линт frontmatter (на рендере)",
	"g2_b": `<p>Каждый отрендеренный <code>.md/.mdc</code> обязан парситься; у <code>SKILL.md</code>
— непустые <code>name</code> + <code>description</code>; незакавыченное значение с
<code>&nbsp;#</code> отклоняется — YAML молча обрезает там строку (до линта это дважды уезжало в
прод сломанным). Держите <code>:&nbsp;</code> и <code>&nbsp;#</code> вне незакавыченных значений,
в том числе через значения токенов.</p>`,
	"g3_h": "Доктрина контента",
	"g3_b": `<p><em>Контент живёт там, где его кто-то поддерживает, — иначе он мёртв с рождения.</em>
Стек-туториалы без владельца гниют; долговечное знание живёт в собственных доках проекта, а не в
разовых скилах. Шаблоны — <em>процессные</em> контракты плюс тонкие справочники по проектам.</p>`,
	"g4_h": "Дисциплина правок",
	"g4_b": `<p>Отрендеренные деревья — build-артефакты; <code>trafaret</code> sha256-идемпотентен,
переустановка всегда безопасна. Конвенция коммитов везде: developer-style сообщение, без
AI-атрибуции. Коммиты в отрендеренных репо идут вместе с коммитом конфигурации.</p>`,

	"howto_h": "Как сделать",
	"ht1_h":   "Править общий скил / агент / правило",
	"ht1_diagram": `sequenceDiagram
  participant A as агент / вы
  participant OS as каталог конфигурации
  participant R as целевые репо
  A->>OS: правка templates/…SKILL.md.tmpl (или manifest.yaml)
  A->>OS: trafaret validate
  Note over OS: опечатки токенов + YAML-линт ловятся здесь
  A->>R: trafaret install --all
  A->>OS: git commit (источники)
  A->>R: git commit (артефакты, по каждому репо)`,
	"ht1_p": `<p>Никогда не правьте отрендеренную копию в репо — следующий install её перезапишет.
Таблица <a href="#state">живого состояния</a> переключается в <span class="pill
warn">ожидает</span>, как только источники и рендеры разъехались.</p>`,
	"ht2_h": "Добавить скил одного проекта",
	"ht2_c": "# обычный SKILL.md, без .tmpl и токенов",
	"ht2_p": `<p>Правила frontmatter действуют и здесь (линт гоняется и по дословным файлам).
Заканчивайте описание scope-клаузой с именем репо и абсолютным путём.</p>`,
	"ht3_h":  "Подключить новый проект",
	"ht3_c1": "# относительно defaults.workspace",
	"ht3_c2": "# YAML-якорь в manifest.yaml",
	"ht3_c3": "# имя из templates/bundles.yaml",
	"ht3_p": `<p>Сначала сверьте тикет-спейс через вашу тикет-CLI — имена досок и состояний в манифесте
должны точно совпадать с workspace. Затем <code>trafaret validate &amp;&amp; trafaret install myproj</code>.</p>`,
	"ht4_h": "Добавить свой бандл",
	"ht4_p": `<p>Напишите шаблон в <code>&lt;config&gt;/templates/skills/&lt;name&gt;/SKILL.md.tmpl</code>
(или в <code>agents/</code> / <code>cursor/rules/</code>), перечислите его в бандле в
<code>&lt;config&gt;/templates/bundles.yaml</code> — задайте <code>requires</code>, если он читает
тикет-протокол или <code>tools.db</code> — и добавьте имя бандла в <code>bundles:</code> проекта.
Ваш <code>bundles.yaml</code> заменяет встроенный, так что скопируйте нужные встроенные записи.
Предпосылки шаблона (доска, реально работающие гейты, раскладка доков) проверяете вы — см.
<code>docs/onboarding.md</code>; не подделывайте гейты, на которые будет опираться скил.</p>`,
	"ht5_h":       "Диагностика",
	"col_symptom": "симптом", "col_fix": "причина / что делать",
	"trouble_rows": `<tr><td><code>render error: … map has no entry for key</code></td><td>опечатка
токена в шаблоне или отсутствующее поле манифеста — <code>trafaret validate</code> назовёт файл; сверьтесь
с <code>docs/token-reference.md</code></td></tr>
<tr><td><code>frontmatter … contains ' #'</code></td><td>ловушка YAML-обрезания — закавычьте
значение или уберите <code>#</code></td></tr>
<tr><td><code>no bundles and no projects/… sources</code></td><td>запись манифеста ничего не
устанавливает — добавьте бандл или каталог <code>projects/&lt;name&gt;/</code></td></tr>
<tr><td>удалённые скилы в репо «воскресли»</td><td>конкурентный агент их «восстановил» — удалите
снова с явным сообщением коммита; удаляя рендер, удаляйте и источник здесь, иначе install вернёт
всё назад</td></tr>
<tr><td>скил виден в Cursor дважды</td><td>кто-то заново создал <code>.cursor/skills</code> —
удалите его; Cursor нативно читает <code>.claude/skills</code> и копии не дедуплицирует</td></tr>
<tr><td>агент применил чужой спейс/ветку</td><td>scope-гарды стоят, но проверьте раскладку
workspace — лучше маленький multi-root, чем открытый целиком монорепо</td></tr>
<tr><td><code>bundle "…" is not in templates/bundles.yaml</code></td><td>реестр — это собственный
<code>templates/bundles.yaml</code> конфигурации, он перекрывает встроенный — добавьте бандл туда (или
верните нужные встроенные записи)</td></tr>`,

	"where_h":    "Что где править",
	"col_change": "хочу поменять…", "col_edit": "правьте это", "col_then": "затем",
	"where_rows": `<tr><td>поведение общего скила (все проекты)</td><td><code>&lt;config&gt;/templates/skills/&lt;name&gt;/SKILL.md.tmpl</code></td><td rowspan="7"><code>trafaret validate</code><br><code>trafaret install --all</code><br>коммит источников + отрендеренных репо</td></tr>
<tr><td>контракт сабагента</td><td><code>&lt;config&gt;/templates/agents/&lt;name&gt;.md.tmpl</code> <span class="desc">(держите его указателем — скил остаётся единственным источником чеклиста)</span></td></tr>
<tr><td>Cursor-правило</td><td><code>&lt;config&gt;/templates/cursor/rules/&lt;name&gt;.mdc.tmpl</code></td></tr>
<tr><td>доску / ветку / гейты / пути доков проекта</td><td>блок этого проекта в <code>manifest.yaml</code></td></tr>
<tr><td>скил ровно одного проекта</td><td><code>projects/&lt;name&gt;/skills/&lt;skill&gt;/SKILL.md</code></td></tr>
<tr><td>скил моей CLI — во всех репо и харнесах</td><td><code>global/skills/&lt;skill&gt;/SKILL.md</code> <span class="desc">(homes: в <code>manifest.yaml</code>)</span></td></tr>
<tr><td>boot-док ролевого агента или его скил</td><td><code>agents/&lt;role&gt;/AGENT.md</code> · <code>agents/&lt;role&gt;/skills/&lt;skill&gt;/SKILL.md</code> <span class="desc">(скилы ставятся в homes как глобальные)</span></td></tr>
<tr><td>какие артефакты получает проект</td><td>список <code>bundles:</code> в <code>manifest.yaml</code></td></tr>
<tr><td>какие шаблоны ставит бандл</td><td><code>&lt;config&gt;/templates/bundles.yaml</code></td></tr>
<tr><td>какие есть токены и что они значат</td><td colspan="2"><code>docs/token-reference.md</code> (контракт шаблонов)</td></tr>
<tr><td>эту страницу</td><td colspan="2"><code>site_page.go</code> (скелет) + <code>site_strings.go</code> (тексты EN/RU) — таблицы чинятся сами</td></tr>`,

	"side_live": "перерендеривается из репозитория при каждом обновлении",
	"prev":      "назад", "next": "дальше",
}

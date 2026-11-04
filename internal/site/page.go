package site

import "html/template"

// The trafaret serve site: a multi-page layout — fixed full-height sidebar, one page
// per section, full-viewport content. One skeleton template + per-language
// string maps (strings.go); the page is chosen via .Page.

type sitePageData struct {
	*siteData
	Lang     string // "en" | "ru"
	Page     string // slug from sitePages
	T        map[string]template.HTML
	PageList []sitePageLink
	Prev     *sitePageLink
	Next     *sitePageLink
}

type sitePageLink struct {
	Slug  string
	Title template.HTML
	No    string // "01".."06", for the sidebar
}

// sitePages is the navigation order; titles come from the T map by key.
var sitePages = []struct{ Slug, TitleKey string }{
	{"overview", "nav_model"},
	{"state", "nav_state"},
	{"catalog", "nav_catalog"},
	{"guard", "nav_guard"},
	{"howto", "nav_howto"},
	{"where", "nav_where"},
}

const siteSkeleton = `<!DOCTYPE html>
<html lang="{{.Lang}}">
<head>
<meta charset="utf-8">
<title>trafaret · {{range .PageList}}{{if eq $.Page .Slug}}{{.Title}}{{end}}{{end}}</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
  :root {
    --bg: #fbfbfc; --fg: #1b1e23; --muted: #67707b; --line: #e4e7eb;
    --card: #ffffff; --soft: #f2f4f7; --accent: #0b6bcb; --accent-soft: #e8f1fb;
    --ok: #1a7f37; --ok-soft: #e3f2e8; --warn: #b35900; --warn-soft: #fdf0e3;
    --bad: #c62828; --bad-soft: #fdecea; --code-bg: #f0f2f4;
    --side-bg: #14171c; --side-fg: #c8cfd8; --side-muted: #78828e; --side-line: #242a32;
    --shadow: 0 1px 3px rgba(15,23,42,.06);
  }
  @media (prefers-color-scheme: dark) {
    :root {
      --bg: #0e1013; --fg: #d9dee5; --muted: #8b949e; --line: #262c33;
      --card: #15181d; --soft: #1a1e24; --accent: #58a6ff; --accent-soft: #12233a;
      --ok: #3fb950; --ok-soft: #12291a; --warn: #d29922; --warn-soft: #2b2214;
      --bad: #f85149; --bad-soft: #331a1a; --code-bg: #1c2027;
      --side-bg: #0a0c0f; --side-line: #1c2127; --shadow: none;
    }
  }
  * { box-sizing: border-box; }
  body { margin: 0; background: var(--bg); color: var(--fg); min-height: 100vh;
         font: 15px/1.65 -apple-system, "SF Pro Text", "Segoe UI", sans-serif;
         display: flex; }
  h1, h2, h3, h4 { font-family: "Avenir Next", -apple-system, sans-serif; }
  .prose, .card p, .card ol, .card ul { font-family: "Charter", Georgia, serif; font-size: 15.5px; }
  .prose code, .card code { font-family: ui-monospace, "SF Mono", Menlo, monospace; }
  .prose { max-width: 72rem; }

  /* ---- sidebar ---- */
  aside {
    width: 232px; flex: none; background: var(--side-bg); color: var(--side-fg);
    display: flex; flex-direction: column; position: sticky; top: 0; height: 100vh;
    border-right: 1px solid var(--side-line);
  }
  .sbrand { padding: 1.3rem 1.2rem 1rem; font-family: "Avenir Next", -apple-system, sans-serif; }
  .sbrand b { font-size: 1.05rem; letter-spacing: -.01em; color: #fff; }
  .sbrand small { display: block; color: var(--side-muted); font-size: .78rem; margin-top: .15rem; }
  aside nav { flex: 1; padding: .4rem .6rem; overflow-y: auto; }
  aside nav a {
    display: flex; align-items: center; gap: .65rem; padding: .48rem .65rem; margin: .1rem 0;
    color: var(--side-fg); text-decoration: none; border-radius: 8px; font-size: .875rem;
  }
  aside nav a .n { font: 700 .68rem ui-monospace, Menlo, monospace; color: var(--side-muted);
    width: 1.15rem; text-align: right; flex: none; }
  aside nav a:hover { background: rgba(255,255,255,.05); }
  aside nav a.cur { background: rgba(88,166,255,.14); color: #fff; }
  aside nav a.cur .n { color: #58a6ff; }
  .sfoot { padding: .9rem 1.2rem 1.1rem; border-top: 1px solid var(--side-line);
    display: flex; flex-direction: column; gap: .6rem; }
  .langsw { display: flex; border: 1px solid var(--side-line); border-radius: 8px;
    overflow: hidden; font-size: .78rem; align-self: flex-start; }
  .langsw a { padding: .28rem .8rem; text-decoration: none; color: var(--side-muted); font-weight: 600; }
  .langsw a.cur { background: #58a6ff; color: #0a0c0f; }
  .slive { font-size: .72rem; color: var(--side-muted); line-height: 1.5; }
  .slive .live { margin-right: .3rem; }

  /* ---- content ---- */
  .content { flex: 1; min-width: 0; padding: 2.4rem clamp(1.2rem, 3.5vw, 3.2rem) 4rem; }
  .pagehead { margin: 0 0 1.6rem; }
  .pagehead .crumb { font: 700 .72rem ui-monospace, Menlo, monospace; color: var(--accent);
    letter-spacing: .08em; text-transform: uppercase; }
  .pagehead h1 { font-size: 1.75rem; letter-spacing: -.02em; margin: .2rem 0 0; }
  .live { display: inline-block; padding: .1rem .55rem; border-radius: 999px;
    background: var(--ok-soft); color: var(--ok); font-size: .72rem; font-weight: 700;
    letter-spacing: .04em; vertical-align: 3px; }
  .stamp { color: var(--muted); font-size: .82rem; margin-top: .4rem; }

  h2 { font-size: 1.18rem; letter-spacing: -.01em; margin: 2.6rem 0 .6rem; }
  h3 { font-size: 1.02rem; margin: 1.7rem 0 .45rem; }
  .muted { color: var(--muted); }

  code, pre { font: 12.5px/1.55 ui-monospace, "SF Mono", Menlo, monospace; }
  code { background: var(--code-bg); padding: .1rem .35rem; border-radius: 4px; }
  pre { background: var(--card); border: 1px solid var(--line); border-radius: 10px;
    padding: .85rem 1.05rem; overflow-x: auto; box-shadow: var(--shadow); max-width: 60rem; }
  pre code { background: none; padding: 0; }

  .tablewrap { overflow-x: auto; background: var(--card); border: 1px solid var(--line);
    border-radius: 10px; box-shadow: var(--shadow); margin: .7rem 0 1.1rem; }
  table { border-collapse: collapse; width: 100%; font-size: .875rem; }
  th, td { text-align: left; padding: .5rem .75rem; border-bottom: 1px solid var(--line); vertical-align: top; }
  tr:last-child td { border-bottom: none; }
  th { color: var(--muted); font-weight: 600; white-space: nowrap; font-size: .78rem;
    text-transform: uppercase; letter-spacing: .05em; background: var(--soft); }
  tbody tr:hover { background: var(--soft); }
  .desc { color: var(--muted); font-size: .84rem; }
  .desc code { font-size: .78rem; }

  .pill { display: inline-block; padding: .1rem .55rem; border-radius: 999px; font-size: .78rem; font-weight: 700; }
  .pill.ok  { background: var(--ok-soft);  color: var(--ok); }
  .pill.warn{ background: var(--warn-soft); color: var(--warn); }
  .pill.bad { background: var(--bad-soft); color: var(--bad); }
  .pill.origin { font-size: .72rem; font-weight: 600; letter-spacing: .02em;
    background: var(--soft); color: var(--muted); border: 1px solid var(--line); }
  .pill.origin.templates { background: var(--accent-soft); color: var(--accent); border-color: transparent; }
  .pill.origin.global { background: var(--ok-soft); color: var(--ok); border-color: transparent; }
  .pill.origin.role { background: var(--warn-soft); color: var(--warn); border-color: transparent; }

  .notice { border: 1px solid var(--line); border-left-width: 4px; border-radius: 10px;
    padding: .8rem 1.05rem; margin: 1rem 0; background: var(--card); box-shadow: var(--shadow); }
  .notice.bad { border-color: var(--bad); background: var(--bad-soft); }
  .notice.warn { border-color: var(--warn); background: var(--warn-soft); }
  .notice strong { display: block; margin-bottom: .25rem; }
  .notice code { background: none; padding: 0; white-space: pre-wrap; word-break: break-word; }

  .cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(340px, 1fr)); gap: .9rem; margin: 1rem 0; }
  .card { background: var(--card); border: 1px solid var(--line); border-radius: 12px;
    padding: 1rem 1.15rem; box-shadow: var(--shadow); }
  .card h4 { margin: 0 0 .4rem; font-size: .95rem; }
  .card p, .card ol { margin: .4rem 0; font-size: .88rem; }

  .mermaid { background: var(--card); border: 1px solid var(--line); border-radius: 12px;
    padding: 1rem; margin: 1rem 0; overflow-x: auto; text-align: center; box-shadow: var(--shadow); }

  .filter { width: 100%; max-width: 24rem; padding: .45rem .8rem; margin: .3rem 0 .4rem;
    border: 1px solid var(--line); border-radius: 8px; background: var(--card); color: var(--fg); font-size: .88rem; }
  .filter:focus { outline: 2px solid var(--accent-soft); border-color: var(--accent); }

  .tbltitle { display: flex; align-items: baseline; gap: .6rem; padding: .65rem .75rem .55rem;
    border-bottom: 1px solid var(--line); background: var(--soft); border-radius: 10px 10px 0 0; }
  .tbltitle strong { font-family: "Avenir Next", -apple-system, sans-serif; font-size: .95rem; }
  .count { background: var(--accent-soft); color: var(--accent); border-radius: 999px;
    padding: .05rem .55rem; font-size: .74rem; font-weight: 700; }

  .pager { display: flex; justify-content: space-between; gap: 1rem; margin-top: 3.2rem;
    border-top: 1px solid var(--line); padding-top: 1.1rem; font-size: .92rem; }
  .pager a { text-decoration: none; color: var(--accent); font-weight: 600;
    display: inline-flex; align-items: center; gap: .5rem; }
  .pager .lbl { display: block; color: var(--muted); font-size: .7rem; font-weight: 500;
    text-transform: uppercase; letter-spacing: .06em; }

  a { color: var(--accent); }

  /* narrow screens: sidebar becomes a top bar */
  @media (max-width: 880px) {
    body { flex-direction: column; }
    aside { width: 100%; height: auto; position: static; flex-direction: row;
      align-items: center; flex-wrap: wrap; }
    .sbrand { padding: .7rem 1rem; }
    aside nav { display: flex; flex-wrap: wrap; padding: .3rem; }
    aside nav a { padding: .3rem .55rem; }
    .sfoot { border-top: none; margin-left: auto; flex-direction: row; align-items: center; }
    .slive { display: none; }
  }
</style>
</head>
<body>

<aside>
  <div class="sbrand"><b>trafaret</b><small>{{.T.title}}</small></div>
  <nav>
    {{$d := .}}
    {{range $p := .PageList}}<a href="/{{$d.Lang}}/{{$p.Slug}}"{{if eq $d.Page $p.Slug}} class="cur"{{end}}><span class="n">{{$p.No}}</span>{{$p.Title}}</a>
    {{end}}
  </nav>
  <div class="sfoot">
    <span class="langsw">
      <a href="/en/{{.Page}}"{{if eq .Lang "en"}} class="cur"{{end}} data-setlang="en">EN</a>
      <a href="/ru/{{.Page}}"{{if eq .Lang "ru"}} class="cur"{{end}} data-setlang="ru">RU</a>
    </span>
    <span class="slive"><span class="live">LIVE</span>{{.T.side_live}}<br>{{.GeneratedAt}}</span>
  </div>
</aside>

<div class="content">

{{if eq .Page "overview"}}
  <div class="pagehead">
    <span class="crumb">trafaret</span>
    <h1>{{.T.hero_h1}}</h1>
    <div class="prose"><p>{{.T.hero_p}} <span class="live">LIVE</span></p></div>
    <p class="stamp">{{.T.rendered_at}} {{.GeneratedAt}} · <code>{{.RepoDir}}</code> · {{.T.local_only}}</p>
  </div>
  <div class="prose">{{.T.model_p}}</div>
  <div class="mermaid">{{.T.model_diagram}}</div>
  <div class="prose">{{.T.model_list}}</div>
  <h2>{{.T.adapters_h}}</h2>
  <div class="prose">{{.T.adapters_p}}</div>
  <div class="tablewrap"><table>
  <thead><tr><th>{{.T.col_item}}</th><th>claude</th><th>codex</th><th>cursor</th></tr></thead>
  <tbody>{{.T.adapters_rows}}</tbody>
  </table></div>
  <h2>{{.T.quick_h}}</h2>
  <pre><code>make install                       {{.T.quick_c1}}
trafaret list                      {{.T.quick_c2}}
trafaret validate                  {{.T.quick_c3}}
trafaret diff &lt;project|home&gt;       {{.T.quick_c4}}
trafaret install &lt;project|home&gt;    {{.T.quick_c5}}
trafaret install --all             {{.T.quick_c6}}
make site                          {{.T.quick_c7}}</code></pre>
  <p class="prose">{{.T.quick_flags}}</p>
{{end}}

{{if eq .Page "state"}}
  <div class="pagehead">
    <span class="crumb">{{.T.nav_state}}</span>
    <h1>{{.T.state_h}} <span class="live">LIVE</span></h1>
  </div>
  <div class="prose">{{.T.state_p}}</div>

  <h2>{{.T.state_projects_h}}</h2>
  {{if .Projects}}<div class="tablewrap"><table>
  <thead><tr><th>{{.T.col_project}}</th><th>{{.T.col_harnesses}}</th><th>{{.T.col_bundles}}</th><th>{{.T.col_skills}}</th><th>{{.T.col_status}}</th></tr></thead>
  <tbody>
  {{range .Projects}}<tr>
    <td><strong>{{.Name}}</strong><br><span class="desc">{{.RepoRoot}}</span></td>
    <td>{{.Harnesses}}</td>
    <td>{{.Bundles}}</td>
    <td class="desc">{{.Skills}}</td>
    <td>{{if .Err}}<span class="pill bad">{{$.T.st_error}}</span><br><span class="desc">{{.Err}}</span>
        {{else if .InSync}}<span class="pill ok">{{$.T.st_insync}}</span> <span class="desc">{{.Skip}} {{$.T.st_files}}</span>
        {{else}}<span class="pill warn">{{$.T.st_pending}}</span><br><span class="desc">{{.Install}} {{$.T.st_new}} · {{.Update}} {{$.T.st_changed}} → <code>trafaret install {{.Name}}</code></span>{{end}}</td>
  </tr>{{end}}
  </tbody></table></div>{{else}}<p class="desc">{{.T.state_none}}</p>{{end}}

  <h2>{{.T.state_homes_h}}</h2>
  {{if .Homes}}<div class="tablewrap"><table>
  <thead><tr><th>{{.T.col_home}}</th><th>{{.T.col_harness}}</th><th>{{.T.col_bundles}}</th><th>{{.T.col_skills}}</th><th>{{.T.col_status}}</th></tr></thead>
  <tbody>
  {{range .Homes}}<tr>
    <td><strong>{{.Name}}</strong><br><span class="desc">{{.Dir}}</span></td>
    <td>{{.Harness}}</td>
    <td>{{.Bundles}}</td>
    <td class="desc">{{.Skills}}</td>
    <td>{{if .Err}}<span class="pill bad">{{$.T.st_error}}</span><br><span class="desc">{{.Err}}</span>
        {{else if .InSync}}<span class="pill ok">{{$.T.st_insync}}</span> <span class="desc">{{.Skip}} {{$.T.st_files}}</span>
        {{else}}<span class="pill warn">{{$.T.st_pending}}</span><br><span class="desc">{{.Install}} {{$.T.st_new}} · {{.Update}} {{$.T.st_changed}} → <code>trafaret install {{.Name}}</code></span>{{end}}</td>
  </tr>{{end}}
  </tbody></table></div>{{else}}<p class="desc">{{.T.state_none}}</p>{{end}}
{{end}}

{{if eq .Page "catalog"}}
  <div class="pagehead">
    <span class="crumb">{{.T.nav_catalog}}</span>
    <h1>{{.T.catalog_h}} <span class="live">LIVE</span></h1>
  </div>
  <div class="prose">{{.T.catalog_p}}</div>
  {{if .SourceErr}}<div class="notice bad"><strong>{{.T.src_err_h}}</strong><code>{{.SourceErr}}</code><p class="desc">{{.T.src_err_p}}</p></div>
  {{else}}
  <input class="filter" id="skillfilter" type="search" placeholder="{{.T.filter_ph}}">

  <div class="tablewrap"><div class="tbltitle"><strong>{{.T.catalog_skills}}</strong><span class="count">{{len .Skills}}</span></div><table class="filterable">
  <thead><tr><th>{{.T.col_skill}}</th><th>{{.T.col_when}}</th><th>{{.T.col_source}}</th><th>{{.T.col_origin}}</th><th class="muted">{{.T.col_words}}</th></tr></thead>
  <tbody>
  {{range .Skills}}<tr>
    <td><strong>{{.Name}}</strong></td>
    <td class="desc">{{.Description}}</td>
    <td class="desc"><code>{{.Path}}</code></td>
    <td><span class="pill origin {{.Origin}}">{{.Origin}}</span></td>
    <td>{{.Words}}</td>
  </tr>{{end}}
  </tbody></table></div>

  <div class="tablewrap"><div class="tbltitle"><strong>{{.T.catalog_agents}}</strong><span class="count">{{len .Agents}}</span> <span class="desc">{{.T.agents_note}}</span></div><table class="filterable">
  <thead><tr><th>{{.T.col_agent}}</th><th>{{.T.col_purpose}}</th><th class="muted">{{.T.col_words}}</th></tr></thead>
  <tbody>
  {{range .Agents}}<tr>
    <td><strong>{{.Name}}</strong><br><span class="desc">{{.Path}}</span></td>
    <td class="desc">{{.Description}}</td><td>{{.Words}}</td>
  </tr>{{end}}
  </tbody></table></div>

  <div class="tablewrap"><div class="tbltitle"><strong>{{.T.catalog_rules}}</strong><span class="count">{{len .Rules}}</span> <span class="pill origin">.mdc · Cursor</span> <span class="desc">{{.T.rules_note}}</span></div><table class="filterable">
  <thead><tr><th>{{.T.col_rule}}</th><th>{{.T.col_purpose}}</th><th class="muted">{{.T.col_words}}</th></tr></thead>
  <tbody>
  {{range .Rules}}<tr>
    <td><strong>{{.Name}}</strong><br><span class="desc">{{.Path}}</span></td>
    <td class="desc">{{.Description}}</td><td>{{.Words}}</td>
  </tr>{{end}}
  </tbody></table></div>
  {{end}}

  {{if .ProjSkills}}<h2>{{.T.catalog_proj}}</h2>{{end}}
  {{range .ProjSkills}}
  <div class="tablewrap"><div class="tbltitle"><strong>{{.Project}}</strong><span class="count">{{len .Skills}}</span> <span class="desc">projects/{{.Project}}/skills/</span></div><table class="filterable">
  <thead><tr><th>{{$.T.col_skill}}</th><th>{{$.T.col_when}}</th><th class="muted">{{$.T.col_words}}</th></tr></thead>
  <tbody>
  {{range .Skills}}<tr>
    <td><strong>{{.Name}}</strong></td><td class="desc">{{.Description}}</td><td>{{.Words}}</td>
  </tr>{{end}}
  </tbody></table></div>
  {{end}}

  <h2>{{.T.bundles_h}}</h2>
  <div class="prose">{{.T.bundles_p}}</div>
  <div class="tablewrap"><table>
  <thead><tr><th>{{.T.col_bundle}}</th><th>{{.T.col_skills}}</th><th>{{.T.col_agents}}</th><th>{{.T.col_rules}}</th><th>{{.T.col_requires}}</th><th>{{.T.col_usedby}}</th></tr></thead>
  <tbody>
  {{range .Bundles}}<tr>
    <td><strong>{{.Name}}</strong></td>
    <td>{{if .Skills}}{{.Skills}}{{else}}—{{end}}</td>
    <td>{{if .Agents}}{{.Agents}}{{else}}—{{end}}</td>
    <td>{{if .Rules}}{{.Rules}}{{else}}—{{end}}</td>
    <td>{{if .Requires}}<code>{{.Requires}}</code>{{else}}—{{end}}</td>
    <td class="desc">{{.UsedBy}}</td>
  </tr>{{end}}
  </tbody></table></div>
{{end}}

{{if eq .Page "guard"}}
  <div class="pagehead">
    <span class="crumb">{{.T.nav_guard}}</span>
    <h1>{{.T.guard_h}}</h1>
  </div>
  <div class="cards">
    <div class="card"><h4>{{.T.g1_h}}</h4>{{.T.g1_b}}</div>
    <div class="card"><h4>{{.T.g2_h}}</h4>{{.T.g2_b}}</div>
    <div class="card"><h4>{{.T.g3_h}}</h4>{{.T.g3_b}}</div>
    <div class="card"><h4>{{.T.g4_h}}</h4>{{.T.g4_b}}</div>
  </div>
{{end}}

{{if eq .Page "howto"}}
  <div class="pagehead">
    <span class="crumb">{{.T.nav_howto}}</span>
    <h1>{{.T.howto_h}}</h1>
  </div>
  <h3>{{.T.ht1_h}}</h3>
  <div class="mermaid">{{.T.ht1_diagram}}</div>
  <div class="prose">{{.T.ht1_p}}</div>
  <h3>{{.T.ht2_h}}</h3>
  <pre><code>mkdir -p projects/&lt;name&gt;/skills/&lt;skill&gt;
$EDITOR projects/&lt;name&gt;/skills/&lt;skill&gt;/SKILL.md   {{.T.ht2_c}}
trafaret install &lt;name&gt;</code></pre>
  <div class="prose">{{.T.ht2_p}}</div>
  <h3>{{.T.ht3_h}}</h3>
  <pre><code>engine: "&gt;=0.3.0"
projects:
  myproj:
    repoRoot: myproj          {{.T.ht3_c1}}
    bundles: [ship]           {{.T.ht3_c2}}
    vars:                     {{.T.ht3_c3}}
      branch: main
      tracker: { kind: github, repo: me/myproj }
      gates:
        - { name: test, cmd: make test }</code></pre>
  <div class="prose">{{.T.ht3_p}}</div>
  <h3>{{.T.ht4_h}}</h3>
  <pre><code>homes:
  me-claude:
    harness: claude
    dir: ~/.claude            {{.T.ht4_c1}}
    skills: [standup]         {{.T.ht4_c2}}</code></pre>
  <div class="prose">{{.T.ht4_p}}</div>
  <h3>{{.T.ht5_h}}</h3>
  <pre><code># templates/bundles.yaml
review:
  skills: [review]
  agents: [reviewer]
  rules: [repo-stance]
  requires: [branch, tracker.kind]</code></pre>
  <div class="prose">{{.T.ht5_p}}</div>
  <h3>{{.T.ht6_h}}</h3>
  <div class="prose">{{.T.ht6_p}}</div>
  <h3>{{.T.ht7_h}}</h3>
  <div class="tablewrap"><table>
  <thead><tr><th>{{.T.col_symptom}}</th><th>{{.T.col_fix}}</th></tr></thead>
  <tbody>{{.T.trouble_rows}}</tbody>
  </table></div>
{{end}}

{{if eq .Page "where"}}
  <div class="pagehead">
    <span class="crumb">{{.T.nav_where}}</span>
    <h1>{{.T.where_h}}</h1>
  </div>
  <div class="tablewrap"><table>
  <thead><tr><th>{{.T.col_change}}</th><th>{{.T.col_edit}}</th><th>{{.T.col_then}}</th></tr></thead>
  <tbody>{{.T.where_rows}}</tbody>
  </table></div>
{{end}}

<div class="pager">
  <span>{{if .Prev}}<a href="/{{.Lang}}/{{.Prev.Slug}}"><span>←</span> <span><span class="lbl">{{.T.prev}}</span>{{.Prev.Title}}</span></a>{{end}}</span>
  <span>{{if .Next}}<a href="/{{.Lang}}/{{.Next.Slug}}"><span style="text-align:right"><span class="lbl">{{.T.next}}</span>{{.Next.Title}}</span> <span>→</span></a>{{end}}</span>
</div>

</div>

<script src="/mermaid.min.js"></script>
<script>
  mermaid.initialize({
    startOnLoad: true,
    theme: window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'neutral',
    flowchart: { htmlLabels: true }
  });
  document.querySelectorAll('[data-setlang]').forEach(a =>
    a.addEventListener('click', () => localStorage.setItem('trafaret-lang', a.dataset.setlang)));
  const f = document.getElementById('skillfilter');
  if (f) f.addEventListener('input', () => {
    const q = f.value.toLowerCase();
    document.querySelectorAll('table.filterable tbody tr').forEach(tr =>
      tr.style.display = tr.textContent.toLowerCase().includes(q) ? '' : 'none');
  });
</script>
</body>
</html>
`

// langIndex is served at "/": it forwards to the saved language (default EN).
const langIndex = `<!DOCTYPE html><html><head><meta charset="utf-8"><script>
location.replace('/' + (localStorage.getItem('trafaret-lang') || 'en') + '/overview');
</script></head><body></body></html>`

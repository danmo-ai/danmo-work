<script setup lang="ts">
import { ref } from 'vue'
import ProductHero from '../components/ProductHero.vue'
import { useReveal } from '../composables/useReveal'

const root = ref<HTMLElement | null>(null)
useReveal(root)

const pillars = [
  {
    title: 'Human ↔ AI Office co-edit',
    copy: 'On Document Stage: Intent → Propose → Review → Commit. AI Diff lets you Keep, Revert, or accept hunks against a pre-turn snapshot. Text stays source of truth — not chat dumps, not multiplayer CRDT.',
  },
  {
    title: 'Pure LLM-driven',
    copy: 'No hand-maintained graph, role router, or mode switch. The model plans Tool Calls on one Agent Loop — full use of its reasoning and chain-of-thought.',
  },
  {
    title: 'Same thinking chain',
    copy: 'Multi-agent delegation via delegate_agent with hard context isolation. The child returns a Report; the parent continues on the same chain.',
  },
  {
    title: 'Everything is a Tool',
    copy: 'Sub-agents, skills, knowledge, memory, files, MCP, and ask_user — one abstraction. Humans join the loop; co-thinking, not command-and-forget.',
  },
  {
    title: 'Log is state',
    copy: 'Every Tool Call is persisted. Resume, replay, and visualize the thinking process through ultra-long runs — edit a result and continue.',
  },
]

const beats = [
  {
    num: '01',
    title: 'Intent',
    copy: 'Select a paragraph, slide, or cell range. Add an instruction. The toolbar builds an [office-edit] turn.',
  },
  {
    num: '02',
    title: 'Propose',
    copy: 'Same Agent Loop: read_file + apply_patch. A pre-turn snapshot is captured before the write.',
  },
  {
    num: '03',
    title: 'Review',
    copy: 'Stage banner: View Diff · Keep · Revert. Optionally accept only the hunks you want.',
  },
  {
    num: '04',
    title: 'Commit',
    copy: 'Keep leaves Markdown / CSV SoT. Revert restores the snapshot. The trail stays in Turn Log.',
  },
]
</script>

<template>
  <div ref="root">
    <ProductHero
      name="Work"
      accent="#2a7a82"
      tagline="Human ↔ AI co-edit for long-horizon work."
      lede="A general-purpose Work Agent with a coding-agent-grade loop. On Document Stage you and the model co-edit docs, slides, and sheets — propose, review AI Diff, keep or revert — while every Tool Call stays on one thinking trail."
      github="https://github.com/danmo-ai/danmo-work"
    >
      <template #cta>
        <a class="btn ghost" href="#office-coedit">See Office co-edit</a>
      </template>
    </ProductHero>

    <section id="office-coedit" class="coedit container">
      <header class="reveal section-head">
        <p class="kicker">Document Stage</p>
        <h2>Office co-edit, not chat dumps</h2>
        <p>
          You and the Agent are co-editors. Four beats on Stage — Intent, Propose, Review, Commit —
          for docs, slides, and sheets. Text remains source of truth; the review layer is what we deepen.
        </p>
      </header>

      <ol class="beats reveal">
        <li v-for="(b, i) in beats" :key="b.num" :style="{ transitionDelay: `${i * 70}ms` }">
          <span class="num">{{ b.num }}</span>
          <h3>{{ b.title }}</h3>
          <p>{{ b.copy }}</p>
        </li>
      </ol>

      <div class="pair coedit-shots">
        <figure class="reveal" style="transition-delay: 60ms">
          <img
            src="/screenshots/ui-office-coedit.png"
            alt="Human and AI co-editing a report on Document Stage"
            width="3456"
            height="2168"
          />
          <figcaption>Human ↔ AI co-edit — Agent trail beside the live document</figcaption>
        </figure>
        <figure class="reveal" style="transition-delay: 120ms">
          <img
            src="/screenshots/ui-ai-doc-modify.png"
            alt="Select a paragraph and choose AI polish, expand, or modify"
            width="3456"
            height="2168"
          />
          <figcaption>AI modify — select text → polish / expand / rewrite</figcaption>
        </figure>
      </div>

      <div class="demo-frame reveal">
        <div class="demo-bar">
          <strong>Feature tour</strong>
          <span>ZH / EN · auto-play</span>
          <a
            class="demo-open"
            href="/demos/office-coedit-tour.html?lang=en&tour=1"
            target="_blank"
            rel="noopener noreferrer"
          >
            Open full screen
          </a>
        </div>
        <iframe
          class="demo-iframe"
          title="Danmo Work Office co-edit tour"
          src="/demos/office-coedit-tour.html?lang=en&tour=1"
          loading="lazy"
        />
      </div>

      <div class="surface-grid">
        <article class="reveal surface">
          <h3>Doc</h3>
          <p>GFM Markdown · TipTap · selection or whole-doc polish with AI Diff after the turn.</p>
        </article>
        <article class="reveal surface" style="transition-delay: 80ms">
          <h3>Slides</h3>
          <p>Markdown pages split by <code>---</code> · page-scoped edit · Present HTML derived by Stage.</p>
        </article>
        <article class="reveal surface" style="transition-delay: 140ms">
          <h3>Sheet</h3>
          <p>CSV / <code>.danmo-sheet.json</code> · selection range · fill · review banner on change.</p>
        </article>
      </div>
    </section>

    <section class="shots container">
      <figure class="reveal featured">
        <img
          src="/screenshots/ui-browser-annotate.png"
          alt="Select a page element and annotate to request an edit"
          width="3456"
          height="2168"
        />
        <figcaption>
          Point at the page — click a DOM element, add a note, inject into Composer. Select → annotate →
          edit.
        </figcaption>
      </figure>

      <div class="pair">
        <figure class="reveal" style="transition-delay: 80ms">
          <img
            src="/screenshots/ui-market-report.png"
            alt="Research report generated in Danmo Work"
            width="3456"
            height="2168"
          />
          <figcaption>Research &amp; HTML report with live preview</figcaption>
        </figure>
        <figure class="reveal" style="transition-delay: 140ms">
          <img
            src="/screenshots/ui-cooking-demo.png"
            alt="Interactive cooking demo in Danmo Work"
            width="3456"
            height="2168"
          />
          <figcaption>Interactive demo with playback controls</figcaption>
        </figure>
      </div>

      <figure class="reveal" style="transition-delay: 100ms">
        <img
          src="/screenshots/ui-snake-game.png"
          alt="Snake game built with Danmo Work"
          width="3456"
          height="2168"
        />
        <figcaption>Ship a playable page, then iterate with element annotate</figcaption>
      </figure>
    </section>

    <section class="pillars container">
      <h2 class="reveal">Core difference</h2>
      <ul>
        <li
          v-for="(p, i) in pillars"
          :key="p.title"
          class="reveal"
          :style="{ transitionDelay: `${i * 70}ms` }"
        >
          <h3>{{ p.title }}</h3>
          <p>{{ p.copy }}</p>
        </li>
      </ul>
    </section>

    <section class="shots container">
      <h2 class="reveal section-title">LLM-driven expert team</h2>
      <p class="reveal section-lede">
        No hand-wired crew graph. In Team mode you state a clear goal; the lead model plans
        <code>delegate_agent</code> calls with explicit goal and context — parallel researchers,
        then a document agent — and continues when children Report back.
      </p>
      <div class="pair">
        <figure class="reveal" style="transition-delay: 60ms">
          <img
            src="/screenshots/ui-team-intent.png"
            alt="Team mode with a detailed research brief and clear requirements"
            width="3456"
            height="2168"
          />
          <figcaption>Clear intent — detailed brief, Team mode on</figcaption>
        </figure>
        <figure class="reveal" style="transition-delay: 120ms">
          <img
            src="/screenshots/ui-team-delegate.png"
            alt="Lead agent delegates to researchers then document expert with an explicit goal"
            width="3456"
            height="2168"
          />
          <figcaption>Clear delegation — goal + context on each child agent</figcaption>
        </figure>
      </div>
    </section>

    <section class="shots container">
      <h2 class="reveal section-title">WeChat channel</h2>
      <p class="reveal section-lede">
        One WeChat account, multiple projects. Chat from your phone; each conversation stays in its
        project and syncs with the desktop.
      </p>
      <div class="pair weixin">
        <figure class="reveal" style="transition-delay: 80ms">
          <img
            src="/screenshots/wx1.png"
            alt="WeChat-tagged session in Danmo Work desktop"
            width="3456"
            height="2168"
          />
          <figcaption>Desktop — WeChat-tagged session in the bound project</figcaption>
        </figure>
        <figure class="reveal" style="transition-delay: 140ms">
          <img
            src="/screenshots/wx2.png"
            alt="DQ-Teams AI chat in WeChat on phone"
            width="1170"
            height="2532"
          />
          <figcaption>Phone — same Agent Loop, tools on your machine</figcaption>
        </figure>
      </div>
    </section>

    <section class="shots container">
      <h2 class="reveal section-title">Experts, skills &amp; runtime</h2>
      <p class="reveal section-lede">
        Capability units you give the model — not a hand-maintained workflow graph.
      </p>
      <div class="trio">
        <figure class="reveal" style="transition-delay: 60ms">
          <img
            src="/screenshots/ui-expert-prompts.png"
            alt="Expert system prompt editor"
            width="3456"
            height="2168"
          />
          <figcaption>Expert prompt editor</figcaption>
        </figure>
        <figure class="reveal" style="transition-delay: 120ms">
          <img
            src="/screenshots/ui-skill-editor.png"
            alt="Skill library editor"
            width="3456"
            height="2168"
          />
          <figcaption>Skill library (Agentskills)</figcaption>
        </figure>
        <figure class="reveal" style="transition-delay: 180ms">
          <img
            src="/screenshots/ui-runtime-settings.png"
            alt="Runtime and sandbox settings"
            width="3456"
            height="2168"
          />
          <figcaption>Runtime — delegation depth &amp; OS sandbox</figcaption>
        </figure>
      </div>
    </section>

    <section class="workspace container reveal">
      <h2>Three-pane workspace</h2>
      <p>
        Project sidebar · agent execution log · right panel (Plan / Files / Memory / Changes /
        Terminal / Browser). Describe a goal, watch tools stream, co-edit on Document Stage, join via
        ask_user or page annotate — peer collaboration on long-horizon work.
      </p>
      <p class="tour-links">
        <a href="/demos/product-tour.html?lang=en&tour=1" target="_blank" rel="noopener noreferrer">
          Product tour
        </a>
        <a href="/demos/office-coedit-tour.html?lang=en&tour=1" target="_blank" rel="noopener noreferrer">
          Office co-edit tour
        </a>
      </p>
    </section>
  </div>
</template>

<style scoped>
.btn.ghost {
  display: inline-flex;
  align-items: center;
  min-height: 2.75rem;
  padding: 0.65rem 1.25rem;
  font-size: 0.9rem;
  font-weight: 500;
  border-radius: 4px;
  border: 1px solid rgba(255, 255, 255, 0.18);
  color: var(--label);
  transition:
    border-color 0.25s var(--ease-out),
    background 0.25s var(--ease-out),
    transform 0.25s var(--ease-out);
}

.btn.ghost:hover {
  border-color: rgba(255, 255, 255, 0.35);
  background: rgba(255, 255, 255, 0.04);
  transform: translateY(-1px);
}

.coedit {
  padding-bottom: 4rem;
}

.section-head {
  max-width: 40rem;
  margin-bottom: 2rem;
}

.kicker {
  font-size: 0.75rem;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--mineral);
  font-weight: 500;
  margin-bottom: 0.85rem;
}

.section-head h2,
.section-title,
.pillars h2,
.workspace h2 {
  font-family: var(--font-display);
  font-size: clamp(1.5rem, 3vw, 2rem);
  font-weight: 700;
  letter-spacing: -0.035em;
  margin-bottom: 0.75rem;
}

.section-head p,
.section-lede {
  color: var(--label-muted);
  font-weight: 300;
  line-height: 1.6;
}

.beats {
  list-style: none;
  margin: 0 0 2rem;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 1rem;
}

.beats li {
  border-top: 1px solid var(--ink-line);
  padding-top: 1rem;
}

.beats .num {
  display: block;
  font-family: var(--font-display);
  font-size: 0.75rem;
  letter-spacing: 0.1em;
  color: var(--mineral);
  margin-bottom: 0.45rem;
}

.beats h3 {
  font-family: var(--font-display);
  font-size: 1.05rem;
  font-weight: 600;
  margin-bottom: 0.45rem;
}

.beats p {
  color: var(--label-muted);
  font-size: 0.9rem;
  font-weight: 300;
  line-height: 1.5;
}

.demo-frame {
  border: 1px solid var(--ink-line);
  border-radius: 12px;
  overflow: hidden;
  background: #0b1220;
  margin-bottom: 1.5rem;
}

.demo-bar {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
  padding: 0.7rem 1rem;
  border-bottom: 1px solid var(--ink-line);
  font-size: 0.8rem;
  color: var(--label-faint);
}

.demo-bar strong {
  color: var(--label);
  font-family: var(--font-display);
  font-weight: 600;
}

.demo-open {
  margin-left: auto;
  color: var(--mineral);
  font-weight: 500;
}

.demo-open:hover {
  color: var(--label);
}

.demo-iframe {
  width: 100%;
  height: min(72vh, 720px);
  border: 0;
  display: block;
  background: #0b1220;
}

.surface-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1rem;
}

.surface {
  border-left: 3px solid var(--mineral);
  padding: 0.35rem 0 0.35rem 1rem;
}

.surface h3 {
  font-family: var(--font-display);
  font-size: 1.05rem;
  font-weight: 600;
  margin-bottom: 0.4rem;
}

.surface p {
  color: var(--label-muted);
  font-size: 0.9rem;
  font-weight: 300;
  line-height: 1.5;
}

.surface code {
  font-size: 0.82em;
  color: var(--lacquer);
}

.shots {
  display: grid;
  gap: 1.25rem;
  padding-bottom: 4rem;
}

.shots img {
  width: 100%;
  height: auto;
  display: block;
  object-fit: contain;
}

.shots figcaption {
  margin-top: 0.75rem;
  font-size: 0.8rem;
  color: var(--label-faint);
}

.pair {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1.25rem;
}

.coedit-shots {
  margin: 2rem 0 2.25rem;
}

.coedit-shots img {
  width: 100%;
  height: auto;
  display: block;
  object-fit: contain;
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.coedit-shots figcaption {
  margin-top: 0.75rem;
  font-size: 0.8rem;
  color: var(--label-faint);
}

.pair.weixin {
  align-items: start;
}

.pair.weixin figure:last-child img {
  max-width: 22rem;
  margin-inline: auto;
}

.trio {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1.25rem;
}

.section-lede {
  max-width: 40rem;
  margin-bottom: 1.75rem;
}

.pillars {
  padding-bottom: 4rem;
}

.pillars h2,
.workspace h2 {
  margin-bottom: 2rem;
}

.pillars ul {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  border-top: 1px solid var(--ink-line);
}

.pillars li {
  padding: 1.75rem 0;
  border-bottom: 1px solid var(--ink-line);
  border-left: 3px solid transparent;
  padding-left: 1.15rem;
  transition: border-color 0.3s var(--ease-out);
}

.pillars li:hover {
  border-left-color: var(--mineral);
}

.pillars h3 {
  font-family: var(--font-display);
  font-size: 1.2rem;
  font-weight: 600;
  margin-bottom: 0.5rem;
}

.pillars p {
  color: var(--label-muted);
  max-width: 40rem;
  font-weight: 300;
}

.workspace {
  padding-bottom: 2rem;
  max-width: 40rem;
}

.workspace p {
  color: var(--label-muted);
  font-weight: 300;
  font-size: 1.05rem;
  line-height: 1.65;
  margin-top: -0.5rem;
}

.tour-links {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  margin-top: 1.25rem !important;
}

.tour-links a {
  color: var(--mineral);
  font-size: 0.95rem;
  font-weight: 500;
}

.tour-links a:hover {
  color: var(--label);
}

@media (max-width: 900px) {
  .beats,
  .surface-grid,
  .trio {
    grid-template-columns: 1fr;
  }

  .demo-iframe {
    height: 640px;
  }
}

@media (max-width: 700px) {
  .pair {
    grid-template-columns: 1fr;
  }
}
</style>

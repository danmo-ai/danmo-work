<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useReveal } from '../composables/useReveal'

const root = ref<HTMLElement | null>(null)
useReveal(root)

const products = [
  {
    name: 'Make',
    path: '/make',
    accent: 'var(--cinnabar)',
    title: 'Image & video, on your machine.',
    copy: 'Plugin-style generation with MLX and CUDA. Infinite canvas, lineage, and models you control.',
  },
  {
    name: 'Work',
    path: '/work',
    accent: 'var(--mineral)',
    title: 'Human ↔ AI Office co-edit on one trail.',
    copy: 'Coding-agent-grade loop for long-horizon work. Co-edit docs, slides, and sheets with AI Diff review — propose, keep, or revert. Log is state.',
  },
  {
    name: 'Inbox',
    path: '/inbox',
    accent: 'var(--lacquer)',
    title: 'Inbox as information flow.',
    copy: 'AI organizes email into Today, topics, and todos — so you act on what matters.',
  },
] as const
</script>

<template>
  <div ref="root" class="home">
    <section class="hero">
      <div class="hero-wash" aria-hidden="true" />
      <div class="hero-media" aria-hidden="true">
        <img
          src="/screenshots/ui-office-coedit.png"
          alt=""
          width="1440"
          height="900"
        />
        <div class="hero-fade" />
      </div>

      <div class="container hero-copy">
        <p class="brand-lockup">
          <span class="brand-en">Danmo</span>
          <span class="brand-zh" lang="zh-Hans">丹墨</span>
        </p>
        <h1 class="headline">Local-first AI for create, collaborate, and inbox.</h1>
        <p class="lede">
          Three desktop apps. One suite. Models and agents that run where your work already lives.
        </p>
        <div class="ctas">
          <a class="btn primary" href="#products">Explore products</a>
          <a
            class="btn ghost"
            href="https://github.com/danmo-ai"
            target="_blank"
            rel="noopener noreferrer"
          >
            View on GitHub
          </a>
        </div>
      </div>
    </section>

    <section id="products" class="products">
      <div class="container">
        <header class="section-head reveal">
          <h2>Three surfaces. One pigment.</h2>
          <p>
            Named for cinnabar and ink — Danmo (丹墨) is a local-first suite for making, thinking,
            and focusing.
          </p>
        </header>

        <div class="product-list">
          <article
            v-for="(product, i) in products"
            :key="product.name"
            class="product reveal"
            :style="{ '--accent': product.accent, transitionDelay: `${i * 80}ms` }"
          >
            <div class="product-meta">
              <p class="product-name">{{ product.name }}</p>
              <h3>{{ product.title }}</h3>
              <p class="product-copy">{{ product.copy }}</p>
              <RouterLink class="text-link" :to="product.path">
                Learn more
                <span aria-hidden="true">→</span>
              </RouterLink>
            </div>
          </article>
        </div>
      </div>
    </section>

    <section class="showcase">
      <div class="container showcase-grid">
        <figure class="shot reveal">
          <img
            src="/screenshots/ui-ai-doc-modify.png"
            alt="Danmo Work AI polish expand and modify on a selected paragraph"
            width="3456"
            height="2168"
          />
          <figcaption>Work — Human ↔ AI Office co-edit on Document Stage</figcaption>
        </figure>
        <figure class="shot reveal" style="transition-delay: 100ms">
          <img
            src="/screenshots/ui-team-delegate.png"
            alt="Danmo Work LLM-driven expert team delegation with clear goals"
            width="3456"
            height="2168"
          />
          <figcaption>Work — pure LLM-driven expert team delegation</figcaption>
        </figure>
      </div>
    </section>

    <section class="thesis">
      <div class="container thesis-inner reveal">
        <h2>Not another cloud dashboard.</h2>
        <p>
          Danmo runs on your Mac or workstation. Make drives local MLX and CUDA backends.
          Work and Inbox bring your own models and keys. Your canvas, your Trace, your inbox —
          without shipping every draft to someone else’s GPU.
        </p>
      </div>
    </section>

    <section class="open">
      <div class="container open-inner reveal">
        <h2>Open source by default.</h2>
        <p>
          Make, Work, Inbox, the dq-ui design system, and the Experts &amp; Skills market —
          all under the danmo-ai organization.
        </p>
        <a
          class="btn primary"
          href="https://github.com/danmo-ai"
          target="_blank"
          rel="noopener noreferrer"
        >
          github.com/danmo-ai
        </a>
      </div>
    </section>
  </div>
</template>

<style scoped>
.hero {
  position: relative;
  min-height: 100svh;
  display: grid;
  align-items: end;
  overflow: hidden;
  isolation: isolate;
}

.hero-wash {
  position: absolute;
  inset: 0;
  z-index: 0;
  background:
    radial-gradient(ellipse 70% 55% at 15% 20%, rgba(201, 55, 86, 0.28), transparent 60%),
    radial-gradient(ellipse 55% 50% at 85% 15%, rgba(42, 122, 130, 0.32), transparent 55%),
    radial-gradient(ellipse 80% 60% at 50% 100%, rgba(12, 13, 16, 0.2), transparent 40%),
    linear-gradient(180deg, #0c0d10 0%, #101218 45%, #0c0d10 100%);
  animation: wash-drift 18s ease-in-out infinite alternate;
}

.hero-media {
  position: absolute;
  inset: 0;
  z-index: 1;
}

.hero-media img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: center top;
  opacity: 0.42;
  transform: scale(1.04);
  animation: hero-settle 1.6s var(--ease-out) both;
}

.hero-fade {
  position: absolute;
  inset: 0;
  background:
    linear-gradient(180deg, rgba(12, 13, 16, 0.55) 0%, rgba(12, 13, 16, 0.2) 35%, rgba(12, 13, 16, 0.92) 78%, var(--ink) 100%),
    linear-gradient(90deg, rgba(12, 13, 16, 0.75) 0%, transparent 45%, rgba(12, 13, 16, 0.35) 100%);
}

.hero-copy {
  position: relative;
  z-index: 2;
  padding-bottom: clamp(3.5rem, 12vh, 6.5rem);
  padding-top: 7rem;
  max-width: 42rem;
  animation: copy-rise 1.1s var(--ease-out) 0.15s both;
}

.brand-lockup {
  display: flex;
  align-items: baseline;
  gap: 0.85rem;
  margin-bottom: 1.25rem;
}

.brand-en {
  font-family: var(--font-display);
  font-weight: 700;
  font-size: clamp(3.2rem, 9vw, 5.5rem);
  letter-spacing: -0.055em;
  line-height: 0.95;
  background: linear-gradient(120deg, #fff 20%, #f0d4c0 55%, #c4a574 100%);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

.brand-zh {
  font-family: var(--font-display);
  font-size: clamp(1.35rem, 3vw, 1.85rem);
  font-weight: 600;
  color: var(--cinnabar);
  letter-spacing: 0.12em;
}

.headline {
  font-family: var(--font-display);
  font-weight: 600;
  font-size: clamp(1.35rem, 2.8vw, 1.85rem);
  letter-spacing: -0.03em;
  line-height: 1.25;
  max-width: 22ch;
  color: var(--label);
}

.lede {
  margin-top: 1rem;
  color: var(--label-muted);
  font-size: 1.05rem;
  max-width: 34rem;
  font-weight: 300;
}

.ctas {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  margin-top: 1.75rem;
}

.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 2.75rem;
  padding: 0.65rem 1.25rem;
  font-size: 0.9rem;
  font-weight: 500;
  border-radius: 4px;
  transition:
    background 0.25s var(--ease-out),
    color 0.25s var(--ease-out),
    border-color 0.25s var(--ease-out),
    transform 0.25s var(--ease-out);
}

.btn.primary {
  background: var(--cinnabar);
  color: #fff;
}

.btn.primary:hover {
  background: #d64565;
  transform: translateY(-1px);
}

.btn.ghost {
  border: 1px solid rgba(255, 255, 255, 0.18);
  color: var(--label);
}

.btn.ghost:hover {
  border-color: rgba(255, 255, 255, 0.35);
  background: rgba(255, 255, 255, 0.04);
}

.products {
  padding: clamp(4.5rem, 10vh, 7rem) 0;
}

.section-head {
  max-width: 36rem;
  margin-bottom: 3.5rem;
}

.section-head h2 {
  font-family: var(--font-display);
  font-size: clamp(1.85rem, 4vw, 2.6rem);
  font-weight: 700;
  letter-spacing: -0.04em;
  line-height: 1.15;
}

.section-head p {
  margin-top: 1rem;
  color: var(--label-muted);
  font-weight: 300;
}

.product-list {
  display: flex;
  flex-direction: column;
  border-top: 1px solid var(--ink-line);
}

.product {
  display: grid;
  padding: 2.25rem 0;
  border-bottom: 1px solid var(--ink-line);
  border-left: 3px solid transparent;
  padding-left: 1.25rem;
  transition: border-color 0.35s var(--ease-out);
}

.product:hover {
  border-left-color: var(--accent);
}

.product-name {
  font-size: 0.75rem;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--accent);
  margin-bottom: 0.65rem;
}

.product h3 {
  font-family: var(--font-display);
  font-size: clamp(1.35rem, 2.5vw, 1.75rem);
  font-weight: 600;
  letter-spacing: -0.03em;
  max-width: 22ch;
}

.product-copy {
  margin-top: 0.75rem;
  color: var(--label-muted);
  max-width: 38rem;
  font-weight: 300;
}

.text-link {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  margin-top: 1.25rem;
  font-size: 0.9rem;
  font-weight: 500;
  color: var(--label);
  transition: gap 0.25s var(--ease-out);
}

.text-link:hover {
  gap: 0.65rem;
}

.showcase {
  padding: 0 0 clamp(4rem, 8vh, 6rem);
}

.showcase-grid {
  display: grid;
  grid-template-columns: 1.15fr 0.85fr;
  gap: 1.25rem;
  align-items: end;
}

.shot {
  margin: 0;
  overflow: hidden;
}

.shot img {
  width: 100%;
  height: auto;
  display: block;
  object-fit: contain;
  transition: transform 0.8s var(--ease-out);
}

.shot:hover img {
  transform: scale(1.03);
}

.shot figcaption {
  margin-top: 0.85rem;
  font-size: 0.8rem;
  color: var(--label-faint);
}

.thesis {
  padding: clamp(4.5rem, 10vh, 7rem) 0;
  background:
    linear-gradient(180deg, transparent, rgba(42, 122, 130, 0.08)),
    var(--paper);
  color: var(--paper-ink);
}

.thesis-inner {
  max-width: 40rem;
}

.thesis h2 {
  font-family: var(--font-display);
  font-size: clamp(1.85rem, 4vw, 2.5rem);
  font-weight: 700;
  letter-spacing: -0.04em;
  line-height: 1.15;
}

.thesis p {
  margin-top: 1.25rem;
  font-size: 1.1rem;
  line-height: 1.65;
  color: rgba(26, 28, 34, 0.72);
  font-weight: 300;
}

.open {
  padding: clamp(4.5rem, 10vh, 7rem) 0;
}

.open-inner {
  max-width: 36rem;
}

.open h2 {
  font-family: var(--font-display);
  font-size: clamp(1.85rem, 4vw, 2.5rem);
  font-weight: 700;
  letter-spacing: -0.04em;
}

.open p {
  margin: 1rem 0 1.75rem;
  color: var(--label-muted);
  font-weight: 300;
}

@keyframes wash-drift {
  from {
    transform: scale(1) translate(0, 0);
  }
  to {
    transform: scale(1.06) translate(-1.5%, 1%);
  }
}

@keyframes hero-settle {
  from {
    opacity: 0;
    transform: scale(1.08);
  }
  to {
    opacity: 0.42;
    transform: scale(1.04);
  }
}

@keyframes copy-rise {
  from {
    opacity: 0;
    transform: translateY(24px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (max-width: 860px) {
  .showcase-grid {
    grid-template-columns: 1fr;
  }

  .hero-copy {
    max-width: 100%;
  }
}

@media (max-width: 560px) {
  .brand-lockup {
    flex-direction: column;
    gap: 0.25rem;
  }

  .ctas {
    flex-direction: column;
  }

  .btn {
    width: 100%;
  }
}

@media (prefers-reduced-motion: reduce) {
  .hero-wash,
  .hero-media img,
  .hero-copy {
    animation: none;
  }

  .hero-media img {
    opacity: 0.42;
    transform: scale(1.02);
  }
}
</style>

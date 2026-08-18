import { DOCUMENTATION } from '@/lib/documentation'

// The in-app manual: what Athena is for, how study sessions work, and what
// the Knowledge Engine adds. Content comes from lib/documentation.ts; this
// screen is layout only.
//
// Scrolls inside its own container rather than growing <main>, matching
// StudyChatScreen's transcript — the shell's <main> is a flex item that must
// not stretch past the viewport.
function DocumentationScreen() {
  return (
    <div className="thin-scroll mx-auto flex w-full max-w-3xl flex-col gap-10 overflow-y-auto pr-2">
      <header className="flex flex-col gap-2">
        <p className="font-mono text-[11px] tracking-[0.16em] text-primary uppercase">
          Documentation
        </p>
        <h2 className="font-heading text-2xl font-bold text-balance text-foreground">
          A study companion that remembers what you learn
        </h2>
        <p className="text-muted-foreground">
          What Athena is for, how a study session works, and what the Knowledge Engine adds.
        </p>
      </header>

      <nav
        aria-label="Contents"
        className="flex flex-col gap-2 rounded-lg border border-border p-4"
      >
        <p className="font-mono text-[10px] tracking-[0.12em] text-muted-foreground uppercase">
          Contents
        </p>
        {DOCUMENTATION.map((section, index) => (
          <a
            key={section.id}
            href={`#${section.id}`}
            className="flex items-baseline gap-2.5 text-sm text-foreground hover:text-primary focus-visible:ring-2 focus-visible:ring-primary focus-visible:outline-none"
          >
            {/* Decorative ordinal: hidden from the accessible name so the
                link reads as just the section title. */}
            <span className="font-mono text-[11px] text-primary" aria-hidden="true">
              {String(index + 1).padStart(2, '0')}
            </span>
            {section.title}
          </a>
        ))}
      </nav>

      {DOCUMENTATION.map((section) => (
        <section key={section.id} id={section.id} className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <div className="flex items-center gap-2.5">
              <h3 className="font-heading text-xl font-bold text-foreground">{section.title}</h3>
              {section.status === 'planned' && (
                <span className="rounded-full border border-primary/40 px-2 py-0.5 font-mono text-[10px] tracking-[0.08em] text-primary uppercase">
                  Planned
                </span>
              )}
            </div>
            <p className="text-sm text-muted-foreground">{section.summary}</p>
          </div>

          <div className="flex flex-col gap-3">
            {/* Static, never-reordered prose, so the index is a stable key
                and avoids deriving one from the paragraph text. */}
            {section.body.map((paragraph, index) => (
              <p key={`${section.id}-${index}`} className="text-foreground">
                {paragraph}
              </p>
            ))}
          </div>

          <dl className="flex flex-col gap-2.5 border-l-2 border-border pl-4">
            {section.topics.map((topic) => (
              <div key={topic.term} className="flex flex-col gap-0.5">
                <dt className="text-sm font-bold text-foreground">{topic.term}</dt>
                <dd className="text-sm text-muted-foreground">{topic.description}</dd>
              </div>
            ))}
          </dl>
        </section>
      ))}
    </div>
  )
}

export default DocumentationScreen

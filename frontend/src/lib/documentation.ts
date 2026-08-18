// The in-app manual. Content lives here rather than inside the screen's JSX
// so it stays readable, testable, and editable without touching layout.
//
// Sections carry a status because the manual documents the Knowledge Engine
// before it ships: describing an unbuilt feature as if it already worked
// would mislead the reader, so the screen renders a visible marker instead.

export type DocStatus = 'available' | 'planned'

export interface DocTopic {
  term: string
  description: string
}

export interface DocSection {
  id: string
  title: string
  status: DocStatus
  summary: string
  body: string[]
  topics: DocTopic[]
}

const DOCUMENTATION: DocSection[] = [
  {
    id: 'what-is-athena',
    title: 'Why Athena exists',
    status: 'available',
    summary: 'A study companion that knows who you are and keeps what you learn.',
    body: [
      'Asking a chatbot to explain something works fine once. The trouble starts the second time. A general-purpose assistant meets you as a stranger every session: it does not know your field, your level, or that you covered this exact topic last week. It answers in walls of text when you would learn more from a question. And nothing you work through together survives closing the window.',
      'Athena is built for the opposite. It knows your background, teaches by asking rather than lecturing, and accumulates — so your tenth session starts from everything the first nine established.',
    ],
    topics: [
      {
        term: 'It runs on your machine',
        description:
          'Your account, study history, notes and knowledge base are files on your own computer. There is no Athena server holding your data.',
      },
      {
        term: 'Your own OpenRouter key',
        description:
          'Athena reaches language models through OpenRouter with your key, so you see what each session costs and are never locked to one model vendor.',
      },
      {
        term: 'Your knowledge comes first',
        description:
          'Once you have given it your notes, Athena checks them before asking a model anything. Your own material outranks whatever the model would have improvised.',
      },
      {
        term: 'Nothing becomes truth by itself',
        description:
          'Models are confidently wrong sometimes. Anything Athena learns from a conversation waits for your approval before it counts.',
      },
    ],
  },
  {
    id: 'study-sessions',
    title: 'Study sessions',
    status: 'available',
    summary: 'The core of the app: a Socratic conversation about a topic you choose.',
    body: [
      'Before your first session Athena asks who you are — your name, what to call the assistant, your field, experience level, goals, preferred study style, and whether the assistant should speak English or Portuguese. That profile is rebuilt into the instructions behind every reply, so the same question gets a different answer depending on who is asking. Change any of it in Settings and the next message reflects it.',
      'Sessions live in folders in the sidebar, like a file explorer. There is always a General folder as the fallback; you can rename it but not delete it. Drag sessions between folders, and deleting a folder moves its sessions back to General rather than destroying them. Any session reopens at any time with its full history.',
      'A session starts with you naming a topic. Athena opens with a short greeting and one focused question — no lecture, no summary. It is finding out where you actually are before deciding what to teach. From there it is a real back-and-forth: you answer, it gives brief feedback, then asks a follow-up that builds on it. Replies stay short by design; ask for the long version and only then does it go deep.',
    ],
    topics: [
      {
        term: 'Personalised replies',
        description:
          'Your field, level, goals and study style shape every answer. Set your level to beginner and explanations start from the ground; set it to advanced and Athena skips the preamble.',
      },
      {
        term: 'Folders and history',
        description:
          'Conversations grouped by subject in the sidebar tree, reopenable at any time with everything that was said.',
      },
      {
        term: 'Streaming answers',
        description:
          'Replies arrive word by word rather than all at once, and render as proper Markdown: headings, lists, tables and syntax-highlighted code.',
      },
      {
        term: 'Careful spending',
        description:
          'Athena picks a model to match the task instead of always reaching for the most expensive one, and records the tokens and cost of every call. If your credit runs out mid-session it retries once against a free model and tells you.',
      },
    ],
  },
  {
    id: 'knowledge-engine',
    title: 'The Knowledge Engine',
    status: 'planned',
    summary: 'Your notes become searchable, and what you learn becomes a library you own.',
    body: [
      'Study sessions already work, but they forget. Close the app and next week Athena has no idea what you covered, and the notes you have kept for years sit in a folder it has never opened. The Knowledge Engine fixes both.',
      'You point Athena at a folder and it reads every Markdown and text file inside, splitting each into passages it can find again later. Finding is by meaning, not wording: ask "how does Go handle parallelism?" and it surfaces your note that says "the scheduler multiplexes M:N" — not one shared word, exactly the right passage. That is why old, messily written notes are still worth importing.',
      'From then on your sessions consult those notes first, and every reply shows which ones it drew on. When a session has taught you something worth keeping, you ask Athena to extract it: it rereads the conversation and proposes concept cards — what a thing is, its properties, its trade-offs, what it connects to. Nothing is saved until you choose.',
      'Every card starts as a draft and waits in a review queue, because a model wrote it and models are confidently wrong. Approving is what separates "a model said so" from "I checked, and it is right". The moment you approve, the card joins the same memory as your imported notes and comes back on its own in later sessions.',
    ],
    topics: [
      {
        term: 'Import notes',
        description:
          'Point Athena at a folder of Markdown or text files. Re-importing later skips what has not changed and never duplicates anything.',
      },
      {
        term: 'Search by meaning',
        description:
          'Finds the passage that is about your question even when it shares no words with it, so you do not depend on having used the right vocabulary when you wrote the note.',
      },
      {
        term: 'Cited sources',
        description:
          'Replies list the notes and cards they used, and how closely each matched, so you can tell what came from your material and what did not.',
      },
      {
        term: 'Source modes',
        description:
          'Choose where an answer may come from: notes fills gaps with the model, strict-notes answers only from your material and says so when it finds nothing, and web ignores your notes entirely.',
      },
      {
        term: 'Extract knowledge',
        description:
          'Turns a conversation into concept cards on demand. Ignore them and nothing at all is written.',
      },
      {
        term: 'Review queue',
        description:
          'Every card waits for your approval, so one confident mistake never becomes something Athena teaches back to you for months.',
      },
      {
        term: 'Knowledge library',
        description:
          'Everything organised by subject, with editing, deprecating and deleting — and the actions offered always match the card state.',
      },
    ],
  },
  {
    id: 'reference',
    title: 'Quick reference',
    status: 'available',
    summary: 'Every capability in one line.',
    body: [
      'A scannable index of what the app does. Items belonging to the Knowledge Engine are described above as planned.',
    ],
    topics: [
      { term: 'Local account', description: 'Your data stays on your machine.' },
      { term: 'Profile', description: 'Field, level, goals and style shape every reply.' },
      { term: 'Folders', description: 'Conversations grouped by subject, reopenable any time.' },
      { term: 'Socratic study', description: 'Opens with a question and keeps turns short.' },
      { term: 'Settings', description: 'Change your profile or key; it applies immediately.' },
      { term: 'Import notes', description: 'Athena reads your Markdown folder.' },
      { term: 'Meaning search', description: 'Finds the right passage regardless of wording.' },
      { term: 'Source modes', description: 'Decide whether the model may fill gaps.' },
      { term: 'Concept cards', description: 'Conversations become structured knowledge.' },
      { term: 'Review queue', description: 'You approve before anything counts as true.' },
    ],
  },
]

// plannedSectionIds reports which sections describe features that are
// specified but not yet built, so the screen can mark them.
function plannedSectionIds(): string[] {
  return DOCUMENTATION.filter((section) => section.status === 'planned').map(
    (section) => section.id,
  )
}

export { DOCUMENTATION, plannedSectionIds }

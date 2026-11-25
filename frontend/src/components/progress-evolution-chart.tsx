import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import type { ProgressEvolution, ProgressEvolutionPoint } from '@/lib/progress'

// Line chart of the results of a topic over its attempts: Quiz and Arch Lab
// series on a fixed 0-100 axis, with a dashed line at the minimum score. Drawn
// in plain SVG with the theme's tokens, so it follows light and dark. The
// backend orders and numbers the points. See
// specs/phases/phase-05-progress/03-evolution-chart.md.

const WIDTH = 560
const HEIGHT = 220
const PAD = { top: 12, right: 16, bottom: 28, left: 36 }
const TICKS = [0, 25, 50, 75, 100]

interface SeriesStyle {
  color: string
  // Dot radius: the later series is smaller, so a dot that coincides with an
  // earlier one leaves it visible and hoverable around it.
  radius: number
  dash?: string
}

const QUIZ_STYLE: SeriesStyle = { color: 'var(--primary)', radius: 5 }
const LAB_STYLE: SeriesStyle = { color: '#5fbf8f', radius: 3 }

export function ProgressEvolutionChart({ evolution }: { evolution: ProgressEvolution }) {
  const { t, i18n } = useTranslation('progressTab')
  const gradientId = useId()
  if (!evolution.enough) {
    return <p className="text-sm text-muted-foreground">{t('detail.evolution.notEnough')}</p>
  }
  const longest = Math.max(evolution.quiz.length, evolution.lab.length)
  const innerWidth = WIDTH - PAD.left - PAD.right
  const innerHeight = HEIGHT - PAD.top - PAD.bottom
  const x = (order: number) =>
    PAD.left + (longest <= 1 ? innerWidth / 2 : ((order - 1) / (longest - 1)) * innerWidth)
  const y = (score: number) => PAD.top + (1 - score / 100) * innerHeight
  const formatDate = (iso: string) => new Date(iso).toLocaleDateString(i18n.language)
  const series = [
    { key: 'quiz', label: t('detail.evolution.quiz'), points: evolution.quiz, style: QUIZ_STYLE },
    { key: 'lab', label: t('detail.evolution.lab'), points: evolution.lab, style: LAB_STYLE },
  ].filter((entry) => entry.points.length > 0)
  const pointName = (key: string, point: ProgressEvolutionPoint) =>
    key === 'quiz' ? t('detail.evolution.quizItem', { number: point.itemNumber }) : point.title

  return (
    <figure className="flex flex-col gap-3 rounded-2xl border border-primary/25 bg-gradient-to-b from-card to-muted p-4 shadow-lg shadow-black/40 ring-1 ring-white/5">
      <svg
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        role="img"
        aria-label={t('detail.evolution.chartLabel')}
        className="w-full"
      >
        <defs>
          {series.map(({ key, style }) => (
            <linearGradient key={key} id={`${gradientId}-${key}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={style.color} stopOpacity="0.35" />
              <stop offset="100%" stopColor={style.color} stopOpacity="0" />
            </linearGradient>
          ))}
        </defs>
        {TICKS.map((tick) => (
          <g key={tick}>
            <line
              x1={PAD.left}
              x2={WIDTH - PAD.right}
              y1={y(tick)}
              y2={y(tick)}
              stroke="var(--border)"
            />
            <text
              x={PAD.left - 6}
              y={y(tick)}
              textAnchor="end"
              dominantBaseline="middle"
              fontSize="10"
              fill="var(--muted-foreground)"
            >
              {tick}
            </text>
          </g>
        ))}
        {Array.from({ length: longest }, (_, i) => (
          <text
            key={i}
            x={x(i + 1)}
            y={HEIGHT - 8}
            textAnchor="middle"
            fontSize="10"
            fill="var(--muted-foreground)"
          >
            {i + 1}
          </text>
        ))}
        <line
          data-testid="pass-score-line"
          x1={PAD.left}
          x2={WIDTH - PAD.right}
          y1={y(evolution.passScore)}
          y2={y(evolution.passScore)}
          stroke="var(--muted-foreground)"
          strokeDasharray="5 4"
        />
        {series.map(({ key, points, style }) => (
          <g key={`${key}-line`}>
            {points.length > 1 && (
              <polygon
                pointerEvents="none"
                fill={`url(#${gradientId}-${key})`}
                points={`${x(points[0].order)},${y(0)} ${points.map((p) => `${x(p.order)},${y(p.score)}`).join(' ')} ${x(points[points.length - 1].order)},${y(0)}`}
              />
            )}
            <polyline
              pointerEvents="none"
              fill="none"
              stroke={style.color}
              strokeWidth="2.5"
              strokeLinejoin="round"
              strokeLinecap="round"
              style={{ filter: `drop-shadow(0 2px 4px ${style.color})` }}
              points={points.map((p) => `${x(p.order)},${y(p.score)}`).join(' ')}
            />
          </g>
        ))}
        {/* The dots come last, so no fill or line covers one. */}
        {series.map(({ key, points, style }) => (
          <g key={`${key}-points`} data-series={key}>
            {points.map((point) => (
              <circle
                key={point.order}
                cx={x(point.order)}
                cy={y(point.score)}
                r={style.radius}
                fill={style.color}
                stroke="var(--card)"
                strokeWidth="2"
                tabIndex={0}
              >
                <title>
                  {`${pointName(key, point)} · ${t('detail.evolution.assessment', { order: point.order })} · ${point.score}% · ${formatDate(point.at)}`}
                </title>
              </circle>
            ))}
          </g>
        ))}
      </svg>
      <figcaption className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
        {series.map(({ key, label, style }) => (
          <span key={key} className="flex items-center gap-1.5">
            <span
              aria-hidden="true"
              className="inline-block h-0.5 w-4"
              style={{ background: style.color }}
            />
            {label}
          </span>
        ))}
        <span className="flex items-center gap-1.5">
          <span
            aria-hidden="true"
            className="inline-block w-4 border-t border-dashed border-muted-foreground"
          />
          {t('detail.evolution.passScore', { score: evolution.passScore })}
        </span>
      </figcaption>
    </figure>
  )
}

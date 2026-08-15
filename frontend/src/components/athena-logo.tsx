import { cn } from '@/lib/utils'

interface AthenaLogoProps {
  className?: string
}

interface Leaf {
  x: number
  y: number
  rotation: number
  scale: number
}

const CENTER = 60
const RADIUS = 30
// Degrees sweep from near the base of the wreath up to its open tip, using
// the standard math convention (0deg = +x, 90deg = up) before conversion to
// SVG's clockwise rotate().
const LEAF_ANGLES = [248, 219, 190, 161, 132]
const LEAF_SCALE_RANGE: [number, number] = [1.2, 0.6]
// Leaves are rotated 75% of the way to fully radial, so they angle up and
// outward like a real branch instead of fanning out into a sunburst.
const ROTATION_BLEND = 0.75

// Lays out one wreath branch as leaves fanned outward along an arc,
// shrinking toward the open tip. `mirror` reflects the branch across the
// vertical axis to produce the opposite side from the same leaf shape.
function branchLeaves(mirror: boolean): Leaf[] {
  return LEAF_ANGLES.map((angleDeg, index) => {
    const angle = (angleDeg * Math.PI) / 180
    const x = CENTER + RADIUS * Math.cos(angle)
    const y = CENTER - RADIUS * Math.sin(angle)
    const rotation = (90 - angleDeg) * ROTATION_BLEND
    const t = index / (LEAF_ANGLES.length - 1)
    const scale = LEAF_SCALE_RANGE[0] + (LEAF_SCALE_RANGE[1] - LEAF_SCALE_RANGE[0]) * t
    return mirror ? { x: 2 * CENTER - x, y, rotation: -rotation, scale } : { x, y, rotation, scale }
  })
}

const LEAVES = [...branchLeaves(false), ...branchLeaves(true)]

// A single leaf, base at the origin and tip pointing up (-y); positioned via
// translate/rotate/scale onto the wreath arc. Deliberately short and wide —
// at the small sizes this renders at (e.g. h-12), a thin leaf anti-aliases
// away into an unreadable spike, while a fat one still reads as a leaf.
const LEAF_PATH = 'M0,0 C7,-2.88 7,-7.92 0,-12 C-7,-7.92 -7,-2.88 0,0 Z'

// Hand-authored monogram (geometric "A" pediment inside a laurel wreath) — a
// nod to Athena's namesake, in the same spirit as GreekKeyDivider: no image
// asset, colored via currentColor so it inherits the caller's theme class.
function AthenaLogo({ className }: AthenaLogoProps) {
  return (
    <svg
      role="img"
      aria-label="Athena"
      viewBox="0 0 120 120"
      fill="none"
      className={cn('text-primary', className)}
    >
      {LEAVES.map((leaf, index) => (
        <path
          key={index}
          d={LEAF_PATH}
          fill="currentColor"
          transform={`translate(${leaf.x} ${leaf.y}) rotate(${leaf.rotation}) scale(${leaf.scale})`}
        />
      ))}
      <path
        d="M60 22 L35 92 M60 22 L85 92 M45 68 L75 68"
        stroke="currentColor"
        strokeWidth="6"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}

export { AthenaLogo }

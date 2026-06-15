/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import fs from 'node:fs'
import path from 'node:path'

const GOLDEN_ANGLE = Math.PI * (3 - Math.sqrt(5))

export function createRng(seed = 1) {
  let state = seed >>> 0
  return function rng() {
    state += 0x6d2b79f5
    let t = state
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

export function parseOBJVertices(objText) {
  const values = []
  const lines = objText.split(/\r?\n/)
  for (const rawLine of lines) {
    const line = rawLine.trim()
    if (!line || line.charAt(0) === '#') continue
    if (!line.startsWith('v ')) continue
    const parts = line.split(/\s+/)
    if (parts.length < 4) continue
    const x = Number.parseFloat(parts[1])
    const y = Number.parseFloat(parts[2])
    const z = Number.parseFloat(parts[3])
    if (Number.isFinite(x) && Number.isFinite(y) && Number.isFinite(z)) {
      values.push(x, y, z)
    }
  }
  return new Float32Array(values)
}

export function computeBounds(points) {
  let minX = Infinity
  let minY = Infinity
  let minZ = Infinity
  let maxX = -Infinity
  let maxY = -Infinity
  let maxZ = -Infinity

  for (let i = 0; i < points.length; i += 3) {
    const x = points[i]
    const y = points[i + 1]
    const z = points[i + 2]
    if (x < minX) minX = x
    if (y < minY) minY = y
    if (z < minZ) minZ = z
    if (x > maxX) maxX = x
    if (y > maxY) maxY = y
    if (z > maxZ) maxZ = z
  }

  return {
    minX,
    minY,
    minZ,
    maxX,
    maxY,
    maxZ,
    width: maxX - minX,
    height: maxY - minY,
    depth: maxZ - minZ,
  }
}

export function removeMouthBackArtifact(rawPositions) {
  const b = computeBounds(rawPositions)
  const cx = (b.minX + b.maxX) * 0.5
  const halfWidth = Math.max(b.width * 0.5, 1e-6)
  const height = Math.max(b.height, 1e-6)
  const depth = Math.max(b.depth, 1e-6)
  const kept = []

  for (let i = 0; i < rawPositions.length; i += 3) {
    const x = rawPositions[i]
    const y = rawPositions[i + 1]
    const z = rawPositions[i + 2]
    const ax = Math.abs((x - cx) / halfWidth)
    const y01 = (y - b.minY) / height
    const z01 = (z - b.minZ) / depth
    const upperMouthBackArtifact =
      ax < 0.31 && y01 > 0.275 && y01 < 0.405 && z01 > 0.835 && z01 < 0.92

    if (!upperMouthBackArtifact) kept.push(x, y, z)
  }

  return new Float32Array(kept)
}

export function centerAndScale(points, targetSpan = 2.2) {
  const b = computeBounds(points)
  const cx = (b.minX + b.maxX) * 0.5
  const cy = (b.minY + b.maxY) * 0.5
  const cz = (b.minZ + b.maxZ) * 0.5
  const scale = targetSpan / (Math.max(b.width, b.height, b.depth) || 1)
  const out = new Float32Array(points.length)

  for (let i = 0; i < points.length; i += 3) {
    out[i] = (points[i] - cx) * scale
    out[i + 1] = (points[i + 1] - cy) * scale
    out[i + 2] = (points[i + 2] - cz) * scale
  }

  return out
}

export function normalizePointCount(points, targetCount, seed = 1) {
  const sourceCount = Math.floor(points.length / 3)
  if (sourceCount === 0) return new Float32Array(targetCount * 3)
  if (sourceCount === targetCount) return new Float32Array(points)

  const out = new Float32Array(targetCount * 3)
  const rng = createRng(seed)
  const copyCount = Math.min(sourceCount, targetCount)

  for (let i = 0; i < copyCount; i += 1) {
    out[i * 3] = points[i * 3]
    out[i * 3 + 1] = points[i * 3 + 1]
    out[i * 3 + 2] = points[i * 3 + 2]
  }

  if (sourceCount > targetCount) return out

  for (let i = sourceCount; i < targetCount; i += 1) {
    const src = Math.floor(rng() * sourceCount)
    const jitter = 0.0035
    out[i * 3] = points[src * 3] + (rng() - 0.5) * jitter
    out[i * 3 + 1] = points[src * 3 + 1] + (rng() - 0.5) * jitter
    out[i * 3 + 2] = points[src * 3 + 2] + (rng() - 0.5) * jitter
  }

  return out
}

export function makeFaceVariant(points, variant) {
  const out = new Float32Array(points)
  const eyeY = 0.22
  const eyeZ = 0.42
  const eyeXs = [-0.31, 0.31]

  for (let i = 0; i < out.length; i += 3) {
    const x = out[i]
    const y = out[i + 1]
    const z = out[i + 2]
    const side = x < 0 ? eyeXs[0] : eyeXs[1]
    const dx = (x - side) / 0.24
    const dy = (y - eyeY) / 0.15
    const dz = (z - eyeZ) / 0.5
    const influence = Math.exp(-(dx * dx + dy * dy + dz * dz))

    if (variant === 'closed') {
      out[i + 1] = y + (eyeY - y) * influence * 0.22
      out[i + 2] = z - influence * 0.035
    } else {
      out[i + 1] = y + Math.sign(dy || 1) * influence * 0.045
      out[i + 2] = z + influence * 0.05
    }
  }

  return out
}

export function generateBlackHolePoints(count) {
  const rng = createRng(42069)
  const out = new Float32Array(count * 3)

  for (let i = 0; i < count; i += 1) {
    const band = i / count
    let x
    let y
    let z

    if (band < 0.72) {
      const radius = 0.28 + Math.pow(rng(), 1.9) * 1.55
      const angle = i * GOLDEN_ANGLE + radius * 2.35 + (rng() - 0.5) * 0.22
      const diskWarp = Math.sin(angle * 2.0 + radius * 3.0) * 0.025
      x = Math.cos(angle) * radius
      y = (rng() - 0.5) * (0.045 + radius * 0.018) + diskWarp
      z = Math.sin(angle) * radius * 0.38 + (rng() - 0.5) * 0.035
    } else if (band < 0.9) {
      const angle = i * GOLDEN_ANGLE
      const radius = 0.18 + rng() * 0.18
      const polar = Math.acos(1 - 2 * rng())
      x = Math.sin(polar) * Math.cos(angle) * radius
      y = Math.cos(polar) * radius
      z = Math.sin(polar) * Math.sin(angle) * radius
    } else {
      const angle = i * GOLDEN_ANGLE
      const radius = 1.2 + rng() * 0.85
      const stream = (rng() - 0.5) * 0.18
      x = Math.cos(angle) * radius
      y = stream + Math.sin(angle * 3) * 0.14
      z = Math.sin(angle) * radius * 0.55
    }

    out[i * 3] = x
    out[i * 3 + 1] = y
    out[i * 3 + 2] = z
  }

  return centerAndScale(out, 4.6)
}

export function generateLorenzAttractorPoints(count) {
  const out = new Float32Array(count * 3)
  const sigma = 10
  const rho = 28
  const beta = 8 / 3
  const dt = 0.0048
  const warmupSteps = 1600
  let x = 0.11
  let y = 0
  let z = 0

  const step = () => {
    const dx = sigma * (y - x)
    const dy = x * (rho - z) - y
    const dz = x * y - beta * z
    x += dx * dt
    y += dy * dt
    z += dz * dt
  }

  for (let i = 0; i < warmupSteps; i += 1) {
    step()
  }

  for (let i = 0; i < count; i += 1) {
    step()
    const ribbon = Math.sin(i * GOLDEN_ANGLE) * 0.018

    out[i * 3] = x * 0.58 + ribbon
    out[i * 3 + 1] = (z - 24) * 0.48
    out[i * 3 + 2] = y * 0.5 - ribbon
  }

  return centerAndScale(out, 3.0)
}

export function writeFloat32Binary(filePath, floatArray) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true })
  fs.writeFileSync(filePath, Buffer.from(floatArray.buffer))
}

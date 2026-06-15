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
import { fileURLToPath } from 'node:url'
import {
  centerAndScale,
  generateBlackHolePoints,
  generateLorenzAttractorPoints,
  makeFaceVariant,
  normalizePointCount,
  parseOBJVertices,
  removeMouthBackArtifact,
  writeFloat32Binary,
} from './pointcloud-utils.mjs'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const POINT_COUNT = 30000
const sourceDir = path.join(__dirname, 'pointcloud-source')
const outputDir = path.join(__dirname, '..', 'public', 'pointclouds')

function readFaceSource(name) {
  const filePath = path.join(sourceDir, name)
  const objText = fs.readFileSync(filePath, 'utf8')
  const parsed = parseOBJVertices(objText)
  const cleaned = removeMouthBackArtifact(parsed)
  return centerAndScale(cleaned, 2.45)
}

function buildFaceAsset(sourceFile, variant, seed) {
  const base = readFaceSource(sourceFile)
  const variantPoints = makeFaceVariant(base, variant)
  return normalizePointCount(variantPoints, POINT_COUNT, seed)
}

const assets = [
  {
    id: 'face-closed',
    role: 'face',
    label: 'Closed eye face',
    file: 'face-closed.bin',
    points: buildFaceAsset('face-previous.obj', 'closed', 101),
  },
  {
    id: 'face-open',
    role: 'face',
    label: 'Open eye face',
    file: 'face-open.bin',
    points: buildFaceAsset('face-current.obj', 'open', 102),
  },
  {
    id: 'black-hole',
    role: 'generated',
    label: 'Black hole point cloud',
    file: 'black-hole.bin',
    points: generateBlackHolePoints(POINT_COUNT),
  },
  {
    id: 'lorenz-attractor',
    role: 'generated',
    label: 'Lorenz attractor point cloud',
    file: 'lorenz-attractor.bin',
    source: 'Lorenz system sigma=10 rho=28 beta=8/3',
    points: generateLorenzAttractorPoints(POINT_COUNT),
  },
]

fs.mkdirSync(outputDir, { recursive: true })

const manifest = {
  version: 1,
  pointCount: POINT_COUNT,
  format: 'float32-xyz-little-endian',
  generatedAt: '2026-06-15',
  assets: assets.map((asset) => {
    writeFloat32Binary(path.join(outputDir, asset.file), asset.points)
    return {
      id: asset.id,
      role: asset.role,
      label: asset.label,
      source: asset.source,
      file: asset.file,
      url: `/pointclouds/${asset.file}`,
      pointCount: POINT_COUNT,
      byteLength: asset.points.byteLength,
    }
  }),
}

fs.writeFileSync(
  path.join(outputDir, 'manifest.json'),
  `${JSON.stringify(manifest, null, 2)}\n`
)

console.log(
  `Generated ${assets.length} point clouds at ${POINT_COUNT.toLocaleString('en-US')} points each.`
)
for (const asset of manifest.assets) {
  console.log(`${asset.id}: ${asset.byteLength} bytes`)
}
